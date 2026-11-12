package teams

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// proposing gives the harness a home of its own, so a team made in its
// default folder lands there, and an agent session that proposes
func proposing(t *testing.T) (*harness, protocol.SessionInfo) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	h := newHarness(t, 0)
	info := protocol.SessionInfo{ID: "session-proposer", Kind: protocol.KindAgent, Title: "Plan my mornings"}
	h.sessions.mu.Lock()
	h.sessions.infos[info.ID] = info
	h.sessions.mu.Unlock()
	return h, info
}

func newsProposal(session string) protocol.TeamProposeParams {
	return protocol.TeamProposeParams{
		Team:    protocol.TeamCreateParams{Template: "news", Name: "Morning news", Purpose: "Reads the news on Linux desktops for me."},
		Reason:  "You asked for a digest of Linux desktop news every Monday.",
		Session: session,
		Credentials: []protocol.CredentialNeed{
			{Name: "FEEDS_TOKEN", Hosts: []string{"feeds.example.org"}, Why: "The reader reads your saved feeds with it."},
		},
	}
}

func TestAProposalIsCheckedAsCreateWould(t *testing.T) {
	h, info := proposing(t)
	for _, bad := range []protocol.TeamCreateParams{
		{Name: " ", Members: []protocol.Member{{Name: "Ana", Harness: "fake"}}},
		{Name: "Ops", Members: []protocol.Member{{Name: "Ana", Harness: "nowhere"}}},
		{Name: "Ops", Members: []protocol.Member{{Name: "Ana", Harness: "fake"}}, Routines: []protocol.RoutineDraft{{Title: "Often", Brief: "Look.", Schedule: "* * * * *"}}},
		{Name: "Ops", Members: []protocol.Member{{Name: "Ana", Harness: "fake"}}, Approvals: "everything"},
	} {
		created := refused(t, h, protocol.MTeamsCreate, bad, protocol.CodeBadRequest)
		proposed := refused(t, h, protocol.MTeamsPropose, protocol.TeamProposeParams{Team: bad, Reason: "To try.", Session: info.ID}, protocol.CodeBadRequest)
		if proposed.Message != created.Message {
			t.Fatalf("a proposal was refused with %q where create says %q", proposed.Message, created.Message)
		}
	}
	for _, bad := range []protocol.TeamProposeParams{
		{Team: protocol.TeamCreateParams{Template: "system"}},
		{Team: protocol.TeamCreateParams{Template: "system", Folder: paths.Home()}, Reason: "To look after this computer."},
		{Team: protocol.TeamCreateParams{Template: "system", Access: protocol.TeamAccess{Writable: []string{"~/Documents"}}}, Reason: "To look after this computer."},
		{Team: protocol.TeamCreateParams{Template: "system"}, Reason: "To look after this computer.", Credentials: []protocol.CredentialNeed{{Name: "API_TOKEN", Hosts: []string{"*.example.org"}, Why: "To read the API."}}},
	} {
		refused(t, h, protocol.MTeamsPropose, bad, protocol.CodeBadRequest)
	}
	refused(t, h, protocol.MTeamsPropose, protocol.TeamProposeParams{Team: protocol.TeamCreateParams{Template: "system"}, Reason: "To look after this computer.", Session: "session-gone"}, protocol.CodeNotFound)
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 || len(h.s.listTeams().Teams) != 0 {
		t.Fatalf("refused proposals left %d approvals and %d teams", len(waiting), len(h.s.listTeams().Teams))
	}

	asked := do[protocol.Approval](t, h, protocol.MTeamsPropose, newsProposal(info.ID))
	if _, err := os.Stat(filepath.Join(paths.TeamsHome(), "morning-news")); !os.IsNotExist(err) {
		t.Fatalf("proposing made the team's folder: %v", err)
	}
	if asked.Kind != protocol.ApprovalTeam || asked.Team != "" || asked.Session != info.ID || !asked.Expires.IsZero() || asked.Proposal == nil {
		t.Fatalf("the proposal waits as %+v", asked)
	}
	team := asked.Proposal.Team
	if len(team.Members) != 3 || team.Members[1].Harness != "claude" || team.Members[1].Model != "haiku" || len(team.Routines) != 2 {
		t.Fatalf("the template did not fill the proposal: %+v", team)
	}
	for _, routine := range team.Routines {
		if routine.Enabled {
			t.Fatalf("a proposed routine arrives on: %+v", routine)
		}
	}
	if needs := asked.Proposal.Credentials; len(needs) != 2 || needs[0].Name != "FEEDS_TOKEN" || needs[1].Name != "NEWS_API_KEY" {
		t.Fatalf("the proposal asks for %+v, want its own credential then the template's", needs)
	}
	asks := h.ledger(t)
	if len(asks) == 0 || asks[0].Action != "approval.ask" || asks[0].Actor != info.Title || asks[0].Initiator != protocol.InitiatorAgent {
		t.Fatalf("the ledger records the proposal as %+v", asks)
	}
}

