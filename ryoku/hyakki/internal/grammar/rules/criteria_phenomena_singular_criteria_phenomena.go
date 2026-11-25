package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

// SingularCriteriaPhenomena checks if criteria or phenomena is used as singular.
type SingularCriteriaPhenomena struct{ expression expr.Expr }

func NewSingularCriteriaPhenomena() *SingularCriteriaPhenomena {
	return &SingularCriteriaPhenomena{expression: expr.NewSequenceExprWith(expr.NewWordSet([]string{"this", "that", "a", "one"})).TWs().Then(expr.NewWordSet([]string{"criteria", "phenomena"}))}
}
func (r *SingularCriteriaPhenomena) Expr() expr.Expr { return r.expression }
func (r *SingularCriteriaPhenomena) Description() string {
	return "The words “criteria” and “phenomena” are the plurals of “criterion” and “phenomenon”, respectively. They are often incorrectly used as singular."
}
func (r *SingularCriteriaPhenomena) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *SingularCriteriaPhenomena) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) < 3 {
		return nil
	}
	target := tokens[2]
	replacement := ""
	switch strings.ToLower(target.GetStr(source)) {
	case "criteria":
		replacement = "criterion"
	case "phenomena":
		replacement = "phenomenon"
	default:
		return nil
	}
	return &lint.Lint{Span: target.Span, LintKind: lint.Repetition, Message: "You used a plural noun as singular.", Priority: 63, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(replacement))}}
}
