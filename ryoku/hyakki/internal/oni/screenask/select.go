package screenask

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	i18n "ryoku-i18n"
)

// MaxSelectionAnswer bounds a select-and-ask answer in runes, the size of one
// speech bubble.
const MaxSelectionAnswer = 600

// Selection is a parsed select-and-ask reply: the answer about the picked
// spot, and whether a step by step walkthrough would help.
type Selection struct {
	Answer string
	Walk   bool
}

// ParseSelection reads a select-and-ask reply. It must be exactly one JSON
// object holding a string "answer" and a boolean "walk", each once and
// nothing else, with only white space around it: a code fence, prose or any
// other text before or after the object is refused. An answer carrying an
// action tag is refused whole; reply tags are dropped from it, hidden runes,
// markdown and em dashes are stripped, and it is cut on a word boundary to
// MaxSelectionAnswer runes. An answer left empty is refused.
func ParseSelection(reply string) (Selection, error) {
	if len(reply) > maxReply {
		return Selection{}, selectRefusal(CodeMalformed, "reply is too long")
	}
	body := strings.TrimSpace(reply)
	if body == "" {
		return Selection{}, refuse(CodeEmpty, "reply is empty")
	}
	if body[0] != '{' {
		return Selection{}, selectRefusal(CodeMalformed, "reply is not a bare JSON object")
	}
	dec := json.NewDecoder(strings.NewReader(body))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return Selection{}, selectRefusal(CodeMalformed, "reply is not a JSON object")
	}
	var (
		answer       string
		walk         bool
		gotA, gotW   bool
		keyTok, vTok json.Token
		err          error
	)
	for dec.More() {
		if keyTok, err = dec.Token(); err != nil {
			return Selection{}, selectRefusal(CodeMalformed, "reply is not valid JSON")
		}
		key, _ := keyTok.(string)
		if vTok, err = dec.Token(); err != nil {
			return Selection{}, selectRefusal(CodeMalformed, "reply is not valid JSON")
		}
		switch key {
		case "answer":
			s, ok := vTok.(string)
			if !ok || gotA {
				return Selection{}, selectRefusal(CodeMalformed, "answer is not one string")
			}
			answer, gotA = s, true
		case "walk":
			b, ok := vTok.(bool)
			if !ok || gotW {
				return Selection{}, selectRefusal(CodeMalformed, "walk is not one boolean")
			}
			walk, gotW = b, true
		default:
			return Selection{}, selectRefusal(CodeMalformed, "reply carries a field other than answer and walk")
		}
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return Selection{}, selectRefusal(CodeMalformed, "reply is not valid JSON")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Selection{}, selectRefusal(CodeMalformed, "reply has text after the JSON object")
	}
	if !gotA || !gotW {
		return Selection{}, selectRefusal(CodeMalformed, "reply lacks answer or walk")
	}
	_, prose, action := scan(answer)
	if action != "" {
		return Selection{}, refuse(CodeAction, "answer asks for the action "+action)
	}
	text := cut(clean(prose, true), MaxSelectionAnswer)
	if text == "" {
		return Selection{}, refuse(CodeEmpty, "reply has no answer")
	}
	return Selection{Answer: text, Walk: walk}, nil
}

// selectRefusal is a refusal whose line fits an answer rather than a point.
func selectRefusal(code, reason string) *Refusal {
	r := refuse(code, reason)
	r.Line = i18n.T("I could not make sense of the answer. Try asking again.")
	return r
}
