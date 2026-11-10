package ask

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// act.go is what Ask may do once the user lets it change the machine: run a
// command and write a file. A read-only Ask is never offered either tool. With
// approvals "ask" every change waits for the user's answer; with "auto" only a
// destructive command does, so the one mistake that cannot be taken back is
// still looked at by a person.

const (
	actTimeout    = 2 * time.Minute
	actOutput     = 24 * 1024
	writeLimit    = 256 * 1024
	replaceLimit  = 512 * 1024
	actDiffRunes  = 4000
	optionAllow   = "allow"
	optionAlways  = "always"
	optionRefuse  = "refuse"
	refusedToTool = "The user refused this. Do not try the same change another way. Tell them it was not done and ask what they want instead."
)

var actToolset = []llm.Tool{
	{Name: "run_command", Description: "Run one shell command on this machine as the user. A command that only reads runs at once. One that changes something follows the approval rule in your instructions. It cannot become root.", Parameters: schema(`{"type":"object","properties":{"command":{"type":"string"},"why":{"type":"string","description":"One short line the user reads before allowing it."}},"required":["command","why"],"additionalProperties":false}`)},
	{Name: "write_file", Description: "Create or replace one whole text file the user owns. Read the file first, then send its complete new content.", Parameters: schema(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"},"why":{"type":"string","description":"One short line the user reads before allowing it."}},"required":["path","content","why"],"additionalProperties":false}`)},
}

// acts reports whether mode lets Ask change the machine.
func acts(mode string) bool {
	return mode == protocol.ApprovalsAsk || mode == protocol.ApprovalsAuto
}

func actTool(name string) bool { return name == "run_command" || name == "write_file" }

// waits reports whether one change waits for the user. Reading never does and
// a destructive command always does; between them the mode decides, unless
// the user already allowed changes for this conversation.
func waits(mode, tier string, readOnly, allowed bool) bool {
	switch {
	case readOnly:
		return false
	case tier == "danger":
		return true
	case mode == protocol.ApprovalsAuto:
		return false
	}
	return !allowed
}

type pendingRequest struct {
	answer  chan string
	options []protocol.PermissionOption
	sink    core.Sink
}

// act runs one of the two tools that change the machine and reports it as a
// tool line of its own, since what it shows (a command, a file's change)
// differs from the read tools.
func (r *runner) act(ctx context.Context, call llm.ToolCall, sink core.Sink, cwd string) (string, string) {
	mode := r.currentMode()
	if !acts(mode) {
		output := i18n.T("Refused: Ask is read-only in this conversation. Use propose so the user can review and run it.")
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: call.Name, Kind: protocol.ToolOther, Status: protocol.ToolFailed, Output: output})
		return output, protocol.ToolFailed
	}
	if call.Name == "write_file" {
		return r.writeFile(ctx, call, sink, mode, cwd)
	}
	return r.runCommand(ctx, call, sink, mode, cwd)
}

func (r *runner) runCommand(ctx context.Context, call llm.ToolCall, sink core.Sink, mode, cwd string) (string, string) {
	var args struct {
		Command string `json:"command"`
		Why     string `json:"why"`
	}
	title := i18n.T("Running a command")
	fail := func(output string) (string, string) {
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: title, Kind: protocol.ToolExecute, Status: protocol.ToolFailed, Input: args.Command, Output: output})
		return output, protocol.ToolFailed
	}
	if err := decodeArgs(call.Arguments, &args); err != nil {
		return fail(err.Error())
	}
	args.Command = strings.TrimSpace(args.Command)
	if why := strings.TrimSpace(args.Why); why != "" {
		title = truncateRunes(why, 80)
	}
	if args.Command == "" {
		return fail(i18n.T("The command is empty."))
	}
	if command.Elevates(args.Command) {
		return fail(i18n.T("Refused: this needs a password on a terminal, which Ask does not have. Use propose so the user can run it there."))
	}
	tier, _ := command.Classify(args.Command)
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: title, Kind: protocol.ToolExecute, Status: protocol.ToolPending, Input: args.Command})
	if waits(mode, tier, command.ReadOnly(args.Command), r.allowedAll()) {
		switch r.cleared(ctx, sink, call.ID, title, protocol.ToolExecute, args.Command, tier != "danger") {
		case answerCancelled:
			return fail(i18n.T("Cancelled before it ran."))
		case answerRefused:
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolFailed, Output: i18n.T("Not run: you refused it.")})
			return refusedToTool, protocol.ToolFailed
		}
	}
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolRunning})
	output, err := runBounded(ctx, actTimeout, cwd, actOutput, "sh", "-c", args.Command)
	status := protocol.ToolDone
	if err != nil {
		status = protocol.ToolFailed
		output = strings.TrimSpace(output + "\n" + err.Error())
	}
	if output == "" {
		output = i18n.T("Done, with no output.")
	}
	output = truncateRunes(output, 6000)
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: status, Output: output})
	return output, status
}

