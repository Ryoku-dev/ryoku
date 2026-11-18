package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ByTheBook struct{ expression expr.Expr }

func NewByTheBook() *ByTheBook {
	return &ByTheBook{expression: expr.NewSequenceExprWordSet([]string{"all", "done", "everything", "go", "it", "play", "playing"}).TWs().TAco("by").TWs().TAco("the").TWs().TAco("books").ThenAnyOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().ThenConjunction()})}
}
func (r *ByTheBook) Expr() expr.Expr                  { return r.expression }
func (r *ByTheBook) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ByTheBook) Description() string              { return "Corrects `by the books` to `by the book`." }
func (r *ByTheBook) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(toks, src, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "books") })
	if tok == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("book", tok.GetCh(src))}
	result.Message = "Did you mean the idiom `by the book`?"
	return &result
}
