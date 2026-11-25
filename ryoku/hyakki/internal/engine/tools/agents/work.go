package agents

import (
	"sync"
	"time"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// MaxSteps is how many of a subagent's latest steps its box keeps; StepCount
// still counts them all.
const MaxSteps = 8

// outputRunes caps what a subagent returned as shown in its box.
const outputRunes = 4000

// board is the subagents of one task call, as the app's subagent boxes draw
// them. Children report from their own goroutines; every change goes out
// whole through emit, under the lock, so the app never sees reports out of
// order.
type board struct {
	mu      sync.Mutex
	work    []protocol.AgentWork
	started []time.Time
	emit    func([]protocol.AgentWork)
	now     func() time.Time
}

func newBoard(work []protocol.AgentWork, emit func([]protocol.AgentWork), now func() time.Time) *board {
	if now == nil {
		now = time.Now
	}
	return &board{work: work, started: make([]time.Time, len(work)), emit: emit, now: now}
}

// start marks child i running.
func (b *board) start(i int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started[i] = b.now()
	b.work[i].Status = protocol.ToolRunning
	b.publish()
}

// report takes a child's progress report.
func (b *board) report(i int, latest protocol.AgentWork) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if final(b.work[i].Status) {
		return
	}
	merge(&b.work[i], latest)
	if latest.Status != protocol.ToolPending && latest.Status != "" && !final(latest.Status) {
		b.work[i].Status = latest.Status
	}
	b.work[i].Doing = latest.Doing
	b.tick(i)
	b.publish()
}

// finish ends child i with status and what it returned.
func (b *board) finish(i int, status string, last protocol.AgentWork, output string, tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	merge(&b.work[i], last)
	b.work[i].Status = status
	b.work[i].Doing = ""
	if b.work[i].Tokens == 0 {
		b.work[i].Tokens = tokens
	}
	if output != "" {
		b.work[i].Output, _ = tool.ClipRunes(output, outputRunes)
	}
	b.tick(i)
	b.publish()
}

// stop marks every child that has not ended as stopped.
func (b *board) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	changed := false
	for i := range b.work {
		if !final(b.work[i].Status) {
			b.work[i].Status, b.work[i].Doing = protocol.ToolStopped, ""
			b.tick(i)
			changed = true
		}
	}
	if changed {
		b.publish()
	}
}

// snapshot copies the board.
func (b *board) snapshot() []protocol.AgentWork {
	b.mu.Lock()
	defer b.mu.Unlock()
	return copyWork(b.work)
}

func (b *board) tick(i int) {
	if !b.started[i].IsZero() {
		b.work[i].Millis = b.now().Sub(b.started[i]).Milliseconds()
	}
}

func (b *board) publish() {
	if b.emit != nil {
		b.emit(copyWork(b.work))
	}
}

// merge folds a report into the box: the tool's own ID, kind and task stay,
// a report without steps keeps the steps an earlier one gave, and only the
// latest MaxSteps are kept.
func merge(into *protocol.AgentWork, latest protocol.AgentWork) {
	if len(latest.Steps) > 0 {
		steps := latest.Steps
		if len(steps) > MaxSteps {
			steps = steps[len(steps)-MaxSteps:]
		}
		into.Steps = append([]protocol.AgentStep(nil), steps...)
	}
	into.StepCount = max(into.StepCount, latest.StepCount, len(latest.Steps))
	if latest.Tokens > 0 {
		into.Tokens = latest.Tokens
	}
	if latest.Model != "" {
		into.Model = latest.Model
	}
	if latest.Output != "" {
		into.Output, _ = tool.ClipRunes(latest.Output, outputRunes)
	}
}

func final(status string) bool {
	return status == protocol.ToolDone || status == protocol.ToolFailed || status == protocol.ToolStopped
}

func copyWork(work []protocol.AgentWork) []protocol.AgentWork {
	out := make([]protocol.AgentWork, len(work))
	for i, w := range work {
		w.Steps = append([]protocol.AgentStep(nil), w.Steps...)
		out[i] = w
	}
	return out
}
