package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	store "ryoku-rashin/internal/session"
)

const (
	storedPassword = "correct-horse-battery-staple-9"
	printedToken   = "glpat-1234567890abcdefghij"
	streamedToken  = "sk-proj-1234567890abcdefghijklmnop"
)

func eventsOnDisk(t *testing.T, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(paths.SessionDir(id), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// A stored credential the user typed, a token a tool printed and one the
// model streamed in pieces all leave the session's log once the turn ends,
// and the rest of the turn is kept word for word.
func TestATurnLeavesNoSecretOnDisk(t *testing.T) {
	t.Cleanup(redact.Register(func(text string) string { return strings.ReplaceAll(text, storedPassword, "‹MAIL_PASSWORD›") }))
	runner := &fakeRunner{prompt: func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "Logged in. The token is "})
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: streamedToken[:10]})
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: "c1", Title: "cat .env", Kind: protocol.ToolExecute, Status: protocol.ToolDone, Output: "GITLAB=" + printedToken + "\nREGION=eu-west-1"})
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: streamedToken[10:] + ", keep it safe."})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "My mail password is " + storedPassword + ", log in for me."}, nil); err != nil {
		t.Fatal(err)
	}
	waitTurnEnd(t, collector)
	text := eventsOnDisk(t, info.ID)
	for _, secret := range []string{storedPassword, printedToken, streamedToken[:10], streamedToken[10:]} {
		if strings.Contains(text, secret) {
			t.Fatalf("%q is on disk: %s", secret, text)
		}
	}
	for _, kept := range []string{"My mail password is ‹MAIL_PASSWORD›, log in for me.", "Logged in. The token is " + redact.Mark, ", keep it safe.", `REGION=eu-west-1`, `"title":"cat .env"`} {
		if !strings.Contains(text, kept) {
			t.Fatalf("%q was lost: %s", kept, text)
		}
	}
	if got, _ := manager.Info(info.ID); strings.Contains(got.Preview, streamedToken[10:]) || strings.Contains(got.Title, storedPassword) {
		t.Fatalf("the session's title or preview keeps a secret: %+v", got)
	}
}

