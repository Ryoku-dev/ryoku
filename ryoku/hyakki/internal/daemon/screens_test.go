package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// screenAsks answers the screen manager's asks for a screen the way a test
// says, counting them
type screenAsks struct {
	mu      sync.Mutex
	count   int
	answers chan string
}

func (a *screenAsks) ask(ctx context.Context, _ string, _ protocol.PermissionData) (string, error) {
	a.mu.Lock()
	a.count++
	a.mu.Unlock()
	select {
	case answer := <-a.answers:
		return answer, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (a *screenAsks) asked() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.count
}

func shortAgentWait(t *testing.T) {
	previous := agentWait
	agentWait = 50 * time.Millisecond
	t.Cleanup(func() { agentWait = previous })
}

// upScreen is a screen that is up without a nested session behind it
func upScreen(m *screenManager, id string) *agentScreen {
	ready := make(chan struct{})
	close(ready)
	item := &agentScreen{ready: ready, end: func() {}, last: time.Now(), state: protocol.ScreenState{Session: id, State: protocol.ScreenRunning, Holder: protocol.HolderAgent}}
	m.items[id] = item
	return item
}

func TestAgentScreenCallStopsWaitingForConsentWithoutAskingTwice(t *testing.T) {
	shortAgentWait(t)
	asks := &screenAsks{answers: make(chan string, 1)}
	m := newScreenManager(func(string, any) {}, asks.ask, func() bool { return true })
	look := protocol.ScreenActParams{ID: "s", By: protocol.HolderAgent, Action: protocol.ScreenAction{Kind: "look"}}
	for range 2 {
		result, err := m.act(context.Background(), look)
		if err != nil || result.Waiting != protocol.ScreenWaitConsent {
			t.Fatalf("an unanswered ask = %#v, %v; want a call that says it waits on consent", result, err)
		}
	}
	if asks.asked() != 1 {
		t.Fatalf("the user was asked %d times for one screen", asks.asked())
	}
	asks.answers <- "deny"
	if _, err := m.act(context.Background(), look); err == nil {
		t.Fatal("the agent was not told the user declined")
	}
	if result, err := m.act(context.Background(), look); err != nil || result.Waiting != protocol.ScreenWaitConsent || asks.asked() != 2 {
		t.Fatalf("after hearing the answer, the next call = %#v, %v with %d asks; want a fresh ask", result, err, asks.asked())
	}
}

func TestStoppingAScreenTakesBackItsUnansweredAsk(t *testing.T) {
	shortAgentWait(t)
	asks := &screenAsks{answers: make(chan string)}
	m := newScreenManager(func(string, any) {}, asks.ask, func() bool { return true })
	if result, _ := m.act(context.Background(), protocol.ScreenActParams{ID: "s", By: protocol.HolderAgent, Action: protocol.ScreenAction{Kind: "look"}}); result.Waiting != protocol.ScreenWaitConsent {
		t.Fatalf("result = %#v", result)
	}
	stopped := make(chan struct{})
	go func() {
		m.stop("s")
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopping a screen waited for the user to answer its ask")
	}
	if m.busy() {
		t.Fatal("a stopped screen kept the daemon busy")
	}
}

func TestAgentActionWaitsOutTheUserWithoutActing(t *testing.T) {
	shortAgentWait(t)
	m := newScreenManager(func(string, any) {}, nil, nil)
	item := upScreen(m, "s")
	item.held = make(chan struct{})
	item.state.Holder = protocol.HolderUser
	result, err := m.act(context.Background(), protocol.ScreenActParams{ID: "s", By: protocol.HolderAgent, Action: protocol.ScreenAction{Kind: "type", Text: "x"}})
	if err != nil || result.Waiting != protocol.ScreenWaitUser {
		t.Fatalf("an action while the user holds the screen = %#v, %v", result, err)
	}
}

func TestAskStaysUpPastTheCallAndEndsWithTheHandBack(t *testing.T) {
	shortAgentWait(t)
	var notices int
	m := newScreenManager(func(event string, _ any) {
		if event == protocol.EvtNotice {
			notices++
		}
	}, nil, nil)
	upScreen(m, "s")
	ask := protocol.ScreenControlParams{ID: "s", Op: protocol.ScreenAsk, Reason: "Sign in, then hand it back."}
	for range 2 {
		state, err := m.control(context.Background(), ask)
		if err != nil || state.Ask == "" {
			t.Fatalf("an ask the user has not finished = %#v, %v; want it still up", state, err)
		}
	}
	if notices != 1 {
		t.Fatalf("the user was told %d times about one ask", notices)
	}
	look := protocol.ScreenActParams{ID: "s", By: protocol.HolderAgent, Action: protocol.ScreenAction{Kind: "wait", Seconds: 0.01}}
	if result, err := m.act(context.Background(), look); err != nil || result.Waiting != protocol.ScreenWaitUser {
		t.Fatalf("the agent acted while the user had its ask: %#v, %v", result, err)
	}
	if _, err := m.control(context.Background(), protocol.ScreenControlParams{ID: "s", Op: protocol.ScreenTake}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.control(context.Background(), protocol.ScreenControlParams{ID: "s", Op: protocol.ScreenRelease}); err != nil {
		t.Fatal(err)
	}
	if result, err := m.act(context.Background(), look); err != nil || result.Waiting != "" {
		t.Fatalf("after the hand back the agent's action = %#v, %v", result, err)
	}
	agentWait = time.Minute
	cancelled, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := m.control(cancelled, ask); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a call the harness gave up on = %v", err)
	}
	if state := m.items["s"].snapshot(); state.Ask != "" {
		t.Fatalf("a cancelled call left its ask up: %#v", state)
	}
}

func TestDesktopWithoutScreensRefusesBeforeAskingTheUser(t *testing.T) {
	shortAgentWait(t)
	asks := &screenAsks{answers: make(chan string, 1)}
	m := newScreenManager(func(string, any) {}, asks.ask, func() bool { return false })
	if _, err := m.act(context.Background(), protocol.ScreenActParams{ID: "s", By: protocol.HolderAgent, Action: protocol.ScreenAction{Kind: "look"}}); err == nil {
		t.Fatal("a desktop that cannot host a screen gave the agent one")
	}
	if asks.asked() != 0 {
		t.Fatal("the user was asked for a screen this desktop cannot host")
	}
}
