package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// fixedKeeper keeps the vault's key in memory in place of the login keyring.
type fixedKeeper struct{ key []byte }

func (k fixedKeeper) Key(context.Context, bool, bool) ([]byte, error) {
	return bytes.Clone(k.key), nil
}

// fakeClock is the service's clock. It moves a millisecond on every read, so
// things made one after another keep their order, and jumps when told.
type fakeClock struct {
	mu sync.Mutex
	at time.Time
	// hook runs once at the next read, outside the lock
	hook func()
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	c.at = c.at.Add(time.Millisecond)
	at, hook := c.at, c.hook
	c.hook = nil
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	return at
}

// beforeNextRead runs f once when the clock is next read, before the read
// returns, so a test can act at an exact point of the service's work.
func (c *fakeClock) beforeNextRead(f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hook = f
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func (c *fakeClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

// fakeSessions stands in for the session manager: it records what Teams asks
// of each session and keeps each session's goal.
type fakeSessions struct {
	mu        sync.Mutex
	count     int
	infos     map[string]protocol.SessionInfo
	made      []protocol.SessionNewParams
	started   []string
	goals     []protocol.GoalParams
	continued []continued
	stopped   []string
}

// continued is one note a session was continued with.
type continued struct{ session, text string }

func newFakeSessions() *fakeSessions {
	return &fakeSessions{infos: make(map[string]protocol.SessionInfo)}
}

func (f *fakeSessions) Info(id string) (protocol.SessionInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info, ok := f.infos[id]
	return info, ok
}

func (f *fakeSessions) List(protocol.SessionKind) []protocol.SessionInfo { return nil }

func (f *fakeSessions) New(_ context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	info := protocol.SessionInfo{
		ID: fmt.Sprintf("session-%d", f.count), Kind: p.Kind, Cwd: p.Cwd, Origin: p.Origin, State: protocol.StateIdle,
		Team: p.Team, TeamTask: p.TeamTask, Member: p.Member, Approvals: p.Approvals,
	}
	f.infos[info.ID] = info
	f.made = append(f.made, p)
	return info, nil
}

func (f *fakeSessions) Prompt(context.Context, protocol.PromptParams) (protocol.PromptResult, error) {
	return protocol.PromptResult{}, nil
}

func (f *fakeSessions) Start(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, id)
	return nil
}

func (f *fakeSessions) SetModel(context.Context, string, string) error { return nil }

func (f *fakeSessions) Goal(_ context.Context, p protocol.GoalParams) (protocol.SessionInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.goals = append(f.goals, p)
	info := f.infos[p.ID]
	status := ""
	switch p.Op {
	case protocol.GoalSet:
		info.Goal = &protocol.Goal{Objective: p.Objective, Status: protocol.GoalActive, MaxTurns: p.MaxTurns}
	case protocol.GoalComplete:
		status = protocol.GoalDone
	case protocol.GoalStop:
		status = protocol.GoalStopped
	}
	if status != "" && info.Goal != nil {
		goal := *info.Goal
		goal.Status = status
		info.Goal = &goal
	}
	f.infos[p.ID] = info
	return info, nil
}

func (f *fakeSessions) Continue(_ context.Context, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.continued = append(f.continued, continued{session: id, text: text})
	return nil
}

func (f *fakeSessions) Cancel(context.Context, string) error { return nil }

func (f *fakeSessions) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
	info := f.infos[id]
	info.State = protocol.StateStopped
	f.infos[id] = info
	return nil
}

// Watched keeps a window open, so no desktop notification is posted.
func (f *fakeSessions) Watched() bool { return true }

func (f *fakeSessions) info(id string) protocol.SessionInfo {
	info, _ := f.Info(id)
	return info
}

// madeFor is how the session of a task was made.
func (f *fakeSessions) madeFor(task string) (protocol.SessionNewParams, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.made {
		if p.TeamTask == task {
			return p, true
		}
	}
	return protocol.SessionNewParams{}, false
}

func (f *fakeSessions) wasStarted(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, started := range f.started {
		if started == id {
			return true
		}
	}
	return false
}

func (f *fakeSessions) wasStopped(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, stopped := range f.stopped {
		if stopped == id {
			return true
		}
	}
	return false
}

// goal is the first goal change of op a session got.
func (f *fakeSessions) goal(id, op string) (protocol.GoalParams, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.goals {
		if p.ID == id && p.Op == op {
			return p, true
		}
	}
	return protocol.GoalParams{}, false
}

// notes lists what a session was continued with.
func (f *fakeSessions) notes(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.continued {
		if c.session == id {
			out = append(out, c.text)
		}
	}
	return out
}

func (f *fakeSessions) allNotes() []continued {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]continued(nil), f.continued...)
}

// wakeRecorder stands in for the unit that wakes Rashin for its routines.
type wakeRecorder struct {
	mu    sync.Mutex
	calls []bool
}

func (w *wakeRecorder) set(on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, on)
}

func (w *wakeRecorder) seen() []bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]bool(nil), w.calls...)
}

// broadcasts keeps what the service told its clients.
type broadcasts struct {
	mu   sync.Mutex
	sent []sent
}

type sent struct {
	event string
	data  any
}

func (b *broadcasts) Broadcast(event string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent = append(b.sent, sent{event, data})
}

// lastTeam is the newest team broadcast for id.
func (b *broadcasts) lastTeam(id string) (protocol.Team, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := len(b.sent) - 1; i >= 0; i-- {
		if team, ok := b.sent[i].data.(protocol.Team); ok && b.sent[i].event == protocol.EvtTeam && team.ID == id {
			return team, true
		}
	}
	return protocol.Team{}, false
}

