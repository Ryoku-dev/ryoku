package teams

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// maxRequestReason bounds what an agent says when it asks for a credential.
const maxRequestReason = 1000

// credentialRequest is what an agent asked the user to add, stored with the
// approval.
type credentialRequest struct {
	Name   string   `json:"name"`
	Reason string   `json:"reason"`
	Hosts  []string `json:"hosts"`
}

// vaultInfo is the vault as clients see it: its state always, its
// credentials and mail account only while it is open.
func (s *Service) vaultInfo() protocol.VaultInfo {
	state, reason, keeper := s.vault.State()
	info := protocol.VaultInfo{State: state, Reason: reason, Keeper: keeper, Credentials: []protocol.Credential{}}
	if state == protocol.VaultOpen {
		info.Credentials = s.vault.List()
		info.Mail = s.vault.Mail()
	}
	return info
}

// vaultSnapshot is the vault for the subscribe snapshot and vault events.
func (s *Service) vaultSnapshot() *protocol.VaultInfo {
	info := s.vaultInfo()
	return &info
}

// emitVault tells every client the vault changed.
func (s *Service) emitVault() {
	s.broadcast(protocol.EvtVault, s.vaultSnapshot())
}

// redact replaces every credential value in text with its name.
func (s *Service) redact(text string) string {
	return s.vault.Redact(text)
}

// recordCredential adds one event to the vault's record, marked when no
// Rashin window was open.
func (s *Service) recordCredential(event protocol.VaultEvent) {
	event.Unwatched = s.nobodyWatching()
	_ = s.vault.Record(event)
}

// checkGrantTeams refuses a grant to a team that does not exist.
func (s *Service) checkGrantTeams(teams []string) error {
	for _, id := range teams {
		if _, ok := s.state.team(id); !ok {
			return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no team %s.", id))
		}
	}
	return nil
}

// emitGrantTeams refreshes the teams a credential is granted to.
func (s *Service) emitGrantTeams(teams ...[]string) {
	var seen []string
	for _, list := range teams {
		for _, id := range list {
			if !slices.Contains(seen, id) {
				seen = append(seen, id)
				s.emitTeam(id)
			}
		}
	}
}

func (s *Service) vaultAdd(_ context.Context, p protocol.VaultAddParams) (protocol.Credential, error) {
	if err := s.checkGrantTeams(p.Teams); err != nil {
		return protocol.Credential{}, err
	}
	c, err := s.vault.Add(p)
	if err != nil {
		return protocol.Credential{}, err
	}
	s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Outcome: protocol.VaultAdded, Detail: i18n.T("You added it.")})
	s.record(protocol.Activity{Actor: "you", Action: "credential.add", Target: c.Name, OK: true, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("Added %s to the vault.", c.Name)})
	s.emitVault()
	s.emitGrantTeams(c.Teams)
	return c, nil
}

func (s *Service) vaultUpdate(_ context.Context, p protocol.VaultUpdateParams) (protocol.Credential, error) {
	before, ok := s.vault.Get(p.ID)
	if !ok {
		return protocol.Credential{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That credential is not in the vault."))
	}
	var grants struct {
		Teams []string `json:"teams"`
	}
	if len(p.Patch) > 0 && json.Unmarshal(p.Patch, &grants) == nil {
		if err := s.checkGrantTeams(grants.Teams); err != nil {
			return protocol.Credential{}, err
		}
	}
	c, err := s.vault.Update(p)
	if err != nil {
		return protocol.Credential{}, err
	}
	if p.Value != "" {
		s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Outcome: protocol.VaultRotated, Detail: i18n.Tf("You replaced its value; this is version %d.", c.Version)})
		s.record(protocol.Activity{Actor: "you", Action: "credential.rotate", Target: c.Name, OK: true, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("Replaced the value of %s.", c.Name)})
	}
	s.emitVault()
	s.emitGrantTeams(before.Teams, c.Teams)
	return c, nil
}

