package demo

import (
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// the team an agent proposed while it polished the provider cards, with a credential it will ask for
func demoProposal() protocol.TeamProposeParams {
	translator := i18n.T("Translator")
	return protocol.TeamProposeParams{
		Session: "agent-done",
		Reason:  i18n.T("I added eleven strings to the provider cards, and the German and Portuguese catalogues were already behind. A team can draft what is missing every Monday and hand you one report to review, so you never open a session for it."),
		Team: protocol.TeamCreateParams{
			Name:    i18n.T("Translations"),
			Purpose: i18n.T("Drafts the German and Portuguese strings Ryotunes is missing each week, and tells me which need a native speaker."),
			Members: []protocol.Member{
				{
					Name: translator, Title: i18n.T("Drafts the translations"), Lead: true, Harness: "claude", Model: "sonnet",
					Instructions: i18n.T("You find the strings the German and Portuguese catalogues lack, draft each through DeepL with http_request and the DEEPL_API_KEY credential, and hand the drafts to the proofreader."),
				},
				{
					Name: i18n.T("Proofreader"), Title: i18n.T("Reads them as a native speaker would"), Harness: "codex", Model: "gpt-5.6",
					Instructions: i18n.T("You read each draft in context, fix what reads stiffly, and mark the strings a native speaker should check."),
				},
			},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{TaskTurns: 20},
			Routines: []protocol.RoutineDraft{{
				Title: i18n.T("Monday catalogue pass"), Member: translator, Schedule: "0 10 * * 1",
				Brief: i18n.T("Draft every string the German and Portuguese catalogues lack since the last pass, and report them with the ones a native speaker should check."),
			}},
		},
		Credentials: []protocol.CredentialNeed{{
			Name: "DEEPL_API_KEY", Hosts: []string{"api-free.deepl.com"},
			Why: i18n.T("The translator drafts each missing string through DeepL's API with it."),
		}},
	}
}

// makes the team a proposal describes once the user approves it, its routines off as every proposed one arrives
func (s *server) makeProposedTeam(index int) (protocol.Team, error) {
	t := &s.data.teams
	proposal := t.approvals[index].Proposal
	if proposal == nil {
		return protocol.Team{}, badRequest(i18n.T("This proposal names no team."))
	}
	params := proposal.Team
	params.Routines = slices.Clone(params.Routines)
	for routine := range params.Routines {
		params.Routines[routine].Enabled = false
	}
	team, err := s.createTeam(params, false)
	if err != nil {
		return protocol.Team{}, err
	}
	t.approvals[index].Team, t.approvals[index].Result = team.ID, i18n.Tf("Made the team %s.", team.Name)
	return team, nil
}

// one credential request for each need the vault does not hold yet, the same request a team's agent makes
func (s *server) askForCredentials(team protocol.Team, needs []protocol.CredentialNeed) {
	t := &s.data.teams
	now := time.Now()
	for _, need := range needs {
		if s.credentialNamed(need.Name) >= 0 {
			continue
		}
		id := t.newID("approval")
		t.actions[id] = demoAction{request: protocol.AgentCredentialRequestParams{Name: need.Name, Reason: need.Why, Hosts: need.Hosts}}
		request := protocol.Approval{
			ID: id, Team: team.ID, Kind: protocol.ApprovalRequest, Status: protocol.ApprovalPending,
			Title:   i18n.Tf("%s asks for a credential: %s", team.Name, need.Name),
			Detail:  i18n.Tf("%s\nWhere it would go: %s", need.Why, strings.Join(need.Hosts, ", ")),
			Created: now, Expires: now.Add(time.Duration(s.data.snapshot.Settings.Teams.ApprovalHours) * time.Hour),
		}
		// newest first, as the daemon lists them
		t.approvals = slices.Insert(t.approvals, 0, request)
		s.tellTeams(protocol.EvtApproval, request)
	}
}
