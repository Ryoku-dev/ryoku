package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

func compoundNounsIsContentWord(tok doc.Token, src []rune) bool {
	md, ok := tok.Kind.AsWord()
	if !ok || md == nil {
		return false
	}
	return tok.Span.Len() > 1 && (md.IsNoun() || md.IsAdjective() || md.IsVerb() || md.IsAdverb()) && !(md.IsDeterminer() || md.IsConjunction()) && (!md.Preposition || text.EqAnyIgnoreASCIICaseStr(tok.GetCh(src), []string{"bar", "through"}))
}
func compoundNounsPredicate(closed, open *dict.DictWordMetadata) bool {
	return open == nil && closed != nil && closed.IsNoun() && !closed.IsProperNoun()
}

type CompoundNouns struct{ linters []lint.Linter }

func NewCompoundNouns() *CompoundNouns {
	return &CompoundNouns{[]lint.Linter{NewCompoundNounAfterDetAdj(), NewCompoundNounBeforeAuxVerb(), NewCompoundNounAfterPossessive()}}
}
func (r *CompoundNouns) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, l := range r.linters {
		out = append(out, l.Lint(d)...)
	}
	return lint.RemoveOverlaps(out)
}
func (r *CompoundNouns) Description() string {
	return "Detects compound nouns split by a space and suggests merging them when both parts form a valid noun."
}