func (s *Service) vaultRemove(_ context.Context, id string) error {
	c, ok := s.vault.Get(id)
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That credential is not in the vault."))
	}
	if err := s.vault.Remove(id); err != nil {
		return err
	}
	s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Outcome: protocol.VaultRemoved, Detail: i18n.T("You removed it with every version of its value.")})
	s.record(protocol.Activity{Actor: "you", Action: "credential.remove", Target: c.Name, OK: true, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("Removed %s from the vault.", c.Name)})
	s.emitVault()
	s.emitGrantTeams(c.Teams)
	return nil
}

func (s *Service) vaultEvents(p protocol.VaultEventsParams) (protocol.VaultEventsResult, error) {
	events, err := s.vault.Events(p)
	if err != nil {
		return protocol.VaultEventsResult{}, err
	}
	return protocol.VaultEventsResult{Events: events}, nil
}

// vaultMail sets or removes the mail account; the vault checks the server,
// port, security and password credential.
func (s *Service) vaultMail(_ context.Context, p protocol.MailAccountParams) (protocol.VaultInfo, error) {
	if p.Account != nil {
		for _, trust := range p.Account.Trusted {
			if err := s.checkGrantTeams([]string{trust.Team}); err != nil {
				return protocol.VaultInfo{}, err
			}
		}
	}
	if err := s.vault.SetMail(p.Account); err != nil {
		return protocol.VaultInfo{}, err
	}
	s.emitVault()
	return s.vaultInfo(), nil
}

// vaultMailTest logs in to the mail account's server and quits without
// sending anything.
func (s *Service) vaultMailTest(ctx context.Context) protocol.MailTestResult {
	account := s.vault.Mail()
	if account == nil {
		return protocol.MailTestResult{Text: i18n.T("There is no mail account to test.")}
	}
	password, name := "", ""
	if account.Credential != "" {
		c, _ := s.vault.Get(account.Credential)
		name = c.Name
		var err error
		if password, err = s.vault.Reveal(account.Credential); err != nil {
			return protocol.MailTestResult{Text: err.Error()}
		}
	}
	if err := testMail(ctx, *account, password); err != nil {
		reason := s.redact(vault.Scrub(err.Error(), password, name))
		return protocol.MailTestResult{Text: i18n.Tf("The server did not take the login: %s", reason)}
	}
	if account.Credential == "" {
		return protocol.MailTestResult{OK: true, Text: i18n.Tf("Rashin reached %s.", account.Host)}
	}
	return protocol.MailTestResult{OK: true, Text: i18n.Tf("%s took the login.", account.Host)}
}

// dropGrants takes a deleted team out of every credential and mail trust.
func (s *Service) dropGrants(team string) {
	if s.vault.DropTeam(team) == nil {
		s.emitVault()
	}
}

// callerActor names the member a call came from, for the ledger.
func (s *Service) callerActor(team protocol.Team, c caller) string {
	if member, ok := taskMember(team, c.Member); ok {
		return member.Name
	}
	return team.Name
}

// callerTeam resolves a call from an agent to its run and team.
func (s *Service) callerTeam(ctx context.Context, ref protocol.AgentRef) (caller, protocol.Team, error) {
	c, err := s.caller(ctx, ref)
	if err != nil {
		return caller{}, protocol.Team{}, err
	}
	team, ok := s.state.team(c.Team)
	if !ok {
		return caller{}, protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This task's team no longer exists."))
	}
	return c, team, nil
}

// vaultOpen makes sure the vault can serve an agent's credential use, asking
// the keyring to unlock when it is locked, and tells clients when that
// changed the vault's state.
func (s *Service) vaultOpen() error {
	before, _, _ := s.vault.State()
	err := s.vault.Unlock()
	if after, _, _ := s.vault.State(); after != before {
		s.emitVault()
	}
	return err
}

// trustedFor reports whether a team may send a credential to a host without
// asking; a trusted *.example.org covers every name under it.
func trustedFor(c protocol.Credential, team, host string) bool {
	for _, trust := range c.Trusted {
		if trust.Team == team && hostrule.Allows([]string{trust.Host}, host) {
			return true
		}
	}
	return false
}

