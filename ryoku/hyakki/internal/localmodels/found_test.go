package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func foundTestFile(t *testing.T, root string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, syntheticGGUF(16384, "tools tool_call", 2_000_000_000, false), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func foundAt(models []protocol.LocalFoundModel, path string) (protocol.LocalFoundModel, bool) {
	for _, model := range models {
		if model.Path == path {
			return model, true
		}
	}
	return protocol.LocalFoundModel{}, false
}

func TestFoundLayoutsAndHFLinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	layouts := []struct{ dir, source string }{
		{".cache/lm-studio/models/author/model", protocol.LocalFoundLMStudio},
		{".cache/llama.cpp/models", protocol.LocalFoundLlamaCpp},
		{".local/share/Jan/data/models/model", protocol.LocalFoundFolder},
		{".cache/gpt4all", protocol.LocalFoundFolder},
		{"koboldcpp/models", protocol.LocalFoundFolder},
		{"text-generation-webui/user_data/models", protocol.LocalFoundFolder},
		{"Models", protocol.LocalFoundFolder},
		{"AI", protocol.LocalFoundFolder},
		{"llm-test", protocol.LocalFoundFolder},
		{"Downloads", protocol.LocalFoundFolder},
	}
	for i, item := range layouts {
		foundTestFile(t, filepath.Join(home, item.dir, "model-"+string(rune('a'+i))+".gguf"))
	}
	custom := filepath.Join(t.TempDir(), "my-model.gguf")
	foundTestFile(t, custom)
	blob := foundTestFile(t, filepath.Join(home, ".cache", "huggingface", "hub", "models--someone--demo", "blobs", "weight"))
	snapshot := filepath.Join(home, ".cache", "huggingface", "hub", "models--someone--demo", "snapshots", "revision", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(snapshot), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(blob, snapshot); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	found, err := m.Found(context.Background(), []string{filepath.Dir(custom)})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) < len(layouts)+2 {
		t.Fatalf("found %d models, want at least %d: %+v", len(found), len(layouts)+2, found)
	}
	for _, item := range layouts {
		count := 0
		for _, model := range found {
			if model.Source == item.source && strings.Contains(model.Path, item.dir) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("missing layout %s (%s): %+v", item.dir, item.source, found)
		}
	}
	var linked bool
	for _, model := range found {
		if model.Path == snapshot {
			linked = true
			if model.Params == 0 || model.Context != 16384 || !model.Tools {
				t.Errorf("missing GGUF metadata: %+v", model)
			}
		}
	}
	if !linked {
		t.Fatal("Hugging Face snapshot symlink not found")
	}
}

func TestFoundOllamaManifestBlobAndBoundedWalk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	models := filepath.Join(home, ".ollama", "models")
	digest := strings.Repeat("a", 64)
	blob := foundTestFile(t, filepath.Join(models, "blobs", "sha256-"+digest))
	manifest := filepath.Join(models, "manifests", "registry.ollama.ai", "library", "model", "latest")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"layers": []map[string]string{{"mediaType": "application/vnd.ollama.image.model", "digest": "sha256:" + digest}}})
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join(home, "Models", "loop")
	if err := os.MkdirAll(filepath.Dir(loop), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(loop), loop); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	found, err := m.Found(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ollama, ok := foundAt(found, blob)
	if !ok || ollama.Source != protocol.LocalFoundOllama || ollama.Name != "model:latest" {
		t.Fatalf("ollama blob / symlink loop: %+v", found)
	}
	linked, err := m.UseFound(blob)
	if err != nil || linked.Name != "model:latest" {
		t.Fatalf("Ollama label lost on use: %+v, %v", linked, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Found(ctx, nil); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestFoundSplitProjectorInPlaceForgetNeverDeletesOriginal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := filepath.Join(home, "Models")
	first := foundTestFile(t, filepath.Join(dir, "demo-Q4_K_M-00001-of-00002.gguf"))
	second := foundTestFile(t, filepath.Join(dir, "demo-Q4_K_M-00002-of-00002.gguf"))
	projector := foundTestFile(t, filepath.Join(dir, "mmproj-demo-Q4_K_M.gguf"))
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	found, err := m.Found(context.Background(), nil)
	discovered, ok := foundAt(found, first)
	if err != nil || !ok || !discovered.Vision {
		t.Fatalf("split scan: %+v, %v", found, err)
	}
	model, err := m.UseFound(second)
	if err != nil || model.State != protocol.LocalInstalled || model.Path != first {
		t.Fatalf("in-place use: %+v, %v", model, err)
	}
	e, ok := m.entry(model.ID)
	if !ok || len(e.Files) != 3 || e.Files[0].SHA256 != "" || e.External != dir {
		t.Fatalf("external entry: %+v", e)
	}
	found, err = m.Found(context.Background(), nil)
	discovered, ok = foundAt(found, first)
	if err != nil || !ok || discovered.Model != model.ID {
		t.Fatalf("in-use scan: %+v, %v", found, err)
	}
	if err := m.Download(context.Background(), model.ID); err == nil {
		t.Fatal("linked source scheduled for download")
	}
	if _, err := m.Remove(context.Background(), model.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second, projector} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("forget deleted %s: %v", path, err)
		}
	}
	if _, err := os.Stat(manifestPath(e)); !os.IsNotExist(err) {
		t.Fatalf("manifest remains: %v", err)
	}
}

