package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// MultipleSequentialPronouns checks pronouns used right after each other,
// a common mistake made during revision.
type MultipleSequentialPronouns struct{ expression expr.Expr }

func NewMultipleSequentialPronouns() *MultipleSequentialPronouns {
	pronouns := func(t doc.Token, _ []rune) bool {
		return t.Kind.IsSubjectPronoun() || t.Kind.IsObjectPronoun() || t.Kind.IsPossessivePronoun() || t.Kind.IsPossessiveDeterminer()
	}
	return &MultipleSequentialPronouns{expression: expr.NewSequenceExprWith(pronouns).ThenOneOrMore(expr.NewSequenceExprWhitespace().Then(pronouns))}
}
func (r *MultipleSequentialPronouns) Expr() expr.Expr { return r.expression }
func (r *MultipleSequentialPronouns) Description() string {
	return "When editing work to change point of view (i.e. first-person or third-person) it is common to add pronouns while neglecting to remove old ones. This rule catches cases where you have multiple disparate pronouns in sequence."
}
func (r *MultipleSequentialPronouns) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *MultipleSequentialPronouns) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	var suggestions []lint.Suggestion
	if len(toks) == 3 {
		a, b := toks[0], toks[2]
		if a.Kind.IsObjectPronoun() && (b.Kind.IsPossessiveDeterminer() || b.Kind.IsSubjectPronoun()) {
			return nil
		}
		raw := b.GetStr(src)
		if a.Kind.IsPossessiveDeterminer() && (raw == "US" || raw == "IT") {
			return nil
		}
		if a.GetStr(src) == "US" && b.Kind.IsSubjectPronoun() {
			return nil
		}
		suggestions = []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), a.GetCh(src)...)), lint.ReplaceWith(append([]rune(nil), b.GetCh(src)...))}
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Repetition
	result.Priority = 63
	result.Message = "There are too many personal pronouns in sequence here."
	result.Suggestions = suggestions
	return &result
}
