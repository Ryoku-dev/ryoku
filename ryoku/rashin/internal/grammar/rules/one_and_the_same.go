package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OneAndTheSame struct{ expression expr.Expr }

func NewOneAndTheSame() *OneAndTheSame {
	return &OneAndTheSame{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"are", "were"}).TWs().Then(expr.NewFixedPhraseFromPhrase("one in the same")), expr.NewSequenceExprWith(expr.NewFixedPhraseFromPhrase("one in the same")).TWs().TAco("as")})}
}
func (r *OneAndTheSame) Expr() expr.Expr                  { return r.expression }
func (r *OneAndTheSame) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OneAndTheSame) Description() string {
	return "This linter flags instances of the nonstandard phrase `one in the same`. The correct, more accepted form is `one and the same`"
}
func (r *OneAndTheSame) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	var part []doc.Token
	if toks[len(toks)-1].GetStr(src) == "as" {
		part = toks[:len(toks)-2]
	} else {
		part = toks[2:]
	}
	span, ok := doc.Span(part)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Priority = 127
	l.Message = "The actual idiom is with the word `and`."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("one and the same", span.GetContent(src))}
	return &l
}
