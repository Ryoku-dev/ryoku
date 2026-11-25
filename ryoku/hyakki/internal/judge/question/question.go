// Package question holds the questions Rashin asks its judge (engine.Judge)
// and reads its answers: the JSON Schemas that constrain a light model's
// reply, the three-way yes/unsure/no bucket small models answer best, the
// unexpected-stop question the engine asks after a text-only turn, and the
// judged-rule question. It imports nothing of Rashin's, so the engine and
// the judge both use it.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/judge/unexpected-stop.json@c7fff67
// and packages/coding-agent/src/session/unexpected-stop-classifier.ts@c7fff67.
package question

import (
	"encoding/json"
	"strings"
)

// Answers of the YesNo bucket.
const (
	Yes    = "yes"
	Unsure = "unsure"
	No     = "no"
)

// AnswerField is the property every schema built here asks for.
const AnswerField = "answer"

// Labels is a JSON Schema for an object whose one field, "answer", is one of
// labels. Three labels at most suit light models: they bucket far better
// than they rank on a longer scale.
func Labels(labels ...string) json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			AnswerField: map[string]any{"type": "string", "enum": labels},
		},
		"required":             []string{AnswerField},
		"additionalProperties": false,
	}
	raw, _ := json.Marshal(schema)
	return raw
}

// YesNo is the yes/unsure/no bucket.
var YesNo = Labels(Yes, Unsure, No)

// Answer reads the "answer" field of a verdict's JSON, lower-cased; "" when
// there is none.
func Answer(raw json.RawMessage) string {
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	s, _ := v[AnswerField].(string)
	return strings.ToLower(strings.TrimSpace(s))
}

// UnsureYes is the yes-probability of an unsure answer: just under every
// threshold here, so a judge that cannot tell never triggers anything.
const UnsureYes = 0.4

// YesProbability turns a YesNo verdict into the probability that the answer
// is yes: p for yes, 1-p for no, UnsureYes for unsure, 0 for anything else.
// p is the verdict's own confidence.
func YesProbability(raw json.RawMessage, p float64) float64 {
	switch Answer(raw) {
	case Yes:
		return p
	case No:
		return 1 - p
	case Unsure:
		return UnsureYes
	}
	return 0
}

// UnexpectedStopThreshold is the yes-probability at which a turn counts as
// stopped short.
const UnexpectedStopThreshold = 0.5

// RequestChars caps the user's request shown to the judge: enough for the
// ask and its limits ("don't change anything") without a pasted log
// crowding a small model's window.
const RequestChars = 2000

// MessageChars caps the assistant message shown to the judge.
const MessageChars = 4000

// UnexpectedStop is the question whether a text-only final message
// announced work and then ended without doing it. request is the user's
// latest prompt, "" when unknown. Ask it with schema YesNo.
func UnexpectedStop(request, message string) string {
	var b strings.Builder
	b.WriteString(`Did the assistant stop unexpectedly? Yes means its message announces work it is about to do now (call a tool, edit, check, continue) and then ends without doing it. A message that answers or reports on the request, offers options or optional next steps, or asks the user to decide something the request left open is a normal end, even when it says what it would do next. Asking permission for work the request already asked for is an unexpected stop.

Unexpected stops (yes):
- "I should do the same for the JS eval worker. Doing that now."
- "Let me run the tests next."
- "I'll fix that now."
- request "fix the failing test", message "Found the bug. Should I fix it?"

Normal ends (no):
- "I've completed the task."
- "Is there anything else I can help with?"
- "The fix is done and tests pass."
- "Option 1 is simpler. If you want it, I'll start by finding where the flag is set."
- request "explain the crash, don't change anything", message "The cache is stale. Should I fix it?"
`)
	if r := strings.TrimSpace(request); r != "" {
		b.WriteString("\n<request>\n")
		b.WriteString(clip(r, RequestChars))
		b.WriteString("\n</request>\n")
	}
	b.WriteString("\n<message>\n")
	b.WriteString(clip(strings.TrimSpace(message), MessageChars))
	b.WriteString("\n</message>")
	return b.String()
}

// Stopped reads an UnexpectedStop verdict.
func Stopped(raw json.RawMessage, p float64) bool {
	return YesProbability(raw, p) >= UnexpectedStopThreshold
}

// RuleThreshold is the yes-probability at which a judged rule counts as
// violated.
const RuleThreshold = 0.7

// OutputChars caps the output a judged rule is checked against.
const OutputChars = 6000

// RuleViolated is the question whether output (subject says what it is:
// "reply", "reasoning" or "edit call") breaks a judged rule, asked as the
// rule's own yes/no question. Ask it with schema YesNo.
func RuleViolated(ruleQuestion, subject, output string) string {
	var b strings.Builder
	b.WriteString("Answer this question about the assistant's ")
	b.WriteString(subject)
	b.WriteString(" below. Yes means the rule is broken.\n\nQuestion: ")
	b.WriteString(strings.TrimSpace(ruleQuestion))
	b.WriteString("\n\n<output>\n")
	b.WriteString(clip(strings.TrimSpace(output), OutputChars))
	b.WriteString("\n</output>")
	return b.String()
}

// clip keeps the first max bytes of s on a rune boundary.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8Start(s[cut]) {
		cut--
	}
	return s[:cut] + "\n[cut]"
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }
