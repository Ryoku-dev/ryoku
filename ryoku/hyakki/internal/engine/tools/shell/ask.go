package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Ask runs a command once the user lets it change the machine. With
// approvals "ask" every change waits for the user's answer; with "auto"
// only a destructive one does, so the one mistake that cannot be taken back
// is still looked at by a person. Once a conversation may act the mode
// decides, not which of the two tools the model happened to pick.

const (
	// A walk over a whole home folder takes a minute on a full disk, and
	// cutting it at fifteen seconds turned "which file is largest" into a
	// command for the user to run.
	askTimeout = 2 * time.Minute
	// a command the user started themselves may take its time: they see it
	// run and can stop it
	userRunTimeout = 10 * time.Minute
	queryKeep      = 16 * 1024
	actKeep        = 24 * 1024
	// userTold caps the runes of a user run's output Ask is handed.
	userTold = 6000

	refusedToTool = "The user refused this. Do not try the same change another way. Tell them it was not done and ask what they want instead."
)

type runCommandTool struct{}

func (runCommandTool) Name() string    { return tool.RunCommand }
func (runCommandTool) Kind() tool.Kind { return protocol.ToolExecute }
func (runCommandTool) ReadOnly() bool  { return false }

func (runCommandTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return llm.Tool{
			Name:        tool.RunCommand,
			Description: "Run one shell command on this machine as the user, never as root.",
			Parameters:  schema(`{"type":"object","properties":{"command":{"type":"string"},"why":{"type":"string"}},"required":["command"]}`),
		}
	}
	return llm.Tool{
		Name:        tool.RunCommand,
		Description: "Run one shell command on this machine as the user. A command that only reads runs at once. One that changes something follows the approval rule in your instructions. It cannot become root.",
		Parameters:  schema(`{"type":"object","properties":{"command":{"type":"string"},"why":{"type":"string","description":"One short line the user reads before allowing it."}},"required":["command","why"],"additionalProperties":false}`),
	}
}

func (runCommandTool) Present(args json.RawMessage) tool.Presentation {
	a, err := decodeCommand(args)
	if err != nil {
		return tool.Presentation{Title: i18n.T("Running a command"), Input: clipLine(string(args), 400)}
	}
	return tool.Presentation{Title: changeTitle(a), Input: clip(a.Command, 400)}
}

