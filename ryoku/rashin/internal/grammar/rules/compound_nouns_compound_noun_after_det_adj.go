package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// CompoundNounAfterDetAdj finds two adjacent words separated by whitespace that
// would be a valid noun if joined.
type CompoundNounAfterDetAdj struct {
	expression expr.Expr
	split      *expr.MergeableWords
}

func NewCompoundNounAfterDetAdj() *CompoundNounAfterDetAdj {
	split := expr.NewMergeableWords(compoundNounsPredicate)
	return &CompoundNounAfterDetAdj{expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(func(t doc.Token, s []rune) bool {
		return t.Kind.IsDeterminer() || (t.Kind.IsAdjective() && string(text.ToLower(t.GetCh(s))) != "go")
	}).TWs().Then(compoundNounsIsContentWord).TWs().Then(expr.ButNot(compoundNounsIsContentWord, expr.NewInflectionOfBe())), expr.NewSequenceExprAnything().TAny().Then(split)}), split}
}
func (r *CompoundNounAfterDetAdj) Expr() expr.Expr                  { return r.expression }
func (r *CompoundNounAfterDetAdj) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CompoundNounAfterDetAdj) Description() string {
	return "Detects compound nouns split by a space and suggests merging them when both parts form a valid noun. Has checks to avoid erroneous cases."
}
func (r *CompoundNounAfterDetAdj) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 || text.EqStr(toks[0].GetCh(src), "that") {
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
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(word, span.GetContent(src))}
	l.Message = fmt.Sprintf("Did you mean the closed compound noun “%s”?", string(word))
	return &l
}
