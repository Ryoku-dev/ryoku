package weir

import (
	"fmt"
	"strings"
	"testing"
)

func assertPassesAll(t *testing.T, r *WeirLinter) {
	t.Helper()
	if failures := r.RunTests(); len(failures) != 0 {
		t.Fatalf("failed tests: %#v", failures)
	}
}
func TestSimpleRightClickLinter(t *testing.T) {
	source := "\n            expr main <([right, middle, left] $click), ( )>\n            let message \"Hyphenate this mouse command\"\n            let description \"Hyphenates right-click style mouse commands.\"\n            let kind \"Punctuation\"\n            let becomes \"-\"\n\n            test \"Right click the icon.\" \"Right-click the icon.\"\n            test \"Please right click on the link.\" \"Please right-click on the link.\"\n            test \"They right clicked the submit button.\" \"They right-clicked the submit button.\"\n            test \"Right clicking the item highlights it.\" \"Right-clicking the item highlights it.\"\n            test \"Right clicks are tracked in the log.\" \"Right-clicks are tracked in the log.\"\n            test \"He RIGHT CLICKED the file.\" \"He RIGHT-CLICKED the file.\"\n            test \"Left click the checkbox.\" \"Left-click the checkbox.\"\n            test \"Middle click to open in a new tab.\" \"Middle-click to open in a new tab.\"\n\n            allows \"This test contains the correct version of right-click and therefore shouldn't error.\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 9 {
		t.Fatal(fmt.Sprintf("test count=%d, want 9", got))
	}
}
func TestGSuite(t *testing.T) {
	source := "\n            expr main [(G [Suite, Suit]), (Google Apps for Work)]\n            let message \"Use the updated brand.\"\n            let description \"`G Suite` or `Google Apps for Work` is now called `Google Workspace`\"\n            let kind \"Miscellaneous\"\n            let becomes \"Google Workspace\"\n            let strategy \"Exact\"\n\n            test \"We migrated from G Suite last year.\" \"We migrated from Google Workspace last year.\"\n            test \"This account is still labeled as Google Apps for Work.\" \"This account is still labeled as Google Workspace.\"\n            test \"The pricing page mentions G Suit for legacy plans.\" \"The pricing page mentions Google Workspace for legacy plans.\"\n            test \"New customers sign up for Google Workspace.\" \"New customers sign up for Google Workspace.\"\n\n            allows \"This test contains the correct version of Google Workspace and therefore shouldn't error.\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 5 {
		t.Fatal(fmt.Sprintf("test count=%d, want 5", got))
	}
}
func TestArrayPrefersLongestMatchOverFirstMatch(t *testing.T) {
	for _, main := range []string{"[(capitalized off of), (capitalized off)]", "[(capitalized off), (capitalized off of)]"} {
		source := strings.ReplaceAll("\n            expr main {main}\n            let message \"Use the replacement.\"\n            let description \"Regression test for overlapping Weir array options.\"\n            let kind \"Miscellaneous\"\n            let becomes \"replacement\"\n            let strategy \"Exact\"\n\n            test \"capitalized off of\" \"replacement\"\n            ", "{main}", main)
		r, err := NewWeirLinter(source)
		if err != nil {
			t.Fatal(err)
		}
		assertPassesAll(t, r)
	}
}
func TestGSuiteWithRefs(t *testing.T) {
	source := "\n            expr a (G [Suite, Suit])\n            expr b (Google Apps For Work)\n            expr incorrect [@a, @b]\n\n            expr main @incorrect\n            let message \"Use the updated brand.\"\n            let description \"`G Suite` or `Google Apps for Work` is now called `Google Workspace`\"\n            let kind \"Miscellaneous\"\n            let becomes \"Google Workspace\"\n            let strategy \"Exact\"\n\n            test \"We migrated from G Suite last year.\" \"We migrated from Google Workspace last year.\"\n            test \"This account is still labeled as Google Apps for Work.\" \"This account is still labeled as Google Workspace.\"\n            test \"The pricing page mentions G Suit for legacy plans.\" \"The pricing page mentions Google Workspace for legacy plans.\"\n            test \"New customers sign up for Google Workspace.\" \"New customers sign up for Google Workspace.\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 4 {
		t.Fatal(fmt.Sprintf("test count=%d, want 4", got))
	}
}
func TestScopeDefaultsToChunk(t *testing.T) {
	source := "\n            expr main one**two\n            let message \"Use three.\"\n            let description \"Test chunk-scoped Weir.\"\n            let kind \"Miscellaneous\"\n            let becomes \"three\"\n            let strategy \"Exact\"\n\n            allows \"one, two.\"\n        "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if _, ok := r.IntoSentenceLinter(); ok {
		t.Fatal("default scope converted to sentence")
	}
	if _, ok := r.IntoChunkLinter(); !ok {
		t.Fatal("default scope did not convert to chunk")
	}
}
func TestSentenceScopeCanMatchAcrossChunks(t *testing.T) {
	source := "\n            expr main one**two\n            let message \"Use three.\"\n            let description \"Test sentence-scoped Weir.\"\n            let kind \"Miscellaneous\"\n            let becomes \"three\"\n            let strategy \"Exact\"\n            let scope \"Sentence\"\n\n            test \"one, two.\" \"three.\"\n        "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if _, ok := r.IntoSentenceLinter(); !ok {
		t.Fatal("sentence scope did not convert")
	}
}
func TestInvalidScopeErrors(t *testing.T) {
	source := "\n            expr main one\n            let message \"\"\n            let description \"\"\n            let kind \"Miscellaneous\"\n            let becomes \"\"\n            let scope \"Paragraph\"\n        "
	r, err := NewWeirLinter(source)
	if err != (Error{Kind: InvalidScope}) {
		t.Fatalf("got %v, want invalid scope", err)
	}
	_ = r
}
func TestFailsOnUnresolvedExpr(t *testing.T) {
	source := "\n            expr main @missing\n            let message \"\"\n            let description \"\"\n            let kind \"Miscellaneous\"\n            let becomes \"\"\n            let strategy \"Exact\"\n        "
	r, err := NewWeirLinter(source)
	if err != (Error{Kind: UnableToResolveExpr, Value: "missing"}) {
		t.Fatalf("got %v, want unresolved missing", err)
	}
	_ = r
}
func TestWildcard(t *testing.T) {
	source := "\n            expr main <(NOUN * NOUN), (* NOUN), *>\n            let message \"\"\n            let description \"\"\n            let kind \"Miscellaneous\"\n            let becomes \"\"\n            let strategy \"Exact\"\n\n            test \"I like trees and plants of all kinds\" \"I like trees  plants of all kinds\"\n            test \"homework tempts teachers\" \"homework  teachers\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 2 {
		t.Fatal(fmt.Sprintf("test count=%d, want 2", got))
	}
}
func TestDashes(t *testing.T) {
	source := "\n            expr main --\n            let message \"\"\n            let description \"\"\n            let kind \"Miscellaneous\"\n            let becomes \"-\"\n            let strategy \"Exact\"\n\n            test \"This--and--that\" \"This-and-that\"\n\n            allows \"this-and-that\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 2 {
		t.Fatal(fmt.Sprintf("test count=%d, want 2", got))
	}
}
func TestFailsOnIgnoreTest(t *testing.T) {
	source := "\n            expr main test\n            let message \"\"\n            let description \"\"\n            let kind \"Miscellaneous\"\n            let becomes \"-\"\n            let strategy \"Exact\"\n\n            allows \"test\"\n            "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(r.RunTests()); got != 1 {
		t.Fatalf("failed count=%d, want 1", got)
	}
}
func TestErrorsProperlyWithMissingExpr(t *testing.T) {
	source := "expr main"
	r, err := NewWeirLinter(source)
	if err != (Error{Kind: ExpectedVariableUndefined}) {
		t.Fatal("expected undefined variable error")
	}
	_ = r
}
func TestBecomesArrayWithManyAlternatives(t *testing.T) {
	source := "\n expr main (the fact)\n let message \"Consider alternative phrasing\"\n let description \"Test that all 'becomes' alternatives can be reached\"\n let kind \"Miscellaneous\"\n let becomes [\"the allegation\", \"the idea\", \"the claim\", \"the story\", \"the rumor\"]\n let strategy \"Exact\"\n\n test \"There is truth to the fact that people like images.\" \"There is truth to the allegation that people like images.\"\n test \"There is truth to the fact that people like images.\" \"There is truth to the idea that people like images.\"\n test \"There is truth to the fact that people like images.\" \"There is truth to the claim that people like images.\"\n test \"There is truth to the fact that people like images.\" \"There is truth to the story that people like images.\"\n test \"There is truth to the fact that people like images.\" \"There is truth to the rumor that people like images.\"\n\n allows \"There is truth to the story that people like images.\"\n "
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	assertPassesAll(t, r)
	if got := r.CountTests(); got != 6 {
		t.Fatal(fmt.Sprintf("test count=%d, want 6", got))
	}
}
func FuzzDoesNotPanic(f *testing.F) {
	for _, s := range []string{"", "expr main", "let", "expr main [", "expr main @", "test \""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _, _ = NewWeirLinter(s) })
}
