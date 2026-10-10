package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// An in-flight `ryoku update` publishes its progress to a run-state file the
// Hub's Updates page watches ($XDG_RUNTIME_DIR/ryoku-update.json). The file
// carries the ordered steps (with when each began and how long it took), the
// current label, a rolling narrative, the live line the running subprocess
// last printed, the run's own watchdog verdict, and, on failure, the error and
// the pre-update snapshot id. Every write is atomic (temp + rename), so a
// watcher never reads a half-written file, and best-effort, so a write failure
// never blocks the update.
//
// Only a run that has begun writes the file. Code that narrates outside an
// update (`ryoku track`, tests driving the channel helpers) prints and nothing
// more: a "running" document with no live owner is exactly what pinned the Hub
// on a phantom run it could never leave.

const runLogCap = 12

type stepState string

const (
	stepPending stepState = "pending"
	stepRunning stepState = "running"
	stepDone    stepState = "ok"
	stepFailed  stepState = "failed"
	stepSkipped stepState = "skipped"
)

type runStep struct {
	Key   string    `json:"key"`
	Label string    `json:"label"`
	State stepState `json:"state"`
	Began int64     `json:"began,omitempty"` // unix ms the step started running
	Took  int64     `json:"took,omitempty"`  // ms it ran, once settled
}

// promptSpec is a question the run is blocked on. Kind "secret" is the
// password the Hub collects for sudo; its answer travels over a FIFO
// (see gui.go), never through this file.
type promptSpec struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind,omitempty"` // choice (default) | secret
	Title   string   `json:"title"`
	Detail  string   `json:"detail"`
	Options []string `json:"options,omitempty"`
	Error   string   `json:"error,omitempty"` // why the previous answer was refused
}

// runState is the JSON document the GUI reads.
type runState struct {
	Phase              string      `json:"phase"` // idle | running | prompt | auth | done | error
	PID                int         `json:"pid,omitempty"`
	UI                 string      `json:"ui,omitempty"`      // hub | terminal
	Started            int64       `json:"started,omitempty"` // unix ms the run began
	Beat               int64       `json:"beat,omitempty"`    // unix ms of this write
	Step               string      `json:"step"`
	Label              string      `json:"label"`
	Progress           float64     `json:"progress"`
	Steps              []runStep   `json:"steps,omitempty"`
	Log                []string    `json:"log,omitempty"`
	Activity           string      `json:"activity,omitempty"`   // the last line the running work printed
	ActivityAt         int64       `json:"activityAt,omitempty"` // unix ms of that line
	Watch              *watchState `json:"watch,omitempty"`
	Error              string      `json:"error,omitempty"`
	Snapshot           string      `json:"snapshot,omitempty"`
	SnapshotsSupported bool        `json:"snapshotsSupported"`
	SnapshotReason     string      `json:"snapshotReason,omitempty"`
	LogPath            string      `json:"logPath,omitempty"`
	Prompt             *promptSpec `json:"prompt,omitempty"`
}

// progress is the singleton run-state publisher for the update in this process.
var progress = &publisher{}

type publisher struct {
	mu                 sync.Mutex
	active             bool
	phase              string
	pid                int
	ui                 string
	started            int64
	steps              []runStep
	log                []string
	snapshot           string
	snapshotsSupported bool
	snapshotReason     string
	errMsg             string
	prompt             *promptSpec
	activity           string
	activityAt         int64
	sub                float64 // how far the running step is, read off its output ("(3/12)")
	watch              *watchState
	lastWrite          time.Time
	stopBeat           chan struct{}
}

func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return "/tmp"
}

func runStatePath() string { return filepath.Join(runtimeDir(), "ryoku-update.json") }

