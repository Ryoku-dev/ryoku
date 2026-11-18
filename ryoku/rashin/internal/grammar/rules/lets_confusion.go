package rules

import "ryoku-rashin/internal/grammar/lint"

type LetsConfusion struct{ *mergeLintersLinter }

func NewLetsConfusion() *LetsConfusion {
	return &LetsConfusion{&mergeLintersLinter{rules: []lint.Linter{NewLetUsRedundancy(), NewNoContractionWithVerb()}, description: "It's often hard to determine where the subject should go with the word `let`. This rule attempts to find common errors with redundancy and contractions that may lead to confusion for readers."}}
}