// agentHTTP makes one request with a credential the caller's team may use,
// asking the user first unless the team is trusted for that host.
func (s *Service) agentHTTP(ctx context.Context, p protocol.AgentHTTPParams) (protocol.AgentHTTPResult, error) {
	c, team, err := s.callerTeam(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	if err := s.vaultOpen(); err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	actor := s.callerActor(team, c)
	credential, ok := s.vault.Find(strings.TrimSpace(p.Credential))
	if !ok || credential.Use != protocol.CredentialHeader || !slices.Contains(credential.Teams, team.ID) {
		if ok {
			s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: team.ID, Task: c.Task, Outcome: protocol.VaultRefused, Detail: i18n.T("The team may not use it in a request.")})
			s.record(protocol.Activity{Actor: actor, Action: "credential.refuse", Target: credential.Name, OK: false, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.T("The team may not use it in a request.")})
		}
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("Your team has no credential named %q for requests. The credentials tool lists the ones it has.", p.Credential))
	}
	request, err := newEgress(credential, p)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	if !hostrule.Allows(credential.Hosts, request.Host) {
		s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: team.ID, Task: c.Task, Host: request.Host, Outcome: protocol.VaultRefused, Detail: i18n.T("The host is not one it may go to.")})
		s.record(protocol.Activity{Actor: actor, Action: "credential.refuse", Target: credential.Name, OK: false, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("To %s: not a host it may go to.", request.Host)})
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s may not go to %s. It may go to: %s.", credential.Name, request.Host, strings.Join(credential.Hosts, ", ")))
	}
	if request.SaveAs != "" {
		if err := checkSaveAs(team.Folder, request.SaveAs); err != nil {
			return protocol.AgentHTTPResult{}, err
		}
	}
	if trustedFor(credential, team.ID, request.Host) {
		return s.performRequest(ctx, team, c.Task, actor, protocol.InitiatorAgent, request)
	}
	approval := protocol.Approval{
		Kind:   protocol.ApprovalCredential,
		Title:  i18n.Tf("Let %s send %s to %s", team.Name, credential.Name, request.Host),
		Detail: s.redact(request.detail()),
		Always: i18n.Tf("Always let %s send %s to %s", team.Name, credential.Name, request.Host),
	}
	if _, err := s.ask(ctx, c, approval, request); err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: team.ID, Task: c.Task, Host: request.Host, Outcome: protocol.VaultAsked, Detail: i18n.T("Waiting for you to allow the first use on this host.")})
	return protocol.AgentHTTPResult{Waiting: i18n.Tf("Waiting for the user to let you send %s to %s. End your turn now; Rashin continues this task with their answer.", credential.Name, request.Host)}, nil
}

// effectCredential runs the request a credential approval held, once or
// after trusting the team for the host, or tells the agent it was refused.
func (s *Service) effectCredential(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	var request egressRequest
	if err := json.Unmarshal(rec.Action, &request); err != nil {
		return "", protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The approved request could not be read."))
	}
	if p.Decision != protocol.DecideOnce && p.Decision != protocol.DecideAlways {
		s.recordCredential(protocol.VaultEvent{Credential: request.Credential, Name: request.Name, Team: rec.Team, Task: rec.Task, Host: request.Host, Outcome: protocol.VaultDenied, Detail: i18n.T("It was not allowed.")})
		return answerWithNote(i18n.Tf("The user did not let you send %s to %s.", request.Name, request.Host), p.Note), nil
	}
	team, ok := s.state.team(rec.Team)
	if !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This task's team no longer exists."))
	}
	if p.Decision == protocol.DecideAlways {
		if err := s.vault.Trust(request.Credential, team.ID, request.Host); err != nil {
			return "", err
		}
		s.emitVault()
	}
	actor := s.callerActor(team, caller{Team: rec.Team, Task: rec.Task, Member: rec.Member, Session: rec.Session})
	result, err := s.performRequest(ctx, team, rec.Task, actor, protocol.InitiatorAgent, request)
	if err != nil {
		return "", err
	}
	rec.Result = i18n.Tf("The server answered %d.", result.Status)
	return answerWithNote(httpAnswer(result), p.Note), nil
}

// answerWithNote adds the user's note to what the agent is told.
func answerWithNote(text, note string) string {
	if note = strings.TrimSpace(note); note != "" {
		return text + "\n" + i18n.Tf("Their note: %s", note)
	}
	return text
}

