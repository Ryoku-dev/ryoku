package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestFastHandoffTurnDoesNotSuppressFollowingGoalProgress(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	thirdStarted := make(chan struct{})
	releaseObserver := make(chan struct{})
	var firstOnce, observerOnce sync.Once
	unblockFirst := func() { firstOnce.Do(func() { close(releaseFirst) }) }
	unblockObserver := func() { observerOnce.Do(func() { close(releaseObserver) }) }
	t.Cleanup(unblockFirst)
	t.Cleanup(unblockObserver)
	var rounds atomic.Int32
	var observing atomic.Bool
	manager, info, _ := goalManager(t, func(turn int, text string, sink core.Sink, manager *sessionManager, id string) (core.TurnResult, error) {
		rounds.Store(int32(turn))
		stepTurn(turn, sink)
		switch turn {
		case 1:
			close(firstStarted)
			<-releaseFirst
		case 2:
			if !strings.Contains(text, "handoff-result") {
				t.Error("delivery was replaced by an ordinary goal continuation")
			}
		case 3:
			close(thirdStarted)
			_, err := manager.Goal(context.Background(), protocol.GoalParams{ID: id, Op: protocol.GoalComplete, Summary: "Delivery and subsequent goal progress completed"})
			return core.TurnResult{Stop: protocol.StopEnd}, err
		}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	})
	observedSession, err := manager.get(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A slow observer makes the delivered turn finish before startPrompt's
	// caller returns. The reservation must already have been consumed then.
	manager.changed = func() {
		// Persisted tool events call this hook while holding the session lock.
		// Only delay the unlocked launch observer, never the tool itself.
		if !observedSession.mu.TryLock() {
			return
		}
		launched := observedSession.running && observedSession.log.Info().Turns == 2
		observedSession.mu.Unlock()
		if launched && observing.CompareAndSwap(false, true) {
			select {
			case <-thirdStarted:
			case <-releaseObserver:
			}
		}
	}
	if _, err := manager.Goal(context.Background(), protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: "Deliver then continue", MaxTurns: 4}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("first goal turn did not start")
	}
	reserved := make(chan struct{})
	delivered := make(chan error, 1)
	go func() {
		delivered <- manager.ContinueAfter(context.Background(), info.ID, func() (string, bool, error) {
			close(reserved)
			return "handoff-result", true, nil
		})
	}()
	select {
	case <-reserved:
	case <-time.After(5 * time.Second):
		t.Fatal("handoff did not reserve its delivery")
	}
	unblockFirst()
	select {
	case <-thirdStarted:
	case <-time.After(5 * time.Second):
		unblockObserver()
		current, _ := manager.Info(info.ID)
		session, _ := manager.get(info.ID)
		session.mu.Lock()
		handoffs, running, finishing, reserved, turning := session.handoffs, session.running, session.finishing, session.reserveDone != nil, session.turnDone != nil
		session.mu.Unlock()
		t.Fatalf("goal progress stalled: rounds=%d state=%s goal=%+v handoffs=%d running=%v finishing=%v reserved=%v turning=%v", rounds.Load(), current.State, current.Goal, handoffs, running, finishing, reserved, turning)
	}
	unblockObserver()
	select {
	case err := <-delivered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the accepted delivery did not return")
	}
	waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool { return goal.Status == protocol.GoalDone })
	if err := manager.Stop(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
}

func TestObsoleteConcurrentHandoffDoesNotStrandAnActiveGoal(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	rejectedReserved := make(chan struct{})
	releaseRejected := make(chan struct{})
	thirdStarted := make(chan struct{})
	var firstOnce, rejectedOnce sync.Once
	unblockFirst := func() { firstOnce.Do(func() { close(releaseFirst) }) }
	unblockRejected := func() { rejectedOnce.Do(func() { close(releaseRejected) }) }
	manager, info, prompts := goalManager(t, func(turn int, text string, sink core.Sink, manager *sessionManager, id string) (core.TurnResult, error) {
		stepTurn(turn, sink)
		switch turn {
		case 1:
			close(firstStarted)
			<-releaseFirst
		case 2:
			if text != "accepted-result" {
				t.Errorf("the accepted handoff was not delivered: %q", text)
			}
		case 3:
			close(thirdStarted)
			_, err := manager.Goal(context.Background(), protocol.GoalParams{ID: id, Op: protocol.GoalComplete, Summary: "Continued after the obsolete delivery was withdrawn"})
			return core.TurnResult{Stop: protocol.StopEnd}, err
		}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	})
	t.Cleanup(func() {
		unblockFirst()
		unblockRejected()
		_ = manager.Stop(context.Background(), info.ID)
	})
	wait := func(name string, ready <-chan struct{}) {
		t.Helper()
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not complete", name)
		}
	}
	if _, err := manager.Goal(context.Background(), protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: "Continue after valid results, without replaying obsolete results", MaxTurns: 4}); err != nil {
		t.Fatal(err)
	}
	wait("first turn", firstStarted)
	rejected := make(chan error, 1)
	go func() {
		rejected <- manager.ContinueAfter(context.Background(), info.ID, func() (string, bool, error) {
			close(rejectedReserved)
			<-releaseRejected
			return "", false, nil
		})
	}()
	wait("obsolete handoff reservation", rejectedReserved)
	acceptedReserved := make(chan struct{})
	accepted := make(chan error, 1)
	go func() {
		accepted <- manager.ContinueAfter(context.Background(), info.ID, func() (string, bool, error) {
			close(acceptedReserved)
			return "accepted-result", true, nil
		})
	}()
	wait("accepted handoff reservation", acceptedReserved)
	unblockFirst()
	select {
	case err := <-accepted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accepted handoff did not return")
	}
	waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool {
		return goal.Turns == 2 && goal.Status == protocol.GoalActive
	})
	session, err := manager.get(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	session.mu.Lock()
	secondTurnDone := session.turnDone
	session.mu.Unlock()
	if secondTurnDone != nil {
		wait("second turn bookkeeping", secondTurnDone)
	}
	unblockRejected()
	select {
	case err := <-rejected:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("obsolete handoff did not return")
	}
	wait("automatic progress after the obsolete handoff", thirdStarted)
	settled := waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool {
		return goal.Status == protocol.GoalDone && goal.Turns == 3
	})
	if settled.Goal.Turns != 3 || len(prompts()) != 3 {
		t.Fatalf("handoff accounting or delivery was duplicated: goal=%+v prompts=%q", settled.Goal, prompts())
	}
}

