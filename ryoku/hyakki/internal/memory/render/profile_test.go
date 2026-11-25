package render

import (
	"strings"
	"testing"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

func TestProfileComesBeforeLearnedFacts(t *testing.T) {
	card := "- A remembered fact"
	if got := Profile(protocol.MemoryProfile{}, card); got != card {
		t.Fatalf("empty profile changed card: %q", got)
	}
	profile := protocol.MemoryProfile{Name: "Rae", Work: "Research", Traits: []string{"brief", "with examples"}, About: "Use metric units"}
	got := Profile(profile, card)
	if !strings.HasPrefix(got, "- Name to use: Rae\n") || !strings.HasSuffix(got, card) {
		t.Fatalf("profile order: %q", got)
	}
	if !strings.Contains(AskSystem(got, true, true, 1, 0), "Preferred answer style: brief, with examples") {
		t.Fatal("Ask lost profile preferences")
	}
}

func TestProfileKeepsCardWithinLimit(t *testing.T) {
	profile := protocol.MemoryProfile{Name: strings.Repeat("n", 80), Work: strings.Repeat("w", 240), About: strings.Repeat("a", 600)}
	got := Profile(profile, strings.Repeat("- remembered fact\n", 200))
	if utf8.RuneCountInString(got) > protocol.MemoryCardMax {
		t.Fatal("profile exceeded card cap")
	}
	got = Profile(protocol.MemoryProfile{Name: "</user_card>"}, "")
	if strings.Contains(got, "</user_card>") {
		t.Fatal("profile can close card fence")
	}
}
