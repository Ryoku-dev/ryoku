package ask

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// run.go runs a command where it can be watched. A command that takes a
// minute and says nothing until it ends reads as a hang, so its output is a
// terminal of the session: it arrives piece by piece under the line of the
// call that started it, in the app and in the log alike.

const (
	// a command that ends sooner than this shows its output once, as a
	// result; a line that opened and shut again for every quick look would
	// make a transcript flicker
	watchAfter = 700 * time.Millisecond
	// how much of one command's output is shown as it comes
	watchLimit = 256 * 1024
	// a command the user started themselves may take its time: they see it
	// run and can stop it
	userRunTimeout = 10 * time.Minute
)

// watched is the output of one running command. What the model is told is
// kept whole up to a limit; what the user watches is sent on as it comes.
type watched struct {
	sink core.Sink
	call string

	mu       sync.Mutex
	kept     limitedBuffer
	attached bool
	held     []byte
	shown    int
}

// attach makes the output a terminal of the tool call and sends on what was
// held back so far.
func (w *watched) attach() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.attached {
		return
	}
	w.attached = true
	w.sink.Emit(protocol.EvTool, protocol.ToolData{Call: w.call, Terminal: w.call})
	w.sendLocked(false)
}

func (w *watched) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.kept.Write(p)
	if room := watchLimit - w.shown - len(w.held); room > 0 {
		w.held = append(w.held, p[:min(len(p), room)]...)
	}
	if w.attached {
		w.sendLocked(false)
	}
	return len(p), nil
}

// sendLocked sends what is held as text. A character cut in two by a write
// waits for its other half unless the command has ended.
func (w *watched) sendLocked(last bool) {
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
	w.sink.Emit(protocol.EvTerminal, protocol.TerminalData{Terminal: w.call, Delta: delta})
}

// end closes the terminal if the output became one, with a last line when
// the command did not end by itself. It reports whether it did.
func (w *watched) end(exit int, note string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.attached {
		return false
	}
	w.sendLocked(true)
	if note != "" {
		w.sink.Emit(protocol.EvTerminal, protocol.TerminalData{Terminal: w.call, Delta: "\n[" + note + "]\n"})
	}
	done := protocol.TerminalData{Terminal: w.call, Done: true}
	if exit >= 0 {
		done.Exit = &exit
	}
	w.sink.Emit(protocol.EvTerminal, done)
	return true
}

// watch runs one shell command for the tool call. Its output shows as it
// comes: from the start when the user is waiting on this command alone, else
// once the command has run long enough to be worth watching. It returns what
// the command printed, cut to limit, and its exit status.
func watch(ctx context.Context, sink core.Sink, call, line, cwd string, timeout time.Duration, limit int, fromStart bool) (string, int, error) {
	output := &watched{sink: sink, call: call, kept: limitedBuffer{limit: limit}}
	if fromStart {
		output.attach()
	} else {
		later := time.AfterFunc(watchAfter, output.attach)
		defer later.Stop()
	}
	exit, err := runInto(ctx, timeout, cwd, output, "sh", "-c", line)
	note := ""
	if err != nil {
		note = err.Error()
	}
	output.end(exit, note)
	output.mu.Lock()
	text := output.kept.text()
	output.mu.Unlock()
	return text, exit, err
}

// query runs a system_query: a command the classifier rates a pure read.
func (r *runner) query(ctx context.Context, call llm.ToolCall, sink core.Sink, cwd string) string {
	title, kind, input := toolPresentation(call)
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: title, Kind: kind, Status: protocol.ToolRunning, Input: input})
	var args struct {
		Command string `json:"command"`
	}
	output, status := "", protocol.ToolDone
	if err := decodeArgs(call.Arguments, &args); err != nil {
		output, status = err.Error(), protocol.ToolFailed
	} else if reason := command.WhyNotReadOnly(args.Command); reason != "" {
		output = i18n.Tf("Refused: %s. Use propose so the user can review and run this command.", reason)
	} else {
		var err error
		output, _, err = watch(ctx, sink, call.ID, args.Command, cwd, commandTimeout, 16*1024, false)
		if err != nil {
			status = protocol.ToolFailed
			output = strings.TrimSpace(output + "\n" + failure(err))
		}
	}
	output = truncateRunes(output, 6000)
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: status, Output: output})
	return output
}

// failure is what the model is told went wrong. Running out of time is the
// one failure it can do something about, so that one says what.
func failure(err error) string {
	var slow ranOut
	if errors.As(err, &slow) {
		return err.Error() + " Narrow what it looks at: for disk use stay on one filesystem (du -x, find -xdev) or start from the home folder."
	}
	return err.Error()
}

// userRun runs the command the user started from the conversation and
// returns what the model is then told, so it answers from the output. ok is
// false when the turn was stopped while the command ran.
func (r *runner) userRun(ctx context.Context, in core.PromptInput, sink core.Sink, cwd string) (llm.Message, bool, error) {
	line := strings.TrimSpace(in.Run)
	if reason, needs := command.NeedsTerminal(line); needs {
		return llm.Message{}, false, fmt.Errorf(i18n.T("This command needs a terminal: %s."), reason)
	}
	call := fmt.Sprintf("run-%d", in.Turn)
	// the turn opens with the command as the user's own line, so the call does not say it again
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call, Title: i18n.T("Running"), Kind: protocol.ToolExecute, Status: protocol.ToolRunning})
	began := time.Now()
	output, exit, err := watch(ctx, sink, call, line, cwd, userRunTimeout, actOutput, true)
	took := max(1, int(time.Since(began).Round(time.Second).Seconds()))
	status, title := protocol.ToolDone, i18n.Tf("Ran in %d s", took)
	switch {
	case ctx.Err() != nil:
		// the user stopped it, which is not the command failing
		title = i18n.Tf("Stopped after %d s", took)
	case err != nil && exit > 0:
		status, title = protocol.ToolFailed, i18n.Tf("Ended with exit status %d", exit)
	case err != nil:
		status, title = protocol.ToolFailed, i18n.Tf("Stopped after %d s", took)
	}
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call, Title: title, Status: status})
	if ctx.Err() != nil {
		return llm.Message{}, false, nil
	}
	var told strings.Builder
	told.WriteString("I ran this command myself, here in the conversation:\n\n")
	told.WriteString(line)
	switch {
	case err != nil && exit < 0:
		fmt.Fprintf(&told, "\n\n%s", err.Error())
	default:
		fmt.Fprintf(&told, "\n\nIt ended with exit status %d.", exit)
	}
	if output == "" {
		told.WriteString(" It printed nothing.")
	} else {
		told.WriteString(" It printed:\n\n")
		told.WriteString(bothEnds(output, 6000))
	}
	told.WriteString("\n\nAnswer what I asked before from this output, in a sentence or two. Do not propose this command again.")
	return llm.Message{Role: llm.RoleUser, Content: told.String()}, true, nil
}

// bothEnds keeps the start and the end of a long output: a listing is read
// from its top, and what went wrong is said at the bottom.
func bothEnds(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	half := limit / 2
	return string(runes[:half]) + "\n[...]\n" + string(runes[len(runes)-half:])
}
