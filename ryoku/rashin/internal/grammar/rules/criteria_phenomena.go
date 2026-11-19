package rules

import "ryoku-rashin/internal/grammar/lint"

type CriteriaPhenomena struct{ *mergeLintersLinter }

func NewCriteriaPhenomena() *CriteriaPhenomena {
	return &CriteriaPhenomena{&mergeLintersLinter{rules: []lint.Linter{NewSingularCriteriaPhenomena(), NewPluralCriterionPhenomenon()}, description: "The words “criteria” and “phenomena” are the plurals of “criterion” and “phenomenon”, respectively. They are often incorrectly used with the wrong number."}}
}
