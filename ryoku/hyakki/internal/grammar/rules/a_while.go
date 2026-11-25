package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type AWhile struct {
	expression *expr.ExprMap[aWhileReplacement]
}

func NewAWhile() *AWhile {
	m := expr.NewExprMap[aWhileReplacement]()
	m.Insert(expr.NewSequenceExprWith(expr.NewUPOSSet([]pos.UPOS{pos.VERB})).TWs().TAco("a").TWs().TAco("while"), aWhileReplacement{"awhile", "Use the single word `awhile` when it follows a verb."})
	m.Insert(expr.NewSequenceExprUnless(expr.NewUPOSSet([]pos.UPOS{pos.VERB})).TWs().TAco("awhile"), aWhileReplacement{"a while", "When not used after a verb, spell this duration as `a while`."})
	return &AWhile{expression: m}
}
func (r *AWhile) Expr() expr.Expr                  { return r.expression }
func (r *AWhile) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AWhile) Description() string {
	return "Enforces `awhile` after verbs and `a while` everywhere else."
}
func (r *AWhile) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	v, ok := r.expression.Lookup(0, toks, src)
	if !ok || len(toks) < 3 {
		return nil
	}
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Typo, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(v.replacement, span.GetContent(src))}, Message: v.message, Priority: 127}
}

type aWhileReplacement struct{ replacement, message string }
