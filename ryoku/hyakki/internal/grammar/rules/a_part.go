package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type APart struct{ expression expr.Expr }

func NewAPart() *APart {
	return &APart{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("a part from"), expr.NewFixedPhraseFromPhrase("apart of"), expr.NewFixedPhraseFromPhrase("fall a part"), expr.NewFixedPhraseFromPhrase("far a part")})}
}
func (r *APart) Expr() expr.Expr { return r.expression }
func (r *APart) Description() string {
	return "Finds and corrects common mistakes between 'a part' and 'apart'"
}
func (r *APart) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *APart) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	var suggestions []lint.Suggestion
	var message string
	switch strings.ToLower(span.GetContentString(s)) {
	case "a part from":
		suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("apart from")), lint.ReplaceWith([]rune("a part of"))}
		message = "If you mean 'except for', use 'apart from'. If you mean 'a piece belonging to', use 'a part of'. Keep it this way if referring to the origin of a piece."
	case "apart of":
		suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("a part of")), lint.ReplaceWith([]rune("apart from"))}
		message = "Did you mean 'a part of' (a piece belonging to) or 'apart from' (except for)?"
	case "far a part":
		suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("far apart"))}
		message = "If you mean 'separated by a distance', use 'far apart'. If referring to the distance of a piece, keep it this way."
	case "fall a part":
		suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("fall apart"))}
		message = "'Fall apart' meaning 'collapse into pieces' or 'stop functioning' is written as two words."
	default:
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: suggestions, Message: message, Priority: 50}
}
