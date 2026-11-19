// Package linttest ports Harper's plain-English rule assertions.
package linttest

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"sort"
	"testing"
)

// TestingT permits testing.T and lightweight recorders for helper tests.
type TestingT interface {
	Helper()
	Fatalf(string, ...any)
}

func asLinter(l any) lint.Linter {
	if v, ok := l.(lint.Linter); ok {
		return v
	}
	if v, ok := l.(lint.ExprLinter); ok {
		return lint.AsLinter(v)
	}
	panic("not a Harper linter")
}
func AssertNoLints(t TestingT, text string, l any) { t.Helper(); AssertLintCount(t, text, l, 0) }
func AssertLintCount(t TestingT, text string, l any, count int) {
	t.Helper()
	actual := asLinter(l).Lint(doc.NewPlainEnglishCurated(text))
	if len(actual) != count {
		t.Fatalf("Expected %q to create %d lints, got %d: %+v", text, count, len(actual), actual)
	}
}
func AssertSuggestionCount(t TestingT, text string, l any, count int) {
	t.Helper()
	n := 0
	for _, v := range asLinter(l).Lint(doc.NewPlainEnglishCurated(text)) {
		n += len(v.Suggestions)
	}
	if n != count {
		t.Fatalf("Expected %d suggestions, got %d", count, n)
	}
}
func search(text string, l lint.Linter, needle string, depth int, first bool) bool {
	if depth > lint.MaxSuggestionTransformationDepth {
		return false
	}
	if text == needle {
		return true
	}
	ls := l.Lint(doc.NewPlainEnglishCurated(text))
	sort.SliceStable(ls, func(i, j int) bool { return ls[i].Priority < ls[j].Priority })
	if len(ls) == 0 {
		return false
	}
	for _, s := range ls[0].Suggestions {
		chars := []rune(text)
		s.Apply(ls[0].Span, &chars)
		if search(string(chars), l, needle, depth+1, first) {
			return true
		}
		if first {
			break
		}
	}
	return false
}
func AssertSuggestionResult(t TestingT, text string, l any, needle string) {
	t.Helper()
	if !search(text, asLinter(l), needle, 0, false) {
		t.Fatalf("No suggestion sequence produced %q from %q", needle, text)
	}
}
func AssertFirstSuggestionResult(t TestingT, text string, l any, needle string) {
	t.Helper()
	if !search(text, asLinter(l), needle, 0, true) {
		t.Fatalf("The primary suggestion sequence failed to produce %q from %q", needle, text)
	}
}
func AssertNotInSuggestionResult(t TestingT, text string, l any, bad string) {
	t.Helper()
	if search(text, asLinter(l), bad, 0, false) {
		t.Fatalf("A suggestion sequence produced undesired result %q", bad)
	}
}
func AssertGoodAndBadSuggestions(t TestingT, text string, l any, good, bad []string) {
	t.Helper()
	missing := map[string]bool{}
	forbidden := map[string]bool{}
	for _, s := range good {
		missing[s] = true
	}
	for _, s := range bad {
		forbidden[s] = true
	}
	for _, v := range asLinter(l).Lint(doc.NewPlainEnglishCurated(text)) {
		for _, s := range v.Suggestions {
			chars := []rune(text)
			s.Apply(v.Span, &chars)
			result := string(chars)
			if forbidden[result] {
				t.Fatalf("Found bad suggestion %q", result)
				return
			}
			delete(missing, result)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("Missing good suggestions: %v", missing)
	}
}
func AssertLintMessage(t TestingT, text string, l any, expected string) {
	t.Helper()
	ls := asLinter(l).Lint(doc.NewPlainEnglishCurated(text))
	if len(ls) == 0 {
		t.Fatalf("Expected lint message %q, but no lints found", expected)
		return
	}
	if ls[0].Message != expected {
		t.Fatalf("Expected lint message %q, got %q", expected, ls[0].Message)
	}
}
func AssertEachNoLints(t TestingT, cases []string, l any) {
	t.Helper()
	for _, s := range cases {
		AssertNoLints(t, s, l)
	}
}
func AssertEachLints(t TestingT, cases []string, l any) {
	t.Helper()
	for _, s := range cases {
		AssertLintCount(t, s, l, 1)
	}
}
func AssertEachSuggestionResult(t TestingT, cases [][2]string, l any) {
	t.Helper()
	for _, s := range cases {
		AssertSuggestionResult(t, s[0], l, s[1])
	}
}

var _ TestingT = (*testing.T)(nil)
