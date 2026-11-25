package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTo struct{ expression expr.Expr }

func NewToTo() *ToTo                             { return &ToTo{expression: expr.NewSequenceExprAco("to").TWsH().TAco("to")} }
func (r *ToTo) Expr() expr.Expr                  { return r.expression }
func (r *ToTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ToTo) Description() string {
	return "Corrects the typo `to to` by either removing the duplication or changing it to `to do`."
}
func (r *ToTo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *ToTo) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	if toks[1].Kind.IsWhitespace() && lint.SurroundedByWords(ctx, func(before, after *doc.Token) bool {
		return after.Kind.IsVerbLemma() && text.EqAnyIgnoreASCIICaseStr(before.GetCh(src), toToPrecedingWords)
	}) {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	first, ok := doc.Span(toks[:2])
	if !ok {
		return nil
	}
	value := append(append([]rune(nil), first.GetContent(src)...), 'd', 'o')
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Typo
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", span.GetContent(src)), lint.ReplaceWithMatchCase(value, span.GetContent(src))}
	l.Message = "Is the second `to` supposed to be `do` or not there at all?"
	return &l
}

var toToPrecedingWords = []string{"according", "appeal", "appealed", "appealing", "appeals", "apply", "applied", "applies", "applying", "connect", "connected", "connecting", "connects", "have", "had", "having", "has", "need", "needed", "needing", "needs", "pray", "prayed", "praying", "prays", "refer", "referred", "referring", "refers", "resort", "resorted", "resorting", "resorts"}
