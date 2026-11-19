package shell

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/bash.ts@c7fff67
// and packages/coding-agent/src/prompts/tools/bash.md@c7fff67 (timeout
// bounds, moving a long call to a background job, result lines).

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const (
	bashTimeout = 300 * time.Second
	bashKeep    = 256 * 1024
)

// backgroundAfter is how long bash waits before a still running command
// moves to a background job. A var so tests need not wait.
var backgroundAfter = 60 * time.Second

type bashTool struct{ jobs *jobTable }

func (bashTool) Name() string    { return tool.Bash }
func (bashTool) Kind() tool.Kind { return protocol.ToolExecute }
func (bashTool) ReadOnly() bool  { return false }

func (bashTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() || !tool.Offers(p.Mode, p.Class, tool.Wait) {
		return llm.Tool{
			Name:        tool.Bash,
			Description: "Run a shell command in the project folder. Use it to build, test, and run programs.",
			Parameters:  schema(`{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer","description":"Seconds, default 300."}},"required":["command"]}`),
		}
	}
	return llm.Tool{
		Name: tool.Bash,
		Description: "Run one bash command and return its output, stdout and stderr merged. " +
			"Each call starts a fresh shell in the session's folder, so cd does not carry over: pass cwd instead. " +
			"timeout is in seconds, default 300, from 1 to 3600. " +
			"A command still running after 60 s moves to a background job bg_N and keeps running until its timeout; async true moves it at once. " +
			"Background results do not arrive by themselves: call wait with the job id when you need one, and do other work meanwhile. " +
			"Prefer read, grep, find_files and edit over cat, grep, find and sed -i. " +
			"sudo, doas and su are refused: there is no terminal for a password.",
		Parameters: schema(`{"type":"object","properties":{` +
			`"command":{"type":"string"},` +
			`"timeout":{"type":"integer","minimum":1,"maximum":3600,"description":"Seconds, default 300."},` +
			`"cwd":{"type":"string","description":"Folder to run in, relative to the session's folder."},` +
			`"async":{"type":"boolean","description":"Start as a background job at once."}` +
			`},"required":["command"],"additionalProperties":false}`),
	}
}

func (bashTool) Present(args json.RawMessage) tool.Presentation {
	a, err := decodeCommand(args)
	if err != nil || a.Command == "" {
		return tool.Presentation{Title: i18n.T("Running a command"), Input: clipLine(string(args), 400)}
	}
	return tool.Presentation{Title: clipLine(a.Command, 80), Input: clip(a.Command, 400)}
}

func (b bashTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decodeCommand(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if a.Command == "" {
		return tool.Failed(i18n.T("The command is empty."))
	}
	timeout := bashTimeout
	if a.HasTimeout {
		timeout = time.Duration(min(max(a.Timeout, 1), 3600)) * time.Second
	}
	dir := env.Cwd
	if a.Cwd != "" {
		dir = a.Cwd
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(env.Cwd, dir)
		}
	}
	checked, err := checkedDirectory(ctx, env.Exec, dir, env.Cwd)
	if err != nil {
		return tool.Failed(err.Error())
	}
	dir = checked
	if command.Elevates(a.Command) {
		return tool.Failed(i18n.T("Refused: this needs a password on a terminal, which Rashin does not have. Give the user the command to run in their own terminal."))
	}
	if marker, secret := command.ReadsSecretAt(a.Command, dir); secret {
		return tool.Failed(i18n.Tf("Refused: this names %s, and what a command prints is sent to the model. If it is needed, ask the user to run it themselves.", marker))
	}
	if refused, ok := gate(ctx, call, env, clipLine(a.Command, 80), a.Command); !ok {
		return refused
	}
	emit(env, protocol.ToolData{Status: protocol.ToolRunning})

	req := runRequest{argv: []string{"bash", "-c", a.Command}, dir: dir, timeout: timeout, keep: bashKeep}
	// a job nobody can read back is no use: only a profile with wait moves one
	if tool.Offers(env.Profile.Mode, env.Profile.Class, tool.Wait) {
		req.async = a.Async
		if timeout > backgroundAfter {
			req.background = backgroundAfter
		}
		req.move = func(process string, started time.Time) string {
			return b.jobs.add(env.Session.ID, env.Exec, process, a.Command, timeout, started)
		}
	}
	o := runWatched(ctx, env.Exec, watcherFor(env, call), req)
	return bashResult(o, env, timeout)
}

func checkedDirectory(ctx context.Context, x exec.Executor, dir, cwd string) (string, error) {
	cur := filepath.Clean(dir)
	if marker, blocked := command.SecretPathAt(cur, cwd); blocked {
		return "", fmt.Errorf("Refused: working directory %s holds %s, which is never shown to a model", cur, marker)
	}
	for range 8 {
		parts := strings.Split(strings.TrimPrefix(cur, string(filepath.Separator)), string(filepath.Separator))
		path := string(filepath.Separator)
		followed := false
		for i, part := range parts {
			if part != "" {
				path = filepath.Join(path, part)
			}
			info, err := x.Stat(ctx, path)
			if err != nil {
				return "", fmt.Errorf("Working directory does not exist: %s: %w", path, err)
			}
			if info.Kind == exec.KindSymlink {
				dest := info.Target
				if !filepath.IsAbs(dest) {
					dest = filepath.Join(filepath.Dir(path), dest)
				}
				cur = filepath.Join(append([]string{dest}, parts[i+1:]...)...)
				if marker, blocked := command.SecretPathAt(cur, cwd); blocked {
					return "", fmt.Errorf("Refused: working directory %s holds %s, which is never shown to a model", cur, marker)
				}
				followed = true
				break
			}
			if info.Kind != exec.KindDir {
				return "", fmt.Errorf("Working directory is not a folder: %s", path)
			}
		}
		if !followed {
			return cur, nil
		}
	}
	return "", fmt.Errorf("Working directory has too many symlinks: %s", dir)
}

// bashResult is what the model is told of one bash call.
func bashResult(o runOutcome, env tool.Env, timeout time.Duration) tool.Result {
	l := limitsOf(env)
	switch {
	case o.err != nil:
		return tool.Failed(i18n.Tf("The command could not run: %v", o.err))
	case o.stopped:
		return tool.Failed(i18n.T("Stopped before it finished."))
	case o.job != "":
		text := i18n.Tf("Still running after %d s, so it is now background job %s. It keeps running until it has run %d s in total. Call wait with id %s when you need its result, and do other work meanwhile.",
			seconds(o.exit.Elapsed), o.job, seconds(timeout), o.job)
		if so := output(o.exit); so != "" {
			text += "\n\n" + i18n.T("Output so far:") + "\n" + shape(lastLines(so, l.TailLines), l)
		}
		return tool.Result{Text: text}
	}
	text := output(o.exit)
	if text == "" {
		text = i18n.T("(no output)")
	}
	if status, failed := exitLine(o.exit); failed {
		return tool.Result{Text: shape(text+"\n\n"+status, l), IsError: true}
	}
	return tool.Result{Text: shape(text, l)}
}

// exitLine says how a process that did not succeed ended.
func exitLine(exit exec.Exit) (string, bool) {
	switch {
	case exit.TimedOut:
		return i18n.Tf("Command timed out after %d s.", seconds(exit.Elapsed)), true
	case exit.Killed:
		return i18n.T("Command was stopped."), true
	case exit.Signal != "":
		return i18n.Tf("Command was killed by signal %s.", exit.Signal), true
	case exit.Code != 0:
		return i18n.Tf("Command exited with code %d.", exit.Code), true
	}
	return "", false
}
