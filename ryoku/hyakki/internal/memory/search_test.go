package memory

import (
	"context"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/index/embed"
	"ryoku-rashin/internal/protocol"
)

// potion loads the bundled vectors model as the daemon does.
func potion() (Embedder, func(), error) {
	model, err := embed.Load()
	if err != nil {
		return nil, nil, err
	}
	return model, embed.Release, nil
}

func TestSearchWords(t *testing.T) {
	st := testStore(t)
	for _, text := range []string{
		"Keeps the Rashin daemon logs in the state folder",
		"Prefers fish as the login shell",
		"Writes Go with the standard library first",
		"Has a fish tank at home with two goldfish and a fish named Bubbles",
	} {
		mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: text})
	}
	found, err := st.Search(context.Background(), "login shell", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].Text != "Prefers fish as the login shell" {
		t.Fatalf("found = %+v", found)
	}
	if found, _ := st.Search(context.Background(), "goldfish", []string{protocol.MemoryProjects}, 3); len(found) != 0 {
		t.Fatalf("scopes ignored: %+v", found)
	}
}

func TestSearchMeaning(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(StoreConfig{Dir: filepath.Join(dir, "vault"), TeamDir: filepath.Join(dir, "teams"), DB: filepath.Join(dir, "memory.db"), Embed: potion})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, text := range []string{
		"Is allergic to peanuts",
		"Runs the build with make before every commit",
		"Lives in Lisbon with two cats",
		"Prefers dark themes in every editor",
	} {
		mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: text})
	}
	ctx := context.Background()
	found, err := st.Search(ctx, "which foods should I avoid because of a nut allergy reaction", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].Text != "Is allergic to peanuts" {
		t.Fatalf("paraphrase found %+v", found)
	}
	found, err = st.Search(ctx, "home city pets", nil, 1)
	if err != nil || len(found) == 0 || found[0].Text != "Lives in Lisbon with two cats" {
		t.Fatalf("paraphrase found %+v %v", found, err)
	}
	similar, err := st.Similar(ctx, "Likes a dark color scheme", nil, 1)
	if err != nil || len(similar) == 0 || similar[0].Text != "Prefers dark themes in every editor" {
		t.Fatalf("similar = %+v %v", similar, err)
	}

	id := found[0].ID
	if _, err := st.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpForget, ID: id}}); err != nil {
		t.Fatal(err)
	}
	if buried, _ := st.Tombstoned(ctx, "Lives in Lisbon with her two cats", protocol.MemoryYou); !buried {
		t.Fatal("a reworded forgotten fact is not tombstoned")
	}
	if buried, _ := st.Tombstoned(ctx, "Lives in Porto with two cats", protocol.MemoryYou); buried {
		t.Fatal("a fact naming another city is tombstoned")
	}
	if buried, _ := st.Tombstoned(ctx, "Runs the test suite on Fridays", protocol.MemoryYou); buried {
		t.Fatal("an unrelated fact is tombstoned")
	}
	if same, ok, _ := st.Duplicate(ctx, "Is allergic to peanuts.", protocol.MemoryYou); !ok || same == "" {
		t.Fatal("a repeat is not a duplicate")
	}
	if _, ok, _ := st.Duplicate(ctx, "Is not allergic to peanuts", protocol.MemoryYou); ok {
		t.Fatal("a negation is a duplicate")
	}
	st.Release()
	if st.embedder != nil {
		t.Fatal("Release kept the model")
	}
}

func TestSameFact(t *testing.T) {
	cases := []struct {
		left, right string
		same        bool
	}{
		{"Uses fish shell", "Uses the fish shell", true},
		{"Has 2 cats", "Has 3 cats", false},
		{"Works at Acme", "Works at Globex", false},
		{"Likes coffee", "Doesn't like coffee", false},
	}
	for _, c := range cases {
		if got := sameFact(c.left, c.right); got != c.same {
			t.Errorf("sameFact(%q, %q) = %v", c.left, c.right, got)
		}
	}
}
