// Package agents is the task tool, which runs subagents inside the parent's
// turn through tool.Spawner, and the yield tool a subagent ends its run
// with. It also holds the bundled agent kinds and the texts the engine sends
// a child: its first user turn and the reminders to yield.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/task/*@c7fff67,
// packages/coding-agent/src/prompts/agents/{scout,task}.md@c7fff67,
// packages/coding-agent/src/prompts/system/{subagent-user-prompt,subagent-yield-reminder}.md@c7fff67.
package agents

import (
	"fmt"
	"strings"

	"ryoku-rashin/internal/engine/tool"
)

// Agent is a kind of subagent the task tool can run.
type Agent struct {
	Name        string
	Description string
	// ReadOnly kinds never get write tools, whatever the parent may do.
	ReadOnly bool
	// Prompt is the kind's body for the subagent system prompt; CompactPrompt
	// is the same for the compact class, written to the small-model rules.
	Prompt        string
	CompactPrompt string
}

// Bundled kind names.
const (
	Scout  = "scout"
	Worker = "worker"
)

var bundled = []Agent{
	{
		Name:        Scout,
		Description: "Read-only research: finds the files and code that matter and reports them with paths.",
		ReadOnly:    true,
		Prompt: `You are a scout. Investigate the code quickly and report what another agent needs, so it does not have to read everything again.

Work like this:
1. Locate the relevant code with search, find_files and grep; run several searches at once.
2. Read the key sections with line ranges; read a whole file only when it is small.
3. When a search finds nothing, try one other pattern or a wider folder before you conclude it is absent.
4. Note the types, functions and how the files depend on each other.

You only read: leave every file and the system as they are.

Call yield with data in this shape:
Summary: what you found, in a few lines.
Files: one line per file, path:start-end and what that code holds.
How it fits: how the pieces connect, in a few lines.
Report: the full table or list, only when the task asks for one.`,
		CompactPrompt: `You are a scout. Find the code the task asks about and report it.
Search with grep and find_files, then read the matching lines.
Report each finding as path:line and one sentence.
Finish with yield, putting the report in data.

Example of data:
Summary: the socket path is chosen in one function.
Files:
src/ipc/paths.go:40-62 resolveSocketPath picks the runtime dir.`,
	},
	{
		Name:        Worker,
		Description: "Does one delegated task end to end: reads, edits files and runs commands as the task needs.",
		Prompt: `You are a worker. Do the assigned task and nothing beyond it.

- Read only what the task needs; use line ranges for large files.
- Change existing files in preference to adding new ones; add documentation files only when the task asks.
- Leave builds, tests and formatters to the agent that sent you, unless the task asks you to run them.
- Keep your result short and factual: what you changed (paths), what you found, what is still open.

Finish with yield: data holds your result; error holds the exact blocker when you could not finish.`,
		CompactPrompt: `You are a worker. Do the task you are given, step by step, with your tools.
Change only the files the task names or needs.
When you are done, call yield with data: the files you changed and one line on each.
When something blocks you, call yield with error: the exact blocker.

Example of data:
Changed src/ipc/paths.go: resolveSocketPath now falls back to /tmp.`,
	},
}

// Lookup finds a bundled kind by name; "" is the worker.
func Lookup(name string) (Agent, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = Worker
	}
	for _, agent := range bundled {
		if agent.Name == name {
			return agent, true
		}
	}
	return Agent{}, false
}

// Bundled lists the kinds in the order the task tool describes them.
func Bundled() []Agent { return append([]Agent(nil), bundled...) }

// Assignment is a child's first user turn: the task it was given. The shared
// context goes into its system prompt, not here.
func Assignment(req tool.SpawnRequest) string {
	return "Complete this assignment:\n\n" + strings.TrimSpace(req.Task)
}

// MaxYieldReminders is how often a child that stopped without yield is told
// to call it before its last message is taken as its result.
const MaxYieldReminders = 3

// NoYieldWarning is appended to a result taken from a child's last message
// because it never called yield.
const NoYieldWarning = "\n\n[The subagent stopped without calling yield after 3 reminders; this is its last message.]"

// YieldReminder is the user turn sent to a child whose turn ended without a
// tool call: reminder n of max. The last one asks for yield alone.
func YieldReminder(n, max int, compact bool) string {
	if compact {
		if n >= max {
			return "<system-reminder>\nCall yield now. Put your result in data, or the exact blocker in error.\n</system-reminder>"
		}
		return fmt.Sprintf("<system-reminder>\nReminder %d of %d. Continue the task with your next tool call. When the task is done, call yield with your result in data.\n</system-reminder>", n, max)
	}
	if n >= max {
		return fmt.Sprintf("<system-reminder>\nReminder %d of %d. Call yield now with the best result from the work you completed: data holds the result and marks what is still open; error holds a real, concrete blocker.\n</system-reminder>", n, max)
	}
	return fmt.Sprintf(`<system-reminder>
Your last turn ended without a tool call. Reminder %d of %d.
Every turn ends with a tool call. Take the first that applies:
1. The assignment is not finished: call the next tool you need (read, edit, bash, search).
2. It is finished: call yield with your result in data.
3. A real blocker stops you (a missing file, an unavailable service, a contradictory task): call yield with error, saying what you tried and the exact blocker. This reminder is not a blocker.
</system-reminder>`, n, max)
}
