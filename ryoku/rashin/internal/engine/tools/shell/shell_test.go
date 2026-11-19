package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// script is what the fake executor does for one command.
type script struct {
	first []string // printed at once
	later []string // printed when the test lets it end
	code  int
	// hold keeps it running until finish or Kill.
	hold bool
}

type fakeProc struct {
	finish chan struct{}
	kill   chan struct{}
	done   chan struct{}
	once   sync.Once
	killed sync.Once

	mu   sync.Mutex
	out  strings.Builder
	exit exec.Exit
}

func (p *fakeProc) print(delta string, out exec.OutputFunc, id string) {
	p.mu.Lock()
	p.out.WriteString(delta)
	p.mu.Unlock()
	if out != nil {
		out(exec.Output{ID: id, Delta: delta})
	}
}

func (p *fakeProc) state(id string) exec.Exit {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		e := p.exit
		e.Output = p.out.String()
		return e
	default:
		return exec.Exit{ID: id, Running: true, Output: p.out.String()}
	}
}

type fakeExec struct {
	mu      sync.Mutex
	scripts map[string]script
	started []exec.StartRequest
	procs   map[string]*fakeProc
	killed  []string
	links   map[string]string
	next    int
}

func newFakeExec(scripts map[string]script) *fakeExec {
	return &fakeExec{scripts: scripts, procs: map[string]*fakeProc{}, links: map[string]string{}}
}

func (f *fakeExec) Start(_ context.Context, req exec.StartRequest, out exec.OutputFunc) (string, error) {
	f.mu.Lock()
	f.next++
	id := fmt.Sprintf("p%d", f.next)
	s := f.scripts[req.Argv[len(req.Argv)-1]]
	p := &fakeProc{finish: make(chan struct{}), kill: make(chan struct{}), done: make(chan struct{})}
	f.procs[id] = p
	f.started = append(f.started, req)
	f.mu.Unlock()
	go func() {
		for _, d := range s.first {
			p.print(d, out, id)
		}
		exit := exec.Exit{ID: id, Code: s.code}
		if s.hold {
			select {
			case <-p.finish:
			case <-p.kill:
				exit = exec.Exit{ID: id, Code: -1, Signal: "TERM", Killed: true}
			}
		}
		if !exit.Killed {
			for _, d := range s.later {
				p.print(d, out, id)
			}
		}
		p.mu.Lock()
		p.exit = exit
		p.mu.Unlock()
		close(p.done)
	}()
	return id, nil
}

func (f *fakeExec) proc(id string) (*fakeProc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.procs[id]
	if !ok {
		return nil, exec.ErrNoProcess
	}
	return p, nil
}

// finishAll lets every held command end.
func (f *fakeExec) finishAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.procs {
		p.once.Do(func() { close(p.finish) })
	}
}

func (f *fakeExec) Wait(ctx context.Context, id string) (exec.Exit, error) {
	p, err := f.proc(id)
	if err != nil {
		return exec.Exit{}, err
	}
	select {
	case <-p.done:
		return p.state(id), nil
	case <-ctx.Done():
		return exec.Exit{}, ctx.Err()
	}
}

func (f *fakeExec) Peek(_ context.Context, id string) (exec.Exit, error) {
	p, err := f.proc(id)
	if err != nil {
		return exec.Exit{}, err
	}
	return p.state(id), nil
}

func (f *fakeExec) Kill(_ context.Context, id string) error {
	p, err := f.proc(id)
	if err != nil {
		return nil
	}
	f.mu.Lock()
	f.killed = append(f.killed, id)
	f.mu.Unlock()
	p.killed.Do(func() { close(p.kill) })
	return nil
}

func (f *fakeExec) Release(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.procs[id]; ok {
		p.killed.Do(func() { close(p.kill) })
	}
	delete(f.procs, id)
	return nil
}

func (f *fakeExec) Running() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.procs))
	for id := range f.procs {
		ids = append(ids, id)
	}
	return ids
}

func (f *fakeExec) startedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.started)
}