// mailRecipients checks and cleans the addresses of a mail.
func mailRecipients(list []string) ([]string, error) {
	var out []string
	for _, raw := range list {
		address, err := vault.CheckAddress(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, address)
	}
	return out, nil
}

// agentMail sends a mail from the user's account at once when every
// recipient is trusted for the team, and otherwise asks the user first.
func (s *Service) agentMail(ctx context.Context, p protocol.AgentMailParams) (protocol.AgentAnswer, error) {
	c, team, err := s.callerTeam(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	actor := s.callerActor(team, c)
	if !team.Tools.Mail {
		s.record(protocol.Activity{Actor: actor, Action: "mail.refuse", Target: team.Name, OK: false, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.T("The team may not send mail.")})
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Your team may not send mail. The user can turn on Mail in the team's settings."))
	}
	account := s.vault.Mail()
	if account == nil {
		return protocol.AgentAnswer{Text: i18n.T("There is no mail account in Rashin yet, so the mail cannot go. Say so in your report; the user sets one up in the Vault.")}, nil
	}
	if err := s.vaultOpen(); err != nil {
		return protocol.AgentAnswer{}, err
	}
	m := mailParams{Subject: strings.TrimSpace(p.Subject), Body: p.Body}
	if m.To, err = mailRecipients(p.To); err != nil {
		return protocol.AgentAnswer{}, err
	}
	if m.Cc, err = mailRecipients(p.Cc); err != nil {
		return protocol.AgentAnswer{}, err
	}
	recipients := m.recipients()
	switch {
	case len(m.To) == 0:
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say who the mail is to."))
	case len(recipients) > maxMailRecipients:
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A mail may go to at most %d people.", maxMailRecipients))
	case m.Subject == "" || utf8.RuneCountInString(m.Subject) > maxMailSubject || strings.ContainsAny(m.Subject, "\r\n"):
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The subject must be one line of 1 to %d characters.", maxMailSubject))
	case len(m.Body) > maxMailBody || !utf8.ValidString(m.Body):
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The body must be text of at most 64 KiB."))
	}
	trusted := true
	for _, recipient := range recipients {
		trusted = trusted && slices.Contains(account.Trusted, protocol.MailTrust{Team: team.ID, Recipient: strings.ToLower(recipient)})
	}
	to := strings.Join(m.To, ", ")
	if trusted {
		if err := s.sendTeamMail(ctx, team, c.Task, actor, protocol.InitiatorAgent, m); err != nil {
			return protocol.AgentAnswer{}, err
		}
		return protocol.AgentAnswer{Text: i18n.Tf("Sent the mail to %s.", to)}, nil
	}
	approval := protocol.Approval{
		Kind:   protocol.ApprovalMail,
		Title:  i18n.Tf("Send a mail to %s", to),
		Detail: s.redact(m.detail()),
	}
	if len(recipients) == 1 {
		approval.Always = i18n.Tf("Always let %s mail %s", team.Name, recipients[0])
	}
	if _, err := s.ask(ctx, c, approval, m); err != nil {
		return protocol.AgentAnswer{}, err
	}
	return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user to approve the mail to %s. End your turn now; Rashin continues this task with their answer.", to)}, nil
}

// sendTeamMail sends a mail through the account with its password from the
// vault and records the mail and the password's use.
func (s *Service) sendTeamMail(ctx context.Context, team protocol.Team, task, actor, initiator string, m mailParams) error {
	account := s.vault.Mail()
	if account == nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("There is no mail account in Rashin, so the mail cannot go."))
	}
	password, name := "", ""
	if account.Credential != "" {
		c, _ := s.vault.Get(account.Credential)
		name = c.Name
		var err error
		if password, err = s.vault.Reveal(account.Credential); err != nil {
			return err
		}
	}
	to := strings.Join(m.recipients(), ", ")
	err := sendMail(ctx, *account, password, m, s.now())
	reason := ""
	if err != nil {
		reason = s.redact(vault.Scrub(err.Error(), password, name))
	}
	if account.Credential != "" {
		event := protocol.VaultEvent{Credential: account.Credential, Name: name, Team: team.ID, Task: task, Host: account.Host, Outcome: protocol.VaultUsed, Detail: i18n.T("Logged in to send a mail.")}
		if err != nil {
			event.Outcome, event.Detail = protocol.VaultFailed, reason
		}
		s.recordCredential(event)
		s.record(protocol.Activity{Actor: actor, Action: "credential.use", Target: name, OK: err == nil, Team: team.ID, Task: task, Initiator: initiator, Detail: i18n.Tf("To %s: the mail login.", account.Host)})
	}
	if err != nil {
		s.record(protocol.Activity{Actor: actor, Action: "mail.send", Target: m.Subject, OK: false, Team: team.ID, Task: task, Initiator: initiator, Detail: i18n.Tf("To %s; it did not go: %s", to, reason)})
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The mail did not go: %s", reason))
	}
	s.record(protocol.Activity{Actor: actor, Action: "mail.send", Target: m.Subject, OK: true, Team: team.ID, Task: task, Initiator: initiator, Detail: i18n.Tf("To %s: %s", to, m.Subject)})
	return nil
}

