package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ItWouldBe struct{ expression expr.Expr }

func NewItWouldBe() *ItWouldBe                        { return &ItWouldBe{expression: itWouldBeExpr()} }
func (r *ItWouldBe) Expr() expr.Expr                  { return r.expression }
func (r *ItWouldBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ItWouldBe) Description() string {
	return "Replaces the incorrect sequence “I might/would/will (not) be a …” with “it …”, as in “I think **it** would be a shame.”"
}
func (r *ItWouldBe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	return &lint.Lint{Span: toks[2].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("it"))}, Message: "In this construction the pronoun should be “it”, not “I”. e.g. *“I think **it** would be a shame …”*", Priority: 31}
}

func itWouldBeExpr() expr.Expr {
	branch := func(not, adj bool) *expr.SequenceExpr {
		p := expr.NewSequenceExprWordSet([]string{"believe", "doubt", "think", "assume", "guess"}).ThenWhitespace().TAco("i").ThenWhitespace().ThenWordSet([]string{"might", "would", "will"})
		if not {
			p.ThenWhitespace().TAco("not")
		}
		p.ThenWhitespace().TAco("be").ThenWhitespace().TAco("a")
		if adj {
			p.ThenWhitespace().ThenWordSet([]string{"good", "bad", "wonderful", "real"})
		}
		return p.ThenWhitespace().ThenWordSet([]string{"bummer", "pity", "shame", "pleasure", "idea", "experience", "problem", "catastrophe", "disaster", "trap", "challenge"})
	}
	return expr.Or(expr.Or(expr.Or(branch(false, false), branch(false, true)), branch(true, false)), branch(true, true))
}