func (f *fakeExec) Stat(_ context.Context, path string) (exec.FileInfo, error) {
	if target, ok := f.links[path]; ok {
		return exec.FileInfo{Path: path, Kind: exec.KindSymlink, Target: target}, nil
	}
	return exec.FileInfo{Path: path, Kind: exec.KindDir}, nil
}
func (f *fakeExec) Read(context.Context, exec.ReadRequest) (exec.ReadResult, error) {
	return exec.ReadResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Write(context.Context, exec.WriteRequest) (exec.WriteResult, error) {
	return exec.WriteResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Edit(context.Context, exec.EditRequest) (exec.EditResult, error) {
	return exec.EditResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Remove(context.Context, exec.RemoveRequest) error {
	return exec.ErrUnavailable
}
func (f *fakeExec) List(context.Context, exec.ListRequest) (exec.ListResult, error) {
	return exec.ListResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Glob(context.Context, exec.GlobRequest) (exec.GlobResult, error) {
	return exec.GlobResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Grep(context.Context, exec.GrepRequest) (exec.GrepResult, error) {
	return exec.GrepResult{}, exec.ErrUnavailable
}
func (f *fakeExec) Close() error { return nil }

type fakePerms struct {
	mu       sync.Mutex
	refuse   bool
	requests []tool.Request
}

func (p *fakePerms) Allow(_ context.Context, req tool.Request) (tool.Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if p.refuse {
		return tool.Decision{By: "user"}, nil
	}
	return tool.Decision{Allowed: true, By: "policy"}, nil
}

func (p *fakePerms) Choose(context.Context, tool.Question) (string, error) { return "", nil }

type event struct {
	kind string
	data any
}

type fakeSink struct {
	mu     sync.Mutex
	events []event
}

func (s *fakeSink) Emit(kind string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event{kind, data})
}

func (s *fakeSink) Update(func(*protocol.SessionInfo)) {}

// terminal is what one terminal showed, and how it closed.
func (s *fakeSink) terminal(id string) (shown string, opened, done bool, exit *int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		switch d := e.data.(type) {
		case protocol.ToolData:
			if d.Terminal == id {
				opened = true
			}
		case protocol.TerminalData:
			if d.Terminal != id {
				continue
			}
			shown += d.Delta
			if d.Done {
				done, exit = true, d.Exit
			}
		}
	}
	return shown, opened, done, exit
}

type rig struct {
	x     *fakeExec
	perms *fakePerms
	sink  *fakeSink
	env   tool.Env
	tools map[string]tool.Tool
}

func newRig(mode tool.Mode, class tool.Class, readOnly bool, scripts map[string]script) *rig {
	r := &rig{x: newFakeExec(scripts), perms: &fakePerms{}, sink: &fakeSink{}, tools: map[string]tool.Tool{}}
	profile := tool.Profile{Mode: mode, Class: class, ReadOnly: readOnly, Window: 32768}
	r.env = tool.Env{
		Session:     protocol.SessionInfo{ID: "s1"},
		Cwd:         "/work",
		Profile:     profile,
		Limits:      profile.Limits(),
		Exec:        r.x,
		Sink:        r.sink,
		Emit:        func(d protocol.ToolData) { r.sink.Emit(protocol.EvTool, d) },
		Permissions: r.perms,
	}
	for _, t := range Tools() {
		r.tools[t.Name()] = t
	}
	return r
}

func (r *rig) call(t *testing.T, name, id string, args map[string]any) tool.Result {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return r.tools[name].Run(context.Background(), tool.Call{ID: id, Name: name, Args: raw}, r.env)
}

func (r *rig) tiers() []string {
	r.perms.mu.Lock()
	defer r.perms.mu.Unlock()
	var tiers []string
	for _, req := range r.perms.requests {
		tiers = append(tiers, req.Tier)
	}
	return tiers
}

func shorten(t *testing.T, watch, background time.Duration) {
	t.Helper()
	oldWatch, oldBackground := watchAfter, backgroundAfter
	watchAfter, backgroundAfter = watch, background
	t.Cleanup(func() { watchAfter, backgroundAfter = oldWatch, oldBackground })
}

// A command that only reads runs at once in every mode; a change goes to the
// gate with the classifier's tier, and one the classifier cannot read
// through is never put there as a read.
func TestAskCommandsReachTheGateWithTheirTier(t *testing.T) {
	cases := []struct {
		tool, command string
		tier          string // "" = never asked
	}{
		{tool.RunCommand, "printf seen", ""},
		{tool.RunCommand, "touch /tmp/made", protocol.TierWrite},
		{tool.RunCommand, "pacman -S foo", protocol.TierSystem},
		{tool.RunCommand, "touch /tmp/made $(true)", protocol.TierDanger},
		{tool.RunCommand, "rm -rf /usr", protocol.TierDanger},
		{tool.SystemQuery, "printf seen", ""},
		{tool.SystemQuery, "touch /tmp/made", protocol.TierWrite},
	}
	for _, c := range cases {
		r := newRig(tool.ModeAskAct, tool.ClassFull, false, nil)
		res := r.call(t, c.tool, "c1", map[string]any{"command": c.command, "why": "do it"})
		if res.IsError {
			t.Errorf("%s %q failed: %s", c.tool, c.command, res.Text)
		}
		tiers := r.tiers()
		switch {
		case c.tier == "" && len(tiers) != 0:
			t.Errorf("%s %q was put to the user as %v", c.tool, c.command, tiers)
		case c.tier != "" && (len(tiers) != 1 || tiers[0] != c.tier):
			t.Errorf("%s %q reached the gate as %v, want %s", c.tool, c.command, tiers, c.tier)
		}
		if r.x.startedCount() != 1 {
			t.Errorf("%s %q started %d processes", c.tool, c.command, r.x.startedCount())
		}
	}
}

func TestBashPresentsClassifierTierToPermissions(t *testing.T) {
	r := newRig(tool.ModeAgent, tool.ClassFull, false, nil)
	r.call(t, tool.Bash, "read", map[string]any{"command": "printf safe"})
	r.call(t, tool.Bash, "danger", map[string]any{"command": "rm -rf /usr"})
	if tiers := r.tiers(); len(tiers) != 2 || tiers[0] != protocol.TierRead || tiers[1] != protocol.TierDanger {
		t.Fatalf("bash tiers were %v", tiers)
	}
}

// What cannot run in the conversation never starts and is never asked
// about: a password prompt nobody sees, or a credential whose bytes would
// reach the model. In Ask it becomes a proposal the user runs themselves.
func TestCommandsThatCannotRunHereAreRefused(t *testing.T) {
	cases := []struct {
		mode    tool.Mode
		name    string
		command string
		propose bool
	}{
		{tool.ModeAskAct, tool.RunCommand, "sudo pacman -Syu", true},
		{tool.ModeAskAct, tool.RunCommand, "cat ~/.ssh/id_ed25519", true},
		{tool.ModeAskAct, tool.SystemQuery, "cat ~/.ssh/id_ed25519 > /dev/null && cat ~/.ssh/id_ed25519", true},
		{tool.ModeAskAct, tool.SystemQuery, "doas reboot", true},
		{tool.ModeAskRead, tool.SystemQuery, "cat ~/.ssh/id_ed25519", true},
		{tool.ModeAgent, tool.Bash, "sudo make install", false},
		{tool.ModeAgent, tool.Bash, "cat ~/.aws/credentials", false},
	}
	for _, c := range cases {
		r := newRig(c.mode, tool.ClassFull, c.mode == tool.ModeAskRead, nil)
		res := r.call(t, c.name, "c1", map[string]any{"command": c.command, "why": "look"})
		if !res.IsError || !strings.Contains(res.Text, "Refused") {
			t.Errorf("%s %q was answered %q", c.name, c.command, res.Text)
		}
		if r.x.startedCount() != 0 || len(r.tiers()) != 0 {
			t.Errorf("%s %q started or asked", c.name, c.command)
		}
		switch {
		case c.propose && (res.Proposal == nil || len(res.Proposal.Commands) != 1 || !res.Proposal.Commands[0].Terminal || res.Proposal.Commands[0].Command != c.command):
			t.Errorf("%s %q proposed %#v", c.name, c.command, res.Proposal)
		case !c.propose && res.Proposal != nil:
			t.Errorf("%s %q proposed in a mode without plans", c.name, c.command)
		}
	}
}

// A read-only Ask runs nothing that changes the machine, and says to
// propose instead.
func TestAReadOnlyAskRefusesChanges(t *testing.T) {
	r := newRig(tool.ModeAskRead, tool.ClassFull, true, nil)
	query := r.call(t, tool.SystemQuery, "c1", map[string]any{"command": "touch /tmp/made"})
	if !query.IsError || !strings.Contains(query.Text, "Use propose") {
		t.Fatalf("the query was answered %q", query.Text)
	}
	run := r.call(t, tool.RunCommand, "c2", map[string]any{"command": "touch /tmp/made", "why": "make"})
	if !run.IsError || !strings.Contains(run.Text, "read-only") {
		t.Fatalf("run_command was answered %q", run.Text)
	}
	if r.x.startedCount() != 0 || len(r.tiers()) != 0 {
		t.Fatal("a read-only Ask started or asked about a change")
	}
}

// A refusal is told to the model as final and nothing runs.
func TestARefusedChangeDoesNotRun(t *testing.T) {
	r := newRig(tool.ModeAskAct, tool.ClassFull, false, nil)
	r.perms.refuse = true
	res := r.call(t, tool.RunCommand, "c1", map[string]any{"command": "touch /tmp/made", "why": "make"})
	if !res.IsError || res.Text != refusedToTool || res.Shown == "" {
		t.Fatalf("the refusal read %#v", res)
	}
	if r.x.startedCount() != 0 {
		t.Fatal("a refused command started")
	}
}

// Ask is told what a command printed, trimmed, and a failing exit marks
// the call failed with its status.
func TestAskIsToldWhatTheCommandPrinted(t *testing.T) {
	r := newRig(tool.ModeAskAct, tool.ClassFull, false, map[string]script{
		"touch /tmp/a && echo made": {first: []string{"made\n"}},
		"false":                     {code: 3},
	})
	if res := r.call(t, tool.RunCommand, "c1", map[string]any{"command": "touch /tmp/a && echo made", "why": "a"}); res.IsError || res.Text != "made" {
		t.Fatalf("the model was told %#v", res)
	}
	if res := r.call(t, tool.RunCommand, "c2", map[string]any{"command": "false", "why": "b"}); !res.IsError || !strings.Contains(res.Text, "exit status 3") {
		t.Fatalf("a failing command was told %#v", res)
	}
}

// bash takes the timeout in seconds, 300 by default, between 1 and 3600,
// and the names small models send.
func TestBashTimeoutDefaultsAndBounds(t *testing.T) {
	r := newRig(tool.ModeAgent, tool.ClassCompact, false, nil)
	r.call(t, tool.Bash, "c1", map[string]any{"cmd": "make"})
	r.call(t, tool.Bash, "c2", map[string]any{"command": "make", "timeout": 99999})
	r.call(t, tool.Bash, "c3", map[string]any{"command": "make", "timeout": "20"})
	r.call(t, tool.Bash, "c4", map[string]any{"command": "make", "timeout": 0})
	r.call(t, tool.Bash, "c5", map[string]any{"command": "make", "timeout": -10})
	want := []time.Duration{300 * time.Second, 3600 * time.Second, 20 * time.Second, time.Second, time.Second}
	if len(r.x.started) != len(want) {
		t.Fatalf("%d commands started", len(r.x.started))
	}
	for i, req := range r.x.started {
		if req.Timeout != want[i] || req.Argv[len(req.Argv)-1] != "make" || req.Dir != "/work" {
			t.Errorf("call %d started %#v", i+1, req)
		}
	}
}

// A long bash call moves to a background job; wait reads it once it ends
// and then forgets it, and stop ends a job.
func TestABackgroundJobIsWaitedForAndStopped(t *testing.T) {
	shorten(t, time.Hour, 50*time.Millisecond)
	r := newRig(tool.ModeAgent, tool.ClassFull, false, map[string]script{
		"make all":  {first: []string{"building\n"}, later: []string{"built\n"}, hold: true},
		"serve dev": {first: []string{"listening\n"}, hold: true},
	})
	res := r.call(t, tool.Bash, "c1", map[string]any{"command": "make all"})
	if res.IsError || !strings.Contains(res.Text, "bg_1") || !strings.Contains(res.Text, "building") {
		t.Fatalf("the long call was answered %q", res.Text)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		r.x.finishAll()
	}()
	waited := r.call(t, tool.Wait, "c2", map[string]any{"id": "bg_1"})
	if waited.IsError || !strings.Contains(waited.Text, "bg_1 ended with exit code 0") || !strings.Contains(waited.Text, "built") {
		t.Fatalf("wait returned %q", waited.Text)
	}
	if again := r.call(t, tool.Wait, "c3", nil); !strings.Contains(again.Text, "Nothing to wait for") {
		t.Fatalf("a collected job was still waited for: %q", again.Text)
	}

	if res := r.call(t, tool.Bash, "c4", map[string]any{"command": "serve dev", "async": true}); !strings.Contains(res.Text, "bg_2") {
		t.Fatalf("an async call was answered %q", res.Text)
	}
	stopped := r.call(t, tool.Wait, "c5", map[string]any{"id": "bg_2", "stop": true})
	if !strings.Contains(stopped.Text, "bg_2 was stopped") {
		t.Fatalf("stopping returned %q", stopped.Text)
	}
	if len(r.x.killed) == 0 || len(r.x.Running()) != 0 {
		t.Fatalf("killed %v, still running %v", r.x.killed, r.x.Running())
	}
	if unknown := r.call(t, tool.Wait, "c6", map[string]any{"id": "bg_9"}); !strings.Contains(unknown.Text, "Nothing to wait for") {
		t.Fatalf("an unknown job read %q", unknown.Text)
	}
}

func TestEveryAsyncBashCallHasAWaitableJob(t *testing.T) {
	r := newRig(tool.ModeAgent, tool.ClassFull, false, map[string]script{"serve": {hold: true}})
	for i := 1; i <= 6; i++ {
		res := r.call(t, tool.Bash, fmt.Sprintf("c%d", i), map[string]any{"command": "serve", "async": true})
		if res.IsError || !strings.Contains(res.Text, fmt.Sprintf("bg_%d", i)) {
			t.Fatalf("async call %d returned %q", i, res.Text)
		}
	}
	if r.x.startedCount() != 6 {
		t.Fatalf("only %d async processes started", r.x.startedCount())
	}
	r.call(t, tool.Wait, "stop", map[string]any{"stop": true})
	if len(r.x.Running()) != 0 {
		t.Fatalf("stopped jobs left processes: %v", r.x.Running())
	}
}

func TestWaitReportsACommandFailure(t *testing.T) {
	r := newRig(tool.ModeAgent, tool.ClassFull, false, map[string]script{"false": {hold: true, code: 7}})
	r.call(t, tool.Bash, "c1", map[string]any{"command": "false", "async": true})
	r.x.finishAll()
	res := r.call(t, tool.Wait, "c2", map[string]any{"id": "bg_1"})
	if !res.IsError || !strings.Contains(res.Text, "exit code 7") {
		t.Fatalf("a failed job read %#v", res)
	}
}

// A compact profile has no wait, so its bash never leaves a job behind.
func TestCompactBashStaysInTheForeground(t *testing.T) {
	shorten(t, time.Hour, 20*time.Millisecond)
	r := newRig(tool.ModeAgent, tool.ClassCompact, false, map[string]script{
		"make": {first: []string{"a\n"}, later: []string{"b\n"}, code: 2, hold: true},
	})
	go func() {
		time.Sleep(150 * time.Millisecond)
		r.x.finishAll()
	}()
	res := r.call(t, tool.Bash, "c1", map[string]any{"command": "make", "async": true})
	if !res.IsError || !strings.Contains(res.Text, "a\nb") || !strings.Contains(res.Text, "exited with code 2") || strings.Contains(res.Text, "bg_") {
		t.Fatalf("the compact call was answered %q", res.Text)
	}
}

// A quick command stays a plain result; one that runs a while becomes a
// terminal of its call and closes with its exit status.
func TestOutputBecomesATerminalOnlyOnceItTakesAWhile(t *testing.T) {
	shorten(t, 30*time.Millisecond, time.Hour)
	r := newRig(tool.ModeAskAct, tool.ClassFull, false, map[string]script{
		"printf seen":                     {first: []string{"seen"}},
		"printf early; sleep 1; printf l": {first: []string{"early"}, later: []string{"late"}, hold: true},
	})
	r.call(t, tool.SystemQuery, "quick", map[string]any{"command": "printf seen"})
	if _, opened, _, _ := r.sink.terminal("quick"); opened {
		t.Fatal("a quick look was given a terminal")
	}
	go func() {
		time.Sleep(120 * time.Millisecond)
		r.x.finishAll()
	}()
	res := r.call(t, tool.SystemQuery, "slow", map[string]any{"command": "printf early; sleep 1; printf l"})
	shown, opened, done, exit := r.sink.terminal("slow")
	if !opened || shown != "earlylate" || !done || exit == nil || *exit != 0 {
		t.Fatalf("the slow command showed %q opened %v done %v exit %v", shown, opened, done, exit)
	}
	if res.Text != "earlylate" {
		t.Fatalf("the model was told %q", res.Text)
	}
}

// A character the command wrote in two pieces arrives whole.
func TestACharacterSplitAcrossWritesIsShownWhole(t *testing.T) {
	sink := &fakeSink{}
	w := newWatcher(sink, "c", nil)
	w.open()
	whole := "größe"
	w.write(whole[:3])
	w.write(whole[3:])
	w.end(nil, "")
	shown, _, _, _ := sink.terminal("c")
	if shown != whole {
		t.Fatalf("shown as %q", shown)
	}
}

func TestTerminalByteCapNeverShowsPartOfACharacter(t *testing.T) {
	sink := &fakeSink{}
	w := newWatcher(sink, "c", nil)
	w.open()
	w.write(strings.Repeat("a", watchLimit-1) + "é")
	w.end(nil, "")
	shown, _, _, _ := sink.terminal("c")
	if len(shown) != watchLimit-1 || strings.ContainsRune(shown, '�') {
		t.Fatalf("the capped terminal ended with %q", shown[len(shown)-min(len(shown), 8):])
	}
}

// A run the user started needs no keyboard; one that does is refused
// before anything starts, and stopping one kills it and tells the model
// nothing.
func TestAUserRun(t *testing.T) {
	x := newFakeExec(map[string]script{
		"du -sh .": {first: []string{"4.0G\t/home/me\n"}},
		"sleep 30": {hold: true},
	})
	sink := &fakeSink{}
	if _, _, err := UserRun(context.Background(), x, sink, 2, "sudo true", "/home/me"); err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Fatalf("a run that needs a password was answered %v", err)
	}
	if x.startedCount() != 0 {
		t.Fatal("a run that needs a terminal started")
	}
	told, ok, err := UserRun(context.Background(), x, sink, 3, "du -sh .", "/home/me")
	if err != nil || !ok || !strings.Contains(told, "exit status 0") || !strings.Contains(told, "4.0G") {
		t.Fatalf("the run told %q %v %v", told, ok, err)
	}
	if _, opened, done, _ := sink.terminal("run-3"); !opened || !done {
		t.Fatal("a user run was not a terminal from the start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if told, ok, err := UserRun(ctx, x, sink, 4, "sleep 30", "/home/me"); ok || err != nil || told != "" {
		t.Fatalf("a stopped run told %q %v %v", told, ok, err)
	}
	if len(x.killed) == 0 || len(x.Running()) != 0 {
		t.Fatalf("the stopped run was not killed: %v %v", x.killed, x.Running())
	}
}

func TestBashChecksCwdAndResolvedCommandPaths(t *testing.T) {
	r := newRig(tool.ModeAgent, tool.ClassFull, false, nil)
	for _, cwd := range []string{"/home/test/.ssh", "/work/secret"} {
		r.x.links["/work/secret"] = "/home/test/.ssh"
		got := r.call(t, tool.Bash, "cwd", map[string]any{"command": "cat *", "cwd": cwd})
		if !got.IsError || !strings.Contains(got.Text, "working directory") || r.x.startedCount() != 0 {
			t.Fatalf("cwd %s escaped secret gate: %#v", cwd, got)
		}
	}
	r.x.links["/work/parent"] = "/home/test/.ssh"
	intermediate := r.call(t, tool.Bash, "cwd-intermediate", map[string]any{"command": "cat *", "cwd": "parent/keys"})
	if !intermediate.IsError || !strings.Contains(intermediate.Text, "working directory") || r.x.startedCount() != 0 {
		t.Fatalf("intermediate symlink escaped secret gate: %#v", intermediate)
	}
	r.x.links["/work/public"] = "/work/notes"
	got := r.call(t, tool.Bash, "linked", map[string]any{"command": "printf safe", "cwd": "public"})
	if got.IsError || r.x.startedCount() != 1 || r.x.started[0].Dir != "/work/notes" {
		t.Fatalf("safe symlink cwd was not followed: %#v started=%#v", got, r.x.started)
	}
}

func TestEnvironmentDumpCommandsAreRefusedBeforeExecution(t *testing.T) {
	for _, command := range []string{"env", "printenv", "export -p", "set", "declare -p"} {
		r := newRig(tool.ModeAgent, tool.ClassFull, false, nil)
		got := r.call(t, tool.Bash, "dump", map[string]any{"command": command})
		if !got.IsError || r.x.startedCount() != 0 || len(r.tiers()) != 0 {
			t.Errorf("%q escaped secret gate: %#v", command, got)
		}
	}
}

func TestUnboundedReadCommandsNeedDangerApproval(t *testing.T) {
	for _, command := range []string{"dmesg -w", "cat /dev/zero", "cat /dev/urandom"} {
		r := newRig(tool.ModeAgent, tool.ClassFull, false, nil)
		r.call(t, tool.Bash, "unbounded", map[string]any{"command": command})
		if tiers := r.tiers(); len(tiers) != 1 || tiers[0] != protocol.TierDanger {
			t.Errorf("%q reached the gate as %v, want danger", command, tiers)
		}
	}
}
