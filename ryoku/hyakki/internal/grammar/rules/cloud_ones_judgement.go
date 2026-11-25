package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CloudOnesJudgement struct{ expression expr.Expr }

func NewCloudOnesJudgement() *CloudOnesJudgement {
	return &CloudOnesJudgement{expression: expr.NewSequenceExprWordSet([]string{"clown", "clowned", "clowning", "clowns"}).TWs().ThenPossessiveDeterminer().TWs().TSet([]string{"judgement", "judgment", "judgements", "judgments"})}
}
func (r *CloudOnesJudgement) Expr() expr.Expr                  { return r.expression }
func (r *CloudOnesJudgement) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CloudOnesJudgement) Description() string {
	return "Corrects the malapropism `clown one's judgement` to `cloud one's judgement`."
}
func (r *CloudOnesJudgement) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	corr := append([]rune(nil), span.GetContent(src)...)
	corr[3] -= 2
	corr[4] -= 10
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Malapropism
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(corr)}
	result.Message = "Unless this is intentional wordplay, the correct verb is `cloud`."
	return &result
}
