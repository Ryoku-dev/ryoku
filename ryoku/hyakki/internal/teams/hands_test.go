package teams

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// handsStarts records each start of Hands' own loop and its grant check.
type handsStarts struct {
	mu    sync.Mutex
	calls []handsStarted
	err   error
}

type handsStarted struct {
	goal, scope, window string
	live                func() bool
}

func (f *handsStarts) start(_ context.Context, goal, scope, window string, live func() bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.calls = append(f.calls, handsStarted{goal: goal, scope: scope, window: window, live: live})
	return "j" + string(rune('0'+len(f.calls))), nil
}

func (f *handsStarts) seen() []handsStarted {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]handsStarted(nil), f.calls...)
}

// handsHarness is a harness whose Oni lets Hands work in apps, with Hands'
// own loop recorded.
func handsHarness(t *testing.T) (*harness, *handsStarts) {
	t.Helper()
	h := newHarnessWith(t, func(s *protocol.Settings) {
		s.Enabled, s.Oni.Master, s.Oni.Hands = true, true, protocol.OniHandsApp
	})
	starts := &handsStarts{}
	h.s.SetHands(starts.start)
	return h, starts
}

func askHands(t *testing.T, h *harness, ref protocol.AgentRef, goal, scope, window string) protocol.Approval {
	t.Helper()
	answer := do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: goal, Scope: scope, Window: window})
	if !strings.Contains(answer.Text, "End your turn now") {
		t.Fatalf("the agent was told %q", answer.Text)
	}
	return h.pending(t)
}

// An allowed request starts Hands once with exactly the goal and scope the
// member asked for, bound to the run that asked; the member is told Hands
// acts by itself. A second answer runs nothing.
func TestHandsRequestAllowedOnceStartsHandsBoundToItsRun(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	task := h.newTask(t, team, "File the receipt.")
	ref := h.agent(t, task.ID)
	asked := askHands(t, h, ref, "file the receipt in the expenses app", protocol.OniHandsScopeWindow, "0x7")
	if asked.Kind != protocol.ApprovalHands || asked.Always != "" || len(asked.Steps) == 0 || !strings.Contains(asked.Detail, "0x7") ||
		!strings.Contains(asked.Title, "file the receipt") || asked.Task != task.ID {
		t.Fatalf("the Hands approval is %+v", asked)
	}
	if held := h.task(t, task.ID); held.Status != protocol.TaskNeedsYou {
		t.Fatalf("the asking task is %q, want held on the user", held.Status)
	}
	if !h.s.HandsApproval(asked.ID) {
		t.Fatal("a Hands approval is not known as one")
	}
	if scope, window, ok := h.s.HandsAsked(asked.ID); !ok || scope != protocol.OniHandsScopeWindow || window != "0x7" {
		t.Fatalf("the card would show %q %q %v", scope, window, ok)
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideAlways}, protocol.CodeBadRequest)
	if len(starts.seen()) != 0 {
		t.Fatal("a refused answer started Hands")
	}

	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	if decided.Status != protocol.ApprovalApproved {
		t.Fatalf("the answer is %q", decided.Status)
	}
	got := starts.seen()
	if len(got) != 1 || got[0].goal != "file the receipt in the expenses app" || got[0].scope != protocol.OniHandsScopeWindow || got[0].window != "0x7" {
		t.Fatalf("Hands started %+v", got)
	}
	if !got[0].live() {
		t.Fatal("a fresh grant is not live")
	}
	waitFor(t, "the agent to hear Hands started", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "no stream of its actions") || !strings.Contains(note, "j1") {
		t.Fatalf("the agent was told %q", note)
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
	if len(starts.seen()) != 1 {
		t.Fatal("a second answer started Hands again")
	}
	if _, _, ok := h.s.HandsAsked(asked.ID); ok {
		t.Fatal("an answered request still shows on the pill")
	}

	// the grant ends with the task's run
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: task.ID})
	if got[0].live() {
		t.Fatal("the grant outlived its task's run")
	}
}

// A grant dies once its time is up.
func TestHandsGrantExpires(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "Tidy the downloads.").ID)
	asked := askHands(t, h, ref, "sort the downloads folder", protocol.OniHandsScopeNested, "")
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	live := starts.seen()[0].live
	h.clock.advance(handsGrantFor - time.Second)
	if !live() {
		t.Fatal("the grant ended early")
	}
	h.clock.advance(2 * time.Second)
	if live() {
		t.Fatal("an expired grant is still live")
	}
}

