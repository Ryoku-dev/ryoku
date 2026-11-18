package rules

import "ryoku-rashin/internal/grammar/lint"

type NounInsteadOfVerb struct{ *mergeLintersLinter }

func NewNounInsteadOfVerb() *NounInsteadOfVerb {
	return &NounInsteadOfVerb{&mergeLintersLinter{rules: []lint.Linter{NewGeneralNounInsteadOfVerb(), NewEffectAffect()}, description: "Corrects noun/verb confusions such as `advice/advise` and handles the common `effect/affect` mix-up."}}
}
