package rules

import "ryoku-rashin/internal/grammar/lint"

type HopHope struct{ *mergeLintersLinter }

func NewHopHope() *HopHope {
	return &HopHope{&mergeLintersLinter{rules: []lint.Linter{NewToHop(), NewToHope()}, description: "Handles common errors involving `hop` and `hope`. Ensures `hop` is used correctly in phrases like `hop on a bus` while correcting mistaken uses of `hope` in contexts where `hop` is expected."}}
}
