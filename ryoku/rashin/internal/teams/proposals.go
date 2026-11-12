package teams

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// proposeTeam asks the user to approve a team someone proposed: an agent
// through its Rashin tools, or a person or script through the CLI. What the
// proposal leaves empty comes from the template it names; the team is
// checked exactly as teams.create would check it, and nothing is made until
// the user approves it in the Inbox. A proposed team works only in its own
// folder under the Teams home, its routines arrive off, and at most
// ProposalsOpenMax proposals wait at once.
func (s *Service) proposeTeam(ctx context.Context, p protocol.TeamProposeParams) (protocol.Approval, error) {
	p.Reason = strings.TrimSpace(p.Reason)
	if p.Reason == "" || utf8.RuneCountInString(p.Reason) > protocol.ProposalReasonMax {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Say why the team is proposed, in at most %d characters.", protocol.ProposalReasonMax))
	}
	if strings.TrimSpace(p.Team.Folder) != "" || len(p.Team.Access.Writable) > 0 {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A proposed team works only in its own folder. The user can give it more once it exists."))
	}
	p.Session = strings.TrimSpace(p.Session)
	if p.Session != "" && s.sessions != nil {
		if _, ok := s.sessions.Info(p.Session); !ok {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No session %s.", p.Session))
		}
	}
	fromTemplate(&p)
	for index := range p.Team.Routines {
		p.Team.Routines[index].Enabled = false
	}
	needs, err := checkNeeds(p.Credentials)
	if err != nil {
		return protocol.Approval{}, err
	}
	p.Credentials = needs
	if err := s.checkNewTeam(p.Team); err != nil {
		return protocol.Approval{}, err
	}
	p.Team.Name, p.Team.Purpose = strings.TrimSpace(p.Team.Name), strings.TrimSpace(p.Team.Purpose)
	p.Reason = s.redact(p.Reason)
	s.proposing.Lock()
	defer s.proposing.Unlock()
	open := s.state.approvals(func(r approvalRecord) bool {
		return r.Kind == protocol.ApprovalTeam && r.Status == protocol.ApprovalPending
	})
	if len(open) >= protocol.ProposalsOpenMax {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%d proposed teams already wait for the user's answer in the Rashin Inbox. Ask the user to answer them first.", len(open)))
	}
	approval := protocol.Approval{
		Kind:     protocol.ApprovalTeam,
		Title:    i18n.Tf("A new team is proposed: %s", p.Team.Name),
		Detail:   p.Reason,
		Proposal: &p,
	}
	return s.ask(ctx, caller{Session: p.Session}, approval, nil)
}

// fromTemplate fills what a proposed team leaves empty from the template it
// names, and adds the template's credentials after the proposal's own.
func fromTemplate(p *protocol.TeamProposeParams) {
	template, ok := templateByID(p.Team.Template)
	if !ok {
		return
	}
	team := &p.Team
	if strings.TrimSpace(team.Name) == "" {
		team.Name = template.Name
	}
	if strings.TrimSpace(team.Purpose) == "" {
		team.Purpose = template.Purpose
	}
	if len(team.Members) == 0 {
		team.Members = slices.Clone(template.Members)
	}
	if team.Tools == (protocol.TeamTools{}) {
		team.Tools = template.Tools
	}
	if team.Approvals == "" {
		team.Approvals = template.Approvals
	}
	if len(team.Routines) == 0 {
		team.Routines = slices.Clone(template.Routines)
	}
	p.Credentials = append(p.Credentials, template.Credentials...)
}

// checkNeeds checks the credentials a proposed team names as a credential
// request would be checked, each name once, at most
// ProposalCredentialsMax of them.
func checkNeeds(needs []protocol.CredentialNeed) ([]protocol.CredentialNeed, error) {
	out := make([]protocol.CredentialNeed, 0, len(needs))
	for _, need := range needs {
		request, err := checkRequest(need.Name, need.Why, need.Hosts)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(out, func(kept protocol.CredentialNeed) bool { return kept.Name == request.Name }) {
			continue
		}
		out = append(out, protocol.CredentialNeed{Name: request.Name, Hosts: request.Hosts, Why: request.Reason})
	}
	if len(out) > protocol.ProposalCredentialsMax {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A proposed team names at most %d credentials.", protocol.ProposalCredentialsMax))
	}
	return out, nil
}

// proposer names who proposed a team, for the ledger: the proposing
// session by its title, else the user, who proposed it from a terminal.
func (s *Service) proposer(session string) (string, string) {
	if session == "" {
		return protocol.InitiatorYou, protocol.InitiatorYou
	}
	if s.sessions != nil {
		if info, ok := s.sessions.Info(session); ok && info.Title != "" {
			return info.Title, protocol.InitiatorAgent
		}
	}
	return i18n.T("An agent"), protocol.InitiatorAgent
}

// effectTeam answers a proposed team: approving makes it as proposed, then
// asks for each credential it needs that the vault does not hold; a no
// makes nothing.
func (s *Service) effectTeam(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	if rec.Proposal == nil {
		return "", protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The proposed team could not be read."))
	}
	if p.Decision == protocol.DecideDeny {
		rec.Result = i18n.T("No team was made.")
		return "", nil
	}
	team, err := s.makeTeam(ctx, rec.Proposal.Team, rec.Proposal.Credentials)
	if err != nil {
		return "", err
	}
	rec.Team = team.ID
	rec.Result = i18n.Tf("Made the team %s.", team.Name)
	return "", nil
}

// askNeeds opens a credential request for team for each need the vault does
// not hold. A name the vault holds for other teams is not asked for: the
// ledger says the user can let this team use it.
func (s *Service) askNeeds(ctx context.Context, team protocol.Team, needs []protocol.CredentialNeed) {
	asked := make(map[string]bool, len(needs))
	for _, need := range needs {
		request, err := checkRequest(need.Name, need.Why, need.Hosts)
		if err != nil || asked[request.Name] {
			continue
		}
		asked[request.Name] = true
		if existing, ok := s.vault.Find(request.Name); ok {
			if !slices.Contains(existing.Teams, team.ID) {
				s.record(protocol.Activity{
					Actor: "Rashin", Action: "credential.request", Target: request.Name, OK: true,
					Team: team.ID, Initiator: protocol.InitiatorRashin,
					Detail: i18n.Tf("The vault holds %s for other teams. Let %s use it in the vault.", request.Name, team.Name),
				})
			}
			continue
		}
		_ = s.askCredential(ctx, caller{Team: team.ID}, team, request, "Rashin", protocol.InitiatorRashin)
	}
}
