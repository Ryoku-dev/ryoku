package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type Hereby struct{ expression expr.Expr }

func NewHereby() *Hereby {
	return &Hereby{expression: expr.NewSequenceExprWordSeq([]string{"here", "by"}).ThenWhitespace().ThenKindIsButIsNot(doc.TokenKind.IsVerb, doc.TokenKind.IsNoun)}
}
func (r *Hereby) Expr() expr.Expr                  { return r.expression }
func (r *Hereby) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Hereby) Description() string                { return "`Here by` in some contexts should be `hereby`" }
func (r *Hereby) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := text.Span{Start: toks[0].Span.Start, End: toks[2].Span.End}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("hereby", span.GetContent(src))}
	result.Message = "Did you mean the closed compound `hereby`?"
	result.Priority = 127
	return &result
}
