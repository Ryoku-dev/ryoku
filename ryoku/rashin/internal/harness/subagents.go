package harness

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ryoku-rashin/internal/protocol"
)

// Harnesses report subagents two ways. omp's task tool streams its own
// progress in rawOutput.details; Claude Code reports an Agent call and then
// each call its subagent makes as a tool call that names the Agent call in
// _meta.claudeCode.parentToolUseId.

const (
	agentSteps       = 8
	agentOutputRunes = 4000
	agentLineRunes   = 200
)

type ompProgress struct {
	Index             int    `json:"index"`
	ID                string `json:"id"`
	Agent             string `json:"agent"`
	Status            string `json:"status"`
	Task              string `json:"task"`
	Description       string `json:"description"`
	LastIntent        string `json:"lastIntent"`
	CurrentTool       string `json:"currentTool"`
	CurrentToolArgs   string `json:"currentToolArgs"`
	CurrentToolIntent string `json:"currentToolIntent"`
	RecentTools       []struct {
		Tool    string `json:"tool"`
		Args    string `json:"args"`
		Intent  string `json:"intent"`
		IsError bool   `json:"isError"`
	} `json:"recentTools"`
	ToolCount     int    `json:"toolCount"`
	Tokens        int    `json:"tokens"`
	DurationMs    int64  `json:"durationMs"`
	ResolvedModel string `json:"resolvedModel"`
}

type ompResult struct {
	Index         int    `json:"index"`
	ID            string `json:"id"`
	Agent         string `json:"agent"`
	Task          string `json:"task"`
	Description   string `json:"description"`
	ExitCode      int    `json:"exitCode"`
	Output        string `json:"output"`
	Stderr        string `json:"stderr"`
	Error         string `json:"error"`
	Aborted       bool   `json:"aborted"`
	DurationMs    int64  `json:"durationMs"`
	Tokens        int    `json:"tokens"`
	ResolvedModel string `json:"resolvedModel"`
}

