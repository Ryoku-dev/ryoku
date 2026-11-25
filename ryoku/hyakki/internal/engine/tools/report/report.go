// Package report is the report_issue tool (feature grievances): the model's
// own report that a tool did not behave as its description says. Reports are
// kept on this computer for the Learning page; the tool answers at once and
// never blocks the turn.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/report-tool-issue.ts@c7fff67.
package report

import (
	"context"
	"encoding/json"
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Store records a report: *grievances.Store.
type Store interface {
	Add(ctx context.Context, g protocol.Grievance) (protocol.Grievance, error)
}

// Tools returns report_issue. b is told learning.changed after each report;
// nil tells no one.
func Tools(store Store, b core.Broadcaster) []tool.Tool {
	return []tool.Tool{&reportIssue{store: store, broadcast: b}}
}

// noted is all the model hears back: a report never changes its course.
const noted = "Noted, thanks."

const fullDescription = `Automated QA: report a tool whose output did not match its description or the arguments you gave it (wrong lines, ignored parameter, garbled output, a false error). Name the tool and say in one or two sentences what you expected and what you got. False positives are fine. The report is kept on this computer for the user; it does not change the tool or retry anything.`

const compactDescription = `Report a tool that did not behave as described.`

var schema = json.RawMessage(`{"type":"object","properties":{"tool":{"type":"string","description":"The tool's name"},"report":{"type":"string","description":"What you expected and what happened"}},"required":["tool","report"],"additionalProperties":false}`)

type reportIssue struct {
	store     Store
	broadcast core.Broadcaster
}

func (t *reportIssue) Name() string { return tool.ReportIssue }

func (t *reportIssue) Spec(p tool.Profile) llm.Tool {
	description := fullDescription
	if p.Compact() {
		description = compactDescription
	}
	return llm.Tool{Name: tool.ReportIssue, Description: description, Parameters: schema}
}

func (t *reportIssue) Kind() tool.Kind { return protocol.ToolOther }

func (t *reportIssue) ReadOnly() bool { return true }

// Enabled offers the tool only while the grievances feature is on.
func (t *reportIssue) Enabled(env tool.Env) bool {
	return t.store != nil && env.Feature(protocol.FeatureGrievances)
}

func (t *reportIssue) Present(args json.RawMessage) tool.Presentation {
	name, report, ok := parse(args)
	if !ok {
		return tool.Presentation{Title: i18n.T("Reported a tool issue"), Input: string(args)}
	}
	return tool.Presentation{Title: i18n.Tf("Reported an issue with %s", name), Input: report}
}

func (t *reportIssue) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if t.store == nil || !env.Feature(protocol.FeatureGrievances) {
		return tool.Failed("report_issue is turned off.")
	}
	name, report, ok := parse(call.Args)
	if !ok {
		return tool.Failed(`Give "tool" (the tool's name) and "report" (what you expected and what happened).`)
	}
	g := protocol.Grievance{
		Tool:    name,
		Report:  report,
		Source:  source(name),
		Model:   env.Session.Model,
		Session: env.Session.ID,
	}
	if _, err := t.store.Add(ctx, g); err != nil {
		// The model cannot fix a full disk; it hears the same answer.
		return tool.Result{Text: noted, Shown: i18n.T("Could not keep the report")}
	}
	if t.broadcast != nil {
		t.broadcast.Broadcast(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: protocol.LearningGrievances})
	}
	return tool.Result{Text: noted, Shown: i18n.T("Kept for the Learning page")}
}

// parse reads the arguments the way small models write them: the named
// fields, their common aliases, or one text field in "tool: report" form.
func parse(args json.RawMessage) (name, report string, ok bool) {
	var v map[string]any
	if json.Unmarshal(args, &v) != nil {
		return "", "", false
	}
	name = first(v, "tool", "toolName", "tool_name", "name")
	report = first(v, "report", "description", "issue", "error", "message", "problem", "details", "body")
	if name == "" && report != "" {
		name, report = split(report)
	}
	if name == "" || report == "" {
		return "", "", false
	}
	return strings.TrimPrefix(name, "xd://"), report, true
}

func first(v map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// split reads "tool: report" or a tool on the first line. Only a colon and
// a space separate, so a URI in the tool name stays whole.
func split(text string) (string, string) {
	if i := strings.IndexByte(text, '\n'); i > 0 {
		if name, rest := strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:]); name != "" && rest != "" && !strings.Contains(name, " ") {
			return name, rest
		}
	}
	if i := strings.Index(text, ": "); i > 0 {
		return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+2:])
	}
	return "", text
}

// source says whether the tool is one of Rashin's own or came from an MCP
// server; any name outside the canonical table is a provider's.
func source(name string) string {
	for _, mode := range tool.Modes {
		for _, class := range []tool.Class{tool.ClassFull, tool.ClassCompact} {
			if tool.Offers(mode, class, name) {
				return "builtin"
			}
		}
	}
	if strings.HasPrefix(name, "mcp:") {
		if server, _, ok := strings.Cut(strings.TrimPrefix(name, "mcp:"), "/"); ok && server != "" {
			return "mcp:" + server
		}
	}
	return "unknown"
}
