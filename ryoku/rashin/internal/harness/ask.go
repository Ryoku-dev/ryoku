package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// askRunner answers an Ask through a harness. The session's mode is Ask's
// own, an approvals value that decides each call the agent makes
// (askDecides), so the harness's modes never reach the session and setting
// the mode never starts the harness.
type askRunner struct{ *runner }

var _ core.Runner = askRunner{}

// Prompt sends the question on. The harness heard the mode in its brief; a
// mode the user changed since is said again ahead of the next question.
func (a askRunner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	mode := a.askMode()
	a.mu.Lock()
	told := a.askTold
	a.askTold = mode
	a.mu.Unlock()
	if told != "" && told != mode {
		in.Text = brief.AskModeChanged(mode) + "\n\n" + in.Text
	}
	return a.runner.Prompt(ctx, in, askSink{sink})
}

func (a askRunner) SetModel(ctx context.Context, model string, sink core.Sink) error {
	return a.runner.SetModel(ctx, model, askSink{sink})
}

func (a askRunner) SetOption(ctx context.Context, option, value string, sink core.Sink) error {
	return a.runner.SetOption(ctx, option, value, askSink{sink})
}

func (a askRunner) Start(ctx context.Context, sink core.Sink) error {
	return a.runner.Start(ctx, askSink{sink})
}

// SetMode sets what this Ask may do. Going back to read-only also takes back
// an earlier "allow in this conversation".
func (a askRunner) SetMode(_ context.Context, mode string, sink core.Sink) error {
	if !protocol.KnownApprovals(mode) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Ask can be read-only, ask first, or act without asking."))
	}
	if mode == protocol.ApprovalsReadOnly {
		a.mu.Lock()
		a.askAllowed = false
		a.mu.Unlock()
	}
	sink.Update(func(info *protocol.SessionInfo) { info.Mode = mode })
	return nil
}

// askSink keeps the harness's own mode out of an Ask: its mode events are
// dropped and the session's mode stays Ask's.
type askSink struct{ core.Sink }

func (s askSink) Emit(kind string, data any) {
	if kind != protocol.EvMode {
		s.Sink.Emit(kind, data)
	}
}

func (s askSink) Update(change func(*protocol.SessionInfo)) {
	s.Sink.Update(func(info *protocol.SessionInfo) {
		mode := info.Mode
		change(info)
		info.Mode = mode
	})
}

// askVerdict is what an Ask's mode makes of one call the agent asks to make.
type askVerdict int

const (
	askAllows askVerdict = iota
	askAsks
	askRefuses
)

// askMode is the Ask's mode; one that is not an approvals value is read-only.
func (r *runner) askMode() string {
	if mode := r.approvalPolicy(); protocol.KnownApprovals(mode) {
		return mode
	}
	return protocol.ApprovalsReadOnly
}

// askDecides weighs one tool call the way Ask weighs its own: a call that
// only reads runs in every mode, and a command that asks for a password or
// names a credential never runs. Read only refuses the rest with the reason,
// Ask first asks unless the user allowed changes for the conversation, and
// YOLO runs it. A destructive command asks in every mode that may act, and
// danger says the asking must not offer "allow in this conversation".
func (r *runner) askDecides(kind, title string, rawInput json.RawMessage) (verdict askVerdict, reason string, danger bool) {
	read, why := command.ReadOnlyToolCall(kind, title, rawInput)
	if read {
		return askAllows, "", false
	}
	if cmd := command.ToolCallCommand(kind, title, rawInput); cmd != "" {
		if command.Elevates(cmd) {
			return askRefuses, i18n.T("it asks for a password on a terminal Ask does not have, so the user runs it"), false
		}
		if secret, reads := command.ReadsSecret(cmd); reads {
			return askRefuses, i18n.Tf("it reads a credential (%s)", secret), false
		}
		tier, _ := command.Classify(cmd)
		danger = tier == "danger"
	}
	r.mu.Lock()
	allowed := r.askAllowed
	r.mu.Unlock()
	switch r.askMode() {
	case protocol.ApprovalsAsk:
		if allowed && !danger {
			return askAllows, "", false
		}
		return askAsks, "", danger
	case protocol.ApprovalsAuto:
		if danger {
			return askAsks, "", true
		}
		return askAllows, "", false
	default:
		return askRefuses, why, false
	}
}

