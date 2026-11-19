package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestCataloguePinsLFM3(t *testing.T) {
	entries := Catalogue()
	if len(entries) != 1 {
		t.Fatalf("catalogue entries = %d", len(entries))
	}
	e := entries[0]
	if e.ID != "lfm3-omni-2b" || e.Revision != "d508b88f08a5b4286f2d2805a6a8d69a0ff44c72" || e.minBuild() != 11960 || e.Context != 32768 || e.Class != protocol.LocalClassLight || !e.Tools || !e.Vision || !e.Audio || e.Reasoning {
		t.Fatalf("unexpected catalogue entry: %+v", e)
	}
	if len(e.Files) != 2 || e.Files[0].Size != 1099630080 || e.Files[1].Size != 291586720 || e.Files[0].SHA256 != "9f8d30e9d4fd7a9349d586e077988229b3ca0a0e4ad730897563d1eccc9893e2" || e.Files[1].SHA256 != "0a746c0794782a16d4456b230c2a0b5888a2d94a6677211edbed7d7a4177a39a" {
		t.Fatalf("unexpected pinned files: %+v", e.Files)
	}
	entries[0].Files[0].SHA256 = "replaced"
	if Catalogue()[0].Files[0].SHA256 == "replaced" {
		t.Fatal("catalogue files leaked mutable backing array")
	}
}

