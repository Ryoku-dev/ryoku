package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type PiqueInterest struct{ expression expr.Expr }

func NewPiqueInterest() *PiqueInterest {
	return &PiqueInterest{expression: expr.NewSequenceExprWordSet([]string{"peak", "peaked", "peek", "peeked", "peeking", "peaking"}).TWs().ThenKindEither(doc.TokenKind.IsNonPluralNominal, doc.TokenKind.IsPossessiveDeterminer).TWs().TAco("interest")}
}
func (r *PiqueInterest) Expr() expr.Expr                  { return r.expression }
func (r *PiqueInterest) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PiqueInterest) Description() string {
	return "Detects incorrect usage of `peak` or `peek` when the intended word is `pique`, as in the phrase `you've peaked my interest`."
}
func (r *PiqueInterest) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	word := strings.ToLower(string(span.GetContent(src)))
	correct := ""
	switch word {
	case "peak", "peek":
		correct = "pique"
	case "peaked", "peeked":
		correct = "piqued"
	case "peaking", "peeking":
		correct = "piquing"
	default:
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(correct, span.GetContent(src))}, Message: fmt.Sprintf("Did you mean `%s` instead of `%s`?", correct, word), Priority: 31}
}
