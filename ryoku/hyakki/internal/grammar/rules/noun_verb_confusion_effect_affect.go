package rules

import "ryoku-rashin/internal/grammar/lint"

type EffectAffect struct{ *mergeLintersLinter }

func NewEffectAffect() *EffectAffect {
	return &EffectAffect{&mergeLintersLinter{rules: []lint.Linter{NewEffectToAffect(), NewAffectToEffect()}, description: "Guides writers toward the right choice between `effect` and `affect`, correcting each term when it shows up in the other one's role."}}
}
