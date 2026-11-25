package weir

import (
	"fmt"
	"testing"
)

func TestEmbeddedRules(t *testing.T) {
	files := EmbeddedRules()
	if len(files) != 427 {
		t.Fatalf("embedded %d files, want 427", len(files))
	}
	tests, allows := 0, 0
	for _, f := range files {
		t.Run(f.Path, func(t *testing.T) {
			r, err := NewWeirLinter(f.Source)
			if err != nil {
				t.Fatal(err)
			}
			for i, tc := range r.Tests() {
				if tc.Allows {
					allows++
				} else {
					tests++
				}
				t.Run(fmt.Sprintf("%04d", i+1), func(t *testing.T) {
					if fail := r.RunTest(tc); fail != nil {
						t.Fatalf("input %q: got %q, want %q", tc.Input, fail.Got, fail.Expected)
					}
				})
			}
		})
	}
	t.Logf("executed %d test declarations and %d allows declarations", tests, allows)
	if tests != 2222 || allows != 652 {
		t.Errorf("declaration counts = %d tests, %d allows; want 2222 and 652", tests, allows)
	}
}
func TestRuleRegistration(t *testing.T) {
	files := EmbeddedRules()
	public := map[string]bool{}
	for _, f := range files {
		public[f.PublicName] = true
	}
	rules := Rules()
	if len(rules) != len(public) {
		t.Fatalf("registered %d rules, want %d", len(rules), len(public))
	}
	for _, r := range rules {
		if !public[r.Name] {
			t.Fatalf("unexpected public rule %q", r.Name)
		}
		delete(public, r.Name)
	}
	if len(public) != 0 {
		t.Fatalf("missing public rules: %v", public)
	}
}
