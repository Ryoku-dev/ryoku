package writing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestDictionaryNormalizesLimitsAndRemoves(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx := context.Background()
	for _, word := range []string{"  Ryoku  ", "RYOKU"} {
		out, err := WordsAdd(ctx, word)
		if err != nil || len(out.Words) != 1 || out.Words[0] != strings.TrimSpace(word) {
			t.Fatalf("words=%#v, error=%v", out, err)
		}
	}
	for _, word := range []string{"", "two words", "line\nbreak", "123", "\xff"} {
		if _, err := WordsAdd(ctx, word); err == nil {
			t.Fatalf("accepted %q", word)
		}
	}
	if _, err := WordsRemove(ctx, "Ryoku"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(wordsPath())
	if err != nil || len(raw) != 0 {
		t.Fatalf("removed dictionary=%q, error=%v", raw, err)
	}
	var content strings.Builder
	for i := range maxWords {
		n := i
		word := "term"
		for {
			word += string(rune('a' + n%26))
			n /= 26
			if n == 0 {
				break
			}
		}
		fmt.Fprintln(&content, word)
	}
	if err := os.WriteFile(wordsPath(), []byte(content.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := WordsAdd(ctx, "overflow"); err == nil {
		t.Fatal("accepted more than 5,000 words")
	}
}

func TestCancelledDictionaryChangeKeepsFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := WordsAdd(context.Background(), "ryoku"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WordsAdd(ctx, "rashin"); err == nil {
		t.Fatal("accepted cancelled dictionary change")
	}
	raw, err := os.ReadFile(wordsPath())
	if err != nil || string(raw) != "ryoku\n" {
		t.Fatalf("dictionary=%q, error=%v", raw, err)
	}
}

func TestDictionaryPreservesNewestCasingAndReadsOldFiles(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := WordsAdd(context.Background(), "Neighbours"); err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"neighbours", "NEIGHBOURS", "Neighbours"} {
		result, err := WordsAdd(context.Background(), word)
		if err != nil || len(result.Words) != 1 || result.Words[0] != word {
			t.Fatalf("word=%q result=%#v, error=%v", word, result, err)
		}
		raw, err := os.ReadFile(wordsPath())
		if err != nil || string(raw) != word+"\n" {
			t.Fatalf("saved=%q, error=%v", raw, err)
		}
	}
	if err := os.WriteFile(wordsPath(), []byte("ryoku\nneighbours\nNeighbours\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := WordsGet()
	if err != nil || len(result.Words) != 2 || result.Words[0] != "Neighbours" || result.Words[1] != "ryoku" {
		t.Fatalf("legacy dictionary=%#v, error=%v", result, err)
	}
	result, err = WordsRemove(context.Background(), "NEIGHBOURS")
	if err != nil || len(result.Words) != 1 || result.Words[0] != "ryoku" {
		t.Fatalf("case-insensitive removal=%#v, error=%v", result, err)
	}
}
