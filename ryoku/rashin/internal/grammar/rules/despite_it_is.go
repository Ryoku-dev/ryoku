package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// DespiteItIs corrects incorrect verb forms after "despite", for example
// "despite it is" to "despite it being" or "despite its being", and
// "despite I am" to "despite me being" or "despite my being".
type DespiteItIs struct{ expression expr.Expr }

func NewDespiteItIs() *DespiteItIs {
	return &DespiteItIs{expression: expr.NewSequenceExprAco("despite").TWs().Then(expr.NewSequenceExpr().ThenSubjectPronoun()).TWs().Then(expr.NewWordSet([]string{"am", "are", "is", "was", "were"}))}
}
func (r *DespiteItIs) Expr() expr.Expr { return r.expression }
func (r *DespiteItIs) Description() string {
	return "Corrects `despite` being used with the wrong form of `is`."
}
func (r *DespiteItIs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DespiteItIs) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *DespiteItIs) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) < 5 {
		return nil
	}
	subj := t[2]
	be := t[4]
	k := subj.Kind
	if !k.IsPersonalPronoun() || !k.IsSubjectPronoun() {
		return nil
	}
	person, ok := k.GetPronounPerson()
	if !ok {
		return nil
	}
	sg, pl := k.IsSingularPronoun(), k.IsPluralPronoun()
	obj, poss := "", ""
	switch {
	case person == dict.First && sg && !pl:
		obj, poss = "me", "my"
	case person == dict.First && !sg && pl:
		obj, poss = "us", "our"
	case person == dict.Second && sg && pl:
		obj, poss = "you", "your"
	case person == dict.Third && !sg && pl:
		obj, poss = "them", "their"
	case person == dict.Third && sg && !pl:
		switch {
		case text.EqStr(subj.GetCh(s), "he"):
			obj, poss = "him", "his"
		case text.EqStr(subj.GetCh(s), "she"):
			obj, poss = "her", "her"
		case text.EqStr(subj.GetCh(s), "it"):
			obj, poss = "it", "its"
		default:
			return nil
		}
	default:
		return nil
	}
	suggestions := []lint.Suggestion{}
	if text.EqAnyIgnoreASCIICaseStr(subj.GetCh(s), []string{"it", "they"}) {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr("being", be.GetCh(s)))
	}
	ing := lint.FollowedByWord(c, func(w *doc.Token) bool { return w.Kind.IsVerbProgressiveForm() })
	for _, pron := range []string{obj, poss} {
		if !ing {
			pron += " being"
		}
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(pron, be.GetCh(s)))
	}
	result := lint.NewLint()
	result.Span = text.Span{Start: t[2].Span.Start, End: t[4].Span.End}
	result.LintKind = lint.Grammar
	result.Suggestions = suggestions
	result.Message = "Use the gerund form of the verb after `despite`."
	return &result
}