func TestApprovingAProposalMakesTheTeamThenAsksForItsCredentials(t *testing.T) {
	h, info := proposing(t)
	asked := do[protocol.Approval](t, h, protocol.MTeamsPropose, newsProposal(info.ID))
	approved := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})

	teams := h.s.listTeams().Teams
	if len(teams) != 1 || teams[0].Name != "Morning news" || teams[0].Template != "news" || len(teams[0].Members) != 3 {
		t.Fatalf("approving made %+v", teams)
	}
	team := teams[0]
	if approved.Status != protocol.ApprovalApproved || approved.Team != team.ID || approved.Result == "" {
		t.Fatalf("the answered proposal reads %+v", approved)
	}
	if info, err := os.Stat(team.Folder); err != nil || !info.IsDir() || filepath.Dir(team.Folder) != paths.TeamsHome() {
		t.Fatalf("the team works in %s: %v", team.Folder, err)
	}
	routines := h.s.listRoutines(team.ID).Routines
	if len(routines) != 2 {
		t.Fatalf("the team has %d routines, want 2", len(routines))
	}
	for _, routine := range routines {
		member, _ := taskMember(team, routine.Member)
		if routine.Enabled || (routine.Title == "Breaking news" && member.Name != "Reader") {
			t.Fatalf("routine %+v is on or bound to the wrong member", routine)
		}
	}

	waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals
	if len(waiting) != 2 {
		t.Fatalf("%d approvals wait, want a request for each credential", len(waiting))
	}
	for _, request := range waiting {
		if request.Kind != protocol.ApprovalRequest || request.Team != team.ID {
			t.Fatalf("approving opened %+v", request)
		}
	}
	provided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: waiting[0].ID, Decision: protocol.DecideProvide, Value: apiToken})
	if provided.Status != protocol.ApprovalApproved {
		t.Fatalf("providing a requested credential: %+v", provided)
	}
	for _, action := range []string{"approval.approve", "team.create", "credential.request"} {
		if h.ledgerCount(t, action, "") == 0 {
			t.Fatalf("the ledger has no %s", action)
		}
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
	if len(h.s.listTeams().Teams) != 1 {
		t.Fatal("answering a proposal twice made a second team")
	}
}

func TestDenyingAProposalMakesNothing(t *testing.T) {
	h, info := proposing(t)
	asked := do[protocol.Approval](t, h, protocol.MTeamsPropose, newsProposal(info.ID))
	denied := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideDeny})
	if denied.Status != protocol.ApprovalDenied || denied.Team != "" || denied.Result == "" {
		t.Fatalf("the denied proposal reads %+v", denied)
	}
	if teams := h.s.listTeams().Teams; len(teams) != 0 {
		t.Fatalf("denying made %+v", teams)
	}
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 {
		t.Fatalf("denying asked for %+v", waiting)
	}
	if _, err := os.Stat(paths.TeamsHome()); !os.IsNotExist(err) {
		t.Fatalf("denying made a folder: %v", err)
	}
	if h.ledgerCount(t, "approval.deny", "") != 1 || h.ledgerCount(t, "team.create", "") != 0 {
		t.Fatal("the ledger does not record the no alone")
	}
}

func TestASixthOpenProposalIsRefused(t *testing.T) {
	h, info := proposing(t)
	var first protocol.Approval
	for index := range protocol.ProposalsOpenMax {
		asked := do[protocol.Approval](t, h, protocol.MTeamsPropose, newsProposal(info.ID))
		if index == 0 {
			first = asked
		}
	}
	refused(t, h, protocol.MTeamsPropose, newsProposal(info.ID), protocol.CodeConflict)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: first.ID, Decision: protocol.DecideDeny})
	do[protocol.Approval](t, h, protocol.MTeamsPropose, newsProposal(info.ID))
}

func TestATemplateTeamAsksForTheCredentialsItLacks(t *testing.T) {
	h, _ := proposing(t)
	repo := do[protocol.Team](t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{
		Name: "Repo watch", Template: "repo", Folder: filepath.Join(h.dir, "repo"),
		Members: []protocol.Member{{Name: "Reviewer", Lead: true, Harness: "fake"}},
	})
	request := h.pending(t)
	if request.Kind != protocol.ApprovalRequest || request.Team != repo.ID || request.Title == "" {
		t.Fatalf("a Repo watch team asks for %+v", request)
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: request.ID, Decision: protocol.DecideProvide, Value: apiToken})
	if credential, ok := h.s.vault.Find("GITHUB_TOKEN"); !ok || len(credential.Hosts) != 1 || credential.Hosts[0] != "api.github.com" {
		t.Fatalf("the provided token is %+v", credential)
	}

	do[protocol.Team](t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{
		Name: "Second watch", Template: "repo", Folder: filepath.Join(h.dir, "second"),
		Members: []protocol.Member{{Name: "Reviewer", Lead: true, Harness: "fake"}},
	})
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 {
		t.Fatalf("a credential the vault holds was asked for again: %+v", waiting)
	}
}

func TestEveryTemplateHasALevelAndBlankIsLast(t *testing.T) {
	list := templates()
	if list[len(list)-1].ID != "blank" {
		t.Fatalf("the last template is %s", list[len(list)-1].ID)
	}
	levels := map[string]bool{protocol.LevelLight: true, protocol.LevelMedium: true, protocol.LevelAdvanced: true}
	for _, template := range list[:len(list)-1] {
		if !levels[template.Level] {
			t.Fatalf("template %s has level %q", template.ID, template.Level)
		}
		if _, err := checkNeeds(template.Credentials); err != nil {
			t.Fatalf("template %s needs %+v: %v", template.ID, template.Credentials, err)
		}
	}
	news, ok := templateByID("news")
	if !ok || news.Level != protocol.LevelAdvanced || len(news.Members) != 3 || len(news.Routines) != 2 || len(news.Credentials) != 1 {
		t.Fatalf("the News desk template is %+v", news)
	}
}