// oldSession writes an Ask session as one written before turns were
// scrubbed: one finished turn asking prompt.
func oldSession(t *testing.T, id, prompt string) {
	t.Helper()
	log, err := store.Create(protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Title: prompt, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		kind string
		data any
	}{
		{protocol.EvUser, protocol.UserData{Text: prompt}},
		{protocol.EvText, protocol.TextData{Message: "m", Delta: "Done."}},
		{protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}},
	} {
		if _, err := log.Append(step.kind, step.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
}

// Sessions written before turns were scrubbed are scrubbed once; one with
// nothing secret is left byte for byte, and both steps are marked done.
func TestTheScrubMigrationCleansOldSessions(t *testing.T) {
	isolateDaemon(t)
	oldSession(t, "ask-20261101-120000-aaaa", "Use "+printedToken+" for the mirror")
	oldSession(t, "ask-20261101-120000-bbbb", "What is my disk usage?")
	plain := eventsOnDisk(t, "ask-20261101-120000-bbbb")
	d := New(Deps{})
	if err := d.migrateScrubSessions(); err != nil {
		t.Fatal(err)
	}
	if text := eventsOnDisk(t, "ask-20261101-120000-aaaa"); strings.Contains(text, printedToken) || !strings.Contains(text, "Use "+redact.Mark+" for the mirror") {
		t.Fatalf("the old session: %s", text)
	}
	if text := eventsOnDisk(t, "ask-20261101-120000-bbbb"); text != plain {
		t.Fatalf("an ordinary session changed:\n%s\n%s", plain, text)
	}
	if migrationPending(scrubPatterns) || migrationPending(scrubValues) {
		t.Fatal("the migration was not marked")
	}
	list, err := store.List(protocol.KindAsk)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range list {
		if strings.Contains(info.Title, printedToken) {
			t.Fatalf("a title keeps the secret: %q", info.Title)
		}
	}
}

// With the vault locked the migration takes out only what the patterns
// find and owes the rest; the first time the vault opens while the daemon
// is up, the full pass runs once and the step is done.
func TestTheScrubMigrationWaitsForTheVault(t *testing.T) {
	isolateDaemon(t)
	var open atomic.Bool
	t.Cleanup(redact.RegisterStore(func() func(string) string {
		if !open.Load() {
			return func(text string) string { return text }
		}
		return func(text string) string { return strings.ReplaceAll(text, storedPassword, "‹MAIL_PASSWORD›") }
	}, open.Load))
	id := "ask-20261101-120000-cccc"
	oldSession(t, id, "Log in with "+storedPassword+" and push with "+printedToken)
	d := New(Deps{})
	t.Cleanup(d.migrations.stop)
	if err := d.migrateScrubSessions(); err != nil {
		t.Fatal(err)
	}
	if text := eventsOnDisk(t, id); strings.Contains(text, printedToken) || !strings.Contains(text, storedPassword) {
		t.Fatalf("the pass with the vault locked: %s", text)
	}
	if migrationPending(scrubPatterns) || !migrationPending(scrubValues) {
		t.Fatal("a pass without the vault's values was marked as the full one")
	}
	d.Broadcast(protocol.EvtVault, nil)
	if !migrationPending(scrubValues) {
		t.Fatal("the full pass ran while the vault was still locked")
	}
	open.Store(true)
	d.Broadcast(protocol.EvtVault, nil)
	deadline := time.Now().Add(2 * time.Second)
	for migrationPending(scrubValues) {
		if time.Now().After(deadline) {
			t.Fatal("the full pass never ran once the vault opened")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if text := eventsOnDisk(t, id); strings.Contains(text, storedPassword) || !strings.Contains(text, "Log in with ‹MAIL_PASSWORD› and push with "+redact.Mark) {
		t.Fatalf("the full pass: %s", text)
	}
	// a later start finds nothing owed
	if err := New(Deps{}).migrateScrubSessions(); err != nil {
		t.Fatal(err)
	}
}

// waitDone waits for the full scrub's marker.
func waitDone(t *testing.T, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for migrationPending(scrubValues) {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The scrub runs in the background: starting it returns at once, the pass
// finishes on its own, and the daemon's stop ends a pass it is in, which
// stays owed, and waits for it.
func TestTheStartupScrubRunsInTheBackground(t *testing.T) {
	isolateDaemon(t)
	release := make(chan struct{})
	t.Cleanup(redact.RegisterStore(func() func(string) string {
		<-release
		return func(text string) string { return text }
	}, nil))
	id := "ask-20261101-120000-ffff"
	oldSession(t, id, "Use "+printedToken)
	d := New(Deps{})
	started := make(chan struct{})
	go func() { d.scrubInBackground(); close(started) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("starting the scrub waited for the pass")
	}
	if !migrationPending(scrubValues) {
		t.Fatal("the pass finished before its store opened")
	}
	close(release)
	waitDone(t, "the background pass never finished")
	d.migrations.stop()
	if strings.Contains(eventsOnDisk(t, id), printedToken) {
		t.Fatal("the background pass left the token")
	}

	// a pass the daemon stops is owed again, and stop waits for it
	isolateDaemon(t)
	oldSession(t, id, "Use "+printedToken)
	release = make(chan struct{})
	stopping := New(Deps{})
	stopping.scrubInBackground()
	stopped := make(chan struct{})
	go func() { stopping.migrations.stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("stop returned while the pass was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-stopped
	if !migrationPending(scrubValues) || !migrationPending(scrubPatterns) {
		t.Fatal("a stopped pass was marked done")
	}
	if stopping.migrations.start(func() { t.Error("a pass started after stop") }) {
		t.Fatal("a stopped daemon took a new pass")
	}
}

// The pass after the vault opens skips a session deleted under it, takes
// up again one that changed under it, and when it fails, here because the
// vault locked again before it began, looks again instead of giving up for
// the daemon's run.
func TestTheUnlockScrubSkipsGoneSessionsRetriesAndRearms(t *testing.T) {
	isolateDaemon(t)
	retry, rearm := scrubRetry, scrubRearm
	scrubRetry, scrubRearm = 10*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { scrubRetry, scrubRearm = retry, rearm })
	gone, kept := "ask-20261101-120000-bbbb", "ask-20261101-120000-aaaa"
	oldSession(t, kept, "Sign in with "+storedPassword)
	oldSession(t, gone, "Log in with "+storedPassword)
	// 0: locked; 1: open for the look, locked for the pass; 2: open
	var phase, looks atomic.Int32
	ready := func() bool {
		switch phase.Load() {
		case 0:
			return false
		case 1:
			return looks.Add(1) == 1
		}
		return true
	}
	var deleted, touched atomic.Bool
	t.Cleanup(redact.RegisterStore(func() func(string) string {
		return func(text string) string {
			if phase.Load() != 2 {
				return text
			}
			// one session goes while the pass is on, and the other changes
			// once while the pass reads it
			if deleted.CompareAndSwap(false, true) {
				_ = os.RemoveAll(paths.SessionDir(gone))
			}
			if strings.Contains(text, "Sign in with") && touched.CompareAndSwap(false, true) {
				later := time.Now().Add(time.Minute)
				_ = os.Chtimes(filepath.Join(paths.SessionDir(kept), "meta.json"), later, later)
			}
			return strings.ReplaceAll(text, storedPassword, "‹MAIL_PASSWORD›")
		}
	}, ready))
	d := New(Deps{})
	t.Cleanup(d.migrations.stop)
	if err := d.migrateScrubSessions(); err != nil {
		t.Fatal(err)
	}
	phase.Store(1)
	d.Broadcast(protocol.EvtVault, nil)
	time.Sleep(50 * time.Millisecond)
	if !migrationPending(scrubValues) || !strings.Contains(eventsOnDisk(t, kept), storedPassword) {
		t.Fatal("a pass ran with the vault locked again")
	}
	phase.Store(2)
	deadline := time.Now().Add(3 * time.Second)
	for migrationPending(scrubValues) && time.Now().Before(deadline) {
		d.Broadcast(protocol.EvtVault, nil)
		time.Sleep(10 * time.Millisecond)
	}
	waitDone(t, "the failed pass never looked for the vault again")
	if !deleted.Load() || !touched.Load() {
		t.Fatal("the pass met neither a deleted nor a changed session")
	}
	if text := eventsOnDisk(t, kept); strings.Contains(text, storedPassword) || !strings.Contains(text, "‹MAIL_PASSWORD›") {
		t.Fatalf("the session that changed under the pass: %s", text)
	}
	if _, err := os.Stat(paths.SessionDir(gone)); !os.IsNotExist(err) {
		t.Fatalf("the deleted session came back: %v", err)
	}
}

// QA's case: a token typed in the prompt is scrubbed by its prefix, and
// the model's reasoning then restates its body without the prefix, split
// over four deltas with tool calls between them. Every piece leaves the
// log on disk, and the title too.
func TestATokensBodyRestatedWithoutItsPrefixIsScrubbed(t *testing.T) {
	body := "1a2B3c4D5e6F7g8H9i0J1k2L3m4N5o6P7q8R"
	typed := "ghp_" + body
	pieces := []string{body[:9], body[9:18], body[18:27], body[27:]}
	runner := &fakeRunner{prompt: func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvThought, protocol.TextData{Message: "r", Delta: "The user's token is "})
		for index, piece := range pieces {
			sink.Emit(protocol.EvThought, protocol.TextData{Message: "r", Delta: piece})
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: fmt.Sprint("c", index), Title: "look", Status: protocol.ToolDone})
		}
		sink.Emit(protocol.EvThought, protocol.TextData{Message: "r", Delta: ", I will use it."})
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "Done."})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "Push with " + typed}, nil); err != nil {
		t.Fatal(err)
	}
	waitTurnEnd(t, collector)
	text := eventsOnDisk(t, info.ID)
	thoughts := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, `"kind":"thought"`) {
			thoughts++
		}
	}
	if thoughts < 4 {
		t.Fatalf("the thought was written as %d events, not in pieces: %s", thoughts, text)
	}
	for _, secret := range append([]string{typed, body}, pieces...) {
		if strings.Contains(text, secret) {
			t.Fatalf("%q is on disk: %s", secret, text)
		}
	}
	for _, kept := range []string{"Push with " + redact.Mark, "The user's token is " + redact.Mark, ", I will use it.", "Done."} {
		if !strings.Contains(text, kept) {
			t.Fatalf("%q was lost: %s", kept, text)
		}
	}
	if got, _ := manager.Info(info.ID); strings.Contains(got.Title, body) || strings.Contains(got.Preview, body) {
		t.Fatalf("the title or preview keeps the token's body: %+v", got)
	}
}
