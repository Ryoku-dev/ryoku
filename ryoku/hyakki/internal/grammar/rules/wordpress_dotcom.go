package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

// WordPressDotcom ensures the proper capitalization of WordPress.com.
type WordPressDotcom struct{}

func NewWordPressDotcom() *WordPressDotcom { return &WordPressDotcom{} }
func (*WordPressDotcom) Description() string {
	return "Ensures correct capitalization of WordPress.com. This rule verifies that the official stylization of WordPress.com is used when referring to the hosting provider."
}
func (*WordPressDotcom) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, host := range doc.IterHostnames(d.Tokens()) {
		word := host.GetStr(d.GetSource())
		if word != "WordPress.com" && strings.ToLower(word) == "wordpress.com" {
			result = append(result, lint.Lint{Span: host.Span, LintKind: lint.Style, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("WordPress.com"))}, Message: "The WordPress hosting provider should be stylized as `WordPress.com`", Priority: 31})
		}
	}
	return result
}
