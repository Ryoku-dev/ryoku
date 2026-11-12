package honcho

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/backendtest"
	"ryoku-rashin/internal/protocol"
)

func TestObserveSplitsLongTurns(t *testing.T) {
	f := newFake(t, backendtest.Canary)
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	reply := strings.Repeat("word ", 12000) // 60,000 characters
	err := h.Observe(context.Background(), backend.Transcript{
		Session: "long", Kind: protocol.MemoryFromAsk, Scope: protocol.MemoryYou,
		Turns: []backend.Turn{{At: time.Now(), User: "Tell me.", Reply: reply}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	replies := 0
	for _, m := range f.messages {
		if len(m.Content) > contentMax {
			t.Fatalf("a message of %d characters was sent", len(m.Content))
		}
		if m.PeerID != userPeer {
			replies++
		}
	}
	if replies != 3 {
		t.Fatalf("the reply went in %d parts, want 3", replies)
	}
}

func TestObserveRetrySendsEachMessageOnce(t *testing.T) {
	f := newFake(t, backendtest.Canary)
	f.failPost = 2
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	turns := make([]backend.Turn, 80) // 160 messages, two posts
	for i := range turns {
		turns[i] = backend.Turn{Seq: int64(i + 1), At: time.Now(), User: fmt.Sprintf("Message %d.", i), Reply: "Ok."}
	}
	transcript := backend.Transcript{Session: "retry", Kind: protocol.MemoryFromAsk, Scope: protocol.MemoryYou, Turns: turns}
	if err := h.Observe(context.Background(), transcript); err == nil {
		t.Fatal("the failed post was not reported")
	}
	if err := h.Observe(context.Background(), transcript); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if got := len(f.sessionMessages[sessionID("retry")]); got != 160 {
		t.Fatalf("the session holds %d messages after a retry, want 160", got)
	}
}

func TestGroupsResolveToScopes(t *testing.T) {
	f := newFake(t, backendtest.Canary)
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	ctx := context.Background()
	if _, err := h.Remember(ctx, []protocol.Memory{{Scope: protocol.MemoryProjectPrefix + "p1", Text: "The build uses meson."}}); err != nil {
		t.Fatal(err)
	}
	found, err := h.Recall(ctx, backend.Query{Text: "build meson", Scopes: []string{protocol.MemoryProjects}, Limit: 5})
	if err != nil || len(found) != 1 {
		t.Fatalf("Recall in the projects group found %+v, %v", found, err)
	}
	f.mu.Lock()
	peers := slices.Clone(f.peers["rashin-desktop"])
	f.mu.Unlock()
	if slices.Contains(peers, "") || slices.Contains(peers, "rashin-") {
		t.Fatalf("a group became a peer: %q", peers)
	}
}

func TestSessionIDsAreSanitised(t *testing.T) {
	for _, id := range []string{"s.1/2", "ok-1", strings.Repeat("x", 600)} {
		if got := sessionID(id); !protocol.ValidHonchoID(got) {
			t.Fatalf("sessionID(%q) = %q, which Honcho refuses", id, got)
		}
	}
}

func TestScopesStayApart(t *testing.T) {
	f := newFake(t, backendtest.Canary)
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	ctx := context.Background()
	for _, scope := range []string{protocol.MemoryYou, protocol.MemoryProjectPrefix + "a/b c"} {
		if _, err := h.Remember(ctx, []protocol.Memory{{Scope: scope, Text: "The build uses meson."}}); err != nil {
			t.Fatal(err)
		}
	}
	found, err := h.Recall(ctx, backend.Query{Text: "build meson", Scopes: []string{protocol.MemoryProjectPrefix + "a/b c"}, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Scope != protocol.MemoryProjectPrefix+"a/b c" {
		t.Fatalf("Recall in one project found %+v", found)
	}
}

func TestBusyWhileACallIsInFlight(t *testing.T) {
	f := newFake(t, backendtest.Canary)
	f.hold = make(chan struct{})
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	done := make(chan error)
	go func() {
		_, _, err := h.List(context.Background(), protocol.MemoryYou, 0, 10)
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !h.Busy() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !h.Busy() {
		t.Fatal("not Busy while a call waits on the server")
	}
	close(f.hold)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if h.Busy() {
		t.Fatal("still Busy after the call returned")
	}
}

func TestKeyNeverShows(t *testing.T) {
	f := newFake(t, "another-key")
	h := New(f.server.URL, "rashin-desktop", backendtest.Canary)
	_, _, err := h.List(context.Background(), protocol.MemoryYou, 0, 10)
	if err == nil || strings.Contains(err.Error(), backendtest.Canary) {
		t.Fatalf("a refused key gave %v", err)
	}
	f.server.Close()
	status := h.Status(context.Background())
	if status.State != protocol.EngineUnreachable || strings.Contains(status.Detail, backendtest.Canary) {
		t.Fatalf("a gone server gave %+v", status)
	}
}

func TestNoKeySendsNoHeader(t *testing.T) {
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pending_work_units":0,"in_progress_work_units":0}`))
	}))
	defer server.Close()
	h := New(server.URL, "rashin-desktop", "")
	if status := h.Status(context.Background()); status.State != protocol.EngineReady {
		t.Fatalf("Status = %+v", status)
	}
	if header != "" {
		t.Fatalf("an empty key sent Authorization %q", header)
	}
}

func TestLeavesMachine(t *testing.T) {
	if New("http://127.0.0.1:8000", "w", "").Caps().LeavesMachine {
		t.Fatal("a loopback server counts as leaving")
	}
	if !New("https://api.honcho.dev", "w", "").Caps().LeavesMachine {
		t.Fatal("a cloud server does not count as leaving")
	}
}