func (runCommandTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if env.Profile.ReadOnly || env.Profile.Mode == tool.ModeAskRead {
		return tool.Failed(i18n.T("Refused: Ask is read-only in this conversation. Use propose so the user can review and run it."))
	}
	a, err := decodeCommand(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if a.Command == "" {
		return tool.Failed(i18n.T("The command is empty."))
	}
	return runAskChange(ctx, call, env, a)
}

type systemQueryTool struct{}

func (systemQueryTool) Name() string    { return tool.SystemQuery }
func (systemQueryTool) Kind() tool.Kind { return protocol.ToolExecute }

// ReadOnly is true: a query that is more than a read only runs in a session
// allowed to act, through the same gate as run_command.
func (systemQueryTool) ReadOnly() bool { return true }

func (systemQueryTool) Spec(p tool.Profile) llm.Tool {
	description := "Run one command only when it is provably read-only."
	if p.Mode == tool.ModeAskAct {
		description = "Run a command that reads the system. In acting Ask, a command that changes it follows the same approval rule as run_command."
	}
	return llm.Tool{
		Name:        tool.SystemQuery,
		Description: description,
		Parameters:  schema(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"],"additionalProperties":false}`),
	}
}

func (systemQueryTool) Present(args json.RawMessage) tool.Presentation {
	a, err := decodeCommand(args)
	if err != nil {
		return tool.Presentation{Title: i18n.T("Checking"), Input: clipLine(string(args), 400)}
	}
	return tool.Presentation{Title: i18n.T("Checking"), Input: clip(a.Command, 400)}
}

func (systemQueryTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decodeCommand(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if a.Command == "" {
		return tool.Failed(i18n.T("The command is empty."))
	}
	if refused, ok := refuseSecret(a, env.Cwd); ok {
		return refused
	}
	if command.ReadOnly(a.Command) {
		o := runWatched(ctx, env.Exec, watcherFor(env, call), runRequest{
			argv: []string{"sh", "-c", a.Command}, dir: env.Cwd, timeout: askTimeout, keep: queryKeep,
		})
		return askResult(o, env, "")
	}
	// Turning a change down here sent the model to propose, and a user who
	// had said "go ahead" was handed a command to run by hand.
	if env.Profile.Mode == tool.ModeAskAct && !env.Profile.ReadOnly {
		return runAskChange(ctx, call, env, a)
	}
	return tool.Failed(i18n.Tf("Refused: %s. Use propose so the user can review and run this command.", command.WhyNotReadOnly(a.Command)))
}

// runAskChange runs a command of a session allowed to act: refused when it
// cannot run here, put to the gate unless it only reads, then watched.
func runAskChange(ctx context.Context, call tool.Call, env tool.Env, a commandArgs) tool.Result {
	if command.Elevates(a.Command) {
		return proposed(a, i18n.T("Refused: this needs a password on a terminal, which Ask does not have. It is now proposed to the user to run there; do not propose it again."))
	}
	if refused, ok := refuseSecret(a, env.Cwd); ok {
		return refused
	}
	if !command.ReadOnly(a.Command) {
		if refused, ok := gate(ctx, call, env, changeTitle(a), a.Command); !ok {
			return refused
		}
	}
	emit(env, protocol.ToolData{Status: protocol.ToolRunning})
	o := runWatched(ctx, env.Exec, watcherFor(env, call), runRequest{
		argv: []string{"sh", "-c", a.Command}, dir: env.Cwd, timeout: askTimeout, keep: actKeep,
	})
	return askResult(o, env, i18n.T("Done, with no output."))
}

// refuseSecret refuses a command that names a credential, in every mode:
// what a command prints is sent to the model. The user can still run it.
func refuseSecret(a commandArgs, cwd string) (tool.Result, bool) {
	marker, secret := command.ReadsSecretAt(a.Command, cwd)
	if !secret {
		return tool.Result{}, false
	}
	return proposed(a, i18n.Tf("Refused: this names %s, and what a command prints is sent to the model. It is now proposed to the user to run themselves; do not propose it again.", marker)), true
}

// proposed refuses a command and hands it to the user as a plan of one,
// marked when it needs a terminal.
func proposed(a commandArgs, why string) tool.Result {
	tier, _ := command.Classify(a.Command)
	_, terminal := command.NeedsTerminal(a.Command)
	return tool.Result{
		Text:    why,
		IsError: true,
		Proposal: &protocol.CommandPlan{Commands: []protocol.PlannedCommand{{
			Command: a.Command, Why: a.Why, Tier: tier, Terminal: terminal,
		}}},
	}
}

// gate puts one change to the session's approvals and reports whether it
// may run, or the result to return when it may not.
func gate(ctx context.Context, call tool.Call, env tool.Env, title, cmd string) (tool.Result, bool) {
	if env.Permissions == nil {
		return tool.Failed(i18n.T("Not run: nothing here can approve a change.")), false
	}
	emit(env, protocol.ToolData{Status: protocol.ToolPending})
	decision, err := env.Permissions.Allow(ctx, tool.Request{
		Call: call, Title: title, Kind: protocol.ToolExecute, Input: cmd, Tier: changeTier(cmd),
	})
	if err != nil {
		return tool.Failed(i18n.T("Cancelled before it ran.")), false
	}
	if !decision.Allowed {
		told := decision.Reason
		if told == "" {
			told = refusedToTool
		}
		shown := i18n.T("Not run: you refused it.")
		if decision.By != "user" && decision.Reason != "" {
			shown = decision.Reason
		}
		return tool.Result{Text: told, Shown: shown, IsError: true}, false
	}
	return tool.Result{}, true
}

// askResult is what Ask is told of a command it ran.
func askResult(o runOutcome, env tool.Env, empty string) tool.Result {
	if o.stopped {
		return tool.Failed(i18n.T("Stopped before it finished."))
	}
	text := output(o.exit)
	failure, failed := askFailure(o, true)
	if failed {
		text = strings.TrimSpace(text + "\n" + failure)
	}
	if text == "" {
		text = empty
	}
	return tool.Result{Text: shape(text, limitsOf(env)), IsError: failed}
}

// askFailure says what went wrong. Running out of time is the one failure
// the model can do something about, so with hint that one says what.
func askFailure(o runOutcome, hint bool) (string, bool) {
	switch {
	case o.err != nil:
		return i18n.Tf("The command failed: %v", o.err), true
	case o.exit.TimedOut:
		text := timedOut(o.exit.Elapsed)
		if hint {
			text += " " + i18n.T("Narrow what it looks at: for disk use stay on one filesystem (du -x, find -xdev) or start from the home folder.")
		}
		return text, true
	case o.exit.Signal != "":
		return i18n.Tf("The command failed: %v", "signal: "+o.exit.Signal), true
	case o.exit.Code != 0:
		return i18n.Tf("The command failed: %v", fmt.Sprintf("exit status %d", o.exit.Code)), true
	}
	return "", false
}

func changeTitle(a commandArgs) string {
	if a.Why != "" {
		return clip(a.Why, 80)
	}
	return i18n.T("Running a command")
}

func clip(s string, runes int) string {
	clipped, _ := tool.ClipRunes(s, runes)
	return clipped
}

// UserRun runs the command the user started from an Ask conversation
// (session.prompt with run) and returns what the model is then told, so it
// answers from the output. Its output is a terminal from the start, the tool
// line is call "run-<turn>", and it has userRunTimeout and no keyboard. ok
// is false when ctx ended while the command ran: the process group is
// killed and nothing is asked of the model. err is a command that needs a
// terminal (command.NeedsTerminal), refused before anything starts.
func UserRun(ctx context.Context, x exec.Executor, sink core.Sink, turn int, line, cwd string) (told string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if reason, needs := command.NeedsTerminal(line); needs {
		return "", false, fmt.Errorf(i18n.T("This command needs a terminal: %s."), reason)
	}
	call := fmt.Sprintf("run-%d", turn)
	// the turn opens with the command as the user's own line, so the call
	// does not say it again
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call, Title: i18n.T("Running"), Kind: protocol.ToolExecute, Status: protocol.ToolRunning})
	w := newWatcher(sink, call, func() { sink.Emit(protocol.EvTool, protocol.ToolData{Call: call, Terminal: call}) })
	began := time.Now()
	o := runWatched(ctx, x, w, runRequest{
		argv: []string{"sh", "-c", line}, dir: cwd, timeout: userRunTimeout, keep: actKeep, fromStart: true,
	})
	took := seconds(time.Since(began))
	failure, failed := askFailure(o, false)
	exit := -1
	if o.err == nil && !o.stopped && !o.exit.TimedOut && o.exit.Signal == "" {
		exit = o.exit.Code
	}
	status, title := protocol.ToolDone, i18n.Tf("Ran in %d s", took)
	switch {
	case o.stopped || ctx.Err() != nil:
		// the user stopped it, which is not the command failing
		title = i18n.Tf("Stopped after %d s", took)
	case failed && exit > 0:
		status, title = protocol.ToolFailed, i18n.Tf("Ended with exit status %d", exit)
	case failed:
		status, title = protocol.ToolFailed, i18n.Tf("Stopped after %d s", took)
	}
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call, Title: title, Status: status})
	if o.stopped || ctx.Err() != nil {
		return "", false, nil
	}
	var b strings.Builder
	b.WriteString("I ran this command myself, here in the conversation:\n\n")
	b.WriteString(line)
	if failed && exit < 0 {
		fmt.Fprintf(&b, "\n\n%s", failure)
	} else {
		fmt.Fprintf(&b, "\n\nIt ended with exit status %d.", exit)
	}
	if printed := output(o.exit); printed == "" {
		b.WriteString(" It printed nothing.")
	} else {
		b.WriteString(" It printed:\n\n")
		b.WriteString(bothEnds(printed, userTold))
	}
	b.WriteString("\n\nAnswer what I asked before from this output, in a sentence or two. Do not propose this command again.")
	return b.String(), true, nil
}

// bothEnds keeps the start and the end of a long output in runes.
func bothEnds(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	half := limit / 2
	return string(runes[:half]) + "\n[...]\n" + string(runes[len(runes)-half:])
}
