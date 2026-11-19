package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// BeAdjectiveLinter handles "be + verb → be + adjective" confusions.
type BeAdjectiveLinter struct {
	expression expr.Expr
	adjective  string
	exceptions []string
	message    string
}

func NewBeAdjectiveLinter(verb, adjective string, exceptionWords []string, message string) *BeAdjectiveLinter {
	return &BeAdjectiveLinter{adjective: adjective, exceptions: exceptionWords, message: message, expression: expr.ButNot(expr.NewSequenceExpr().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSubjectPronoun().TWs().TSet([]string{"am", "are", "is", "was", "were"}), expr.AsExpr(expr.NewWordSet([]string{"i'm", "we're", "you're", "he's", "she's", "they're", "im", "theyre"}))}).TWs().TAco(verb), expr.NewWord("it"))}
}
func (r *BeAdjectiveLinter) Expr() expr.Expr { return r.expression }
func (r *BeAdjectiveLinter) Description() string {
	return "Detects incorrect use of 'be + verb' instead of `be + adjective`."
}
func (r *BeAdjectiveLinter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BeAdjectiveLinter) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *BeAdjectiveLinter) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	if lint.FollowedByHyphen(c) || lint.PrecededByWord(c, func(w *doc.Token) bool { return text.EqStr(w.GetCh(s), "there") }) || lint.FollowedByWord(c, func(w *doc.Token) bool { return text.EqAnyIgnoreASCIICaseStr(w.GetCh(s), r.exceptions) }) {
		return nil
	}
	tok := t[len(t)-1]
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(r.adjective, tok.GetCh(s))}
	result.Message = r.message
	return &result
}

func NewBeAdjectiveConfusionsLintGroup() *lint.LintGroup {
	group := lint.NewLintGroup()
	group.Add("BeBiased", NewBeAdjectiveLinter("bias", "biased", []string{"adjusted", "corrected", "correcting", "field", "free", "values"}, "Use the adjective 'biased' instead of the noun 'bias'."))
	group.Add("BeConcerned", NewBeAdjectiveLinter("concern", "concerned", nil, "Use the adjective 'concerned' instead of the noun 'concern'."))
	group.Add("BePrejudiced", NewBeAdjectiveLinter("prejudice", "prejudiced", nil, "Use the adjective 'prejudiced' instead of the noun 'prejudice'."))
	group.Add("BeShocked", NewBeAdjectiveLinter("shock", "shocked", []string{"absorbers", "jocks", "resistant", "sensitive"}, "Use the adjective 'shocked' instead of the noun 'shock'."))
	group.Add("BeWorried", NewBeAdjectiveLinter("worry", "worried", []string{"free", "wart", "warts"}, "Use the adjective 'worried' instead of the verb 'worry'."))
	enabled := true
	group.SetAllRulesTo(&enabled)
	return group
}