func TestFailedHandoffDoesNotDispatchQueuedOwnerWork(t *testing.T) {
	for _, failure := range []string{"release-error", "cancel-after-release", "concurrent-obsolete"} {
		t.Run(failure, func(t *testing.T) {
			firstStarted, releaseFirst := make(chan struct{}), make(chan struct{})
			reserved, releaseHandoff := make(chan struct{}), make(chan struct{})
			var firstOnce, handoffOnce sync.Once
			unblockFirst := func() { firstOnce.Do(func() { close(releaseFirst) }) }
			unblockHandoff := func() { handoffOnce.Do(func() { close(releaseHandoff) }) }
			manager, info, prompts := goalManager(t, func(turn int, _ string, sink core.Sink, _ *sessionManager, _ string) (core.TurnResult, error) {
				stepTurn(turn, sink)
				if turn == 1 {
					close(firstStarted)
					<-releaseFirst
				}
				return core.TurnResult{Stop: protocol.StopEnd}, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(func() {
				unblockFirst()
				unblockHandoff()
				cancel()
				_ = manager.Stop(context.Background(), info.ID)
			})
			wait := func(name string, ready <-chan struct{}) {
				t.Helper()
				select {
				case <-ready:
				case <-time.After(5 * time.Second):
					t.Fatalf("%s did not complete", name)
				}
			}
			if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "Original work"}); err != nil {
				t.Fatal(err)
			}
			wait("original turn", firstStarted)
			refused := errors.New("hold transition could not persist")
			finished := make(chan error, 1)
			go func() {
				finished <- manager.ContinueAfter(ctx, info.ID, func() (string, bool, error) {
					close(reserved)
					<-releaseHandoff
					if failure != "cancel-after-release" {
						return "", false, refused
					}
					cancel()
					return "undelivered result", true, nil
				})
			}()
			wait("handoff reservation", reserved)
			var withdrawObsolete func()
			var obsoleteFinished <-chan error
			if failure == "concurrent-obsolete" {
				obsoleteReserved, releaseObsolete := make(chan struct{}), make(chan struct{})
				var once sync.Once
				withdrawObsolete = func() { once.Do(func() { close(releaseObsolete) }) }
				t.Cleanup(withdrawObsolete)
				finished := make(chan error, 1)
				obsoleteFinished = finished
				go func() {
					finished <- manager.ContinueAfter(context.Background(), info.ID, func() (string, bool, error) {
						close(obsoleteReserved)
						<-releaseObsolete
						return "", false, nil
					})
				}()
				wait("second obsolete reservation", obsoleteReserved)
			}
			if _, err := manager.queue(context.Background(), protocol.QueueParams{ID: info.ID, Text: "Queued owner request"}); err != nil {
				t.Fatal(err)
			}
			session, err := manager.get(info.ID)
			if err != nil {
				t.Fatal(err)
			}
			session.mu.Lock()
			originalDone := session.turnDone
			session.mu.Unlock()
			unblockFirst()
			wait("original turn bookkeeping", originalDone)
			unblockHandoff()
			select {
			case err := <-finished:
				want := refused
				if failure == "cancel-after-release" {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Fatalf("failed delivery returned %v, want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("failed delivery did not return")
			}
			if withdrawObsolete != nil {
				withdrawObsolete()
				select {
				case err := <-obsoleteFinished:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("obsolete concurrent delivery did not return")
				}
			}
			current, _ := manager.Info(info.ID)
			if len(current.Queue) != 1 || current.Queue[0].Text != "Queued owner request" || len(prompts()) != 1 {
				t.Fatalf("failed delivery dispatched work before its controller could fence it: queue=%+v prompts=%q", current.Queue, prompts())
			}
			if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "Do not reuse the failed execution"}); err == nil {
				t.Fatal("a failed execution accepted another owner prompt before task retry")
			}
			if _, err := manager.Goal(context.Background(), protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: "Do not undo a failed delivery fence"}); err == nil {
				t.Fatal("resetting a goal revived the failed execution")
			}
			if current, _ := manager.Info(info.ID); current.Goal != nil {
				t.Fatal("a refused goal reset changed the failed execution's goal")
			}
		})
	}
}