func writeState(st runState) {
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	path := runStatePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// readState decodes the current run-state; ok is false when there is none.
func readState() (runState, bool) {
	var st runState
	b, err := os.ReadFile(runStatePath())
	if err != nil || json.Unmarshal(b, &st) != nil {
		return st, false
	}
	return st, true
}

func nowMs() int64 { return time.Now().UnixMilli() }

// updateUI is who drives this run: "hub" when the Hub launched it (no
// terminal; questions and the password come back through the run-state),
// else "terminal".
func updateUI() string {
	if os.Getenv("RYOKU_UPDATE_UI") == "hub" {
		return "hub"
	}
	return "terminal"
}

// begin sets the ordered step list for this run (all pending) and marks the
// run running. A stage2 is the same pid after the exec handoff, so it adopts
// what stage1 published -- the step timings, the narrative, the start time --
// and the run reads as one continuous bar.
func (p *publisher) begin(steps []runStep) (resumed bool) {
	p.mu.Lock()
	p.pid = os.Getpid()
	p.ui = updateUI()
	p.steps = make([]runStep, len(steps))
	copy(p.steps, steps)
	for i := range p.steps {
		p.steps[i].State = stepPending
	}
	p.log = nil
	p.snapshotsSupported, p.snapshotReason = snapshotCapability()
	p.started = nowMs()
	if prev, ok := readState(); ok && prev.PID == p.pid && prev.Phase == "running" {
		resumed = true
		p.started = prev.Started
		p.log = prev.Log
		p.snapshot = prev.Snapshot
		for i := range p.steps {
			for _, old := range prev.Steps {
				if old.Key != p.steps[i].Key {
					continue
				}
				p.steps[i].Began, p.steps[i].Took = old.Began, old.Took
				switch old.State {
				case stepDone, stepSkipped:
					p.steps[i].State = old.State
				case stepRunning:
					// stage1 only execs once its step succeeded
					p.steps[i].State = stepDone
					if old.Began > 0 {
						p.steps[i].Took = nowMs() - old.Began
					}
				}
			}
		}
	}
	p.active = true
	p.phase = "running"
	p.prompt = nil
	p.errMsg = ""
	p.write()
	if p.stopBeat == nil {
		p.stopBeat = make(chan struct{})
		go p.heartbeat(p.stopBeat)
	}
	shown := make([]runStep, len(p.steps))
	copy(shown, p.steps)
	started := time.UnixMilli(p.started)
	p.mu.Unlock()
	con.begin(shown, resumed, started)
	return resumed
}

func (p *publisher) indexOf(key string) int {
	for i := range p.steps {
		if p.steps[i].Key == key {
			return i
		}
	}
	return -1
}

// settle closes the running step i with the given state and its duration.
func (p *publisher) settle(i int, st stepState) {
	if p.steps[i].State == stepRunning && p.steps[i].Began > 0 {
		p.steps[i].Took = nowMs() - p.steps[i].Began
	}
	p.steps[i].State = st
}

// at marks the step with key running and every earlier still-open step done,
// then publishes. The current label and progress follow from the step states.
func (p *publisher) at(key string) {
	p.mu.Lock()
	idx := p.indexOf(key)
	if idx < 0 {
		p.mu.Unlock()
		return
	}
	for i := range idx {
		if p.steps[i].State == stepPending || p.steps[i].State == stepRunning {
			p.settle(i, stepDone)
		}
	}
	p.steps[idx].State = stepRunning
	p.steps[idx].Began = nowMs()
	p.activity, p.activityAt, p.sub = "", 0, 0
	p.write()
	p.mu.Unlock()
	con.enter(key)
}

// markDone forces the named steps done (used by stage2 for the steps stage1
// already ran before the exec handoff).
func (p *publisher) markDone(keys ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range keys {
		if i := p.indexOf(k); i >= 0 {
			p.settle(i, stepDone)
		}
	}
}

// skip marks a step skipped (e.g. the AUR step on a box with no yay).
func (p *publisher) skip(key string) {
	p.mu.Lock()
	if i := p.indexOf(key); i >= 0 {
		p.steps[i].State = stepSkipped
	}
	p.mu.Unlock()
	con.skip(key)
}

// logf is the run's narrative: a line worth keeping on screen. It lands under
// the running step in the console, in the GUI's log ring, and in the raw log.
func (p *publisher) logf(format string, a ...any) {
	p.narrate(true, fmt.Sprintf(format, a...))
}

// detailf is narrative too low to keep ("Ryotunes is current"): it shows as
// the running step's live line and lands in the log, but leaves no trace in
// the curated view.
func (p *publisher) detailf(format string, a ...any) {
	p.narrate(false, fmt.Sprintf(format, a...))
}

func (p *publisher) narrate(keep bool, line string) {
	logNarrative(line)
	p.mu.Lock()
	if !p.active {
		p.mu.Unlock()
		con.loose(line)
		return
	}
	if keep {
		p.log = append(p.log, line)
		if len(p.log) > runLogCap {
			p.log = p.log[len(p.log)-runLogCap:]
		}
	}
	p.activity, p.activityAt = line, nowMs()
	p.write()
	p.mu.Unlock()
	if keep {
		con.note(noteInfo, line)
	} else {
		con.activity(line)
	}
}

// fracRe reads "(3/12)" (pacman) or "[145/350]" (ninja) off a live line.
var fracRe = regexp.MustCompile(`^[\(\[]\s*(\d+)\s*/\s*(\d+)\s*[\)\]]`)

// observe takes one line the running work printed (via the console's capture).
// The file is rewritten at most twice a second: a pacman transaction prints
// far faster than any reader needs, and the heartbeat covers the gaps.
func (p *publisher) observe(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active {
		return
	}
	p.activity, p.activityAt = line, nowMs()
	if m := fracRe.FindStringSubmatch(line); m != nil {
		x, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[2])
		if y > 0 && x <= y {
			p.sub = float64(x) / float64(y)
		}
	}
	if time.Since(p.lastWrite) >= 500*time.Millisecond {
		p.write()
	}
}

// currentFraction is fraction for a reader outside the publisher (the
// console's progress bar).
func (p *publisher) currentFraction() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fraction()
}

// fraction is the run's completion: settled steps count whole, the running
// step counts by what its own output says (half when it says nothing). The
// caller holds the lock.
func (p *publisher) fraction() float64 {
	if len(p.steps) == 0 {
		return 0
	}
	var done float64
	for _, s := range p.steps {
		switch s.State {
		case stepDone, stepSkipped, stepFailed:
			done++
		case stepRunning:
			if p.sub > 0 {
				done += 0.1 + 0.85*p.sub
			} else {
				done += 0.5
			}
		}
	}
	return done / float64(len(p.steps))
}

