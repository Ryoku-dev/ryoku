// Package askuser is the ask tool: mid-turn, the model puts one question
// with a few options to the user on a question card and waits for the pick.
// A team task asks through its own ask_user tool (the team's Inbox) instead.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/ask.ts@c7fff67
// and packages/coding-agent/src/prompts/tools/ask.md@c7fff67, flattened to
// one question and its options for the card.
package askuser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Option limits: the card shows two to six choices.
const (
	MinOptions  = 2
	MaxOptions  = 6
	optionRunes = 120
)

// Tools is the ask tool.
func Tools() []tool.Tool { return []tool.Tool{askTool{}} }

type askTool struct{}

func (askTool) Name() string    { return tool.Ask }
func (askTool) Kind() tool.Kind { return protocol.ToolOther }
func (askTool) ReadOnly() bool  { return true }

func (askTool) Spec(p tool.Profile) llm.Tool {
	params := json.RawMessage(`{"type":"object","properties":{` +
		`"question":{"type":"string","description":"The question, with what the user needs to decide."},` +
		`"options":{"type":"array","items":{"type":"string"},"description":"2 to 6 answers, each a short phrase."},` +
		`"recommended":{"type":"integer","description":"The number of the option you suggest, from 1."}` +
		`},"required":["question","options"]}`)
	if p.Compact() {
		return llm.Tool{Name: tool.Ask, Description: "Ask the user to pick one of a few options, and wait for the answer.", Parameters: params}
	}
	return llm.Tool{
		Name: tool.Ask,
		Description: `Ask the user one question with 2 to 6 options and wait for their pick.
- Ask only when the choices lead to materially different results and the user must decide. Otherwise act on the code, the docs and the conventions: pick the conservative choice, go on, and say what you chose.
- Put what the user needs to decide (findings, tradeoffs) in the question.
- Each option is a short phrase; recommended is the number of the one you suggest.
- For several decisions, ask the most important one first and the next in a later call.`,
		Parameters: params,
	}
}

func (askTool) Present(args json.RawMessage) tool.Presentation {
	q, problem := parse(args)
	if problem != "" {
		return tool.Presentation{Title: i18n.T("Ask you"), Input: string(args)}
	}
	return tool.Presentation{Title: i18n.T("Ask you"), Input: q.question}
}

func (askTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	q, problem := parse(call.Args)
	if problem != "" {
		return tool.Failed(problem)
	}
	if env.Permissions == nil {
		return tool.Failed("No one can answer here. Put the question in your reply instead.")
	}
	choices := make([]protocol.Choice, len(q.options))
	for i, option := range q.options {
		choices[i] = protocol.Choice{ID: strconv.Itoa(i + 1), Name: option}
	}
	recommended := ""
	if q.recommended > 0 {
		recommended = strconv.Itoa(q.recommended)
	}
	picked, err := env.Permissions.Choose(ctx, tool.Question{Call: call, Question: q.question, Options: choices, Recommended: recommended})
	if err != nil {
		return tool.Result{Text: "The question was withdrawn before the user answered.", Shown: i18n.T("Not answered"), IsError: true}
	}
	for _, choice := range choices {
		if choice.ID == picked {
			return tool.Result{
				Text:  fmt.Sprintf("The user chose option %s: %s", choice.ID, choice.Name),
				Shown: choice.Name,
			}
		}
	}
	return tool.Result{
		Text:  "The user dismissed the question without choosing. Go on with the conservative choice and say which one you took, or ask again in your reply.",
		Shown: i18n.T("Dismissed"),
	}
}

type question struct {
	question    string
	options     []string
	recommended int
}

// parse reads {question, options, recommended}. Options may be strings or
// objects with a label; recommended may be a number, a numeric string or the
// option's text.
func parse(raw json.RawMessage) (question, string) {
	var args struct {
		Question    string          `json:"question"`
		Title       string          `json:"title"`
		Options     json.RawMessage `json:"options"`
		Choices     json.RawMessage `json:"choices"`
		Recommended json.RawMessage `json:"recommended"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return question{}, "The arguments are not a JSON object: " + err.Error()
	}
	q := question{question: strings.TrimSpace(args.Question)}
	if q.question == "" {
		q.question = strings.TrimSpace(args.Title)
	}
	if q.question == "" {
		return question{}, "Give the question."
	}
	list := bytes.TrimSpace(args.Options)
	if len(list) == 0 || string(list) == "null" {
		list = bytes.TrimSpace(args.Choices)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(list, &items); err != nil {
		return question{}, fmt.Sprintf("Give options: a list of %d to %d short answers.", MinOptions, MaxOptions)
	}
	seen := make(map[string]bool)
	for _, item := range items {
		option := optionText(item)
		if option == "" || seen[strings.ToLower(option)] {
			continue
		}
		seen[strings.ToLower(option)] = true
		q.options = append(q.options, protocol.ClipWords(option, optionRunes))
	}
	if len(q.options) < MinOptions || len(q.options) > MaxOptions {
		return question{}, fmt.Sprintf("Give %d to %d different options; this call has %d.", MinOptions, MaxOptions, len(q.options))
	}
	q.recommended = recommendation(args.Recommended, q.options)
	return q, ""
}

func optionText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var object struct {
		Label string `json:"label"`
		Name  string `json:"name"`
		Text  string `json:"text"`
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &object) != nil {
		return ""
	}
	for _, text := range []string{object.Label, object.Name, object.Text, object.Value} {
		if strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// recommendation is the 1-based option the model suggests, 0 for none or
// one it cannot be matched to.
func recommendation(raw json.RawMessage, options []string) int {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		if n >= 1 && n <= len(options) {
			return n
		}
		return 0
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return 0
	}
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && n <= len(options) {
			return n
		}
		return 0
	}
	for i, option := range options {
		if strings.EqualFold(option, s) {
			return i + 1
		}
	}
	return 0
}