// askRefused shows the user what Rashin refused for an Ask and why, as a
// request in the transcript it answered itself; request is "" for one the
// agent made without asking first. The agent hears the refusal itself and is
// briefed to explain it.
func (r *runner) askRefused(request, call, kind, title, reason string) {
	text := i18n.Tf("Rashin refused it: %s.", reason)
	if r.askMode() == protocol.ApprovalsReadOnly {
		text = i18n.Tf("This Ask is Read only, so Rashin refused it: %s. Switch the conversation to Ask first to let it make changes.", reason)
	}
	r.mu.Lock()
	if request == "" {
		r.synthetic++
		request = r.generationIDLocked("refused", fmt.Sprint(r.synthetic))
	}
	r.mu.Unlock()
	refused := protocol.PermissionOption{ID: "refused", Label: i18n.T("Refused"), Kind: protocol.OptRejectOnce}
	r.emit(protocol.EvPermission, protocol.PermissionData{Request: request, Call: call, Title: title, Kind: kind, Input: text, Options: []protocol.PermissionOption{refused}})
	r.emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Option: refused.ID, Outcome: "selected", By: "policy"})
}

// askOptions are the choices an Ask's permission request offers the user:
// the harness's own, less "allow always" for a destructive command.
func askOptions(options []protocol.PermissionOption, danger bool) []protocol.PermissionOption {
	if !danger {
		return options
	}
	kept := make([]protocol.PermissionOption, 0, len(options))
	for _, option := range options {
		if option.Kind != protocol.OptAllowAlways {
			kept = append(kept, option)
		}
	}
	return kept
}

// askAnswer is the option an Ask sends the harness for the one the user
// picked. "Always" is kept by Rashin, not the harness, so the harness asks
// again next time and a mode changed later still decides: allowing always
// allows changes for the rest of the conversation, refusing always refuses
// this once.
func (r *runner) askAnswer(picked string, options []protocol.PermissionOption) string {
	kind := ""
	for _, option := range options {
		if option.ID == picked {
			kind = option.Kind
		}
	}
	once := ""
	switch kind {
	case protocol.OptAllowAlways:
		r.mu.Lock()
		r.askAllowed = true
		r.mu.Unlock()
		once = protocol.OptAllowOnce
	case protocol.OptRejectAlways:
		once = protocol.OptRejectOnce
	default:
		return picked
	}
	for _, option := range options {
		if option.Kind == once {
			return option.ID
		}
	}
	return picked
}

// rejectOption is the harness option that turns a call down once, else for good.
func rejectOption(options []protocol.PermissionOption) string {
	reject := ""
	for _, option := range options {
		if option.Kind == protocol.OptRejectOnce {
			return option.ID
		}
		if reject == "" && option.Kind == protocol.OptRejectAlways {
			reject = option.ID
		}
	}
	return reject
}

// askReadable refuses a file an Ask reads for the agent when it holds a credential.
func askReadable(path string) error {
	if secret, blocked := command.SecretPath(path); blocked {
		return errors.New(i18n.Tf("Rashin does not read credential files for Ask (%s).", secret))
	}
	return nil
}

// terminalLine is the command line a terminal request runs: what a shell is
// handed with -c, else the program and its arguments, quoted as a shell would
// need them.
func terminalLine(program string, args []string) string {
	switch filepath.Base(program) {
	case "sh", "bash", "zsh", "dash", "fish":
		if len(args) == 2 && args[0] == "-c" {
			return args[1]
		}
	}
	words := make([]string, 0, len(args)+1)
	for _, word := range append([]string{program}, args...) {
		if word != "" && strings.Trim(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./=:,+@%") == "" {
			words = append(words, word)
			continue
		}
		words = append(words, "'"+strings.ReplaceAll(word, "'", `'\''`)+"'")
	}
	return strings.Join(words, " ")
}
