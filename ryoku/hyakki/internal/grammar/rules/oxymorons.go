package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// Oxymorons flags oxymoronic phrases.
type Oxymorons struct{ expression expr.Expr }

func NewOxymorons() *Oxymorons {
	phrases := []string{"advancing backwards", "alludes explicitly to", "amateur expert", "build down", "completely obsolescent", "conspicuous absence", "exact estimate", "explicitly alludes to", "found missing", "generally always", "increasingly less", "intense apathy", "mandatory choice", "nonworking mother", "organized mess", "standard Option", "totally obsolescent", "usually always"}
	e := expr.NewFirstMatchOf()
	for _, phrase := range phrases {
		e.Add(expr.NewFixedPhraseFromPhrase(phrase))
	}
	return &Oxymorons{expression: e}
}
func (r *Oxymorons) Expr() expr.Expr                  { return r.expression }
func (r *Oxymorons) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Oxymorons) Description() string {
	return "Flags oxymoronic phrases (e.g. `amateur expert`, `increasingly less`, etc.)."
}
func (*Oxymorons) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.Message = fmt.Sprintf("'%s' is an oxymoron.", string(s[span.Start:span.End]))
	l.Priority = 31
	return &l
}
