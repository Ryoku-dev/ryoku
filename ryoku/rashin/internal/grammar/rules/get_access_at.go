package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

// GetAccessAt corrects "get access at <resource>" while avoiding adverbial, URL, and time expressions.
type GetAccessAt struct{ expression expr.Expr }

func NewGetAccessAt() *GetAccessAt {
	e := expr.NewSequenceExprWordSet([]string{"get", "gets", "got", "getting", "gained", "gaining", "obtain", "obtains", "obtained", "obtaining"}).TWs().TAco("access").TWs().TAco("at")
	return &GetAccessAt{expression: e}
}
func (r *GetAccessAt) Expr() expr.Expr { return r.expression }
func (r *GetAccessAt) Description() string {
	return "Flags `get access at` and suggests replacing `at` with `to`."
}
func (r *GetAccessAt) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GetAccessAt) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	rest := strings.ToLower(string(s[tok.Span.End:]))
	for _, prefix := range []string{" all", " least", " http://", " https://", " www.", " this time", " this moment", " this stage", " this level", " that time", " that moment", " that stage", " that level", " the moment", " the stage", " the level"} {
		if strings.HasPrefix(rest, prefix) {
			return nil
		}
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Grammar
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("to"))}
	l.Message = "Did you mean `to` instead of `at` when specifying what is being accessed?"
	l.Priority = 45
	return &l
}