// A no runs nothing and the member hears so, with the user's note.
func TestHandsRequestDeniedRunsNothing(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "File the receipt.").ID)
	asked := askHands(t, h, ref, "file the receipt", protocol.OniHandsScopeNested, "")
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideDeny, Note: "Not now."})
	if len(starts.seen()) != 0 {
		t.Fatal("a denied request started Hands")
	}
	waitFor(t, "the agent to hear the no", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "Hands did nothing") || !strings.Contains(note, "Not now.") {
		t.Fatalf("the agent was told %q", note)
	}
}

// A member may name only the nested desktop or one window; never the
// browser tab, never the whole desktop, and only while Hands works in apps
// and Hyakki can start it.
func TestHandsRequestRefusesWhatAMemberMayNotAskFor(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "File the receipt.").ID)
	for _, c := range []struct {
		name                string
		scope, window, goal string
		code                string
	}{
		{"the whole desktop", protocol.OniHandsScopeDesktop, "", "anything", protocol.CodeForbidden},
		{"the browser tab", protocol.OniHandsScopeBrowser, "", "anything", protocol.CodeForbidden},
		{"no scope", "", "", "anything", protocol.CodeBadRequest},
		{"a window without its id", protocol.OniHandsScopeWindow, "", "anything", protocol.CodeBadRequest},
		{"a window id with a space", protocol.OniHandsScopeWindow, "0x 7", "anything", protocol.CodeBadRequest},
		{"the nested desktop with a window", protocol.OniHandsScopeNested, "0x7", "anything", protocol.CodeBadRequest},
		{"an empty goal", protocol.OniHandsScopeNested, "", " ", protocol.CodeBadRequest},
		{"a goal of two lines", protocol.OniHandsScopeNested, "", "one\ntwo", protocol.CodeBadRequest},
		{"a goal too long", protocol.OniHandsScopeNested, "", strings.Repeat("a", protocol.AgentHandsGoalMax+1), protocol.CodeBadRequest},
	} {
		t.Run(c.name, func(t *testing.T) {
			refused(t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: c.goal, Scope: c.scope, Window: c.window}, c.code)
		})
	}
	if pending := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(pending) != 0 || len(starts.seen()) != 0 {
		t.Fatalf("a refused request left %d approvals and %d starts", len(pending), len(starts.seen()))
	}

	h.s.SetHands(nil)
	refused(t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: "file it", Scope: protocol.OniHandsScopeNested}, protocol.CodeUnavailable)
	h.s.SetHands(starts.start)
	h.settingsMu.Lock()
	h.settings.Oni.Hands = protocol.OniHandsBrowser
	h.settingsMu.Unlock()
	refused(t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: "file it", Scope: protocol.OniHandsScopeNested}, protocol.CodeDisabled)
	h.settingsMu.Lock()
	h.settings.Oni.Hands, h.settings.Oni.Master = protocol.OniHandsApp, false
	h.settingsMu.Unlock()
	refused(t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: "file it", Scope: protocol.OniHandsScopeNested}, protocol.CodeDisabled)
}

// One task's token never speaks for another task, and a run that changed
// after it asked cannot use the answer: Hands does nothing and the member
// is told to ask again.
func TestHandsRequestBindsToItsOwnTaskAndRun(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	a := h.newTask(t, team, "File the receipt.")
	refA := h.agent(t, a.ID)
	b := do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: "Sort the photos.", Member: team.Members[len(team.Members)-1].ID})
	refB := h.agent(t, b.ID)
	stolen := protocol.AgentRef{Session: refB.Session, Token: refA.Token}
	refused(t, h, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: stolen, Goal: "sort the photos", Scope: protocol.OniHandsScopeNested}, protocol.CodeBadRequest)

	asked := askHands(t, h, refA, "file the receipt", protocol.OniHandsScopeNested, "")
	h.s.mu.Lock()
	h.s.runs[a.ID].Token = runToken()
	h.s.mu.Unlock()
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	if len(starts.seen()) != 0 {
		t.Fatal("an answer for a changed run started Hands")
	}
	decided, _ := h.s.state.approval(asked.ID)
	if !strings.Contains(decided.Result, "stale") {
		t.Fatalf("the record says %q", decided.Result)
	}
}

// Hands that cannot start leaves the answer kept with why, and the member
// hears it failed.
func TestHandsStartFailureIsKept(t *testing.T) {
	h, starts := handsHarness(t)
	starts.err = protocol.Errorf(protocol.CodeConflict, "Hands is busy. Stop it first.")
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "File the receipt.").ID)
	asked := askHands(t, h, ref, "file the receipt", protocol.OniHandsScopeNested, "")
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	decided, _ := h.s.state.approval(asked.ID)
	if !strings.Contains(decided.Result, "busy") {
		t.Fatalf("the record says %q", decided.Result)
	}
	waitFor(t, "the agent to hear the failure", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "did not start") {
		t.Fatalf("the agent was told %q", note)
	}
}

