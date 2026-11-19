package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func TestSpellCheck_OracleSuggestionOrder(t *testing.T) {
	cases := []struct {
		word   string
		values []string
	}{
		{"recieve", []string{"receive", "relieve", "recipe"}},
		{"abandondware", []string{"abandonware"}},
		{"Generallly", []string{"Generally", "Generality"}},
		{"colour", []string{"color", "cloud", "collar"}},
		{"persson", []string{"person", "parson", "pension"}},
		{"childrens", []string{"children's", "children", "childless"}},
		{"v’s", []string{"vs", "Av's", "E's"}},
		{"Nz", []string{"No", "Nu", "NE", "NZ"}},
	}
	rule := NewSpellCheck(dict.Curated(), dict.American)
	for _, c := range cases {
		t.Run(c.word, func(t *testing.T) {
			for range 2 {
				got := rule.Lint(doc.NewPlainEnglishCurated(c.word))
				if len(got) != 1 {
					t.Fatalf("got %d lints, want 1", len(got))
				}
				l := got[0]
				if l.Span != (text.Span{Start: 0, End: len([]rune(c.word))}) || l.Priority != 63 || l.LintKind != lint.Spelling {
					t.Fatalf("incorrect lint metadata: %+v", l)
				}
				if len(l.Suggestions) != len(c.values) {
					t.Fatalf("got %v, want %v", l.Suggestions, c.values)
				}
				for i, s := range l.Suggestions {
					if !s.IsReplaceWith() || string(s.Replacement) != c.values[i] {
						t.Fatalf("got %v, want %v", l.Suggestions, c.values)
					}
				}
				message := "Did you mean to spell `" + c.word + "` this way?"
				if len(c.values) == 1 {
					message = "Did you mean `" + c.values[0] + "`?"
				}
				if l.Message != message {
					t.Fatalf("got message %q, want %q", l.Message, message)
				}
			}
		})
	}
}
