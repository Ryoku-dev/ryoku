// Package goal is the goal tool, through which an agent reads, sets, ends or
// reports blocked the session's goal (the daemon's session.goal), and the
// goal-mode texts the daemon sends on each goal turn.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/goals/tools/goal-tool.ts@c7fff67
// and packages/coding-agent/src/prompts/tools/goal.md@c7fff67.
package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Tools is the goal tool.
func Tools() []tool.Tool { return []tool.Tool{goalTool{}} }

type goalTool struct{}

func (goalTool) Name() string    { return tool.Goal }
func (goalTool) Kind() tool.Kind { return protocol.ToolThink }

// ReadOnly: the goal is Rashin's own session state.
func (goalTool) ReadOnly() bool { return true }

// Enabled: only an Agent session follows a goal.
func (goalTool) Enabled(env tool.Env) bool {
	return env.Session.Kind == protocol.KindAgent && env.Host != nil
}

func (goalTool) Spec(p tool.Profile) llm.Tool {
	params := json.RawMessage(`{"type":"object","properties":{` +
		`"op":{"type":"string","enum":["get","complete","block","set"],"description":"get, complete, block or set."},` +
		`"summary":{"type":"string","description":"For complete: what was done and how you checked it. For block: what stops you and what you need."},` +
		`"objective":{"type":"string","description":"For set: the objective in full."}` +
		`},"required":["op"]}`)
	if p.Compact() {
		return llm.Tool{Name: tool.Goal, Description: "Read the goal (get), finish it (complete), or say what blocks it (block).", Parameters: params}
	}
	return llm.Tool{
		Name: tool.Goal,
		Description: `The goal Rashin keeps sending you on toward, across turns.
- get: the objective and how much of its budget is spent.
- complete with a summary: ends the goal. Call it only once every deliverable is checked against the current state of the work; a spent budget or a finished turn is not completion.
- block with the exact blocker: ends the goal when you need the user.
- set with an objective: starts a goal when none is in progress, for long work the user asked you to see through.`,
		Parameters: params,
	}
}

func (goalTool) Present(args json.RawMessage) tool.Presentation {
	a, _ := parse(args)
	switch a.op {
	case protocol.GoalComplete:
		return tool.Presentation{Title: i18n.T("Mark the goal done"), Input: a.summary}
	case protocol.GoalBlock:
		return tool.Presentation{Title: i18n.T("Report the goal blocked"), Input: a.summary}
	case protocol.GoalSet:
		return tool.Presentation{Title: i18n.T("Set a goal"), Input: a.objective}
	}
	return tool.Presentation{Title: i18n.T("Read the goal")}
}

func (goalTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, problem := parse(call.Args)
	if problem != "" {
		return tool.Failed(problem)
	}
	if env.Host == nil || env.Session.ID == "" {
		return tool.Failed("This session has no goal to work with.")
	}
	params := protocol.GoalParams{ID: env.Session.ID, Op: a.op, Summary: a.summary}
	switch a.op {
	case protocol.GoalSet:
		if env.Context.Team != nil {
			return tool.Failed("A team task's goal is set by its team; report through your team tools instead.")
		}
		var info protocol.SessionInfo
		if err := env.Host.Call(ctx, protocol.MSessionGoal, protocol.GoalParams{ID: env.Session.ID, Op: protocol.GoalGet}, &info); err != nil {
			return tool.Failed(err.Error())
		}
		if g := info.Goal; g != nil && (g.Status == protocol.GoalActive || g.Status == protocol.GoalPaused) {
			return tool.Failed("A goal is already in progress: " + g.Objective + "\nWork on it; ask the user to set a different one if it should change.")
		}
		params = protocol.GoalParams{ID: env.Session.ID, Op: protocol.GoalSet, Objective: a.objective}
	}
	var info protocol.SessionInfo
	if err := env.Host.Call(ctx, protocol.MSessionGoal, params, &info); err != nil {
		return tool.Failed(err.Error())
	}
	g := info.Goal
	if g == nil {
		return tool.Result{Text: "No goal is set for this session."}
	}
	switch a.op {
	case protocol.GoalComplete:
		return tool.Result{Text: "The goal is marked done. End this turn with a short report for the user.", Shown: i18n.T("Goal done")}
	case protocol.GoalBlock:
		return tool.Result{Text: "The goal is marked blocked. End this turn by telling the user what you need.", Shown: i18n.T("Goal blocked")}
	case protocol.GoalSet:
		return tool.Result{Text: "The goal is set. Rashin will send you on toward it after this turn until you complete or block it.", Shown: i18n.T("Goal set")}
	}
	return tool.Result{Text: describe(*g)}
}

// describe is the goal as get reports it.
func describe(g protocol.Goal) string {
	text := fmt.Sprintf("Objective: %s\nStatus: %s\n%s", g.Objective, g.Status, budget(g))
	if g.Status == protocol.GoalPaused {
		text += "\nThe goal is paused: the user resumes it."
	}
	if g.Reason != "" {
		text += "\nReason: " + g.Reason
	}
	return text
}

type args struct {
	op, summary, objective string
}

// parse reads {op, summary, objective}; the words models use for each op
// stand in for it.
func parse(raw json.RawMessage) (args, string) {
	var in struct {
		Op        string `json:"op"`
		Action    string `json:"action"`
		Summary   string `json:"summary"`
		Reason    string `json:"reason"`
		Objective string `json:"objective"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return args{}, "The arguments are not a JSON object: " + err.Error()
	}
	op := strings.ToLower(strings.TrimSpace(in.Op))
	if op == "" {
		op = strings.ToLower(strings.TrimSpace(in.Action))
	}
	switch op {
	case "", "get", "status", "read", "show":
		op = protocol.GoalGet
	case "complete", "completed", "done", "finish", "finished":
		op = protocol.GoalComplete
	case "block", "blocked":
		op = protocol.GoalBlock
	case "set", "create", "start":
		op = protocol.GoalSet
	default:
		return args{}, fmt.Sprintf("Unknown op %q; use get, complete, block or set.", in.Op)
	}
	a := args{op: op, summary: strings.TrimSpace(in.Summary), objective: strings.TrimSpace(in.Objective)}
	if a.summary == "" {
		a.summary = strings.TrimSpace(in.Reason)
	}
	switch {
	case (op == protocol.GoalComplete || op == protocol.GoalBlock) && a.summary == "":
		if op == protocol.GoalComplete {
			return args{}, "Give a summary: what was done and how you checked it."
		}
		return args{}, "Give a summary: what blocks you and what you need from the user."
	case op == protocol.GoalSet && a.objective == "":
		return args{}, "Give the objective in full."
	}
	return a, ""
}
