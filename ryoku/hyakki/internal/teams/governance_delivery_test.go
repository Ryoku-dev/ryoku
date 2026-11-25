package teams

import (
	"context"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

type controlledHoldDelivery struct {
	proceed chan struct{}
	done    chan struct{}
}

type controlledHoldSessions struct {
	*fakeSessions
	accepted chan controlledHoldDelivery
}

func (sessions *controlledHoldSessions) ContinueAfter(ctx context.Context, id string, release func() (string, bool, error)) error {
	request := controlledHoldDelivery{proceed: make(chan struct{}), done: make(chan struct{})}
	defer close(request.done)
	select {
	case sessions.accepted <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-request.proceed:
		return sessions.fakeSessions.ContinueAfter(ctx, id, release)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func receiveHoldDelivery(t *testing.T, sessions *controlledHoldSessions) controlledHoldDelivery {
	t.Helper()
	select {
	case request := <-sessions.accepted:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("parent release did not reserve its continuation")
		return controlledHoldDelivery{}
	}
}

func TestCompletedParentHoldCannotBeDeliveredAgainAfterWaitingAgain(t *testing.T) {
	h, team := governedTeam(t)
	root := h.newTask(t, team, "Receive exactly one delivery per hold")
	ref := h.agent(t, root.ID)
	current := h.task(t, root.ID)
	current.Waiting = protocol.WaitingOnMembers
	if err := h.s.state.putTask(current); err != nil {
		t.Fatal(err)
	}
	sessions := &controlledHoldSessions{fakeSessions: h.sessions, accepted: make(chan controlledHoldDelivery, 2)}
	h.s.sessions = sessions
	h.s.releaseParent(root.ID)
	first := receiveHoldDelivery(t, sessions)
	h.s.releaseParent(root.ID)
	second := receiveHoldDelivery(t, sessions)
	var firstOnce, secondOnce sync.Once
	allowFirst := func() { firstOnce.Do(func() { close(first.proceed) }) }
	allowSecond := func() { secondOnce.Do(func() { close(second.proceed) }) }
	t.Cleanup(allowFirst)
	t.Cleanup(allowSecond)
	allowFirst()
	select {
	case <-first.done:
	case <-time.After(5 * time.Second):
		t.Fatal("first claimed hold did not deliver")
	}
	latest := h.task(t, root.ID)
	if latest.Waiting != "" || latest.ResumeVersion != current.ResumeVersion+1 {
		t.Fatal("accepted delivery did not consume the original hold")
	}
	// The same live execution can wait again before an old release callback
	// arrives. A matching enum/session alone must not consume the new hold.
	latest.Waiting = protocol.WaitingOnMembers
	if err := h.s.state.putTask(latest); err != nil {
		t.Fatal(err)
	}
	allowSecond()
	select {
	case <-second.done:
	case <-time.After(5 * time.Second):
		t.Fatal("obsolete delivery callback did not return")
	}
	if retained := h.task(t, root.ID); retained.Waiting != protocol.WaitingOnMembers || retained.ResumeVersion != latest.ResumeVersion {
		t.Fatal("an old callback consumed a later hold on the same execution")
	}
	h.sessions.mu.Lock()
	defer h.sessions.mu.Unlock()
	count := 0
	for _, continuation := range h.sessions.continued {
		if continuation.session == ref.Session {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("one original hold produced %d deliveries", count)
	}
}

func TestAcceptedManagerChildReleasesSourceBeforeDeliveryMarkerCommits(t *testing.T) {
	h, team := governedTeam(t)
	root := h.newTask(t, team, "Deliver a fast manager decision")
	manager := governedPiece(t, h, root, h.agent(t, root.ID), team.Members[1].ID)
	leaf := governedPiece(t, h, manager, h.agent(t, manager.ID), team.Members[2].ID)
	leafRef := h.agent(t, leaf.ID)
	message, err := h.s.recordTeamMessage(WithAgentSession(context.Background(), leafRef.Session), protocol.TeamMessage{
		Team: team.ID, Task: leaf.ID, From: leaf.Member, To: team.Members[0].ID, Member: team.Members[3].ID,
		Kind: "help", Title: "Fast decision", Body: "Decide through the common manager before the delivery marker is persisted.",
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf = h.task(t, leaf.ID)
	leaf.Waiting = protocol.WaitingOnMembers
	if err := h.s.state.putTask(leaf); err != nil {
		t.Fatal(err)
	}
	decision, accepted, err := h.s.wakeMessage(context.Background(), message)
	if err != nil || !accepted {
		t.Fatalf("manager did not accept the durable request: %v %v", accepted, err)
	}
	stored := h.s.state.messages(func(candidate protocol.TeamMessage) bool { return candidate.ID == message.ID })
	if len(stored) != 1 || stored[0].Delivered || stored[0].Retired {
		t.Fatal("the regression did not reach the pre-marker crash window")
	}
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: h.agent(t, decision.ID), Title: "Decided", Body: "Continue the reviewed approach."})
	waitFor(t, "the source to receive its fast manager result", func() bool { return h.task(t, leaf.ID).Waiting == "" })
	if err := h.s.state.deliverMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	if current := h.task(t, leaf.ID); current.Status != protocol.TaskWorking || current.Member != leaf.Member {
		t.Fatal("marker completion lost or reassigned the released source")
	}
}
