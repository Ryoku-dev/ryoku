package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// yieldTool ends a subagent's run with its result. A valid call returns End,
// with the result (or the failure) as Text, which the Spawner takes as the
// child's output; a bad call fails without End so the child tries again.
type yieldTool struct{}

func (yieldTool) Name() string    { return tool.Yield }
func (yieldTool) Kind() tool.Kind { return protocol.ToolOther }
func (yieldTool) ReadOnly() bool  { return true }

func (yieldTool) Spec(p tool.Profile) llm.Tool {
	params := json.RawMessage(`{"type":"object","properties":{` +
		`"data":{"type":"string","description":"Your result."},` +
		`"error":{"type":"string","description":"The exact blocker, only when you could not finish."}` +
		`}}`)
	if p.Compact() {
		return llm.Tool{Name: tool.Yield, Description: "Finish: data is your result; error is the exact blocker.", Parameters: params}
	}
	return llm.Tool{
		Name: tool.Yield,
		Description: `Finish your assignment and hand back your result. This ends your run.
- data: the result the agent that sent you needs: findings with paths, files changed, what is still open.
- error: only for a real, concrete blocker (a missing file, an unavailable service, a contradictory task): what you tried and what stops you.
Give one of the two.`,
		Parameters: params,
	}
}

func (yieldTool) Present(args json.RawMessage) tool.Presentation {
	if _, failed, problem := parseYield(args); problem == "" && failed {
		return tool.Presentation{Title: i18n.T("Report a blocker")}
	}
	return tool.Presentation{Title: i18n.T("Hand back the result")}
}

func (yieldTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	text, failed, problem := parseYield(call.Args)
	if problem != "" {
		return tool.Failed(problem)
	}
	return tool.Result{Text: text, IsError: failed, End: true}
}

// parseYield reads {data} or {error}. data may be any JSON value, which is
// kept as written; result and output stand in for data.
func parseYield(raw json.RawMessage) (text string, failed bool, problem string) {
	var args struct {
		Data   json.RawMessage `json:"data"`
		Result json.RawMessage `json:"result"`
		Output json.RawMessage `json:"output"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", false, "The arguments are not a JSON object: " + err.Error()
	}
	data := valueText(args.Data)
	if data == "" {
		data = valueText(args.Result)
	}
	if data == "" {
		data = valueText(args.Output)
	}
	blocker := strings.TrimSpace(args.Error)
	switch {
	case data != "" && blocker != "":
		return "", false, "Give data or error, not both."
	case blocker != "":
		return blocker, true, ""
	case data != "":
		return data, false, ""
	}
	return "", false, "Give data with your result, or error with the exact blocker."
}

// valueText is a JSON value as text: a string unquoted, anything else
// indented.
func valueText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var out bytes.Buffer
	if json.Indent(&out, raw, "", "  ") != nil {
		return string(raw)
	}
	return out.String()
}
