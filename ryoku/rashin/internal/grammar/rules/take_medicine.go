package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TakeMedicine struct{ expression expr.Expr }

func NewTakeMedicine() *TakeMedicine {
	return &TakeMedicine{expression: expr.NewSequenceExprWith(takeMedicineDerivedAny([]string{"eat", "eats", "ate", "eating", "eaten"})).TWs().ThenOptional(expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenDeterminer(), expr.NewSequenceExpr().ThenPossessiveDeterminer(), expr.NewSequenceExpr().ThenQuantifier()}).TWs()).ThenOptional(expr.NewSequenceExpr().ThenOneOrMoreAdjectives().TWs()).Then(takeMedicineDerivedAny([]string{"antibiotic", "medicine", "medication", "pill", "tablet", "aspirin", "paracetamol"}))}
}
func (r *TakeMedicine) Expr() expr.Expr                  { return r.expression }
func (r *TakeMedicine) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TakeMedicine) Description() string {
	return "Encourages pairing medicine-related nouns with verbs like `take` or `swallow` instead of `eat`."
}
func (r *TakeMedicine) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	verb := toks[0]
	result := lint.NewLint()
	result.Span = verb.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{takeMedicineReplacementFor(verb, src, "take", "takes", "took", "taken", "taking"), takeMedicineReplacementFor(verb, src, "swallow", "swallows", "swallowed", "swallowed", "swallowing")}
	result.Message = "Use a verb like `take` or `swallow` with medicine instead of `eat`."
	result.Priority = 63
	return &result
}
func takeMedicineDerivedAny(words []string) expr.Expr {
	var result expr.Expr = expr.AsExpr(expr.NewDerivedFromStr(words[0]))
	for _, w := range words[1:] {
		result = expr.Or(result, expr.NewDerivedFromStr(w))
	}
	return result
}
func takeMedicineReplacementFor(verb doc.Token, src []rune, base, thirdPerson, past, pastParticiple, progressive string) lint.Suggestion {
	replacement := base
	if verb.Kind.IsVerbProgressiveForm() {
		replacement = progressive
	} else if verb.Kind.IsVerbThirdPersonSingularPresentForm() {
		replacement = thirdPerson
	} else if verb.Kind.IsVerbPastParticipleForm() && !verb.Kind.IsVerbSimplePastForm() {
		replacement = pastParticiple
	} else if verb.Kind.IsVerbSimplePastForm() {
		replacement = past
	}
	return lint.ReplaceWithMatchCaseStr(replacement, verb.GetCh(src))
}
