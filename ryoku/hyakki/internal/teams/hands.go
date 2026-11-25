package teams

import (
	"context"
	"crypto/subtle"
	"slices"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// A member Hyakki hosts may ask for Hands through teams.agent_hands. The
// member supplies a goal and a scope; the user answers the approval it
// raises, in the app or on Oni's pill; an allowed one starts Hyakki's own
// Hands loop toward the goal, which acts by itself: the member never gets a
// stream of actions. What binds the answer to the run that asked lives in
// memory only, so a restart leaves nothing that could still act.

// HandsStart starts Hyakki's own Hands loop toward goal in scope (nested,
// or window with the window's id) and returns the Hands task's id. live is
// asked before every action; once it answers false the task must end.
type HandsStart func(ctx context.Context, goal, scope, window string, live func() bool) (string, error)

// handsGrantFor is how long an allowed request may keep acting.
const handsGrantFor = 30 * time.Minute

// handsAsk is a pending Hands approval's binding: the run that asked, by
// its token, and the request itself. It is never written to disk.
type handsAsk struct {
	token, goal, scope, window string
}

// handsAction is what a Hands approval keeps on disk: the scope and window,
// never the run's token.
type handsAction struct {
	Scope  string `json:"scope"`
	Window string `json:"window,omitempty"`
}

// SetHands hands the service what starts Hyakki's Hands loop; until it is
// set, a member's Hands request is refused.
func (s *Service) SetHands(start HandsStart) {
	s.mu.Lock()
	s.hands = start
	s.mu.Unlock()
}

// HandsApproval reports whether id is a Hands approval: only the Hyakki
// app answers one through teams.approval_decide.
func (s *Service) HandsApproval(id string) bool {
	rec, ok := s.state.approval(id)
	return ok && rec.Kind == protocol.ApprovalHands
}

// HandsAsked is a pending Hands approval's scope and window, for the card
// on Oni's pill; ok is false for anything else, and for a request the run
// that asked can no longer use.
func (s *Service) HandsAsked(id string) (scope, window string, ok bool) {
	rec, found := s.state.approval(id)
	if !found || rec.Kind != protocol.ApprovalHands || rec.Status != protocol.ApprovalPending {
		return "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ask, kept := s.handsAsks[id]
	if !kept {
		return "", "", false
	}
	return ask.scope, ask.window, true
}

// PresenterDecide answers a Hands approval from the surface drawing Oni:
// allowed once or denied, nothing else. Every other approval, and every
// other answer, is the Hyakki app's.
func (s *Service) PresenterDecide(ctx context.Context, id, decision string) (protocol.Approval, error) {
	if decision != protocol.DecideOnce && decision != protocol.DecideDeny {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app gives this answer."))
	}
	scope, _, ok := s.HandsAsked(id)
	if !ok || !slices.Contains(protocol.AgentHandsScopes, scope) {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app answers this."))
	}
	return s.decideApproval(ctx, protocol.ApprovalDecideParams{ID: id, Decision: decision})
}

// agentHands raises a member's Hands request as a single-use approval.
func (s *Service) agentHands(ctx context.Context, p protocol.AgentHandsParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	// a member held to reading may not ask for anything that acts
	team, _ := s.state.team(c.Team)
	member, _ := protocol.MemberByID(team.Members, c.Member)
	task, _ := s.state.task(c.Task)
	if taskApprovals(team, member, task) == protocol.ApprovalsReadOnly {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This task may only read, so it cannot ask for Hands."))
	}
	scope, window, goal := strings.TrimSpace(p.Scope), strings.TrimSpace(p.Window), strings.TrimSpace(p.Goal)
	switch scope {
	case protocol.OniHandsScopeNested:
		if window != "" {
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The nested desktop takes no window."))
		}
	case protocol.OniHandsScopeWindow:
		if !protocol.ValidAgentHandsWindow(window) {
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the window by its id from the window picker."))
		}
	case protocol.OniHandsScopeDesktop, protocol.OniHandsScopeBrowser:
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("A team's agent may ask Hands only for the nested desktop or one window the user picks."))
	default:
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Ask Hands for the nested desktop or for one window."))
	}
	if !protocol.ValidAgentHandsGoal(goal) {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
			i18n.Tf("Say the goal in one line of at most %d characters.", protocol.AgentHandsGoalMax))
	}
	s.mu.Lock()
	start := s.hands
	s.mu.Unlock()
	if start == nil {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hands cannot take a team's goal on this computer."))
	}
	if oni := protocol.EffectiveOni(s.settings()); !oni.Master || oni.Hands != protocol.OniHandsApp {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Hands is not set to work in apps. The user turns it on in Oni's settings."))
	}
	goal = s.redact(goal)
	asked := protocol.Approval{
		Kind: protocol.ApprovalHands, Title: i18n.Tf("Let Hands work toward: %s", goal),
		Detail: handsWhere(scope, window), Steps: handsSteps(scope),
	}
	// the binding is in place before the card can be answered, and goes
	// with a card the service would not keep
	asked.ID = newID("approval")
	s.mu.Lock()
	if s.handsAsks == nil {
		s.handsAsks = map[string]handsAsk{}
	}
	s.handsAsks[asked.ID] = handsAsk{token: c.Token, goal: goal, scope: scope, window: window}
	s.mu.Unlock()
	if _, err := s.ask(ctx, c, asked, handsAction{Scope: scope, Window: window}); err != nil {
		s.mu.Lock()
		delete(s.handsAsks, asked.ID)
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user to let Hands work in %s. End your turn now; Hyakki continues this task with their answer.", handsPlace(scope, window))}, nil
}

// effectHands carries out the answer to a Hands approval. An allowed one
// still bound to the run that asked becomes a grant and starts Hands; a
// request whose run changed, or that a restart left, is told to ask again.
func (s *Service) effectHands(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	s.mu.Lock()
	ask, kept := s.handsAsks[rec.ID]
	delete(s.handsAsks, rec.ID)
	start := s.hands
	s.mu.Unlock()
	again := i18n.T("This Hands request no longer belongs to the task's current run, because Hyakki restarted or the run changed, so Hands did nothing. Ask again if Hands is still needed.")
	if !kept {
		rec.Result = i18n.T("Hands did nothing: the request was stale.")
		return again, nil
	}
	if rec.Status != protocol.ApprovalApproved {
		rec.Result = i18n.T("Hands did nothing.")
		text := i18n.T("The user did not let Hands work. Hands did nothing.")
		if note := strings.TrimSpace(p.Note); note != "" {
			text += "\n" + i18n.Tf("Their note: %s", note)
		}
		return text, nil
	}
	if start == nil {
		rec.Result = i18n.T("Hands did nothing: the request was stale.")
		return again, nil
	}
	expires := s.now().Add(handsGrantFor)
	live := func() bool { return s.handsLive(rec.Task, rec.Session, ask.token, expires) }
	if !live() {
		rec.Result = i18n.T("Hands did nothing: the request was stale.")
		return again, nil
	}
	task, err := start(ctx, ask.goal, ask.scope, ask.window, live)
	if err != nil {
		return i18n.T("Hands did not start, so nothing was done."), err
	}
	rec.Result = i18n.T("Hands started toward the goal.")
	return i18n.Tf("The user let Hands work once in %s. Hyakki's own Hands now works toward the goal by itself as task %s; you get no stream of its actions. Look at the result yourself before you rely on it.",
		handsPlace(ask.scope, ask.window), task), nil
}

// handsLive says a grant may still act: its task's run is the one that
// asked, by its token, and the grant has not expired.
func (s *Service) handsLive(task, session, token string, expires time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runs[task]
	return r != nil && r.Session == session && token != "" && subtle.ConstantTimeCompare([]byte(r.Token), []byte(token)) == 1 &&
		s.now().Before(expires)
}

// staleHandsApprovals answers every Hands approval nothing in memory binds
// any more, as after a restart: Hands does nothing and the agent is told to
// ask again.
func (s *Service) staleHandsApprovals() {
	for _, rec := range s.state.approvals(func(r approvalRecord) bool {
		return r.Kind == protocol.ApprovalHands && r.Status == protocol.ApprovalPending
	}) {
		s.mu.Lock()
		_, kept := s.handsAsks[rec.ID]
		s.mu.Unlock()
		if kept {
			continue
		}
		note := i18n.T("Hyakki restarted before this was answered.")
		claimed, err := s.claimApproval(rec.ID, protocol.ApprovalExpired, protocol.DecideDeny, note)
		if err != nil {
			continue
		}
		p := protocol.ApprovalDecideParams{ID: rec.ID, Decision: protocol.DecideDeny, Note: note}
		_, _ = s.settleApproval(s.background, claimed, p, "approval.expire", "Hyakki", protocol.InitiatorRashin)
	}
}

// handsPlace names where Hands would work, for the agent.
func handsPlace(scope, window string) string {
	if scope == protocol.OniHandsScopeWindow {
		return i18n.Tf("the window %s", window)
	}
	return i18n.T("the nested desktop Hyakki owns")
}

// handsWhere is a Hands approval's detail: the scope and the window.
func handsWhere(scope, window string) string {
	if scope == protocol.OniHandsScopeWindow {
		return i18n.Tf("Where: one live window you picked (%s). Typing goes in short pieces, each checked before and after; any key you press pauses it.", window)
	}
	return i18n.T("Where: the nested desktop Hyakki owns, never your own desktop.")
}

// handsSteps are the actions a Hands approval allows in its scope.
func handsSteps(scope string) []string {
	steps := []string{
		i18n.T("Look at what is shown"),
		i18n.T("Click on what is shown"),
		i18n.T("Type text in short checked pieces"),
		i18n.T("Press safe keys such as Tab and the arrows"),
		i18n.T("Scroll and wait"),
	}
	if scope == protocol.OniHandsScopeNested {
		steps = append(steps, i18n.T("Open an app from the catalog"), i18n.T("Find files and show their folder, never run them"))
	}
	return steps
}
