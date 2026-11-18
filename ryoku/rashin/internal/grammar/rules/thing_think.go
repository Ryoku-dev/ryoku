package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// ThingThink corrects the typo "thing" for "think".
type ThingThink struct{ expression expr.Expr }

func NewThingThink() *ThingThink {
	indef := expr.NewFirstMatchOf([]expr.Expr{expr.NewWordSet([]string{"anybody", "anyone", "everybody", "everyone"}), expr.NewFixedPhraseFromPhrase("every body")})
	pronoun := expr.NewFirstMatchOf([]expr.Expr{expr.NewWordSet([]string{"I", "you", "we", "they"}), indef})
	verbTo := expr.NewSequenceExprWordSet([]string{"have", "had", "has", "having", "need", "needed", "needs", "needing", "want", "wanted", "wants", "wanting", "try", "tried", "tries", "trying"}).TWs().TAco("to")
	modal := expr.NewWordSet([]string{"can", "cannot", "can't", "could", "couldn't", "may", "might", "mightn't", "must", "mustn't", "shall", "shan't", "should", "shouldn't", "will", "won't"})
	adverb := expr.NewWordSet([]string{"always", "sometimes", "often", "usually", "never"})
	pre := expr.NewFirstMatchOf([]expr.Expr{pronoun, verbTo, modal, adverb})
	return &ThingThink{expression: expr.NewSequenceExprWith(pre).TWs().TAco("thing")}
}
func (r *ThingThink) Expr() expr.Expr                  { return r.expression }
func (r *ThingThink) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThingThink) Description() string {
	return "Corrects the typo `thing` when it should be `think`."
}
func (r *ThingThink) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	return &lint.Lint{Span: tok.Span, LintKind: lint.Typo, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("think", tok.GetCh(src))}, Message: "Did you mean `think`?", Priority: 31}
}
