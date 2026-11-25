// Package shell holds the engine's command tools: bash for Agent, team and
// subagent sessions, run_command and system_query for Ask, and wait for the
// background jobs bash leaves running. Every process starts through
// tool.Env.Exec, so the same tool runs on the user's machine or inside a team
// task's sandbox, and every change passes tool.Env.Permissions with the
// command classifier's tier.
package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// Tools are the command tools. bash and wait share one job table, so wait
// finds the jobs bash moved to the background.
func Tools() []tool.Tool {
	jobs := newJobTable()
	return []tool.Tool{
		bashTool{jobs: jobs},
		waitTool{jobs: jobs},
		systemQueryTool{},
		runCommandTool{},
	}
}

// commandArgs are the arguments of every command tool. Small models send
// the names omp accepts too, so cmd counts as command.
type commandArgs struct {
	Command string
	Why     string
	Cwd     string
	Timeout int
	// HasTimeout tells an absent timeout from zero.
	HasTimeout bool
	Async      bool
}

func decodeCommand(raw json.RawMessage) (commandArgs, error) {
	fields, err := decodeFields(raw)
	if err != nil {
		return commandArgs{}, err
	}
	var a commandArgs
	a.Command = strings.TrimSpace(stringField(fields, "command", "cmd"))
	a.Why = strings.TrimSpace(stringField(fields, "why", "description"))
	a.Cwd = strings.TrimSpace(stringField(fields, "cwd", "workdir"))
	a.Timeout, a.HasTimeout = intField(fields, "timeout")
	a.Async = boolField(fields, "async")
	return a, nil
}

func decodeFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return fields, nil
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func stringField(fields map[string]json.RawMessage, names ...string) string {
	for _, name := range names {
		var s string
		if v, ok := fields[name]; ok && json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

func intField(fields map[string]json.RawMessage, names ...string) (int, bool) {
	for _, name := range names {
		v, ok := fields[name]
		if !ok {
			continue
		}
		var n int64
		if json.Unmarshal(v, &n) == nil {
			return int(n), true
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			if parsed, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func boolField(fields map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		v, ok := fields[name]
		if !ok {
			continue
		}
		var b bool
		if json.Unmarshal(v, &b) == nil {
			return b
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			return strings.EqualFold(strings.TrimSpace(s), "true")
		}
	}
	return false
}

func schema(s string) json.RawMessage { return json.RawMessage(s) }

func emit(env tool.Env, d protocol.ToolData) {
	if env.Emit != nil {
		env.Emit(d)
	}
}

// limitsOf is the turn's limits, or the profile's when the engine left them
// out.
func limitsOf(env tool.Env) tool.Limits {
	if env.Limits.Told > 0 {
		return env.Limits
	}
	return env.Profile.Limits()
}

// shape keeps output under what the model is told, start and end both: a
// listing is read from its top, and what went wrong is said at the bottom.
func shape(text string, l tool.Limits) string {
	if len(text) <= l.Told {
		return text
	}
	return tool.HeadTail(text, l.Head, l.Tail)
}

// lastLines keeps the last n lines of text.
func lastLines(text string, n int) string {
	if n <= 0 {
		return text
	}
	end := len(text)
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '\n' && i != len(text)-1 {
			n--
			if n == 0 {
				return text[i+1 : end]
			}
		}
	}
	return text
}

// changeTier is the tier a command is put to the gate with. The classifier
// is lenient where the read-only check is strict; a command the read-only
// check cannot prove a read is at least a write, so it never runs unasked
// as a read would.
func changeTier(cmd string) string {
	if command.ReadOnly(cmd) {
		return protocol.TierRead
	}
	tier, _ := command.Classify(cmd)
	if tier == protocol.TierRead {
		return protocol.TierWrite
	}
	return tier
}

// output is a process's retained output, trimmed, with a note when the
// executor dropped part of its middle.
func output(exit exec.Exit) string {
	text := strings.TrimSpace(exit.Output)
	if exit.Truncated {
		text += "\n[output trimmed]"
	}
	return text
}

// seconds renders a duration as whole seconds, at least one.
func seconds(d time.Duration) int {
	return max(1, int(d.Round(time.Second).Seconds()))
}

func clipLine(s string, runes int) string {
	if line, _, found := strings.Cut(s, "\n"); found {
		s = line + " …"
	}
	clipped, cut := tool.ClipRunes(s, runes)
	if cut {
		return clipped + "…"
	}
	return clipped
}

// stopContext is for cleanup after ctx may have ended.
func stopContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), stopGrace)
}
