package demo

import (
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// answers an approval: runs what it would run, records the answer, and lets the task go on with what its agent would read
func (s *server) decideApproval(p protocol.ApprovalDecideParams) (protocol.Approval, error) {
	t := &s.data.teams
	index := t.approvalIndex(p.ID)
	if index < 0 {
		return protocol.Approval{}, notFound("approval")
	}
	approval := t.approvals[index]
	if approval.Status != protocol.ApprovalPending {
		return protocol.Approval{}, badRequest(i18n.T("This was already answered."))
	}
	request, choice := approval.Kind == protocol.ApprovalRequest, len(approval.Options) > 0
	switch {
	case p.Decision == protocol.DecideDeny:
	case p.Decision == protocol.DecideChoose && choice:
		if !slices.Contains(approval.Options, p.Choice) {
			return protocol.Approval{}, badRequest(i18n.T("Pick one of the options."))
		}
		t.approvals[index].Choice = p.Choice
	case p.Decision == protocol.DecideProvide && request:
		if strings.TrimSpace(p.Value) == "" {
			return protocol.Approval{}, badRequest(i18n.T("Paste the value first."))
		}
		if name := t.actions[approval.ID].request.Name; s.credentialNamed(name) >= 0 {
			return protocol.Approval{}, badRequest(i18n.Tf("%s is already in the vault.", name))
		}
	case p.Decision == protocol.DecideOnce && !request && !choice:
	case p.Decision == protocol.DecideAlways && !request && !choice && approval.Always != "":
	default:
		return protocol.Approval{}, badRequest(i18n.T("That is not an answer to this."))
	}
	t.approvals[index].Note = strings.TrimSpace(p.Note)
	text := s.approvalEffect(index, p.Decision, p.Value)
	s.settleApproval(index, p.Decision, t.approvals[index].Note)
	if approval.Task != "" {
		s.releaseTeamTask(approval.Task, text)
	}
	return t.approvals[index], nil
}

// records an answer without running anything, as when a task stops or its team goes
func (s *server) settleApproval(index int, decision, note string) {
	t := &s.data.teams
	now := time.Now()
	approval := &t.approvals[index]
	approval.Decision, approval.Note, approval.Decided = decision, note, &now
	approval.Status = protocol.ApprovalApproved
	action, detail := "approval.approve", i18n.T("Approved once.")
	switch decision {
	case protocol.DecideDeny:
		approval.Status, action, detail = protocol.ApprovalDenied, "approval.deny", note
	case protocol.DecideAlways:
		detail = approval.Always
	case protocol.DecideChoose:
		detail = i18n.Tf("Chose: %s", approval.Choice)
	case protocol.DecideProvide:
		detail = i18n.T("Added the value to the vault.")
	}
	s.record(protocol.Activity{Action: action, Target: approval.Title, OK: true, Team: approval.Team, Task: approval.Task, Detail: detail})
	s.tellTeams(protocol.EvtApproval, *approval)
}

// does what an approved action does in the demo and returns what the agent would read; a denial does nothing
func (s *server) approvalEffect(index int, decision, value string) string {
	t := &s.data.teams
	approval := &t.approvals[index]
	action := t.actions[approval.ID]
	approved := decision != protocol.DecideDeny
	note := ""
	if approval.Note != "" {
		note = " " + i18n.Tf("Their note: %s", approval.Note)
	}
	no := i18n.Tf("The user said no: %s.", approval.Title) + note
	member := s.memberName(approval.Team, approval.Member)
	now := time.Now()
	if decision == protocol.DecideChoose {
		chose := i18n.Tf("The user chose: %s", approval.Choice)
		if approval.Note != "" {
			chose += "\n" + i18n.Tf("Their note: %s", approval.Note)
		}
		return chose
	}
	switch approval.Kind {
	case protocol.ApprovalMail:
		if !approved {
			return no
		}
		mail := t.vault.Mail
		if mail == nil {
			approval.Result = i18n.T("There is no mail account, so nothing was sent.")
			return i18n.T("The user approved, but there is no mail account, so nothing was sent.")
		}
		if decision == protocol.DecideAlways {
			mail.Trusted = append(append([]protocol.MailTrust(nil), mail.Trusted...), protocol.MailTrust{Team: approval.Team, Recipient: action.recipient})
		}
		approval.Result = i18n.Tf("Sent to %s.", action.recipient)
		if credential := t.credentialIndex(mail.Credential); credential >= 0 {
			s.useCredential(credential, now)
			s.recordVault(protocol.VaultEvent{
				Credential: mail.Credential, Name: t.vault.Credentials[credential].Name, Team: approval.Team, Task: approval.Task,
				Host: mail.Host, Outcome: protocol.VaultUsed, Detail: i18n.Tf("Sent one mail to %s.", action.recipient),
			})
		}
		s.record(protocol.Activity{Actor: member, Initiator: protocol.InitiatorAgent, Action: "mail.send", Target: approval.Title, OK: true, Team: approval.Team, Task: approval.Task, Detail: approval.Result})
		s.emitVault()
		return i18n.Tf("The user approved: %s. It was sent.", approval.Title) + note
	case protocol.ApprovalCredential:
		if !approved {
			s.recordVault(protocol.VaultEvent{Credential: action.credential, Name: s.credentialName(action.credential), Team: approval.Team, Task: approval.Task, Host: action.host, Outcome: protocol.VaultDenied, Detail: approval.Note})
			s.emitVault()
			return no
		}
		if credential := t.credentialIndex(action.credential); credential >= 0 {
			if decision == protocol.DecideAlways {
				trusted := &t.vault.Credentials[credential].Trusted
				*trusted = append(append([]protocol.CredentialTrust(nil), *trusted...), protocol.CredentialTrust{Team: approval.Team, Host: action.host})
			}
			s.useCredential(credential, now)
		}
		approval.Result = i18n.Tf("%s answered 200.", action.host)
		name := s.credentialName(action.credential)
		s.recordVault(protocol.VaultEvent{Credential: action.credential, Name: name, Team: approval.Team, Task: approval.Task, Host: action.host, Outcome: protocol.VaultUsed, Detail: approval.Detail})
		s.record(protocol.Activity{Actor: member, Initiator: protocol.InitiatorAgent, Action: "credential.use", Target: name, OK: true, Team: approval.Team, Task: approval.Task, Detail: i18n.Tf("Sent %s to %s.", name, action.host)})
		s.emitVault()
		return i18n.Tf("The user approved: %s. The request went out and %s answered 200.", approval.Title, action.host) + note
	case protocol.ApprovalRequest:
		request := action.request
		if !approved {
			return i18n.Tf("The user did not add %s.", request.Name) + note
		}
		trusted := make([]protocol.CredentialTrust, 0, len(request.Hosts))
		for _, host := range request.Hosts {
			trusted = append(trusted, protocol.CredentialTrust{Team: approval.Team, Host: host})
		}
		added, err := s.addCredential(protocol.VaultAddParams{
			Name: request.Name, Description: request.Reason, Use: protocol.CredentialHeader, Header: "Authorization", Prefix: "Bearer ",
			Hosts: request.Hosts, Teams: []string{approval.Team}, Value: value,
		})
		if err != nil {
			approval.Result = err.Error()
			return i18n.Tf("The user tried to add %s, but the vault refused it.", request.Name)
		}
		t.vault.Credentials[t.credentialIndex(added.ID)].Trusted = trusted
		s.emitVault()
		approval.Result = i18n.Tf("Added %s to the vault.", request.Name)
		return i18n.Tf("The user added %s. Use it with http_request.", request.Name) + note
	case protocol.ApprovalBudget:
		team := t.teamIndex(approval.Team)
		if !approved || team < 0 {
			return ""
		}
		raised := budgetRaise(t.teams[team].Budget.MonthTokens)
		t.teams[team].Budget.MonthTokens = raised
		approval.Result = i18n.Tf("Raised the limit to %d tokens a month.", raised)
		s.resumeTeam(team)
		return ""
	}
	if !approved {
		return no
	}
	return i18n.Tf("The user approved: %s.", approval.Title) + note
}

// raised by half, rounded up to a hundred thousand tokens
func budgetRaise(limit int64) int64 {
	raised := limit + (limit+1)/2
	return (raised + 99_999) / 100_000 * 100_000
}

// a held task goes on once nothing else of it waits on the user
func (s *server) releaseTeamTask(id, text string) {
	t := &s.data.teams
	index := t.taskIndex(id)
	if index < 0 || text == "" {
		return
	}
	task := t.tasks[index]
	if task.Status != protocol.TaskNeedsYou && task.Waiting != protocol.WaitingOnYou {
		return
	}
	for _, approval := range t.approvals {
		if approval.Task == id && approval.Status == protocol.ApprovalPending {
			return
		}
	}
	task.Status, task.Waiting, task.Updated = protocol.TaskWorking, "", time.Now()
	t.tasks[index] = task
	s.appendTeamEvents(task.Session,
		later(protocol.EvUser, protocol.UserData{Text: text, Goal: true}),
		later(protocol.EvTurnStart, protocol.TurnStartData{Turn: s.nextTurn(task.Session)}),
	)
	s.settleTeamSession(task)
	s.emitTask(task)
}