func (p *publisher) setSnapshot(id string) {
	p.mu.Lock()
	p.snapshot = id
	p.mu.Unlock()
	con.snapshot(id)
}

// write serializes the current state; the caller holds the lock.
func (p *publisher) write() {
	if !p.active {
		return
	}
	now := nowMs()
	st := runState{
		Phase: p.phase, PID: p.pid, UI: p.ui, Started: p.started, Beat: now,
		Steps: p.steps, Log: p.log, Snapshot: p.snapshot, Error: p.errMsg,
		SnapshotsSupported: p.snapshotsSupported, SnapshotReason: p.snapshotReason,
		Activity: p.activity, ActivityAt: p.activityAt, Prompt: p.prompt,
		LogPath: updateLogPath(), Progress: p.fraction(),
	}
	if p.phase == "running" {
		st.Watch = p.watch
	}
	for _, s := range p.steps {
		if s.State == stepRunning || (p.phase == "error" && s.State == stepFailed) {
			st.Step, st.Label = s.Key, s.Label
		}
	}
	if p.phase == "done" {
		st.Progress = 1
	}
	writeState(st)
	p.lastWrite = time.Now()
}

// heartbeat rewrites the file every two seconds with a fresh beat and the
// watchdog's read of the process tree, so the Hub can tell a quiet compile
// from a hung step, and a live run from one whose process is gone.
func (p *publisher) heartbeat(stop chan struct{}) {
	w := newWatchdog(os.Getpid())
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p.mu.Lock()
			at := p.activityAt
			p.mu.Unlock()
			ws := w.sample(at) // the /proc walk runs outside the lock
			p.mu.Lock()
			p.watch = &ws
			if p.active && (p.phase == "running" || p.phase == "prompt" || p.phase == "auth") {
				p.write()
			}
			p.mu.Unlock()
			con.watchUpdate(ws)
		}
	}
}

func (p *publisher) stopHeartbeat() {
	if p.stopBeat != nil {
		close(p.stopBeat)
		p.stopBeat = nil
	}
}

// finish marks every open step done and publishes a terminal "done" state.
func (p *publisher) finish() {
	p.mu.Lock()
	p.stopHeartbeat()
	for i := range p.steps {
		if p.steps[i].State != stepSkipped && p.steps[i].State != stepFailed && p.steps[i].State != stepDone {
			p.settle(i, stepDone)
		}
	}
	p.phase, p.prompt, p.activity = "done", nil, ""
	p.write()
	p.mu.Unlock()
	con.finish()
}

// fail marks the running step failed and publishes an "error" state carrying
// the reason and the pre-update snapshot the GUI offers to roll back to. Once
// a stop is under way only the stop itself is recorded: the step it killed
// failing at the same moment is not the story.
func (p *publisher) fail(err error) {
	if stopping.Load() && !errors.Is(err, errStopped) {
		return
	}
	p.mu.Lock()
	p.stopHeartbeat()
	for i := range p.steps {
		if p.steps[i].State == stepRunning {
			p.settle(i, stepFailed)
		}
	}
	p.phase, p.prompt, p.errMsg = "error", nil, err.Error()
	snap := p.snapshot
	p.write()
	p.mu.Unlock()
	con.fail(err, snap)
}

// idle clears the run state so the Hub returns to its idle view.
func (p *publisher) idle() {
	p.mu.Lock()
	p.stopHeartbeat()
	p.active = false
	p.mu.Unlock()
	writeState(runState{Phase: "idle"})
}

// ask publishes a question the Hub renders and the run blocks on; resume
// returns the page to the running view.
func (p *publisher) ask(spec *promptSpec) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if spec.Kind == "secret" {
		p.phase = "auth"
	} else {
		p.phase = "prompt"
	}
	p.prompt = spec
	p.write()
}

func (p *publisher) resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.phase, p.prompt = "running", nil
	p.write()
}

// answerPath is the back-channel a choice prompt is answered on: the Hub
// writes the chosen option label here, and `ryoku update` reads it.
func answerPath() string { return filepath.Join(runtimeDir(), "ryoku-update-answer") }

// publishPrompt writes a "prompt" phase the Hub renders as a question with
// option buttons. Any prior answer is cleared first so a stale click cannot
// satisfy this prompt.
func publishPrompt(id, title, detail string, options []string) {
	_ = os.Remove(answerPath())
	progress.ask(&promptSpec{ID: id, Title: title, Detail: detail, Options: options})
}

// awaitAnswer blocks until the back-channel carries a choice or timeout
// elapses, then clears it. Returns the chosen option label and true, or "" and
// false on timeout (the caller treats that as a decline).
func awaitAnswer(timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(answerPath()); err == nil {
			choice := strings.TrimSpace(string(b))
			_ = os.Remove(answerPath())
			if choice != "" {
				return choice, true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", false
}
