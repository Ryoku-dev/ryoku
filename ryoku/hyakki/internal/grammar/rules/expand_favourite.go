package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type ExpandFavourite struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewExpandFavourite(dialect dict.Dialect) *ExpandFavourite {
	return &ExpandFavourite{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewWordStandardCase("fav"), expr.NewWordStandardCase("favs"), expr.NewWordStandardCase("fave"), expr.NewWordStandardCase("faves")}), dialect: dialect}
}
func (r *ExpandFavourite) Expr() expr.Expr { return r.expression }
func (r *ExpandFavourite) Description() string {
	return "Expands the abbreviations `fav` and `fave` to the full word `favorite` or `favourite` for clarity."
}
func (r *ExpandFavourite) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ExpandFavourite) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	abbr := strings.ToLower(span.GetContentString(s))
	expanded := "favourite"
	if r.dialect == dict.American {
		expanded = "favorite"
	}
	if strings.HasSuffix(abbr, "s") {
		expanded += "s"
	}
	return &lint.Lint{Span: span, LintKind: lint.Style, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(expanded, span.GetContent(s))}, Message: fmt.Sprintf("Use `%s` instead of `%s`.", expanded, abbr), Priority: 127}
}
