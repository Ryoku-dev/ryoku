package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type mergeLintersLinter struct {
	rules       []lint.Linter
	description string
}

func (r *mergeLintersLinter) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, rule := range r.rules {
		result = append(result, rule.Lint(d)...)
	}
	return lint.RemoveOverlaps(result)
}
func (r *mergeLintersLinter) Description() string { return r.description }
