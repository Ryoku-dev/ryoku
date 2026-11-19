package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type LookingForwardTo struct{ expression expr.Expr }

func NewLookingForwardTo() *LookingForwardTo {
	return &LookingForwardTo{expression: expr.NewSequenceExprWith(expr.NewFixedPhraseFromPhrase("looking forward to")).TWs().ThenVerb()}
}
func (r *LookingForwardTo) Expr() expr.Expr                  { return r.expression }
func (r *LookingForwardTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LookingForwardTo) Description() string {
	return "This rule identifies instances where the phrase `looking forward to` is followed by a base form verb instead of the required gerund (verb + `-ing` form)."
}
func (r *LookingForwardTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	verb := tok.GetStr(s)
	if strings.HasSuffix(verb, "ing") {
		return nil
	}
	exception := false
	switch verb {
	case "see", "flee", "agree", "knee", "guarantee", "hoe", "toe", "dye", "eye", "singe", "tinge":
		exception = true
	}
	gerund := verb + "ing"
	if strings.HasSuffix(strings.ToLower(verb), "e") && !exception {
		gerund = strings.TrimRight(verb, "e") + "ing"
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.WordChoice
	result.Message = fmt.Sprintf("The verb `%s` must be in the gerund form (verb + -ing) after 'looking forward to'.", verb)
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(gerund, tok.GetCh(s))}
	return &result
}