// harness is a bound service on temporary state with a fake clock, fake
// sessions, a vault kept in memory, a recorded wake unit and what it broadcast.
type harness struct {
	s        *Service
	dir      string
	sessions *fakeSessions
	clock    *fakeClock
	wake     *wakeRecorder
	events   *broadcasts
}

// newHarness binds a service; maxRunning 0 keeps the default.
func newHarness(t *testing.T, maxRunning int) *harness {
	t.Helper()
	h := &harness{
		dir:      t.TempDir(),
		sessions: newFakeSessions(),
		clock:    &fakeClock{at: time.Date(2026, time.November, 11, 10, 0, 0, 0, time.Local)},
		wake:     &wakeRecorder{},
		events:   &broadcasts{},
	}
	keys := fixedKeeper{key: bytes.Repeat([]byte{7}, 32)}
	v := vault.OpenWith(filepath.Join(t.TempDir(), "vault.db"), keys)
	settings := func() protocol.Settings {
		return protocol.Settings{Teams: protocol.TeamsSettings{MaxRunning: maxRunning}}
	}
	s, err := open(h.dir, v, h.wake.set, h.events, settings)
	if err != nil {
		t.Fatal(err)
	}
	s.now = h.clock.now
	t.Cleanup(func() { _ = s.Close() })
	h.s = s
	s.Bind(h.sessions)
	waitFor(t, "the clock to set the wake unit", func() bool { return len(h.wake.seen()) > 0 })
	return h
}

func (h *harness) call(method string, params any) (any, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return h.s.Handle(context.Background(), method, raw)
}

// do calls a method that must succeed with a result of type T.
func do[T any](t *testing.T, h *harness, method string, params any) T {
	t.Helper()
	out, err := h.call(method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	result, ok := out.(T)
	if !ok {
		t.Fatalf("%s returned %T", method, out)
	}
	return result
}

// refused calls a method that must fail with code.
func refused(t *testing.T, h *harness, method string, params any, code string) *protocol.Error {
	t.Helper()
	_, err := h.call(method, params)
	var failure *protocol.Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("%s: got %v, want a %s error", method, err, code)
	}
	return failure
}

// waitFor waits for what the service finishes on its own goroutines.
func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// newTeam creates a team of a lead and one member working in a folder of
// the test's own.
func (h *harness) newTeam(t *testing.T, name string, edit func(*protocol.TeamCreateParams)) protocol.Team {
	t.Helper()
	p := protocol.TeamCreateParams{
		Name: name, Folder: filepath.Join(h.dir, "folders", slugName(name)),
		Members: []protocol.Member{{Name: "Ana", Lead: true, Harness: "fake"}, {Name: "Ben", Harness: "fake"}},
	}
	if edit != nil {
		edit(&p)
	}
	return do[protocol.Team](t, h, protocol.MTeamsCreate, p)
}

func (h *harness) newTask(t *testing.T, team protocol.Team, brief string) protocol.TeamTask {
	t.Helper()
	return do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: brief})
}

func (h *harness) task(t *testing.T, id string) protocol.TeamTask {
	t.Helper()
	task, ok := h.s.state.task(id)
	if !ok {
		t.Fatalf("there is no task %s", id)
	}
	return task
}

// agent waits until a task's latest session has started work, and returns
// what that session's MCP server calls with.
func (h *harness) agent(t *testing.T, id string) protocol.AgentRef {
	t.Helper()
	var ref protocol.AgentRef
	waitFor(t, "the task to start", func() bool {
		task := h.task(t, id)
		if task.Session == "" || h.ledgerCount(t, "task.start", id) != len(task.Sessions) {
			return false
		}
		h.s.mu.Lock()
		defer h.s.mu.Unlock()
		r := h.s.runs[id]
		if r == nil || r.Session != task.Session {
			return false
		}
		ref = protocol.AgentRef{Session: r.Session, Token: r.Token}
		return true
	})
	return ref
}

// pending is the one approval that waits on the user.
func (h *harness) pending(t *testing.T) protocol.Approval {
	t.Helper()
	pending := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals
	if len(pending) != 1 {
		t.Fatalf("%d approvals wait, want 1", len(pending))
	}
	return pending[0]
}

func (h *harness) ledger(t *testing.T) []protocol.Activity {
	t.Helper()
	result, err := h.s.ledger.list(protocol.TeamActivityParams{Limit: ledgerKeep})
	if err != nil {
		t.Fatal(err)
	}
	return result.Activity
}

// ledgerCount counts the ledger's entries of action, for one task or, with
// task empty, for any.
func (h *harness) ledgerCount(t *testing.T, action, task string) int {
	t.Helper()
	count := 0
	for _, a := range h.ledger(t) {
		if a.Action == action && (task == "" || a.Task == task) {
			count++
		}
	}
	return count
}

// noSecret fails when value shows anywhere an agent or the record could read
// it: the results given, what sessions were told, the ledger, the vault's
// record and every file of the teams' state.
func (h *harness) noSecret(t *testing.T, value string, seen ...any) {
	t.Helper()
	check := func(where, text string) {
		if strings.Contains(text, value) {
			t.Errorf("the value shows in %s", where)
		}
	}
	for i, result := range seen {
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		check(fmt.Sprintf("result %d", i), string(raw))
	}
	for _, note := range h.sessions.allNotes() {
		check("a note to "+note.session, note.text)
	}
	raw, err := json.Marshal(h.ledger(t))
	if err != nil {
		t.Fatal(err)
	}
	check("the ledger", string(raw))
	events, err := h.s.vault.Events(protocol.VaultEventsParams{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if raw, err = json.Marshal(events); err != nil {
		t.Fatal(err)
	}
	check("the vault's record", string(raw))
	err = filepath.WalkDir(h.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		check(path, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