// The pill answers a Hands request once or no, never always, and nothing
// else at all.
func TestHandsPresenterAnswersOnlyHandsOnceOrNo(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "File the receipt.").ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "Delete the cache"})
	other := h.pending(t)
	if h.s.HandsApproval(other.ID) {
		t.Fatal("an ordinary question counts as a Hands request")
	}
	var forbidden *protocol.Error
	if _, err := h.s.PresenterDecide(context.Background(), other.ID, protocol.DecideOnce); !errors.As(err, &forbidden) || forbidden.Code != protocol.CodeForbidden {
		t.Fatalf("the pill answered an ordinary question: %v", err)
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: other.ID, Decision: protocol.DecideOnce})
	waitFor(t, "the task to go on", func() bool { return h.task(t, other.Task).Status == protocol.TaskWorking })

	asked := askHands(t, h, ref, "file the receipt", protocol.OniHandsScopeNested, "")
	for _, decision := range []string{protocol.DecideAlways, protocol.DecideChoose, protocol.DecideProvide} {
		if _, err := h.s.PresenterDecide(context.Background(), asked.ID, decision); !errors.As(err, &forbidden) || forbidden.Code != protocol.CodeForbidden {
			t.Fatalf("the pill answered %s: %v", decision, err)
		}
	}
	if _, err := h.s.PresenterDecide(context.Background(), asked.ID, protocol.DecideOnce); err != nil {
		t.Fatal(err)
	}
	if len(starts.seen()) != 1 {
		t.Fatal("the pill's once did not start Hands")
	}
}

// What a member's goal carries of a credential never shows on the card.
func TestHandsRequestRedactsTheGoal(t *testing.T) {
	h, _ := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	addToken(t, h, []string{"api.example.org"}, team)
	ref := h.agent(t, h.newTask(t, team, "Sign in.").ID)
	asked := askHands(t, h, ref, "paste "+apiToken+" into the login form", protocol.OniHandsScopeNested, "")
	h.noSecret(t, apiToken, asked)
}

// A restart leaves nothing that could act: a request that waited is
// answered as stale, the member is told to ask again, and Hands never
// starts.
func TestHandsRequestIsStaleAfterARestart(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	task := h.newTask(t, team, "File the receipt.")
	ref := h.agent(t, task.ID)
	asked := askHands(t, h, ref, "file the receipt", protocol.OniHandsScopeNested, "")
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	v := vault.OpenWith(filepath.Join(t.TempDir(), "vault.db"), fixedKeeper{key: bytes.Repeat([]byte{7}, 32)})
	service, err := open(h.dir, v, func(bool) {}, h.events, h.currentSettings)
	if err != nil {
		t.Fatal(err)
	}
	service.now = h.clock.now
	service.SetModels(listModels)
	service.SetHands(starts.start)
	t.Cleanup(func() { _ = service.Close() })
	h.s = service
	service.Bind(h.sessions)

	stale, _ := h.s.state.approval(asked.ID)
	if stale.Status != protocol.ApprovalExpired || !strings.Contains(stale.Result, "stale") {
		t.Fatalf("after a restart the request is %q: %q", stale.Status, stale.Result)
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
	if len(starts.seen()) != 0 {
		t.Fatal("a restart left a request that started Hands")
	}
	waitFor(t, "the agent to be told to ask again", func() bool {
		for _, note := range h.sessions.notes(ref.Session) {
			if strings.Contains(note, "Ask again") {
				return true
			}
		}
		return false
	})
}

// A member is offered request_hands only while Hyakki can start Hands and
// Hands works in apps.
func TestHandsToolIsOfferedOnlyWhenHandsCanWork(t *testing.T) {
	h, starts := handsHarness(t)
	team := h.newTeam(t, "Desk", nil)
	ref := h.agent(t, h.newTask(t, team, "File the receipt.").ID)
	info := h.sessions.info(ref.Session)
	if tc := h.s.ToolContext(info); tc == nil || !tc.Hands {
		t.Fatalf("the member is not offered Hands: %+v", tc)
	}
	h.s.SetHands(nil)
	if tc := h.s.ToolContext(info); tc == nil || tc.Hands {
		t.Fatal("Hands is offered with nothing to start it")
	}
	h.s.SetHands(starts.start)
	h.settingsMu.Lock()
	h.settings.Oni.Hands = protocol.OniHandsDesktop
	h.settingsMu.Unlock()
	if tc := h.s.ToolContext(info); tc == nil || tc.Hands {
		t.Fatal("Hands is offered to a team while it works only on the nested desktop stage")
	}
}
