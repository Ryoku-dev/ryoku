package rules

import (
	"math"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

// PluralCriterionPhenomenon checks if criterion or phenomenon should be plural after a nearby plural modifier.
type PluralCriterionPhenomenon struct{}

func NewPluralCriterionPhenomenon() *PluralCriterionPhenomenon { return &PluralCriterionPhenomenon{} }
func (*PluralCriterionPhenomenon) Description() string {
	return "The words “criteria” and “phenomena” are the plurals of “criterion” and “phenomenon”, respectively. This rule checks for singular forms used after plural modifiers."
}
func (*PluralCriterionPhenomenon) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, chunk := range d.IterChunks() {
		for i, tok := range chunk {
			if !tok.Kind.IsWord() {
				continue
			}
			noun := strings.ToLower(tok.GetStr(d.GetSource()))
			if noun != "criterion" && noun != "phenomenon" {
				continue
			}
			if !criteriaPhenomenaPluralCriterionPhenomenonHasPluralModifierBefore(chunk, i, d) {
				continue
			}
			replacement := "criteria"
			if noun == "phenomenon" {
				replacement = "phenomena"
			}
			result = append(result, lint.Lint{Span: tok.Span, LintKind: lint.Repetition, Message: "Use the plural form after a plural modifier.", Priority: 63, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(replacement))}})
		}
	}
	return result
}
func criteriaPhenomenaPluralCriterionPhenomenonHasPluralModifierBefore(chunk []doc.Token, index int, d *doc.Document) bool {
	words := 0
	for i := index - 1; i >= 0; i-- {
		tok := chunk[i]
		if tok.Kind.IsWhitespace() {
			continue
		}
		switch criteriaPhenomenaPluralCriterionPhenomenonModifierNumber(tok, d) {
		case 1:
			return true
		case -1:
			return false
		}
		if tok.Kind.IsWord() {
			words++
			if words > 4 {
				return false
			}
		} else if !criteriaPhenomenaPluralCriterionPhenomenonIsParenthetical(tok) {
			return false
		}
	}
	return false
}
func criteriaPhenomenaPluralCriterionPhenomenonModifierNumber(tok doc.Token, d *doc.Document) int {
	if number, ok := tok.Kind.AsNumber(); ok && number.Suffix == nil {
		if math.Abs(number.Value-1) > 2.220446049250313e-16 {
			return 1
		}
		return -1
	}
	if !tok.Kind.IsWord() {
		return 0
	}
	word := strings.ToLower(tok.GetStr(d.GetSource()))
	if tok.Kind.IsUpos(pos.NUM) {
		if word == "one" {
			return -1
		}
		return 1
	}
	if tok.Kind.IsQuantifier() {
		switch word {
		case "each", "every":
			return -1
		case "both", "few", "fewer", "many", "multiple", "several":
			return 1
		}
		return 0
	}
	if tok.Kind.IsDemonstrativeDeterminer() {
		switch word {
		case "these", "those":
			return 1
		case "this", "that":
			return -1
		}
		return 0
	}
	if word == "a" || word == "an" {
		return -1
	}
	return 0
}
func criteriaPhenomenaPluralCriterionPhenomenonIsParenthetical(tok doc.Token) bool {
	p, ok := tok.Kind.AsPunctuation()
	if !ok {
		return false
	}
	switch p.Kind {
	case text.OpenRound, text.CloseRound, text.OpenSquare, text.CloseSquare, text.OpenCurly, text.CloseCurly, text.Hyphen:
		return true
	}
	return false
}
