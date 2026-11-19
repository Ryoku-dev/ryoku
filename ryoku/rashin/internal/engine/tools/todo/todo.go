// Package todo is the todo tool: the model sends its whole task list each
// time, and the list replaces the session's plan (a plan event).
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/todo.ts@c7fff67
// and packages/coding-agent/src/prompts/tools/todo.md@c7fff67.
package todo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Plan entry statuses (protocol.PlanEntry).
const (
	Pending    = "pending"
	InProgress = "in_progress"
	Completed  = "completed"
)

const (
	// MaxEntries caps the list; a longer one is refused so nothing is lost
	// silently.
	MaxEntries   = 50
	contentRunes = 200
)

// Tools is the todo tool.
func Tools() []tool.Tool { return []tool.Tool{todoTool{}} }

type todoTool struct{}

func (todoTool) Name() string    { return tool.Todo }
func (todoTool) Kind() tool.Kind { return protocol.ToolThink }
func (todoTool) ReadOnly() bool  { return true }

func (todoTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return llm.Tool{
			Name:        tool.Todo,
			Description: "Save your whole task list: [x] done, [>] doing now, [ ] to do.",
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"items":{"type":"array","items":{"type":"string"},"description":"Every task, in order, each starting with [x], [>] or [ ]."}` +
				`},"required":["items"]}`),
		}
	}
	return llm.Tool{
		Name: tool.Todo,
		Description: `Keep the task list the user sees. Each call sends the whole list, which replaces the previous one.
- Write it before work that takes 3 or more steps, or when the user gives a list of things to do: list every item they gave separately.
- Mark a task completed as soon as it is done; keep exactly one in_progress, the one you work on now.
- Name tasks by what they achieve, and keep each name the same across calls.
- Call it in the same response as your next real tool call, not on its own.`,
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"todos":{"type":"array","items":{"type":"object","properties":{` +
			`"content":{"type":"string","description":"The task."},` +
			`"status":{"type":"string","enum":["pending","in_progress","completed"]}` +
			`},"required":["content","status"]}}` +
			`},"required":["todos"]}`),
	}
}

func (todoTool) Present(args json.RawMessage) tool.Presentation {
	entries, problem := parse(args)
	if problem != "" {
		return tool.Presentation{Title: i18n.T("Update the plan"), Input: string(args)}
	}
	done := 0
	for _, entry := range entries {
		if entry.Status == Completed {
			done++
		}
	}
	return tool.Presentation{Title: i18n.T("Update the plan"), Input: i18n.Tf("%d of %d done", done, len(entries))}
}

func (todoTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	entries, problem := parse(call.Args)
	if problem != "" {
		return tool.Failed(problem)
	}
	entries = normalize(entries)
	text := summary(entries)
	return tool.Result{Text: text, Shown: text, Plan: &protocol.PlanData{Entries: entries}}
}

