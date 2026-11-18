package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type ToHop struct{ expression expr.Expr }

func NewToHop() *ToHop {
	return &ToHop{expression: expr.NewSequenceExprWordSet([]string{"hoping", "hoped", "hope"}).TWs().TAco("on").TWs().ThenDeterminer().TWs().ThenWordSet([]string{"airplane", "plane", "bus", "call", "train"})}
}
func (r *ToHop) Expr() expr.Expr { return r.expression }
func (r *ToHop) Description() string {
	return "Detects incorrect usage of the words 'hoping,' 'hoped,' or 'hope' when referring to boarding or entering a mode of transportation. Suggests replacing them with the correct verb form such as 'hopping,' 'hopped,' or 'hop.'"
}
func (r *ToHop) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToHop) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) == 0 {
		return nil
	}
	target := tokens[0]
	word := target.GetStr(source)
	correct := ""
	switch strings.ToLower(word) {
	case "hoping":
		correct = "hopping"
	case "hoped":
		correct = "hopped"
	case "hope":
		correct = "hop"
	default:
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(correct, target.GetCh(source))}
	result.Message = fmt.Sprintf("Did you mean to use %s instead of %s in this context?", word, correct)
	return &result
}