func (r *runner) writeFile(ctx context.Context, call llm.ToolCall, sink core.Sink, mode, cwd string) (string, string) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Why     string `json:"why"`
	}
	title := i18n.T("Writing a file")
	path := ""
	fail := func(output string) (string, string) {
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: title, Kind: protocol.ToolEdit, Status: protocol.ToolFailed, Input: path, Output: output})
		return output, protocol.ToolFailed
	}
	if err := decodeArgs(call.Arguments, &args); err != nil {
		return fail(err.Error())
	}
	if strings.TrimSpace(args.Path) == "" {
		return fail(i18n.T("The file path is empty."))
	}
	path = resolvePath(args.Path, cwd)
	// the line already shows the path beside its title, so the title says why
	if why := strings.TrimSpace(args.Why); why != "" {
		title = truncateRunes(why, 80)
	}
	target, previous, existed, err := writable(path, args.Content)
	if err != nil {
		return fail(err.Error())
	}
	diff := &protocol.Diff{Path: path}
	var cutOld, cutNew bool
	diff.Old, cutOld = clipRunes(previous, actDiffRunes)
	diff.New, cutNew = clipRunes(args.Content, actDiffRunes)
	diff.Truncated = cutOld || cutNew
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: title, Kind: protocol.ToolEdit, Status: protocol.ToolPending, Input: path, Diff: diff})
	if previous == args.Content && existed {
		output := i18n.T("The file already says exactly this. Nothing was written.")
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolDone, Output: output})
		return output, protocol.ToolDone
	}
	if waits(mode, "write", false, r.allowedAll()) {
		switch r.cleared(ctx, sink, call.ID, title, protocol.ToolEdit, path, true) {
		case answerCancelled:
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolFailed, Output: i18n.T("Cancelled before it was written.")})
			return i18n.T("Cancelled before it was written."), protocol.ToolFailed
		case answerRefused:
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolFailed, Output: i18n.T("Not written: you refused it.")})
			return refusedToTool, protocol.ToolFailed
		}
	}
	kept := ""
	if existed {
		kept, err = r.keep(path, previous)
		if err != nil {
			output := i18n.Tf("Could not keep a copy of the file before changing it, so it was left alone: %v", err)
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolFailed, Output: output})
			return output, protocol.ToolFailed
		}
	}
	if err := replaceFile(target, args.Content, existed); err != nil {
		output := writeError(path, err)
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolFailed, Output: output})
		return output, protocol.ToolFailed
	}
	output := i18n.Tf("Created %s.", path)
	if existed {
		output = i18n.Tf("Wrote %s. The version before this change is kept at %s.", path, kept)
	}
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: protocol.ToolDone, Output: output})
	return output, protocol.ToolDone
}

