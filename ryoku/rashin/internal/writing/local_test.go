package writing

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"ryoku-rashin/internal/grammar"
	"ryoku-rashin/internal/protocol"
)

func TestLocalRuneOffsetsAndOptions(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, err := WordsAdd(context.Background(), "Ryoku")
	if err != nil {
		t.Fatal(err)
	}
	out, err := checkWith(context.Background(), nil, protocol.WritingCheckParams{Text: "🙂 中文 teh 😀", Engine: "local", Dialect: "in"}, func(ctx context.Context, text string, opts grammar.Options) ([]grammar.Issue, error) {
		if opts.Dialect != "in" || !slices.Contains(opts.Words, "Ryoku") {
			t.Fatalf("options=%#v", opts)
		}
		return []grammar.Issue{{Start: 5, End: 8, Kind: "Spelling", Rule: "SpellCheck", Message: "Check spelling.", Replacements: []string{"the"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Engine != "local" || len(out.Issues) != 1 {
		t.Fatalf("result=%#v", out)
	}
	issue := out.Issues[0]
	if issue.Start != 6 || issue.End != 9 || issue.Category != "spelling" || issue.Severity != "error" || !reflect.DeepEqual(issue.Replacements, []string{"the"}) {
		t.Fatalf("issue=%#v", issue)
	}
}

func TestSavedWordReachesCheckerAfterReload(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := func(ctx context.Context, text string, opts grammar.Options) ([]grammar.Issue, error) {
		if slices.Contains(opts.Words, "RYOKU") {
			return nil, nil
		}
		return []grammar.Issue{{Start: 0, End: 5, Kind: "Spelling", Rule: "SpellCheck"}}, nil
	}
	p := protocol.WritingCheckParams{Text: "Ryoku", Engine: "local"}
	before, err := checkWith(context.Background(), nil, p, fake)
	if err != nil || len(before.Issues) != 1 {
		t.Fatalf("before=%#v, error=%v", before, err)
	}
	if _, err := WordsAdd(context.Background(), "  RYOKU  "); err != nil {
		t.Fatal(err)
	}
	wordsCache.Lock()
	wordsCache.path = ""
	wordsCache.words = nil
	wordsCache.Unlock()
	after, err := checkWith(context.Background(), nil, p, fake)
	if err != nil || len(after.Issues) != 0 {
		t.Fatalf("after=%#v, error=%v", after, err)
	}
}

func TestLocalCancellationStopsChecker(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() { <-started; cancel() }()
	_, err := checkWith(ctx, nil, protocol.WritingCheckParams{Text: "The draft is ready.", Engine: "local"}, func(ctx context.Context, _ string, _ grammar.Options) ([]grammar.Issue, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestKindsFiltersAndDuplicateSpans(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	calls := 0
	fake := func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
		calls++
		return []grammar.Issue{{Start: 0, End: 3, Kind: "Spelling"}, {Start: 0, End: 3, Kind: "Grammar"}, {Start: 4, End: 8, Kind: "Punctuation"}}, nil
	}
	out, err := checkWith(context.Background(), nil, protocol.WritingCheckParams{Text: "teh text", Off: []string{"spelling"}}, fake)
	if err != nil || len(out.Issues) != 2 || out.Issues[0].Category != "grammar" {
		t.Fatalf("result=%#v, error=%v", out, err)
	}
	out, err = checkWith(context.Background(), nil, protocol.WritingCheckParams{Text: "teh text", Kinds: []string{"inclusive"}}, fake)
	if err != nil || calls != 1 || len(out.Issues) != 0 {
		t.Fatalf("result=%#v, calls=%d, error=%v", out, calls, err)
	}
}