// effectMail sends the mail an approval held, trusting its one recipient
// for the team first when the user chose always.
func (s *Service) effectMail(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	var m mailParams
	if err := json.Unmarshal(rec.Action, &m); err != nil {
		return "", protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The approved mail could not be read."))
	}
	to := strings.Join(m.To, ", ")
	if p.Decision != protocol.DecideOnce && p.Decision != protocol.DecideAlways {
		return answerWithNote(i18n.Tf("The user did not send the mail to %s.", to), p.Note), nil
	}
	team, ok := s.state.team(rec.Team)
	if !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This task's team no longer exists."))
	}
	if p.Decision == protocol.DecideAlways {
		if recipients := m.recipients(); len(recipients) == 1 {
			if err := s.vault.TrustMail(team.ID, recipients[0]); err != nil {
				return "", err
			}
			s.emitVault()
		}
	}
	actor := s.callerActor(team, caller{Team: rec.Team, Task: rec.Task, Member: rec.Member, Session: rec.Session})
	if err := s.sendTeamMail(ctx, team, rec.Task, actor, protocol.InitiatorAgent, m); err != nil {
		return "", err
	}
	rec.Result = i18n.Tf("Sent to %s.", to)
	return answerWithNote(i18n.Tf("The user approved: the mail to %s was sent.", to), p.Note), nil
}

