package screenask

import (
	"slices"
	"time"
)

// Answer modes: what a question about the window wants back in words.
const (
	ModeSummarize = "summarize"
	ModeRead      = "read"
	ModeExplain   = "explain"
	ModeDescribe  = "describe"
	ModeCheck     = "check"
	ModeAsk       = "ask"
)

// Modes are every answer mode.
var Modes = []string{ModeSummarize, ModeRead, ModeExplain, ModeDescribe, ModeCheck, ModeAsk}

// Check verdicts.
const (
	VerdictYes    = "yes"
	VerdictNo     = "no"
	VerdictUnsure = "unsure"
)

// maxAnswer bounds the spoken answer; read mode asks for about 150 words.
const maxAnswer = 1500

// Answer is a parsed spoken answer. Verdict is set in check mode when the
// answer opens with yes, no or not sure.
type Answer struct {
	Mode    string
	Text    string
	Verdict string
}

// ParseAnswer reads a spoken answer for mode. Tags are dropped (an answer
// never points), markdown and em dashes are flattened, and the text is cut on
// a word boundary. A reply asking for an action is refused, as is one left
// empty.
func ParseAnswer(mode, reply string) (Answer, error) {
	if !slices.Contains(Modes, mode) {
		return Answer{}, refuse(CodeMalformed, "unknown answer mode "+mode)
	}
	if len(reply) > maxReply {
		return Answer{}, refuse(CodeMalformed, "reply is too long")
	}
	_, prose, action := scan(unfence(reply))
	if action != "" {
		return Answer{}, refuse(CodeAction, "reply asks for the action "+action)
	}
	text := cut(clean(prose, true), maxAnswer)
	if text == "" {
		return Answer{}, refuse(CodeEmpty, "reply has no answer")
	}
	a := Answer{Mode: mode, Text: text}
	if mode == ModeCheck {
		a.Verdict = verdict(text)
	}
	return a, nil
}

// verdict reads the yes, no or not sure a check answer opens with.
func verdict(text string) string {
	words := tokens(text)
	if len(words) == 0 {
		return ""
	}
	switch words[0] {
	case "yes":
		return VerdictYes
	case "no":
		return VerdictNo
	case "unsure":
		return VerdictUnsure
	case "not":
		if len(words) > 1 && words[1] == "sure" {
			return VerdictUnsure
		}
	}
	return ""
}

// Dwell is how long an answer of words words stays up: six seconds plus 400
// ms a word, held between ten seconds and a minute.
func Dwell(words int) time.Duration {
	d := 6*time.Second + time.Duration(min(max(words, 0), 1000))*400*time.Millisecond
	return min(max(d, 10*time.Second), 60*time.Second)
}