// parse reads todos (objects, or strings with markers) or items (strings
// with markers, or objects).
func parse(raw json.RawMessage) ([]protocol.PlanEntry, string) {
	var args struct {
		Todos json.RawMessage `json:"todos"`
		Items json.RawMessage `json:"items"`
		Tasks json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, "The arguments are not a JSON object: " + err.Error()
	}
	list := args.Todos
	for _, other := range []json.RawMessage{args.Items, args.Tasks} {
		if len(bytes.TrimSpace(list)) == 0 || string(bytes.TrimSpace(list)) == "null" {
			list = other
		}
	}
	list = bytes.TrimSpace(list)
	if len(list) == 0 || string(list) == "null" {
		return nil, "Give the whole task list."
	}
	if list[0] == '"' {
		// A list written as a string: a JSON array inside it, or one task per line.
		var s string
		if err := json.Unmarshal(list, &s); err != nil {
			return nil, "The task list is not readable: " + err.Error()
		}
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "[") && json.Valid([]byte(s)) {
			list = json.RawMessage(s)
		} else {
			lines := []json.RawMessage{}
			for _, line := range strings.Split(s, "\n") {
				if strings.TrimSpace(line) != "" {
					quoted, _ := json.Marshal(line)
					lines = append(lines, quoted)
				}
			}
			list, _ = json.Marshal(lines)
		}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(list, &items); err != nil {
		return nil, "The task list must be a list: " + err.Error()
	}
	if len(items) > MaxEntries {
		return nil, fmt.Sprintf("At most %d tasks; group the smaller ones.", MaxEntries)
	}
	entries := make([]protocol.PlanEntry, 0, len(items))
	for n, item := range items {
		var entry protocol.PlanEntry
		item = bytes.TrimSpace(item)
		if len(item) > 0 && item[0] == '"' {
			var line string
			if err := json.Unmarshal(item, &line); err != nil {
				return nil, fmt.Sprintf("Task %d is not readable: %v", n+1, err)
			}
			entry = marked(line)
		} else {
			var object struct {
				Content  string `json:"content"`
				Task     string `json:"task"`
				Text     string `json:"text"`
				Status   string `json:"status"`
				Done     bool   `json:"done"`
				Priority string `json:"priority"`
			}
			if err := json.Unmarshal(item, &object); err != nil {
				return nil, fmt.Sprintf("Task %d is not readable: %v", n+1, err)
			}
			content := firstNonEmpty(object.Content, object.Task, object.Text)
			entry = marked(content)
			if status, ok := status(object.Status); ok {
				entry.Status = status
			} else if object.Status != "" {
				return nil, fmt.Sprintf("Task %d has status %q; use pending, in_progress or completed.", n+1, object.Status)
			}
			if object.Done {
				entry.Status = Completed
			}
			entry.Priority = strings.TrimSpace(object.Priority)
		}
		if entry.Content == "" {
			return nil, fmt.Sprintf("Task %d is empty.", n+1)
		}
		if entry.Status == "dropped" {
			continue
		}
		entry.Content = protocol.ClipWords(entry.Content, contentRunes)
		entries = append(entries, entry)
	}
	return entries, ""
}

// marked reads a task line with an optional marker: [x] done, [>] or [~]
// doing, [ ] to do, [-] dropped; a bullet or number before it is dropped.
func marked(line string) protocol.PlanEntry {
	line = strings.TrimSpace(line)
	line = strings.TrimSpace(strings.TrimLeft(line, "-*•"))
	if i := strings.IndexAny(line, ".)"); i > 0 && i <= 3 && isDigits(line[:i]) && i+1 < len(line) && line[i+1] == ' ' {
		line = strings.TrimSpace(line[i+1:])
	}
	entry := protocol.PlanEntry{Status: Pending}
	if len(line) >= 3 && line[0] == '[' && line[2] == ']' {
		switch line[1] {
		case 'x', 'X':
			entry.Status = Completed
		case '>', '~', '/', '*':
			entry.Status = InProgress
		case '-':
			entry.Status = "dropped"
		}
		line = strings.TrimSpace(line[3:])
	}
	entry.Content = line
	return entry
}

// status reads the statuses models write for the three plan states.
func status(value string) (string, bool) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case "pending", "todo", "to_do", "open", "not_started":
		return Pending, true
	case "in_progress", "inprogress", "active", "doing", "started", "current":
		return InProgress, true
	case "completed", "complete", "done", "finished":
		return Completed, true
	case "cancelled", "canceled", "dropped", "abandoned", "skipped":
		return "dropped", true
	}
	return "", false
}

// normalize keeps one task in progress: the earliest of several, else the
// earliest pending one once a task is done or work has begun, as omp does.
func normalize(entries []protocol.PlanEntry) []protocol.PlanEntry {
	active := -1
	for i := range entries {
		if entries[i].Status != InProgress {
			continue
		}
		if active < 0 {
			active = i
			continue
		}
		entries[i].Status = Pending
	}
	if active < 0 {
		for i := range entries {
			if entries[i].Status == Pending {
				entries[i].Status = InProgress
				break
			}
		}
	}
	return entries
}

// summary tells the model where the list stands.
func summary(entries []protocol.PlanEntry) string {
	if len(entries) == 0 {
		return "The task list is empty."
	}
	done, now, next := 0, "", ""
	for _, entry := range entries {
		switch entry.Status {
		case Completed:
			done++
		case InProgress:
			now = entry.Content
		case Pending:
			if next == "" {
				next = entry.Content
			}
		}
	}
	text := fmt.Sprintf("Saved %d tasks, %d done.", len(entries), done)
	if now != "" {
		text += " Now: " + now + "."
	}
	if next != "" {
		text += " Next: " + next + "."
	}
	if done == len(entries) {
		text += " Every task is done."
	}
	return text
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
