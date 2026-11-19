package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type VeryUnique struct{ expression expr.Expr }

func NewVeryUnique() *VeryUnique {
	return &VeryUnique{expression: expr.NewSequenceExprWordSet([]string{"fairly", "pretty", "rather", "quite", "somewhat", "very"}).TWs().TAco("unique")}
}
func (r *VeryUnique) Expr() expr.Expr { return r.expression }
func (r *VeryUnique) Description() string {
	return "Flags phrases like `very unique`, `pretty unique`, etc., and suggests using `unique` alone or a more precise adjective such as `special`, `rare`, or `unusual`."
}
func (r *VeryUnique) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *VeryUnique) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	var suggestions []lint.Suggestion
	for _, adj := range []string{"special", "rare", "unusual"} {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(t[0].GetStr(s)+" "+adj, span.GetContent(s)))
	}
	suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr("unique", span.GetContent(s)))
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: suggestions, Message: "`Unique` is absolute, so consider using `unique` alone or a more precise adjective such as `special`, `rare`, or `unusual`.", Priority: 57}
}
