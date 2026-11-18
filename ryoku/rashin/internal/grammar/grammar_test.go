package grammar

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

func TestDialectMapping(t *testing.T) {
	for input, want := range map[string]dict.Dialect{"": dict.American, "us": dict.American, "uk": dict.British, "ca": dict.Canadian, "au": dict.Australian, "in": dict.Indian, "UK": dict.British} {
		if got := parseDialect(input); got != want {
			t.Fatalf("%q: %v != %v", input, got, want)
		}
	}
}
func TestCanceledCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Check(ctx, "This is English.", Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestLocalCheckDoesNotGateLanguage(t *testing.T) {
	if _, err := Check(context.Background(), "Esto es español. No debería marcarlo como inglés.", Options{}); err != nil {
		t.Fatal(err)
	}
}
func TestCheckRuneOffsetsAndReplacement(t *testing.T) {
	source := "She has always lived as though she were to the manor born."
	issues, err := Check(context.Background(), source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var found *Issue
	for i := range issues {
		if issues[i].Rule == "ToTheMannerBorn" {
			found = &issues[i]
		}
	}
	if found == nil {
		t.Fatal(issues)
	}
	if string([]rune(source)[found.Start:found.End]) != "to the manor born" || found.Kind != "Eggcorn" || !reflect.DeepEqual(found.Replacements, []string{"to the manner born"}) {
		t.Fatal(*found)
	}
	// Harper 3.2.0 CLI --only ToTheMannerBorn produces this exact record.
	if found.Start != 40 || found.End != 57 || found.Priority != 31 || found.Message != "Use the correct phrase for being naturally suited to something." {
		t.Fatal(*found)
	}
}
func TestDisabledRulesArePerCall(t *testing.T) {
	source := "She has always lived as though she were to the manor born."
	for _, disabled := range []bool{true, false, true, false} {
		opts := Options{}
		if disabled {
			opts.Disabled = []string{"ToTheMannerBorn", "ToTheMannerBorn"}
		}
		issues, err := Check(context.Background(), source, opts)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, issue := range issues {
			found = found || issue.Rule == "ToTheMannerBorn"
		}
		if found == disabled {
			t.Fatal(disabled, issues)
		}
	}
}
func TestUserWordsParticipateInLanguageDetection(t *testing.T) {
	source := "Zorblax Glimmerblax"
	if LooksEnglish(source, nil) {
		t.Fatal("unknown words routed as English")
	}
	words := []string{"Zorblax", "Glimmerblax"}
	if !LooksEnglish(source, words) {
		t.Fatal("user dictionary was omitted from routing")
	}
	issues, err := Check(context.Background(), source, Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range issues {
		if issue.Kind == "Spelling" {
			t.Fatal(issue)
		}
	}
}
func TestRulesMetadataAndCopy(t *testing.T) {
	metadata := Rules()
	if len(metadata) != 970 {
		t.Fatalf("expected 970 Harper rules excluding BoringWords, got %d", len(metadata))
	}
	names := make([]string, len(metadata))
	for i, info := range metadata {
		names[i] = info.Name
		if info.Name == "BoringWords" {
			t.Fatal("excluded thesaurus rule")
		}
		if info.Description == "" {
			t.Fatal(info)
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Fatal("unsorted")
	}
	metadata[0].Name = "changed"
	if Rules()[0].Name == "changed" {
		t.Fatal("metadata aliases shared state")
	}
}
func TestConcurrentCheck(t *testing.T) {
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			issues, err := Check(context.Background(), "She has always lived as though she were to the manor born.", Options{})
			if err != nil || len(issues) == 0 {
				t.Errorf("%v, %v", issues, err)
			}
		})
	}
	wg.Wait()
}

func TestIssueReplacementsNormalizeAndDeduplicate(t *testing.T) {
	entry := lint.NewLint()
	entry.Span = text.Span{Start: 1, End: 3}
	entry.Suggestions = []lint.Suggestion{
		lint.ReplaceWith([]rune("fix")),
		lint.InsertAfter([]rune("!")),
		lint.Remove(),
		lint.ReplaceWith([]rune("fix")),
		lint.ReplaceWith(nil),
	}
	actual := issueFromLint("Example", entry, []rune("é語字a"))
	if !reflect.DeepEqual(actual.Replacements, []string{"fix", "語字!", ""}) {
		t.Fatal(actual)
	}
}

func TestLooksEnglishRouting(t *testing.T) {
	for _, text := range []string{"", "  ", "Our neighbours decided to recieve it.", "recieve", "Ready? Yes!", "Our neighbours decided to recieve it and we were glad to hear the news."} {
		if !LooksEnglish(text, nil) {
			t.Errorf("English text rejected: %q", text)
		}
	}
	for _, text := range []string{"Hola, ¿cómo estás?", "Esto es español.", "你好世界"} {
		if LooksEnglish(text, nil) {
			t.Errorf("non-English text accepted: %q", text)
		}
	}
}

func TestUserDialectWordAcceptedInEveryDialect(t *testing.T) {
	const source = "Our neighbours decided to recieve it."
	matched := func(issues []Issue, word string) bool {
		for _, issue := range issues {
			if issue.Kind == "Spelling" && string([]rune(source)[issue.Start:issue.End]) == word {
				return true
			}
		}
		return false
	}
	before, err := Check(context.Background(), source, Options{Dialect: "us"})
	if err != nil || !matched(before, "neighbours") || !matched(before, "recieve") {
		t.Fatalf("before=%#v, error=%v", before, err)
	}
	for _, dialect := range []string{"us", "uk", "ca", "au", "in"} {
		after, err := Check(context.Background(), source, Options{Dialect: dialect, Words: []string{"Neighbours"}})
		if err != nil || matched(after, "neighbours") || !matched(after, "recieve") {
			t.Fatalf("dialect=%s after=%#v, error=%v", dialect, after, err)
		}
	}
}

func TestUserDictionaryCacheKeepsOnlyLatest(t *testing.T) {
	state := &sharedState{dictionary: dict.Curated()}
	first := state.forWords([]string{"neighbours"})
	if state.forWords([]string{"neighbours"}) != first || first.state(dict.American) != first.state(dict.American) {
		t.Fatal("unchanged dictionary or pool rebuilt")
	}
	second := state.forWords([]string{"colour"})
	if first == second || state.withWords != second {
		t.Fatal("changed words did not replace cached dictionary")
	}
	if state.forWords(nil) != state || state.withWords != nil || state.words != nil {
		t.Fatal("removed dictionary retained its cache")
	}
}

func TestIssueKindDisplayNames(t *testing.T) {
	for kind, want := range map[lint.LintKind]string{lint.WordChoice: "Word Choice", lint.WordOrder: "Word Order"} {
		entry := lint.NewLint()
		entry.LintKind = kind
		if issue := issueFromLint("Example", entry, nil); issue.Kind != want {
			t.Fatalf("kind=%q, want=%q", issue.Kind, want)
		}
	}
}

func BenchmarkCheckUserWords(b *testing.B) {
	source := strings.Repeat("The team reviewed the latest changes this morning and agreed to share the updated draft with everyone before lunch tomorrow. ", 50)
	opts := Options{Words: []string{"ryoku", "rashin", "theia", "hyperion", "orpheus", "kairos", "ryotunes", "ryogami", "ryostore", "ryoshot", "ryolink", "ryovm", "neurmap", "lithic", "quickshell", "matugen", "niri", "hyprland", "glazepkg", "ryoport"}}
	if _, err := Check(context.Background(), source, opts); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := Check(context.Background(), source, opts); err != nil {
			b.Fatal(err)
		}
	}
}
