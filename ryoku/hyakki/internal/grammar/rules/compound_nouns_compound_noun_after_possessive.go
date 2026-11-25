package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

// CompoundNounAfterPossessive looks for closed compound nouns which can be
// condensed due to their position after a possessive noun, which implies ownership.
type CompoundNounAfterPossessive struct {
	expression expr.Expr
	split      *expr.MergeableWords
}

func NewCompoundNounAfterPossessive() *CompoundNounAfterPossessive {
	split := expr.NewMergeableWords(compoundNounsPredicate)
	return &CompoundNounAfterPossessive{expr.NewAll([]expr.Expr{expr.NewSequenceExpr().ThenPossessiveNominal().TWs().Then(compoundNounsIsContentWord).TWs().Then(compoundNounsIsContentWord), expr.NewSequenceExprAnything().TAny().Then(split)}), split}
}
func (r *CompoundNounAfterPossessive) Expr() expr.Expr { return r.expression }
func (r *CompoundNounAfterPossessive) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *CompoundNounAfterPossessive) Description() string {
	return "Detects split compound nouns following a possessive noun and suggests merging them."
}
func (r *CompoundNounAfterPossessive) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	first := strings.ReplaceAll(strings.ToLower(toks[0].GetStr(src)), "’", "'")
	if first == "let's" || first == "that's" {
		return nil
	}
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	word, ok := r.split.GetMergedWord(toks[2], toks[4], src)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Priority = 63
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(word)}
	l.Message = fmt.Sprintf("The possessive noun implies ownership of the closed compound noun “%s”.", string(word))
	return &l
}
