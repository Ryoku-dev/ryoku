package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ClaimToFame struct{ expression expr.Expr }

func NewClaimToFame() *ClaimToFame {
	return &ClaimToFame{expression: expr.NewSequenceExprWordSet([]string{"claim", "claims"}).TWs().ThenWordSeq([]string{"for", "fame"})}
}
func (r *ClaimToFame) Expr() expr.Expr { return r.expression }
func (r *ClaimToFame) Description() string {
	return "Corrects `claim for fame` to the idiom `claim to fame`."
}
func (r *ClaimToFame) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ClaimToFame) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *ClaimToFame) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil {
		b := c.Before
		if len(b) >= 2 && b[len(b)-1].Kind.IsWhitespace() {
			w := b[len(b)-2]
			if text.EqAnyIgnoreASCIICaseStr(w.GetCh(s), []string{"make", "made", "makes", "making"}) {
				return nil
			}
			if (w.Kind.IsDeterminer() || w.Kind.IsAdjective()) && len(b) >= 4 && b[len(b)-3].Kind.IsWhitespace() && b[len(b)-4].Kind.IsWord() && text.EqAnyIgnoreASCIICaseStr(b[len(b)-4].GetCh(s), []string{"make", "made", "makes", "making"}) {
				return nil
			}
		}
		a := c.After
		if len(a) >= 4 && a[0].Kind.IsWhitespace() && a[1].Kind.IsWord() && a[2].Kind.IsWhitespace() && a[3].Kind.IsWord() && text.EqStr(a[1].GetCh(s), "and") && !a[3].Kind.IsConjunction() && a[3].Kind.IsNoun() {
			return nil
		}
	}
	if len(t) < 3 {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	span := t[2].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Eggcorn
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", span.GetContent(s))}
	result.Message = "The correct idiom is `claim to fame`."
	result.Priority = 127
	return &result
}
