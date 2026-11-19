package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// CompoundNounBeforeAuxVerb finds two adjacent words separated by whitespace that
// would be a valid noun if joined.
type CompoundNounBeforeAuxVerb struct {
	expression expr.Expr
	split      *expr.MergeableWords
}

func NewCompoundNounBeforeAuxVerb() *CompoundNounBeforeAuxVerb {
	split := expr.NewMergeableWords(compoundNounsPredicate)
	return &CompoundNounBeforeAuxVerb{expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(compoundNounsIsContentWord).TWs().Then(compoundNounsIsContentWord).ThenAuxiliaryVerb(), expr.NewSequenceExprWith(split).TAny().TAny()}), split}
}
func (r *CompoundNounBeforeAuxVerb) Expr() expr.Expr { return r.expression }
func (r *CompoundNounBeforeAuxVerb) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *CompoundNounBeforeAuxVerb) Description() string {
	return "Detects split compound nouns preceding an action and suggests merging them."
}
func (r *CompoundNounBeforeAuxVerb) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	span, ok := doc.Span(toks[:3])
	if !ok {
		return nil
	}
	word, ok := r.split.GetMergedWord(toks[0], toks[2], src)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Priority = 63
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(word, span.GetContent(src))}
	l.Message = fmt.Sprintf("The auxiliary verb “%s” implies the existence of the closed compound noun “%s”.", toks[4].GetStr(src), string(word))
	return &l
}