func TestAdoptExistingWeightsOnce(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	e := Entry{ID: "local-test", Repo: "Unit/model", Revision: "abcdef123456789012345678901234567890abcd", Files: []File{{Name: "model.gguf", Role: protocol.LocalFileModel, Size: 5}}, Context: 16384}
	content := []byte("model")
	checksum := sha256.Sum256(content)
	e.Files[0].SHA256 = hex.EncodeToString(checksum[:])
	if err := os.MkdirAll(repoDir(e.Repo), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath(e, "model.gguf"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found := readManifest(e); found {
		t.Fatal("unexpected manifest")
	}
	if err := writeCustom([]Entry{e}); err != nil {
		t.Fatal(err)
	}
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	if got, adopt := onDisk(e); got.State != protocol.LocalAvailable || !adopt {
		t.Fatalf("onDisk = %+v, adopt=%v", got, adopt)
	}
	if installed := manager.Inventory(); len(installed) != 0 {
		t.Fatalf("provider inventory adopted files: %+v", installed)
	}
	if listed := manager.Models(context.Background()).Models; listed[len(listed)-1].State != protocol.LocalInstalled {
		t.Fatalf("explicit local models listing did not adopt: %+v", listed)
	}
	if _, found := readManifest(e); !found {
		t.Fatal("verified files were not recorded")
	}
	if got, adopt := onDisk(e); got.State != protocol.LocalInstalled || adopt {
		t.Fatalf("onDisk = %+v, adopt=%v", got, adopt)
	}
	if err := os.WriteFile(filePath(e, "model.gguf"), []byte("wrong"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fetch(context.Background(), manager.client, fetchRequest{Dest: filePath(e, "model.gguf"), Size: 5, SHA256: e.Files[0].SHA256, Offline: true}, func(string, int64) {}); err == nil {
		t.Fatal("offline verification accepted changed weights")
	}
	if _, err := os.Stat(filepath.Join(paths.ModelsDir(), "Unit", "model", "rashin-local-test.json")); err != nil {
		t.Fatal(err)
	}
}

func TestBrokenAdoptionCachesByFileStampWithoutHoldingManagerLock(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	e := Entry{ID: "broken-model", Repo: "Unit/broken", Revision: "abcdef123456789012345678901234567890abcd",
		Files: []File{{Name: "model.gguf", Role: protocol.LocalFileModel, Size: 5}}}
	correct := []byte("model")
	sum := sha256.Sum256(correct)
	e.Files[0].SHA256 = hex.EncodeToString(sum[:])
	if err := os.MkdirAll(repoDir(e.Repo), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filePath(e, "model.gguf")
	if err := os.WriteFile(path, []byte("wrong"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	first := m.modelOnDisk(e)
	if first.State != protocol.LocalBroken || first.Error == "" {
		t.Fatalf("first broken model state: %+v", first)
	}
	stamp := m.badAdopts[e.ID].stamp
	if stamp == "" || m.badAdopts[e.ID].err == nil {
		t.Fatal("broken verification not remembered")
	}
	m.adoptMu.Lock()
	busy := make(chan bool, 1)
	go func() { busy <- m.Busy() }()
	select {
	case <-busy:
	case <-time.After(time.Second):
		t.Fatal("adoption held the manager lock")
	}
	m.adoptMu.Unlock()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o600)
	again := m.modelOnDisk(e)
	if again.State != protocol.LocalBroken || again.Error != first.Error || m.badAdopts[e.ID].stamp != stamp {
		t.Fatalf("unchanged broken model reverified: %+v", again)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, correct, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	if fixed := m.modelOnDisk(e); fixed.State != protocol.LocalInstalled {
		t.Fatalf("changed model not reverified: %+v", fixed)
	}
}

func TestImportLocalGGUFOwnedCopyAndMetadata(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "my-model.gguf")
	data := syntheticGGUF(16384, "{% if tools %}{{ tool_call.function }}{% endif %}", 2_000_000_000, false)
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	model, err := m.addLocal(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if model.State != protocol.LocalInstalled || model.Params != 2_000_000_000 || model.Context != 16384 || !model.Tools {
		t.Fatalf("imported metadata / state = %+v", model)
	}
	e, ok := m.entry(model.ID)
	if !ok || len(e.Files) != 1 || e.Files[0].Size != int64(len(data)) {
		t.Fatalf("entry missing after import: %+v, %v", e, ok)
	}
	sum := sha256.Sum256(data)
	if e.Files[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("import hash = %s", e.Files[0].SHA256)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filePath(e, e.Files[0].Name)); err != nil || info.Size() != int64(len(data)) {
		t.Fatalf("import still depends on the source: %v, %v", info, err)
	}
	if again, err := m.addLocal(context.Background(), filePath(e, e.Files[0].Name)); err != nil || again.ID != model.ID {
		t.Fatalf("re-import not idempotent: %+v, %v", again, err)
	}
	if got := len(readCustom()); got != 1 {
		t.Fatalf("duplicate local entries: %d", got)
	}
}

func TestLocalImportRejectsInvalidGGUF(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "invalid.gguf")
	if err := os.WriteFile(source, []byte("not a GGUF"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	if _, err := m.addLocal(context.Background(), source); err == nil {
		t.Fatal("non-GGUF imported")
	}
	if entries := readCustom(); len(entries) != 0 {
		t.Fatalf("invalid file recorded: %+v", entries)
	}
}

func TestCustomStoreAcceptsOnlyCompleteSplitSets(t *testing.T) {
	e := Entry{ID: "hf-split", Repo: "owner/model", Revision: "abcdef123456789012345678901234567890abcd"}
	for _, name := range []string{"model-Q4-00001-of-00002.gguf", "model-Q4-00002-of-00002.gguf"} {
		e.Files = append(e.Files, File{Name: name, Role: protocol.LocalFileModel, Size: 42, SHA256: strings.Repeat("a", 64)})
	}
	if !validEntryFiles(e) {
		t.Fatal("complete split set lost on reload")
	}
	e.Files = e.Files[:1]
	if validEntryFiles(e) {
		t.Fatal("single missing split part accepted")
	}
}

func TestLocalImportCopiesCompleteSplitSet(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := t.TempDir()
	part1 := filepath.Join(dir, "model-Q4-00001-of-00002.gguf")
	part2 := filepath.Join(dir, "model-Q4-00002-of-00002.gguf")
	for _, name := range []string{part1, part2} {
		if err := os.WriteFile(name, syntheticGGUF(32768, "tools tool_call", 2_000_000_000, false), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	model, err := m.addLocal(context.Background(), part2)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := m.entry(model.ID)
	if !ok || model.State != protocol.LocalInstalled || len(e.Files) != 2 || e.Files[0].Name != filepath.Base(part1) {
		t.Fatalf("split import = %+v, entry=%+v, found=%v", model, e, ok)
	}
	for _, f := range e.Files {
		if _, err := os.Stat(filePath(e, f.Name)); err != nil {
			t.Errorf("missing imported part %s: %v", f.Name, err)
		}
	}
	man, found := readManifest(e)
	if !found || man.Header == nil || man.Header.ModelBytes != 2*e.Files[0].Size {
		t.Fatalf("split metadata missing: %+v, found=%v", man, found)
	}
	if err := os.Remove(filePath(e, e.Files[1].Name)); err != nil {
		t.Fatal(err)
	}
	repaired, err := m.addLocal(context.Background(), part2)
	if err != nil || repaired.State != protocol.LocalInstalled || repaired.ID != model.ID {
		t.Fatalf("reimport did not repair missing split part: %+v, %v", repaired, err)
	}
}

func TestLocalImportMatchesAdjacentProjector(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model-Q4_K_M.gguf")
	projectorPath := filepath.Join(dir, "mmproj-model-Q4_K_M.gguf")
	for _, name := range []string{modelPath, projectorPath} {
		if err := os.WriteFile(name, syntheticGGUF(32768, "tools tool_call", 2_000_000_000, false), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	model, err := m.addLocal(context.Background(), modelPath)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := m.entry(model.ID)
	if !ok || len(e.Files) != 2 || e.Files[1].Name != filepath.Base(projectorPath) ||
		e.Files[1].Role != protocol.LocalFileMmproj || !model.Vision {
		t.Fatalf("projector not paired with model: %+v, %+v", model, e)
	}
	man, found := readManifest(e)
	if !found || man.Header == nil || man.Header.MmprojBytes != e.Files[1].Size {
		t.Fatalf("projector size not recorded: %+v, found=%v", man, found)
	}
}
