package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

type Nobody struct{ expression expr.Expr }

func NewNobody() *Nobody {
	return &Nobody{expression: expr.NewSequenceExprWordSeq([]string{"no", "body"}).ThenWhitespace().ThenVerb()}
}
func (r *Nobody) Expr() expr.Expr                  { return r.expression }
func (r *Nobody) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Nobody) Description() string {
	return "Looks for incorrect spacing inside the closed compound `nobody`."
}
func (r *Nobody) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[:3])
	if !ok {
		return nil
	}
	suffix := s[span.End:]
	for len(suffix) > 0 && unicode.IsSpace(suffix[0]) {
		suffix = suffix[1:]
	}
	if len(suffix) > 0 && suffix[0] == ',' {
		return nil
	}
	end := 0
	for end < len(suffix) && !unicode.IsSpace(suffix[end]) && !((suffix[end] >= '!' && suffix[end] <= '/') || (suffix[end] >= ':' && suffix[end] <= '@') || (suffix[end] >= '[' && suffix[end] <= '`') || (suffix[end] >= '{' && suffix[end] <= '~')) {
		end++
	}
	if text.EqAnyIgnoreASCIICaseStr(suffix[:end], []string{"is", "was", "were", "be", "been", "being"}) {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("nobody", s[span.Start:span.End])}
	l.Message = "Did you mean the closed compound `nobody`?"
	return &l
}
