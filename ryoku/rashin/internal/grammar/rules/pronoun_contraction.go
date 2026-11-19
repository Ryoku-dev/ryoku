package rules

import "ryoku-rashin/internal/grammar/lint"

type PronounContraction struct{ *mergeLintersLinter }

func NewPronounContraction() *PronounContraction {
	return &PronounContraction{&mergeLintersLinter{rules: []lint.Linter{NewShouldContract(), NewAvoidContraction()}, description: "Choosing when to contract pronouns is a challenging art. This rule looks for faults."}}
}
