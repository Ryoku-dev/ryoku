package rules

import "ryoku-rashin/internal/grammar/lint"

type ToTwoToo struct{ *mergeLintersLinter }

func NewToTwoToo() *ToTwoToo {
	return &ToTwoToo{&mergeLintersLinter{rules: []lint.Linter{NewToTooAdjectiveEnd(), NewToTooAdjectivePunct(), NewToTooAdverb(), NewToTooAdjVerbEdPunct(), NewToTooChunkStartComma(), NewToTooDegreeWords(), NewToTooPronounEnd(), NewToTooEos(), NewTooTo()}, description: "Corrects homophone confusion between `to` and `too`."}}
}