// writable checks one write before anyone is asked about it, and returns the
// file the bytes go to (a link is followed, so the link itself survives) and
// what it holds now.
func writable(path, content string) (target, previous string, existed bool, err error) {
	if marker, blocked := command.SecretPath(path); blocked {
		return "", "", false, fmt.Errorf(i18n.T("Refused to write %s because it contains %s."), path, marker)
	}
	if len(content) > writeLimit {
		return "", "", false, errors.New(i18n.T("The new content is over 256 KiB. Ask writes configuration and notes, not data files."))
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return "", "", false, errors.New(i18n.T("The new content is not text."))
	}
	target = path
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		target = resolved
	}
	if marker, blocked := command.SecretPath(target); blocked {
		return "", "", false, fmt.Errorf(i18n.T("Refused to write %s because it contains %s."), target, marker)
	}
	info, statErr := os.Stat(target)
	switch {
	case errors.Is(statErr, fs.ErrNotExist):
		if parent, parentErr := os.Stat(filepath.Dir(target)); parentErr != nil || !parent.IsDir() {
			return "", "", false, fmt.Errorf(i18n.T("The folder %s does not exist. Create it first with run_command."), filepath.Dir(target))
		}
		return target, "", false, nil
	case statErr != nil:
		return "", "", false, statErr
	case !info.Mode().IsRegular():
		return "", "", false, errors.New(i18n.T("Refused to write over something that is not a regular file."))
	case info.Size() > replaceLimit:
		return "", "", false, errors.New(i18n.T("The file is over 512 KiB, too large to replace whole. Change it with a command instead."))
	}
	body, readErr := os.ReadFile(target)
	if readErr != nil {
		return "", "", false, readErr
	}
	if !utf8.Valid(body) || strings.ContainsRune(string(body), 0) {
		return "", "", false, errors.New(i18n.T("The file is not text, so Ask will not replace it."))
	}
	return target, string(body), true, nil
}

// keep saves what a file held before Ask changed it, beside the session's log.
func (r *runner) keep(path, previous string) (string, error) {
	dir := filepath.Join(paths.SessionDir(r.opts.Session.ID), "before")
	if err := paths.EnsureDir(dir); err != nil {
		return "", err
	}
	r.mu.Lock()
	r.kept++
	name := fmt.Sprintf("%03d-%s", r.kept, filepath.Base(path))
	r.mu.Unlock()
	copyPath := filepath.Join(dir, name)
	if err := os.WriteFile(copyPath, []byte(previous), 0o600); err != nil {
		return "", err
	}
	return copyPath, nil
}

// replaceFile writes content beside the file and renames it into place, so a
// reader never sees half of it. A new file is private to the user; a replaced
// one keeps its mode.
func replaceFile(target, content string, existed bool) error {
	mode := fs.FileMode(0o600)
	if existed {
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".rashin-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.WriteString(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, target)
}

func writeError(path string, err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return i18n.Tf("Could not write %s: it is not yours to change. A file under the system belongs to its package or to root; use propose for a command the user runs in a terminal.", path)
	}
	return i18n.Tf("Could not write %s: %v", path, err)
}

const (
	answerAllowed = iota
	answerRefused
	answerCancelled
)

// cleared puts one change to the user and waits for the answer. always offers
// the choice that stops the asking for the rest of the conversation, which a
// destructive command never gets.
func (r *runner) cleared(ctx context.Context, sink core.Sink, call, title, kind, input string, always bool) int {
	options := []protocol.PermissionOption{{ID: optionAllow, Label: i18n.T("Allow once"), Kind: protocol.OptAllowOnce}}
	if always {
		options = append(options, protocol.PermissionOption{ID: optionAlways, Label: i18n.T("Allow in this conversation"), Kind: protocol.OptAllowAlways})
	}
	options = append(options, protocol.PermissionOption{ID: optionRefuse, Label: i18n.T("Refuse"), Kind: protocol.OptRejectOnce})
	answer := make(chan string, 1)
	r.mu.Lock()
	r.asked++
	request := fmt.Sprintf("%s-ask-%d", call, r.asked)
	if r.pending == nil {
		r.pending = make(map[string]pendingRequest)
	}
	r.pending[request] = pendingRequest{answer: answer, options: options, sink: sink}
	r.mu.Unlock()
	sink.Emit(protocol.EvPermission, protocol.PermissionData{Request: request, Call: call, Title: title, Kind: kind, Input: input, Options: options})
	select {
	case option := <-answer:
		if option == optionRefuse {
			return answerRefused
		}
		return answerAllowed
	case <-ctx.Done():
		r.mu.Lock()
		_, open := r.pending[request]
		delete(r.pending, request)
		r.mu.Unlock()
		if open {
			sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Outcome: "cancelled", By: "user"})
		}
		return answerCancelled
	}
}

func (r *runner) currentMode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

func (r *runner) allowedAll() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.allowed
}

// clipRunes cuts text to limit runes and says whether it had to.
func clipRunes(text string, limit int) (string, bool) {
	if utf8.RuneCountInString(text) <= limit {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:limit]), true
}
