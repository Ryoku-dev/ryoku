package teams

import (
	"context"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

type delayedSessionStart struct {
	*fakeSessions
	entered chan struct{}
	release chan struct{}
}

func (s *delayedSessionStart) New(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error) {
	close(s.entered)
	<-s.release
	return s.fakeSessions.New(ctx, p)
}

func TestCloseWaitsForTaskStartupToFinishWriting(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Review", nil)
	sessions := &delayedSessionStart{fakeSessions: h.sessions, entered: make(chan struct{}), release: make(chan struct{})}
	var released sync.Once
	unblock := func() { released.Do(func() { close(sessions.release) }) }
	defer unblock()
	h.s.mu.Lock()
	h.s.sessions = sessions
	h.s.mu.Unlock()
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Title: "Review changes", Brief: "Review the recent changes."})
	select {
	case <-sessions.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the task did not reach session startup")
	}
	closed := make(chan error, 1)
	go func() { closed <- h.s.Close() }()
	select {
	case <-h.s.background.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not cancel background work")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned while task startup could still save state: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not finish after the task starter returned")
	}
}
