// Package textcalls turns tool calls a model wrote as text into ordinary
// llm.ToolCalls, and writes calls and results back as text for models that
// only speak a text dialect. It is pure: no I/O, no gateway imports.
//
// Scanners are streaming: they take text deltas as they arrive and hold back
// only what could still become markup, so visible text flows without the
// markup and a call surfaces the moment its closing tag lands.
//
// Every scanner reads every text form, because small models drift between
// them: the LFM3/Qwen3-Coder XML form
// <tool_call><function=NAME><parameter=K>V</parameter></function></tool_call>
// (also without the <tool_call> wrapper), the Hermes/Qwen JSON form
// <tool_call>{"name":..,"arguments":{..}}</tool_call>, the GLM form
// <tool_call>NAME<arg_key>K</arg_key><arg_value>V</arg_value></tool_call>,
// and the LFM2 Pythonic form <|tool_call_start|>[f(a=1)]<|tool_call_end|>.
// The dialect picks how calls are written back (Encode) and, for Pythonic,
// also accepts a reply that is a bare [f(a=1)] list.
package textcalls

// Derived from oh-my-pi (MIT), packages/ai/src/dialect/types.ts@c7fff67

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"ryoku-rashin/internal/llm"
)

// Dialects. Native means the server returns tool_calls itself; Auto is the
// fallback scanner run over the text of a native model that leaked markup.
const (
	Native   = "native"
	XMLFn    = "xmlfn"
	Hermes   = "hermes"
	Pythonic = "pythonic"
	GLM      = "glm"
	Auto     = "auto"
)

// Dialects are the text dialects, in the order a settings list shows them.
var Dialects = []string{XMLFn, Hermes, Pythonic, GLM}

// Valid reports whether d names a text dialect or Auto.
func Valid(d string) bool {
	if d == Auto {
		return true
	}
	for _, x := range Dialects {
		if x == d {
			return true
		}
	}
	return false
}

// Event is one result of scanning. Exactly one field is set.
type Event struct {
	// Text is visible text with the call markup taken out.
	Text string
	// Reasoning is text inside <think></think> when Options.Thinking is on.
	Reasoning string
	// Start names a call (ID and Name) as soon as both are known.
	Start *llm.ToolCall
	// Call is a finished call; Arguments is a JSON object, coerced against
	// the tool's schema. A Start with the same ID always came before it.
	Call *llm.ToolCall
	// Broken is the markup of a call that parsed to nothing or was cut
	// off. It is never shown; a Start without a Call is followed by it.
	Broken string
	// Fabricated is set once when the model began writing a tool result
	// itself. The scanner drops everything after it: stop the stream.
	Fabricated bool
}

// Options configure a scanner.
type Options struct {
	// Tools are the tools the request offered: their schemas type the
	// argument values and order Pythonic positional arguments.
	Tools []llm.Tool
	// Thinking turns <think>...</think> into Reasoning events.
	Thinking bool
	// NewID mints call ids; nil gives "call_" and 24 hex digits.
	NewID func() string
}

// Scanner turns a stream of text deltas into events.
type Scanner interface {
	// Feed takes the next delta.
	Feed(text string) []Event
	// Flush ends the stream and releases held text. cut says the stream
	// stopped at the token limit: a call still open is then Broken and never
	// runs; otherwise it is finished with the arguments written so far.
	Flush(cut bool) []Event
}

// ForDialect returns a scanner for d, or nil for "", Native and unknown
// names.
func ForDialect(d string, opts Options) Scanner {
	if !Valid(d) {
		return nil
	}
	s := &scanner{
		auto:     d == Auto,
		bare:     d == Pythonic,
		thinking: opts.Thinking,
		tools:    buildShapes(opts.Tools),
		newID:    opts.NewID,
	}
	if s.newID == nil {
		s.newID = NewID
	}
	return s
}

// NewID returns a fresh call id, "call_" and 24 hex digits.
func NewID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "call_" + hex.EncodeToString(b[:])
}

// Detect names the dialect whose call markup appears in text, "" when none
// does. A bare Pythonic list is not detected: prose has brackets too.
func Detect(text string) string {
	best, bestAt := "", -1
	try := func(at int, d string) {
		if at >= 0 && (bestAt < 0 || at < bestAt) {
			best, bestAt = d, at
		}
	}
	try(strings.Index(text, tagPyStart), Pythonic)
	try(strings.Index(text, tagFunction), XMLFn)
	try(strings.Index(text, tagArgKey), GLM)
	if at := strings.Index(text, tagToolCall); at >= 0 {
		try(at, blockDialect(text[at+len(tagToolCall):]))
	}
	return best
}

// Leaks reports whether text holds call markup of any dialect.
func Leaks(text string) bool { return Detect(text) != "" }

// blockDialect reads which form follows a <tool_call> opener.
func blockDialect(rest string) string {
	rest = strings.TrimLeft(rest, " \t\r\n")
	switch {
	case strings.HasPrefix(rest, tagFunction):
		return XMLFn
	case rest == "", rest[0] == '{':
		return Hermes
	case rest[0] == '[':
		if pyLead(rest) == matchYes {
			return Pythonic
		}
		return Hermes
	case isIdentStart(rest[0]):
		return GLM
	}
	return Hermes
}
