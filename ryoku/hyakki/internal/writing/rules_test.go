package writing

import (
	"context"
	"math"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestUTF16RuleOffsets(t *testing.T) {
	text := "🙂 café the the."
	issues := clarityIssues(text)
	for _, issue := range issues {
		if issue.Category == "repetition" {
			if issue.Start != 8 || issue.End != 15 {
				t.Fatalf("repeated-word span = %d:%d, want 8:15", issue.Start, issue.End)
			}
			return
		}
	}
	t.Fatal("missing repeated-word issue")
}

func TestLocalRuleFamilies(t *testing.T) {
	for _, tc := range []struct{ name, text, category string }{
		{"passive", "The report was written yesterday.", "passive"},
		{"weasel", "This is clearly better.", "weasel"},
		{"there", "There are three options.", "wordiness"},
		{"adverb", "She walked quickly.", "adverbs"},
		{"cliche", "At the end of the day, we agreed.", "cliches"},
		{"wordiness", "We met in order to plan.", "wordiness"},
		{"repetition", "The the change works.", "repetition"},
		{"spaces", "Leave  one space.", "formatting"},
		{"quotes", "She said \"yes\" and he said “no”.", "consistency"},
		{"readability", strings.Repeat("word ", 36) + ".", "readability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, issue := range clarityIssues(tc.text) {
				if issue.Category == tc.category {
					return
				}
			}
			t.Fatalf("missing %s for %q", tc.category, tc.text)
		})
	}
}

func TestInclusiveCuratedFamilies(t *testing.T) {
	for _, term := range []string{"chairman", "freshmen", "wheelchair-bound", "illegal alien", "master", "slave", "whitelist", "blacklist"} {
		issues := inclusiveIssues("🙂 café " + term)
		if len(issues) != 1 || issues[0].Start != 8 || issues[0].End != 8+len(term) || len(issues[0].Replacements) == 0 {
			t.Errorf("%q: %#v", term, issues)
		}
	}
	if issues := inclusiveIssues("blacklisting craftsmen masterful work"); len(issues) != 0 {
		t.Fatalf("matched part of a word: %#v", issues)
	}
}

func TestReadabilityKnownText(t *testing.T) {
	stats := Stats("The cat sat on the mat. The dog ran to the park.")
	if stats.Words != 12 || stats.Sentences != 2 {
		t.Fatalf("word/sentence count = %#v", stats)
	}
	if math.Abs(stats.ReadingEase-116.145) > 0.0001 || math.Abs(stats.Grade-(-1.45)) > 0.0001 || math.Abs(stats.Minutes-12.0/230) > 0.0001 {
		t.Fatalf("Flesch results = %#v", stats)
	}
	if stats := Stats("  "); stats != (protocol.WritingStats{}) {
		t.Fatalf("empty text stats = %#v", stats)
	}
}

func TestAITellStockPhrasesScoreHigher(t *testing.T) {
	plain := "I lost my keys on Tuesday. They were under the sofa. I looked for ages before my sister found them. Next time I'll put them by the door."
	stock := "In today's fast-paced world, we delve into a rich tapestry of ideas. Furthermore, it's important to note the possibilities. Moreover, this is a testament to our work. In conclusion, we unlock the potential in the ever-evolving landscape."
	a, b := AITell(plain), AITell(stock)
	if b.Score <= a.Score || len(b.Phrases) == 0 {
		t.Fatalf("plain=%#v, stock=%#v", a, b)
	}
	for _, signal := range b.Signals {
		if signal.Value < 0 || signal.Value > 1 {
			t.Errorf("signal outside 0..1: %#v", signal)
		}
	}
}

func TestRulesNeedNoModelAndFilterOff(t *testing.T) {
	out, err := Check(context.Background(), nil, protocol.WritingCheckParams{
		Text: "The the chairman was informed very quickly.", Kinds: []string{"clarity", "inclusive", "readability", "aitell"},
		Off: []string{"repetition", "inclusive", "adverbs", "passive", "weasel"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Engine != "local" || len(out.Issues) != 0 || out.AITell == nil || out.Stats.Words == 0 {
		t.Fatalf("unexpected local-only result: %#v", out)
	}
}

func TestWeaselExceptionsAndWordBoundaries(t *testing.T) {
	for _, issue := range clarityIssues("Too many friendly people arrive early with excellentness.") {
		if issue.Category == "weasel" || issue.Category == "adverbs" {
			t.Fatalf("unexpected issue: %#v", issue)
		}
	}
}
