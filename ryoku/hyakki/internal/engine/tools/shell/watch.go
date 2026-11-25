package shell

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// A command that takes a minute and says nothing until it ends reads as a
// hang, so its output becomes a terminal of the session: it arrives piece by
// piece under the line of the call that started it, in the app and in the
// log alike.

var (
	// A command that ends sooner shows its output once, as a result; a line
	// that opened and shut again for every quick look would make a transcript
	// flicker. A var so tests need not wait.
	watchAfter = 700 * time.Millisecond
	// How long ending a stopped process may take before the call gives up on
	// collecting what it printed.
	stopGrace = 5 * time.Second
)

// watchLimit caps how much of one command's output is shown as it comes.
const watchLimit = 256 * 1024

// watcher is what the user watches of one running command. What the model
// is told comes from the executor's retained output, not from here.
type watcher struct {
	sink     core.Sink
	terminal string
	// openLine names the terminal on the tool line.
	openLine func()

	mu       sync.Mutex
	attached bool
	closed   bool
	held     []byte
	shown    int
	capped   bool
}

func newWatcher(sink core.Sink, terminal string, openLine func()) *watcher {
	return &watcher{sink: sink, terminal: terminal, openLine: openLine}
}

// watcherFor watches a tool call's command under its own tool line.
func watcherFor(env tool.Env, call tool.Call) *watcher {
	return newWatcher(env.Sink, call.ID, func() { emit(env, protocol.ToolData{Terminal: call.ID}) })
}

// open makes the output a terminal of the call and sends on what was held
// back so far.
func (w *watcher) open() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.attached || w.closed || w.sink == nil {
		return
	}
	w.attached = true
	if w.openLine != nil {
		w.openLine()
	}
	w.sendLocked(false)
}

func (w *watcher) write(delta string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if room := watchLimit - w.shown - len(w.held); room > 0 {
		chunk, cut := tool.ClipBytes(delta, room)
		w.held = append(w.held, chunk...)
		w.capped = w.capped || cut
	} else if delta != "" {
		w.capped = true
	}
	if w.attached {
		w.sendLocked(false)
	}
}

// sendLocked sends what is held. A character cut in two waits for its other
// half unless the command has ended.
func (w *watcher) sendLocked(last bool) {
	whole := len(w.held)
	if !last {
		for back := 1; back <= utf8.UTFMax && back <= whole; back++ {
			if utf8.RuneStart(w.held[whole-back]) {
				if !utf8.FullRune(w.held[whole-back:]) {
					whole -= back
				}
				break
			}
		}
	}
	if whole == 0 {
		return
	}
	delta := strings.ToValidUTF8(string(w.held[:whole]), "\uFFFD")
	w.shown += whole
	w.held = append(w.held[:0], w.held[whole:]...)
	w.sink.Emit(protocol.EvTerminal, protocol.TerminalData{Terminal: w.terminal, Delta: delta})
}

// end closes the terminal if the output became one, with a last line when
// the command did not end by itself, and drops anything written after. exit
// is nil when the command has no exit status of its own.
func (w *watcher) end(exit *int, note string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	if !w.attached {
		return
	}
	if w.capped {
		w.sendLocked(false)
	} else {
		w.sendLocked(true)
	}
	if note != "" {
		w.sink.Emit(protocol.EvTerminal, protocol.TerminalData{Terminal: w.terminal, Delta: "\n[" + note + "]\n"})
	}
	w.sink.Emit(protocol.EvTerminal, protocol.TerminalData{Terminal: w.terminal, Done: true, Exit: exit})
}

// runRequest is one watched process.
type runRequest struct {
	argv    []string
	dir     string
	timeout time.Duration
	keep    int
	// fromStart shows the terminal at once: the user is waiting on this
	// command alone.
	fromStart bool
	// background, above zero, is how long the call waits before it offers
	// the still running process to move; async offers it at once.
	background time.Duration
	async      bool
	// move hands a running process to the job table.
	move func(process string, started time.Time) string
}

