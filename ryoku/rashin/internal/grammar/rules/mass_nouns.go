package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type MassNouns struct {
	massPlurals      *MassPlurals
	nounCountability *NounCountability
}

func NewMassNouns(d dict.Dictionary) *MassNouns {
	return &MassNouns{massPlurals: NewMassPlurals(d), nounCountability: NewNounCountability()}
}
func (r *MassNouns) Lint(d *doc.Document) []lint.Lint {
	return lint.RemoveOverlaps(append(r.massPlurals.Lint(d), r.nounCountability.Lint(d)...))
}
func (r *MassNouns) Description() string { return "Detects mass nouns used as countable nouns." }