// ompAgents reads the subagents omp's task tool reports in its raw output;
// false when the output is not a task tool's
func ompAgents(raw json.RawMessage) ([]protocol.AgentWork, bool) {
	var output struct {
		Details *struct {
			Progress []ompProgress `json:"progress"`
			Results  []ompResult   `json:"results"`
		} `json:"details"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &output) != nil || output.Details == nil {
		return nil, false
	}
	details := output.Details
	if len(details.Progress) == 0 && len(details.Results) == 0 {
		return nil, false
	}
	type entry struct {
		index int
		work  protocol.AgentWork
	}
	entries := map[string]*entry{}
	key := func(id string, index int) string {
		if id != "" {
			return id
		}
		return "#" + strconv.Itoa(index)
	}
	for _, progress := range details.Progress {
		work := protocol.AgentWork{
			ID:        key(progress.ID, progress.Index),
			Name:      progress.Agent,
			Task:      oneLine(firstNonEmpty(progress.Description, idWords(progress.ID), progress.Task)),
			Status:    ompStatus(progress.Status),
			StepCount: progress.ToolCount,
			Tokens:    progress.Tokens,
			Model:     progress.ResolvedModel,
			Millis:    progress.DurationMs,
		}
		if work.Status == protocol.ToolRunning {
			work.Doing = oneLine(firstNonEmpty(progress.CurrentToolIntent, strings.TrimSpace(progress.CurrentTool+" "+progress.CurrentToolArgs), progress.LastIntent))
		}
		for i := len(progress.RecentTools) - 1; i >= 0; i-- {
			step := progress.RecentTools[i]
			work.Steps = append(work.Steps, protocol.AgentStep{Title: oneLine(firstNonEmpty(step.Intent, strings.TrimSpace(step.Tool+" "+step.Args))), Failed: step.IsError})
		}
		if len(work.Steps) > agentSteps {
			work.Steps = work.Steps[len(work.Steps)-agentSteps:]
		}
		entries[work.ID] = &entry{index: progress.Index, work: work}
	}
	for _, result := range details.Results {
		id := key(result.ID, result.Index)
		current := entries[id]
		if current == nil {
			current = &entry{index: result.Index, work: protocol.AgentWork{ID: id, Name: result.Agent, Task: oneLine(firstNonEmpty(result.Description, idWords(result.ID), result.Task))}}
			entries[id] = current
		}
		work := &current.work
		switch {
		case result.Aborted:
			work.Status = protocol.ToolStopped
		case result.ExitCode != 0 || result.Error != "":
			work.Status = protocol.ToolFailed
		default:
			work.Status = protocol.ToolDone
		}
		work.Doing = ""
		output := result.Output
		if work.Status == protocol.ToolFailed && strings.TrimSpace(output) == "" {
			output = firstNonEmpty(result.Error, result.Stderr)
		}
		work.Output = clipRunes(strings.TrimSpace(output), agentOutputRunes)
		if result.DurationMs > 0 {
			work.Millis = result.DurationMs
		}
		if result.Tokens > 0 {
			work.Tokens = result.Tokens
		}
		if result.ResolvedModel != "" {
			work.Model = result.ResolvedModel
		}
	}
	ordered := make([]*entry, 0, len(entries))
	for _, current := range entries {
		ordered = append(ordered, current)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })
	agents := make([]protocol.AgentWork, 0, len(ordered))
	for _, current := range ordered {
		agents = append(agents, current.work)
	}
	return agents, true
}

// keepAgentSteps carries what an earlier report said about each subagent's
// steps into a later one that leaves them out, as omp's final results do
func keepAgentSteps(previous, latest []protocol.AgentWork) []protocol.AgentWork {
	known := make(map[string]protocol.AgentWork, len(previous))
	for _, agent := range previous {
		known[agent.ID] = agent
	}
	for index := range latest {
		before, ok := known[latest[index].ID]
		if !ok {
			continue
		}
		agent := &latest[index]
		if agent.StepCount == 0 {
			agent.StepCount = before.StepCount
		}
		if len(agent.Steps) == 0 {
			agent.Steps = before.Steps
		}
		if agent.Model == "" {
			agent.Model = before.Model
		}
		if agent.Tokens == 0 {
			agent.Tokens = before.Tokens
		}
	}
	return latest
}

// idWords reads the CamelCase ID a model gave an omp task, such as
// ListScreenFiles, as words: List screen files. omp's own task text opens
// with a line of its template, so without a description the ID says more.
func idWords(id string) string {
	runes := []rune(id)
	var spaced strings.Builder
	for i, r := range runes {
		if r == '_' || r == '-' {
			spaced.WriteRune(' ')
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			previous := runes[i-1]
			endsAcronym := unicode.IsUpper(previous) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || endsAcronym {
				spaced.WriteRune(' ')
			}
		}
		spaced.WriteRune(r)
	}
	words := strings.Fields(spaced.String())
	for i := 1; i < len(words); i++ {
		if rest := strings.TrimLeftFunc(words[i], unicode.IsUpper); len(rest) == len(words[i])-1 && rest != "" {
			words[i] = strings.ToLower(words[i])
		}
	}
	return strings.Join(words, " ")
}

func ompStatus(status string) string {
	switch status {
	case "running":
		return protocol.ToolRunning
	case "completed":
		return protocol.ToolDone
	case "failed":
		return protocol.ToolFailed
	case "aborted":
		return protocol.ToolStopped
	}
	return protocol.ToolPending
}

// claudeMeta is what Claude Code says about a tool call beyond ACP
type claudeMeta struct {
	ClaudeCode struct {
		ToolName        string `json:"toolName"`
		ParentToolUseID string `json:"parentToolUseId"`
	} `json:"claudeCode"`
}

func readClaudeMeta(raw json.RawMessage) claudeMeta {
	var meta claudeMeta
	if len(raw) != 0 {
		_ = json.Unmarshal(raw, &meta)
	}
	return meta
}

// spawnsAgent tells an Agent call from Claude Code, the one tool that runs a
// subagent
func (meta claudeMeta) spawnsAgent() bool {
	return meta.ClaudeCode.ToolName == "Agent" || meta.ClaudeCode.ToolName == "Task"
}

// claudeAgent follows one Claude Code subagent from its Agent call and the
// calls it makes
type claudeAgent struct {
	work    protocol.AgentWork
	started time.Time
	order   []string
	steps   map[string]protocol.AgentStep
}

func newClaudeAgent(call string, input json.RawMessage, now time.Time) *claudeAgent {
	var args struct {
		Description  string `json:"description"`
		Prompt       string `json:"prompt"`
		SubagentType string `json:"subagent_type"`
	}
	_ = json.Unmarshal(input, &args)
	return &claudeAgent{
		work:    protocol.AgentWork{ID: call, Name: args.SubagentType, Task: oneLine(firstNonEmpty(args.Description, args.Prompt)), Status: protocol.ToolPending},
		started: now,
		steps:   map[string]protocol.AgentStep{},
	}
}

// follow takes the Agent call's own news: its status and, once done, what
// the subagent returned
func (agent *claudeAgent) follow(input json.RawMessage, data protocol.ToolData, now time.Time) {
	if fresh := newClaudeAgent(agent.work.ID, input, now); fresh.work.Task != "" {
		agent.work.Name, agent.work.Task = fresh.work.Name, fresh.work.Task
	}
	switch data.Status {
	case protocol.ToolRunning, protocol.ToolPending:
		agent.work.Status = data.Status
	case protocol.ToolDone, protocol.ToolFailed:
		agent.work.Status = data.Status
		agent.work.Doing = ""
		if data.Output != "" {
			agent.work.Output = clipRunes(strings.TrimSpace(data.Output), agentOutputRunes)
		}
	}
	agent.work.Millis = now.Sub(agent.started).Milliseconds()
}

// step takes the news of a call the subagent made
func (agent *claudeAgent) step(child protocol.ToolData, now time.Time) {
	if agent.work.Status == protocol.ToolPending {
		agent.work.Status = protocol.ToolRunning
	}
	step, seen := agent.steps[child.Call]
	if !seen {
		agent.order = append(agent.order, child.Call)
	}
	if child.Title != "" {
		step.Title = oneLine(child.Title)
	}
	step.Failed = step.Failed || child.Status == protocol.ToolFailed
	agent.steps[child.Call] = step
	if child.Status == protocol.ToolRunning || child.Status == protocol.ToolPending || !seen {
		agent.work.Doing = step.Title
	}
	agent.work.StepCount = len(agent.order)
	latest := agent.order[max(0, len(agent.order)-agentSteps):]
	agent.work.Steps = make([]protocol.AgentStep, 0, len(latest))
	for _, call := range latest {
		agent.work.Steps = append(agent.work.Steps, agent.steps[call])
	}
	agent.work.Millis = now.Sub(agent.started).Milliseconds()
}

func oneLine(text string) string {
	text, _, _ = strings.Cut(strings.TrimSpace(text), "\n")
	return clipRunes(strings.TrimSpace(text), agentLineRunes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
