package rules

import "ryoku-rashin/internal/grammar/lint"

type ItsContraction struct{ *mergeLintersLinter }

func NewItsContraction() *ItsContraction {
	return &ItsContraction{&mergeLintersLinter{rules: []lint.Linter{NewGeneral(), NewProperNoun()}, description: "Detects places where the possessive `its` should be the contraction `it's`, including before verbs/clauses and before proper nouns after opinion verbs."}}
}
