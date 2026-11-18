package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strconv"
)

type Therein struct{ expression expr.Expr }

func NewTherein() *Therein {
	return &Therein{expression: expr.NewSequenceExprAnyWord().TWs().TAco("there").TWsH().TAco("in")}
}
func (r *Therein) Expr() expr.Expr                  { return r.expression }
func (r *Therein) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Therein) Description() string {
	return "Flags occurrences of `there in` or `there-in` that should be the closed compound word `therein`."
}
func (r *Therein) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *Therein) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	previous := toks[0]
	if previous.Kind.IsObjectPronoun() || text.EqAnyIgnoreASCIICaseStr(previous.GetCh(src), []string{"down", "out", "over", "right", "up", "are", "be", "get", "stood", "was"}) {
		return nil
	}
	if lint.FollowedByWord(ctx, func(t *doc.Token) bool {
		if t.Kind.IsDeterminer() {
			return true
		}
		chars := t.GetCh(src)
		if t.Kind.IsCardinalNumber() && len(chars) == 4 {
			for _, c := range chars {
				if c < '0' || c > '9' {
					return false
				}
			}
			year, err := strconv.Atoi(string(chars))
			return err == nil && year >= 1900 && year <= 2100
		}
		return false
	}) {
		return nil
	}
	span, ok := doc.Span(toks[2:5])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Miscellaneous
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("therein", span.GetContent(src))}
	l.Message = "Use the closed compound 'therein' here."
	return &l
}
