package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// activeTexts are the texts of the active memories about the user.
func activeTexts(t *testing.T, st *Store) map[string]bool {
	t.Helper()
	active, err := st.Active(protocol.MemoryYou)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range active {
		out[m.Text] = true
	}
	return out
}

func TestFailedWriteThenHandEdit(t *testing.T) {
	st := testStore(t)
	jazz := mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "The user hates jazz"}).Items[0].ID
	if err := os.Chmod(st.cfg.Dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "The user plays the cello"}}); err == nil {
		t.Fatal("the file write did not fail")
	}
	_, _ = st.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpForget, ID: jazz}})
	if err := os.Chmod(st.cfg.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeYou(t, st, strings.Replace(youFile(t, st), "## Replaced", "- Drinks green tea\n\n## Replaced", 1))
	if err := st.Sync(); err != nil {
		t.Fatal(err)
	}
	texts := activeTexts(t, st)
	if !texts["The user plays the cello"] || texts["The user hates jazz"] || !texts["Drinks green tea"] {
		t.Fatalf("active = %v", texts)
	}
	if buried, _ := st.Tombstoned(t.Context(), "The user plays the cello", protocol.MemoryYou); buried {
		t.Fatal("the fact the file missed was tombstoned")
	}
	file := youFile(t, st)
	if !strings.Contains(file, "The user plays the cello ^") || strings.Contains(file, "jazz") {
		t.Fatalf("file =\n%s", file)
	}
}

func TestPendingWriteSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	cfg := StoreConfig{Dir: filepath.Join(dir, "vault"), TeamDir: filepath.Join(dir, "teams"), DB: filepath.Join(dir, "memory.db")}
	st, err := OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "Uses fish"})
	if err := os.Chmod(cfg.Dir, 0o500); err != nil {
		t.Fatal(err)
	}
	_, _ = st.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "Uses niri"}})
	_ = st.Close()
	if err := os.Chmod(cfg.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if err := again.Sync(); err != nil {
		t.Fatal(err)
	}
	if file := youFile(t, again); !strings.Contains(file, "Uses niri ^") {
		t.Fatalf("file after restart =\n%s", file)
	}
	if !activeTexts(t, again)["Uses niri"] {
		t.Fatal("the pending fact was lost")
	}
}

func TestHandEditBeforeAWriteIsRead(t *testing.T) {
	st := testStore(t)
	mustApply(t, st, Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "Uses fish"})
	st.mu.Lock()
	tx := st.begin(Origin{By: protocol.MemoryByYou})
	if err := tx.apply(Op{Kind: OpAdd, Scope: protocol.MemoryYou, Text: "Uses niri"}); err != nil {
		st.mu.Unlock()
		t.Fatal(err)
	}
	// the hand edit lands between the change's read of the files and its write
	writeYou(t, st, strings.Replace(youFile(t, st), "## Replaced", "- Drinks green tea\n\n## Replaced", 1))
	_, err := tx.commit()
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	texts := activeTexts(t, st)
	if !texts["Drinks green tea"] || !texts["Uses niri"] || !texts["Uses fish"] {
		t.Fatalf("active = %v", texts)
	}
	if file := youFile(t, st); !strings.Contains(file, "Drinks green tea ^") || !strings.Contains(file, "Uses niri ^") {
		t.Fatalf("file =\n%s", file)
	}
}

func TestKnownProjectNameWritesNothing(t *testing.T) {
	st := testStore(t)
	scope := protocol.MemoryProjectPrefix + "0123456789ab"
	st.nameScope(scope, "Alpha")
	stats := st.db.Stats()
	before := stats.TxStats.GetWrite()
	st.nameScope(scope, "Alpha")
	stats = st.db.Stats()
	if after := stats.TxStats.GetWrite(); after != before {
		t.Fatalf("naming a known project wrote the ledger %d times", after-before)
	}
}

func TestRepositoryRootIsCached(t *testing.T) {
	var keys projectKeys
	dir := t.TempDir()
	keys.root(dir)
	if _, ok := keys.roots[dir]; !ok {
		t.Fatal("the folder's root was not kept")
	}
}