// agentRequest asks the user to add a credential the team needs. The user
// types the value into Rashin; the agent never sees it.
func (s *Service) agentRequest(ctx context.Context, p protocol.AgentCredentialRequestParams) (protocol.AgentAnswer, error) {
	c, team, err := s.callerTeam(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	request := credentialRequest{Name: strings.TrimSpace(p.Name), Reason: strings.TrimSpace(p.Reason)}
	if err := vault.CheckName(request.Name); err != nil {
		return protocol.AgentAnswer{}, err
	}
	if request.Reason == "" || utf8.RuneCountInString(request.Reason) > maxRequestReason {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Say why the team needs it, in at most %d characters.", maxRequestReason))
	}
	if request.Hosts, err = hostrule.Check(p.Hosts); err != nil {
		return protocol.AgentAnswer{}, err
	}
	if len(request.Hosts) == 0 {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say where the credential would go: at least one host."))
	}
	for _, host := range request.Hosts {
		if strings.HasPrefix(host, "*.") {
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Name each host exactly: %s stands for every name under a domain, which only the user may allow.", host))
		}
	}
	if existing, ok := s.vault.Find(request.Name); ok && !slices.Contains(existing.Teams, team.ID) {
		return protocol.AgentAnswer{Text: i18n.Tf("The vault already holds a credential named %s for other teams. Ask the user to let your team use it, or ask for another name.", request.Name)}, nil
	}
	approval := protocol.Approval{
		Kind:   protocol.ApprovalRequest,
		Title:  i18n.Tf("%s asks for a credential: %s", team.Name, request.Name),
		Detail: s.redact(request.Reason + "\n\n" + i18n.Tf("It would go to: %s", strings.Join(request.Hosts, ", "))),
	}
	if _, err := s.ask(ctx, c, approval, request); err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.recordCredential(protocol.VaultEvent{Name: request.Name, Team: team.ID, Task: c.Task, Outcome: protocol.VaultAsked, Detail: i18n.Tf("Asked for it to go to %s.", strings.Join(request.Hosts, ", "))})
	s.record(protocol.Activity{Actor: s.callerActor(team, c), Action: "credential.request", Target: request.Name, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("Asked for it to go to %s.", strings.Join(request.Hosts, ", "))})
	return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user to add %s. End your turn now; Rashin continues this task with their answer.", request.Name)}, nil
}

// effectRequest adds the credential the user provided, sent as a bearer
// token to the hosts asked for. Providing it trusts nothing: the first use
// on each host asks like any other. A credential of that name the team
// already uses gets the new value instead.
func (s *Service) effectRequest(_ context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	var request credentialRequest
	if err := json.Unmarshal(rec.Action, &request); err != nil {
		return "", protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The credential request could not be read."))
	}
	if p.Decision != protocol.DecideProvide {
		return answerWithNote(i18n.Tf("The user did not add %s.", request.Name), p.Note), nil
	}
	if _, ok := s.state.team(rec.Team); !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This task's team no longer exists."))
	}
	var credential protocol.Credential
	var err error
	existing, found := s.vault.Find(request.Name)
	switch {
	case found && (existing.Use != protocol.CredentialHeader || !slices.Contains(existing.Teams, rec.Team)):
		return answerWithNote(i18n.Tf("The vault already holds a credential named %s for other teams, so the value was not added.", request.Name), p.Note), nil
	case found:
		if credential, err = s.vault.Update(protocol.VaultUpdateParams{ID: existing.ID, Value: p.Value}); err != nil {
			return "", err
		}
		s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: rec.Team, Task: rec.Task, Outcome: protocol.VaultRotated, Detail: i18n.Tf("You replaced its value when the team asked; this is version %d.", credential.Version)})
		s.record(protocol.Activity{Actor: "you", Action: "credential.rotate", Target: credential.Name, OK: true, Team: rec.Team, Task: rec.Task, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("Replaced the value of %s.", credential.Name)})
	default:
		credential, err = s.vault.Add(protocol.VaultAddParams{
			Name: request.Name, Description: request.Reason, Use: protocol.CredentialHeader,
			Header: "Authorization", Prefix: "Bearer ", Hosts: request.Hosts, Teams: []string{rec.Team}, Value: p.Value,
		})
		if err != nil {
			return "", err
		}
		s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: rec.Team, Task: rec.Task, Outcome: protocol.VaultAdded, Detail: i18n.T("You added it when the team asked.")})
		s.record(protocol.Activity{Actor: "you", Action: "credential.add", Target: credential.Name, OK: true, Team: rec.Team, Task: rec.Task, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("Added %s to the vault.", credential.Name)})
	}
	rec.Result = i18n.Tf("Added %s to the vault.", credential.Name)
	s.emitVault()
	s.emitTeam(rec.Team)
	return answerWithNote(i18n.Tf("The user added %s. Use it with http_request; its first use on each host waits for the user's approval.", credential.Name), p.Note), nil
}

// agentCredentials lists what the caller's team may use: the credentials it
// may send with requests, where each may go and where it goes without
// asking, and whether it may send mail.
func (s *Service) agentCredentials(ctx context.Context, p protocol.AgentRef) (protocol.AgentCredentialsResult, error) {
	_, team, err := s.callerTeam(ctx, p)
	if err != nil {
		return protocol.AgentCredentialsResult{}, err
	}
	result := protocol.AgentCredentialsResult{Credentials: []protocol.AgentCredential{}}
	for _, c := range s.vault.List() {
		if c.Use != protocol.CredentialHeader || !slices.Contains(c.Teams, team.ID) {
			continue
		}
		entry := protocol.AgentCredential{Name: c.Name, Description: c.Description, Hosts: slices.Clone(c.Hosts)}
		for _, trust := range c.Trusted {
			if trust.Team == team.ID {
				entry.Trusted = append(entry.Trusted, trust.Host)
			}
		}
		result.Credentials = append(result.Credentials, entry)
	}
	result.Mail = team.Tools.Mail && s.vault.Mail() != nil
	return result, nil
}
