package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// crowdFacts are a user's memory of 40 facts, and crowdProbes ask for ten of
// them twice: once in other words than the fact's, once sharing a word with
// it.
var crowdFacts = []string{
	"The user's dog is called Biscuit, a three-year-old beagle",
	"The user plays cello in a community orchestra on Thursdays",
	"The user lives in Lisbon",
	"The user drinks green tea every morning and never coffee",
	"The user drives a 2015 Toyota Corolla",
	"The user works as an ICU nurse on night shifts",
	"The user is learning Japanese and is at JLPT N4 level",
	"The user's main code editor is Helix",
	"The user's login shell is fish",
	"The user's favourite football club is Benfica",
	"The user prefers answers with metric units",
	"The user keeps their notes in Obsidian",
	"The user runs Arch Linux on a ThinkPad X1 Carbon",
	"The user prefers dark themes in every app",
	"The user is vegetarian",
	"The user goes bouldering twice a week",
	"The user's sister Ana lives in Porto",
	"The user reads science fiction, especially Ursula K. Le Guin",
	"The user backs up their files to a Synology NAS every Sunday",
	"The user writes side projects in Go",
	"The user is saving for a trip to Japan next spring",
	"The user prefers short answers without preamble",
	"The user uses Firefox as their main browser",
	"The user listens to jazz while working",
	"The user's home network uses a Ubiquiti router",
	"The user takes the train to work",
	"The user grows tomatoes and basil on their balcony",
	"The user prefers British spelling",
	"The user has a 27-inch 4K monitor",
	"The user's mechanical keyboard has brown switches",
	"The user started running in March and aims for a half marathon",
	"The user's partner is called Rui",
	"The user prefers Python for data analysis",
	"The user uses Bitwarden as their password manager",
	"The user hosts a podcast about urban cycling",
	"The user bakes sourdough bread at weekends",
	"The user's phone is a Pixel 8",
	"The user volunteers at a food bank once a month",
	"The user wants commit messages in the imperative mood",
	"The user studied architecture at university",
}

var crowdProbes = []struct{ paraphrase, overlap, want string }{
	{"What's the name of my pet?", "What is my dog called?", "Biscuit"},
	{"Which instrument do I play?", "When does my orchestra rehearse?", "cello"},
	{"Which city am I based in?", "Where do I live?", "Lisbon"},
	{"What hot beverage do I have when I wake up?", "What tea do I drink?", "green tea"},
	{"What vehicle do I own?", "What car do I drive?", "Corolla"},
	{"What is my job?", "Where do I work as a nurse?", "nurse"},
	{"Which foreign tongue am I studying?", "What language am I learning?", "Japanese"},
	{"Which program do I write code in?", "What is my code editor?", "Helix"},
	{"What do I type commands into?", "What is my login shell?", "fish"},
	{"Which team do I support?", "What is my favourite football club?", "Benfica"},
}

// crowdStore holds the 40 facts with the bundled vectors model.
func crowdStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := OpenStore(StoreConfig{Dir: filepath.Join(dir, "vault"), TeamDir: filepath.Join(dir, "teams"), DB: filepath.Join(dir, "memory.db"), Embed: potion})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, text := range crowdFacts {
		mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: text})
	}
	return st
}

// found reports whether a search for question puts want among its first k.
func found(t *testing.T, st *Store, question, want string, k int) bool {
	t.Helper()
	got, err := st.Search(context.Background(), question, nil, k)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		if strings.Contains(strings.ToLower(m.Text), strings.ToLower(want)) {
			return true
		}
	}
	return false
}

func TestSearchFindsAFactInACrowd(t *testing.T) {
	st := crowdStore(t)
	para, over := 0, 0
	for _, p := range crowdProbes {
		if found(t, st, p.paraphrase, p.want, 8) {
			para++
		} else {
			t.Logf("missed %q", p.paraphrase)
		}
		if found(t, st, p.overlap, p.want, 8) {
			over++
		} else {
			t.Logf("missed %q", p.overlap)
		}
	}
	if para < 9 || over < 9 {
		t.Fatalf("paraphrased questions found %d/10, word-sharing ones %d/10", para, over)
	}
}

func TestSearchReturnsFewerThanKRatherThanNoise(t *testing.T) {
	st := crowdStore(t)
	ctx := context.Background()
	total := 0
	for _, p := range crowdProbes {
		for _, question := range []string{p.paraphrase, p.overlap} {
			got, err := st.Search(ctx, question, nil, 8)
			if err != nil {
				t.Fatal(err)
			}
			total += len(got)
		}
	}
	if average := float64(total) / 20; average > 4 {
		t.Fatalf("the crowd's questions bring %.1f facts each, padded toward 8", average)
	}
	got, err := st.Search(ctx, "What is my login shell?", nil, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || len(got) > 2 || got[0].Text != "The user's login shell is fish" {
		t.Fatalf("a question one fact answers brought %d facts, first %+v", len(got), got)
	}
	if got, _ := st.Search(ctx, "What is the?", nil, 8); len(got) != 0 {
		t.Fatalf("a question of stop words found %d facts", len(got))
	}
}

func TestAForgottenFactStaysForgottenReworded(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(StoreConfig{Dir: filepath.Join(dir, "vault"), TeamDir: filepath.Join(dir, "teams"), DB: filepath.Join(dir, "memory.db"), Embed: potion})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	id := mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "The user prefers Rust for side projects"}).Items[0].ID
	mustApply(t, st, Op{Kind: OpForget, ID: id})
	for _, text := range []string{"The user loves Rust", "The user likes using Rust for hobby projects"} {
		if buried, _ := st.Tombstoned(ctx, text, protocol.MemoryYou); !buried {
			t.Errorf("%q would be learned again", text)
		}
	}
	for _, text := range []string{"The user doesn't like Rust", "The user prefers Go for side projects", "The user lives in Lisbon"} {
		if buried, _ := st.Tombstoned(ctx, text, protocol.MemoryYou); buried {
			t.Errorf("%q is kept back", text)
		}
	}
}
