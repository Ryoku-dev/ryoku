package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NorModalPronoun struct{ expression expr.Expr }

func NewNorModalPronoun() *NorModalPronoun {
	return &NorModalPronoun{expr.NewSequenceExprAco("nor").TWs().ThenSubjectPronoun().TWs().Then(expr.NewModalVerbWithCommonErrors())}
}
func (r *NorModalPronoun) Expr() expr.Expr                  { return r.expression }
func (r *NorModalPronoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*NorModalPronoun) Description() string {
	return "Corrects the order of the pronoun and modal verb after `nor`."
}
func (r *NorModalPronoun) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *NorModalPronoun) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil {
		p := c.Before
		n := len(p)
		if n >= 2 && p[n-1].Kind.IsWhitespace() && p[n-2].Kind.IsSubjectPronoun() {
			return nil
		}
		if n >= 4 && p[n-1].Kind.IsWhitespace() && p[n-2].Kind.IsNoun() && p[n-3].Kind.IsWhitespace() && p[n-4].Kind.IsPossessiveDeterminer() {
			return nil
		}
	}
	if len(t) < 3 {
		return nil
	}
	pron, modal := t[len(t)-3], t[len(t)-1]
	span, _ := doc.Span(t[len(t)-3:])
	value := []rune(modal.GetStr(s) + " " + pron.GetStr(s))
	suggestion := lint.ReplaceWith(value)
	if pron.GetStr(s) != "I" {
		suggestion = lint.ReplaceWithMatchCase(value, span.GetContent(s))
	}
	return &lint.Lint{Span: span, LintKind: lint.Grammar, Suggestions: []lint.Suggestion{suggestion}, Message: "After `nor`, the modal verb should come before the pronoun.", Priority: 127}
}