// runOutcome is how a watched process left the call.
type runOutcome struct {
	process string
	exit    exec.Exit
	// err is a start or executor failure; the process has no exit status.
	err error
	// stopped: ctx ended, the process was killed.
	stopped bool
	// job is set when the process kept running as a background job.
	job string
}

type waited struct {
	exit exec.Exit
	err  error
}

// runWatched runs one process through the executor with its output watched.
// It returns when the process ends, when ctx ends (the process is killed),
// or when the process moves to the background.
func runWatched(ctx context.Context, x exec.Executor, w *watcher, req runRequest) runOutcome {
	if req.fromStart {
		w.open()
	} else {
		later := time.AfterFunc(watchAfter, w.open)
		defer later.Stop()
	}
	started := time.Now()
	process, err := x.Start(ctx, exec.StartRequest{Argv: req.argv, Dir: req.dir, Timeout: req.timeout, Keep: req.keep}, func(out exec.Output) {
		w.write(out.Delta)
	})
	if err != nil {
		w.end(nil, "")
		return runOutcome{err: err}
	}

	// Wait is not tied to ctx: a cancelled turn kills the process itself, and
	// a moved one must keep running after this call.
	waitCtx, stopWaiting := context.WithCancel(context.WithoutCancel(ctx))
	defer stopWaiting()
	done := make(chan waited, 1)
	go func() {
		exit, err := x.Wait(waitCtx, process)
		done <- waited{exit, err}
	}()

	var move <-chan time.Time
	switch {
	case req.move == nil:
	case req.async:
		now := make(chan time.Time, 1)
		now <- started
		move = now
	case req.background > 0:
		timer := time.NewTimer(max(0, time.Until(started.Add(req.background))))
		defer timer.Stop()
		move = timer.C
	}

	for {
		select {
		case got := <-done:
			return finished(ctx, x, w, process, got)
		case <-ctx.Done():
			stopWaiting()
			<-done
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopGrace)
			defer cancel()
			_ = x.Kill(cleanup, process)
			exit, _ := x.Wait(cleanup, process)
			_ = x.Release(cleanup, process)
			w.end(nil, i18n.T("Stopped"))
			return runOutcome{process: process, exit: exit, stopped: true}
		case <-move:
			move = nil
			job := req.move(process, started)
			stopWaiting()
			if got := <-done; got.err == nil {
				// it ended in the meantime: the job is collected at once
				return finished(ctx, x, w, process, got)
			}
			exit, err := x.Peek(context.WithoutCancel(ctx), process)
			if err != nil {
				exit = exec.Exit{ID: process, Running: true, Elapsed: time.Since(started)}
			}
			w.end(nil, i18n.Tf("Moved to background job %s", job))
			return runOutcome{process: process, exit: exit, job: job}
		}
	}
}

// finished closes the terminal of an ended process and forgets it in the
// executor.
func finished(ctx context.Context, x exec.Executor, w *watcher, process string, got waited) runOutcome {
	if got.err != nil {
		cleanup, cancel := stopContext(ctx)
		_ = x.Kill(cleanup, process)
		_ = x.Release(cleanup, process)
		cancel()
		w.end(nil, got.err.Error())
		return runOutcome{process: process, err: got.err}
	}
	note := ""
	var code *int
	switch {
	case got.exit.TimedOut:
		note = timedOut(got.exit.Elapsed)
	case got.exit.Code >= 0:
		c := got.exit.Code
		code = &c
	}
	w.end(code, note)
	release(ctx, x, process)
	return runOutcome{process: process, exit: got.exit}
}

func release(ctx context.Context, x exec.Executor, process string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopGrace)
	defer cancel()
	_ = x.Release(cleanup, process)
}

// timedOut is the line for a command stopped at its time limit.
func timedOut(ran time.Duration) string {
	return i18n.Tf("The command was stopped after %d seconds without finishing.", int(ran.Round(time.Second).Seconds()))
}