func TestFoundChangedSourceBreaksLinkAndIncompleteSplitSkipped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root := foundTestFile(t, filepath.Join(home, "Downloads", "model.gguf"))
	foundTestFile(t, filepath.Join(home, "Downloads", "incomplete-00001-of-00002.gguf"))
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	found, err := m.Found(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := foundAt(found, root); !ok {
		t.Fatalf("regular model missing: %+v", found)
	}
	if _, ok := foundAt(found, filepath.Join(home, "Downloads", "incomplete-00001-of-00002.gguf")); ok {
		t.Fatal("incomplete split offered")
	}
	model, err := m.UseFound(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, append(before, 'x'), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _ := m.entry(model.ID)
	if changed, _ := onDisk(e); changed.State != protocol.LocalBroken {
		t.Fatalf("changed link did not break: %+v", changed)
	}
	if _, err := m.UseFound(root); err != nil {
		t.Fatalf("relink changed source: %v", err)
	}
	if current, _ := onDisk(e); current.State == protocol.LocalInstalled {
		t.Fatal("stale entry should not become valid without reload")
	}
	fresh, _ := m.entry(model.ID)
	if current, _ := onDisk(fresh); current.State != protocol.LocalInstalled {
		t.Fatalf("relink did not refresh stamp: %+v", current)
	}
	// The ID identifies the path, not a weight hash; no full-file checksum is computed.
	sum := sha256.Sum256([]byte(root))
	if !strings.Contains(model.ID, hex.EncodeToString(sum[:12])) {
		t.Fatalf("path identity: %s", model.ID)
	}
}

func TestFoundWalkBudgetAndNoDirectorySymlinkFollow(t *testing.T) {
	root := t.TempDir()
	for i := range 30 {
		name := filepath.Join(root, strings.Repeat("a", i+1)+".gguf")
		foundTestFile(t, name)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	remaining := 10
	visited := 0
	walkFound(context.Background(), root, &remaining, false, func(_ string, _ os.DirEntry) { visited++ })
	if visited > 10 || remaining != 0 {
		t.Fatalf("traversal exceeded budget: %d visited, %d remaining", visited, remaining)
	}
	loops := t.TempDir()
	foundTestFile(t, filepath.Join(loops, "model.gguf"))
	if err := os.Symlink(loops, filepath.Join(loops, "loop")); err != nil {
		t.Fatal(err)
	}
	remaining, visited = 10, 0
	walkFound(context.Background(), loops, &remaining, false, func(_ string, _ os.DirEntry) { visited++ })
	if visited != 1 || remaining != 8 {
		t.Fatalf("followed a symlinked directory: %d visited, %d remaining", visited, remaining)
	}
}

func TestFoundEnvironmentCachesAndHiddenDirectoryRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	hfHome := filepath.Join(t.TempDir(), "hf-home")
	hfHub := filepath.Join(t.TempDir(), "hf-hub")
	llamaCache := filepath.Join(t.TempDir(), "llama-cache")
	t.Setenv("HF_HOME", hfHome)
	t.Setenv("HF_HUB_CACHE", hfHub)
	t.Setenv("LLAMA_CACHE", llamaCache)
	wanted := []struct{ path, source string }{
		{filepath.Join(home, ".cache", "llama.cpp", "cache.gguf"), protocol.LocalFoundLlamaCpp},
		{filepath.Join(llamaCache, "user.gguf"), protocol.LocalFoundLlamaCpp},
		{filepath.Join(hfHome, "hub", "home.gguf"), protocol.LocalFoundHuggingFace},
		{filepath.Join(hfHub, ".snapshot", "hub.gguf"), protocol.LocalFoundHuggingFace},
		{filepath.Join(home, "Models", "shown.gguf"), protocol.LocalFoundFolder},
	}
	for _, item := range wanted {
		foundTestFile(t, item.path)
	}
	hidden := foundTestFile(t, filepath.Join(home, "Models", ".private", "hidden.gguf"))
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	found, err := m.Found(context.Background(), nil)
	if err != nil || len(found) < len(wanted) {
		t.Fatalf("environment cache scan: %+v, %v", found, err)
	}
	for _, item := range wanted {
		matched := false
		for _, candidate := range found {
			if candidate.Path == item.path && candidate.Source == item.source {
				matched = true
			}
			if candidate.Path == hidden {
				t.Fatal("hidden user directory was scanned")
			}
		}
		if !matched {
			t.Errorf("missing %s (%s)", item.path, item.source)
		}
	}
	system := false
	for _, root := range foundRoots(home, nil) {
		if root.path == "/usr/share/ollama/.ollama/models/manifests" && root.ollama {
			system = true
		}
	}
	if !system {
		t.Fatal("system Ollama manifest root absent")
	}
}
