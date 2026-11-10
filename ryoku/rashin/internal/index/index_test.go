package index

import (
	"context"
	"errors"
	"go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/index/extract"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestGitignoreNestedPrecedence(t *testing.T) {
	ignores := map[string][]string{"": {"*.log"}, "nested": {"!keep.log"}}
	if !ignoredBy(ignores, nil, "root.log", false) {
		t.Fatal("root pattern did not ignore root.log")
	}
	if ignoredBy(ignores, nil, "nested/keep.log", false) {
		t.Fatal("deeper negation did not reinclude nested/keep.log")
	}
	ignores["nested"] = []string{"keep.log"}
	if !ignoredBy(ignores, nil, "nested/keep.log", false) {
		t.Fatal("deeper ignore did not override root negation")
	}
	if ignored, _ := matchAny([]string{"/root.log"}, "nested/root.log", false); ignored {
		t.Fatal("anchored root pattern matched a nested path")
	}
	if ignored, _ := matchAny([]string{"/root.log"}, "root.log", false); !ignored {
		t.Fatal("anchored root pattern did not match the root path")
	}
}
func TestTokenFrequenciesKeepBoundedInformativeTerms(t *testing.T) {
	text := strings.Repeat("important ", 5)
	for index := range maxTermsPerChunk + 10 {
		text += "identifier" + strconv.Itoa(index) + " "
	}
	terms := tokenFrequencies(text)
	if len(terms) != maxTermsPerChunk {
		t.Fatalf("terms = %d", len(terms))
	}
	if terms["important"] != 5 {
		t.Fatalf("repeated term was not retained: %#v", terms)
	}
}
func TestBM25RanksObviousChunkFirst(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("bm25")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.replaceFile(fileRecord{Path: "refresh.go", Language: "go"}, extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: "refresh index source refresh"}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.replaceFile(fileRecord{Path: "music.go", Language: "go"}, extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: "play music volume"}}}); err != nil {
		t.Fatal(err)
	}
	ranked, err := store.bm25("refresh source index", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked) == 0 || ranked[0].chunk.Path != "refresh.go" {
		t.Fatalf("ranking = %#v", ranked)
	}
}
func TestSystemProfileIgnoresBrowserAndCredentials(t *testing.T) {
	root := t.TempDir()
	ignored := []string{"google-chrome-beta/Default/a.js", "chromium-dev/Default/a.js", "microsoft-edge-beta/Default/a.js", "discordcanary/settings.json", "opera-gx/config.json", "vivaldi/Default/a.js", "legcord/a.js", "armcord/a.js", "webcord/a.js", "VSCodium/User/settings.json", "code - oss/User/settings.json", "cursor/User/settings.json", "gcloud/legacy_credentials/me/adc.json", "gcloud/application_default_credentials.json", "github-copilot/hosts.json", "hub/config.json", "op/config.json", "bitwarden cli/config.json", "other/access-token.json", "other/client_secret.toml", "safe/Extensions/abc/bg.js"}
	for _, name := range ignored {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), "Item {}\n")
	}
	writeTestFile(t, filepath.Join(root, "ryoku", "theme.qml"), "Item {}\n")
	files, err := walkRoot(context.Background(), root, "config", walkPolicy{System: true, MaxSize: 512 << 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "config/ryoku/theme.qml" {
		t.Fatalf("walked files = %#v", files)
	}
}
func TestSystemProfileSkipsBackupNames(t *testing.T) {
	root := t.TempDir()
	ignored := []string{
		"hypr.bak-20260927224924/config.qml", "backup/config.qml", "backups/config.qml",
		"settings.bak", "settings.bak-2", "settings.backup-copy", "settings.old", "settings~", "settings.orig", "settings.rej",
	}
	for _, name := range ignored {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), "Item {}\n")
	}
	writeTestFile(t, filepath.Join(root, "backup.qml"), "Item {}\n")
	files, err := walkRoot(context.Background(), root, "config", walkPolicy{System: true, MaxSize: 512 << 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "config/backup.qml" {
		t.Fatalf("walked files = %#v", files)
	}
}

func TestSystemProfileDeduplicatesIdenticalShippedConfig(t *testing.T) {
	root := t.TempDir()
	roots := []string{
		filepath.Join(root, "vault"),
		filepath.Join(root, "config"),
		filepath.Join(root, "share"),
		filepath.Join(root, "etc"),
	}
	for _, directory := range roots {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(roots[1], "quickshell", "Same.qml"), "Item { property int value: 1 }\n")
	writeTestFile(t, filepath.Join(roots[2], "config", "quickshell", "Same.qml"), "Item { property int value: 1 }\n")
	writeTestFile(t, filepath.Join(roots[1], "quickshell", "Changed.qml"), "Item { property int value: 2 }\n")
	writeTestFile(t, filepath.Join(roots[2], "config", "quickshell", "Changed.qml"), "Item { property int value: 3 }\n")
	writeTestFile(t, filepath.Join(roots[2], "config", "quickshell", "Shipped.qml"), "Item {}\n")
	engine := &Engine{}
	files, err := engine.walkEntry(context.Background(), registryEntry{Kind: "system", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, file := range files {
		found = append(found, file.Path)
	}
	want := "config/quickshell/Changed.qml,config/quickshell/Same.qml,share/config/quickshell/Changed.qml,share/config/quickshell/Shipped.qml"
	if strings.Join(found, ",") != want {
		t.Fatalf("walked files = %v", found)
	}
}
func TestProjectProfileSkipsVendoredGeneratedAndLargeData(t *testing.T) {
	root := t.TempDir()
	ignored := []string{
		"vendor/example.go", "node_modules/example.js", "third_party/example.go", "dist/example.js", "build/example.go",
		"target/example.rs", ".venv/example.py", "__pycache__/example.py", "web/app.min.js", "web/app.js.map",
		"deps/example.lock", "package-lock.json", "go.sum", "Cargo.lock",
	}
	for _, name := range ignored {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), "package example\n")
	}
	writeTestFile(t, filepath.Join(root, "catalog.json"), `{"data":"`+strings.Repeat("x", maxProjectDataBytes)+`"}`)
	writeTestFile(t, filepath.Join(root, "safe.json"), `{"safe":true}`)
	writeTestFile(t, filepath.Join(root, "large.go"), "package safe\n// "+strings.Repeat("x", maxProjectDataBytes))
	files, err := walkRoot(context.Background(), root, "", walkPolicy{MaxSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	if strings.Join(paths, ",") != "large.go,safe.json" {
		t.Fatalf("walked files = %v", paths)
	}
}
func TestIncrementalRefreshKeepsUnchangedVectorsAndDeletesFiles(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	one := filepath.Join(root, "one.go")
	two := filepath.Join(root, "two.go")
	writeTestFile(t, one, "package sample\nfunc One() {}\n")
	writeTestFile(t, two, "package sample\nfunc Two() {}\n")
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := engine.storeFor(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := store.chunks()
	if err != nil || len(chunks) < 2 {
		t.Fatalf("chunks = %d, err = %v", len(chunks), err)
	}
	var oneChunk chunkRecord
	for _, chunk := range chunks {
		if chunk.Path == "one.go" {
			oneChunk = chunk
			break
		}
	}
	if oneChunk.ID == "" {
		t.Fatal("one.go chunk missing")
	}
	if err := store.putVector(oneChunk.ID, make([]float32, 256)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, two, "package sample\nfunc TwoChanged() {}\n")
	if _, err := engine.Refresh(context.Background(), protocol.IndexRef{ID: info.ID}); err != nil {
		t.Fatal(err)
	}
	_, _, _, vectors, countErr := store.counts()
	if countErr != nil {
		t.Fatal(countErr)
	}
	if vectors != 1 {
		t.Fatalf("unchanged vector count = %d, want 1", vectors)
	}
	if err := os.Remove(one); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Refresh(context.Background(), protocol.IndexRef{ID: info.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, _, vectors, countErr := store.counts()
	if countErr != nil {
		t.Fatal(countErr)
	}
	if files != 1 || vectors != 0 {
		t.Fatalf("after delete: files=%d vectors=%d", files, vectors)
	}
}
func TestImpactIncludesTransitiveDependents(t *testing.T) {
	entry := registryEntry{Roots: []string{t.TempDir()}}
	files := []fileRecord{{Path: "a.ts", Language: "typescript"}, {Path: "b.ts", Language: "typescript"}, {Path: "c.ts", Language: "typescript"}}
	imports := []importRecord{{Source: "a.ts", Target: "./b"}, {Source: "b.ts", Target: "./c"}}
	graph := buildImportGraph(entry, files, imports)
	if !graph.reverse["c.ts"]["b.ts"] {
		t.Fatal("direct dependent missing")
	}
	if count := transitiveCount(graph, "c.ts"); count != 2 {
		t.Fatalf("transitive count = %d, want 2", count)
	}
}
func setTestXDG(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "data"))
}
func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGitignoreSlashlessPatternMatchesLastSegmentOnly(t *testing.T) {
	ignored, _ := matchAny([]string{"secret*.json", "!keep"}, "keep/secret1.json", false)
	if !ignored {
		t.Fatal("parent negation re-included an ignored file")
	}
}

func TestProjectFileCapAndSnippetCap(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "small.go"), "package sample\n")
	if err := os.WriteFile(filepath.Join(root, "large.go"), []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := Open(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	files, err := engine.walkEntry(context.Background(), registryEntry{Kind: "project", Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "small.go" {
		t.Fatalf("files = %#v", files)
	}
	hits := searchHits([]rankedChunk{{chunk: chunkRecord{Path: "long.json", Start: 1, End: 1, Text: strings.Repeat("a", 1000)}}}, "a")
	if len(hits) != 1 || len(hits[0].Snippet) > 400 {
		t.Fatalf("snippet bytes = %d", len(hits[0].Snippet))
	}
}

func TestFindDefAndReferencesRedactOriginalSource(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "credentials.py"), "API_KEY = \"QAASK_PY_SECRET_77e18b\"\n\ndef reveal():\n    password = \"SECOND_SECRET_42\"\n    # -----BEGIN PRIVATE KEY-----\n    # QAASK_PEM_BODY_9931\n    # -----END PRIVATE KEY-----\n    return API_KEY\n\ndef consume():\n    return reveal()\n") // rashin-reviewer:allow private-key
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	ref := protocol.IndexRef{ID: info.ID}
	search, err := engine.Search(context.Background(), protocol.SearchParams{IndexRef: ref, Query: "API_KEY", Mode: protocol.SearchLexical})
	if err != nil {
		t.Fatal(err)
	}
	peek, err := engine.Peek(context.Background(), protocol.PeekParams{IndexRef: ref, Path: "credentials.py", Start: 1, End: 11})
	if err != nil {
		t.Fatal(err)
	}
	def, err := engine.Def(context.Background(), protocol.NameParams{IndexRef: ref, Name: "reveal"})
	if err != nil {
		t.Fatal(err)
	}
	references, err := engine.References(context.Background(), protocol.NameParams{IndexRef: ref, Name: "API_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	var outputs []string
	for _, hit := range search.Hits {
		outputs = append(outputs, hit.Snippet)
	}
	outputs = append(outputs, peek.Text, def.Body)
	for _, reference := range references.References {
		outputs = append(outputs, reference.Text)
	}
	joined := strings.Join(outputs, "\n")
	for _, secret := range []string{"QAASK_PY_SECRET_77e18b", "SECOND_SECRET_42", "QAASK_PEM_BODY_9931", "BEGIN PRIVATE KEY", "END PRIVATE KEY"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("query output leaked %q: %s", secret, joined)
		}
	}
	if strings.Count(peek.Text, "\n") != 10 {
		t.Fatalf("peek line count changed: %q", peek.Text)
	}
}

func TestFindAndDefBareMethodName(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sample.go"), "package sample\ntype Alpha struct{}\nfunc (Alpha) Refresh() int { return 1 }\n")
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	params := protocol.NameParams{IndexRef: protocol.IndexRef{ID: info.ID}, Name: "Refresh"}
	found, err := engine.Find(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Symbols) == 0 || found.Symbols[0].Name != "Alpha.Refresh" {
		t.Fatalf("symbols = %#v", found.Symbols)
	}
	def, err := engine.Def(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(def.Body, "Refresh") {
		t.Fatalf("body = %q", def.Body)
	}
}

func TestNestedGoModulesUseLongestBoundaryPrefix(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mod", "go.mod")
	nested := filepath.Join(root, "nested", "go.mod")
	writeTestFile(t, mod, "module example.com/root\n")
	writeTestFile(t, nested, "module example.com/root/nested\n")
	files := []fileRecord{{Path: "mod/go.mod", Source: mod, Language: "gomod"}, {Path: "nested/go.mod", Source: nested, Language: "gomod"}, {Path: "nested/pkg/pkg.go", Language: "go"}, {Path: "nested/cmd/main.go", Language: "go"}, {Path: "mod/pkg/pkg.go", Language: "go"}}
	imports := []importRecord{{Source: "nested/cmd/main.go", Target: "example.com/root/nested/pkg"}, {Source: "mod/pkg/pkg.go", Target: "example.com/rooted/not-ours"}}
	graph := buildImportGraph(registryEntry{}, files, imports)
	if !graph.reverse["nested/pkg/pkg.go"]["nested/cmd/main.go"] {
		t.Fatalf("graph = %#v", graph.reverse)
	}
	if len(graph.forward["mod/pkg/pkg.go"]) != 0 {
		t.Fatal("module prefix matched without a slash boundary")
	}
}

func TestQMLDirectoryAndSameDirectoryDependencies(t *testing.T) {
	files := []fileRecord{{Path: "a/Main.qml", Language: "qml"}, {Path: "a/Peer.qml", Language: "qml"}, {Path: "b/Widget.qml", Language: "qml"}}
	imports := []importRecord{{Source: "a/Main.qml", Target: "../b"}}
	graph := buildImportGraph(registryEntry{}, files, imports)
	if !graph.reverse["b/Widget.qml"]["a/Main.qml"] {
		t.Fatal("QML directory import did not resolve")
	}
	if !graph.reverse["a/Peer.qml"]["a/Main.qml"] {
		t.Fatal("same-directory QML type did not resolve")
	}
}

func TestDamagedStoreIsReportedAndRebuilt(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sample.go"), "package sample\n")
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	store := engine.stores[info.ID]
	delete(engine.stores, info.ID)
	engine.mu.Unlock()
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(paths.IndexDir(info.ID), "index.db"), make([]byte, 8192), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := engine.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.ID == info.ID {
			if item.Error == "" {
				t.Fatalf("damaged index = %#v", item)
			}
			return
		}
	}
	t.Fatal("damaged index missing from list")
}

func TestSkippableReadErrors(t *testing.T) {
	if !skippableReadError(os.ErrNotExist) || !skippableReadError(os.ErrPermission) || skippableReadError(errors.New("disk failed")) {
		t.Fatal("read error classification is wrong")
	}
}

func TestOpenStoreResetsInterruptedStateAndCancelIsIdle(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("state")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.setMeta(map[string]string{"state": "refreshing"}); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = openStore("state")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	state, err := store.meta("state")
	if err != nil {
		t.Fatal(err)
	}
	if state != "idle" {
		t.Fatalf("state = %q", state)
	}
	engine := &Engine{}
	entry := registryEntry{ID: "state"}
	if _, err = engine.refreshError(store, entry, context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	state, err = store.meta("state")
	if err != nil {
		t.Fatal(err)
	}
	if state != "idle" {
		t.Fatalf("cancelled state = %q", state)
	}
}

func TestImpactCyclesExcludeTarget(t *testing.T) {
	files := []fileRecord{{Path: "a.py", Language: "python"}, {Path: "b.py", Language: "python"}, {Path: "c.py", Language: "python"}}
	imports := []importRecord{{Source: "a.py", Target: "b"}, {Source: "b.py", Target: "a"}, {Source: "c.py", Target: "a"}}
	graph := buildImportGraph(registryEntry{}, files, imports)
	if count := transitiveCount(graph, "a.py"); count != 2 {
		t.Fatalf("count = %d", count)
	}
}

func TestPeekShortWindowIsNotTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.txt")
	writeTestFile(t, path, "one\n")
	_, _, _, truncated, err := readLines(path, 1, 200, 200, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("short file reported truncated")
	}
}

func TestReplaceFilesBatch(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("batch")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	replacements := make([]fileReplacement, 70)
	for index := range replacements {
		path := string(rune('a'+index%26)) + string(rune('A'+index/26)) + ".go"
		replacements[index] = fileReplacement{file: fileRecord{Path: path, Language: "go"}, result: extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: path}}}}
	}
	if err = store.replaceFiles(replacements); err != nil {
		t.Fatal(err)
	}
	files, _, _, _, err := store.counts()
	if err != nil {
		t.Fatal(err)
	}
	if files != 70 {
		t.Fatalf("files = %d", files)
	}
}

func TestPutVectorsBatch(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("vector-batch")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	replacements := []fileReplacement{{file: fileRecord{Path: "a.go", Language: "go"}, result: extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: "alpha"}}}}, {file: fileRecord{Path: "b.go", Language: "go"}, result: extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: "beta"}}}}}
	if err = store.replaceFiles(replacements); err != nil {
		t.Fatal(err)
	}
	chunks, err := store.chunks()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(chunks))
	vectors := make([][]float32, len(chunks))
	for index, chunk := range chunks {
		ids[index] = chunk.ID
		vectors[index] = make([]float32, 256)
	}
	if err = store.putVectors(ids, vectors); err != nil {
		t.Fatal(err)
	}
	_, _, _, count, err := store.counts()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("vectors = %d", count)
	}
}

func TestStoreOperationPanicBecomesRecoverableError(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("panic")
	if err != nil {
		t.Fatal(err)
	}
	err = store.view(func(_ *bbolt.Tx) error { panic("damaged page") })
	if err == nil || !store.isDamaged() {
		t.Fatalf("panic result: damaged=%v err=%v", store.isDamaged(), err)
	}
	if _, statErr := os.Stat(paths.IndexDir("panic")); !os.IsNotExist(statErr) {
		t.Fatalf("damaged directory still exists: %v", statErr)
	}
}

func TestChangedFileKeepsUnchangedChunkVector(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	lines := make([]string, 80)
	for index := range lines {
		lines[index] = "stable line"
	}
	file := filepath.Join(root, "sample.md")
	writeTestFile(t, file, strings.Join(lines, "\n"))
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := engine.storeFor(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := store.chunks()
	if err != nil {
		t.Fatal(err)
	}
	var first chunkRecord
	for _, chunk := range chunks {
		if chunk.Start == 1 {
			first = chunk
			break
		}
	}
	if first.ID == "" {
		t.Fatal("first chunk missing")
	}
	if err = store.putVector(first.ID, make([]float32, 256)); err != nil {
		t.Fatal(err)
	}
	lines[79] = "changed line"
	writeTestFile(t, file, strings.Join(lines, "\n"))
	if _, err = engine.Refresh(context.Background(), protocol.IndexRef{ID: info.ID}); err != nil {
		t.Fatal(err)
	}
	_, _, _, vectors, err := store.counts()
	if err != nil {
		t.Fatal(err)
	}
	if vectors != 1 {
		t.Fatalf("vectors = %d", vectors)
	}
}

func TestProjectRootsRejectRelativeHomeAndAncestors(t *testing.T) {
	setTestXDG(t)
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	tests := []string{".", string(filepath.Separator), home, filepath.Dir(home)}
	for _, root := range tests {
		t.Run(root, func(t *testing.T) {
			if _, addErr := engine.Add(context.Background(), protocol.IndexAddParams{Path: root}); addErr == nil {
				t.Fatalf("unsafe root %q was accepted", root)
			}
		})
	}
}

func TestListStatusAndOtherIndexDoNotWaitForRefresh(t *testing.T) {
	setTestXDG(t)
	settings := func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} }
	engine, err := Open(nil, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstFile := filepath.Join(firstRoot, "first.go")
	writeTestFile(t, firstFile, "package first\nfunc First() string { return \"old\" }\n")
	writeTestFile(t, filepath.Join(secondRoot, "second.go"), "package second\nfunc Second() string { return \"ready\" }\n")
	first, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: firstRoot})
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: secondRoot})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, firstFile, "package first\nfunc First() string { return \"changed\" }\n")
	started := make(chan struct{})
	var once sync.Once
	engine.readSource = func(ctx context.Context, path string) ([]byte, error) {
		if path == firstFile {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return readIndexSource(ctx, path)
	}
	refreshDone := make(chan error, 1)
	go func() {
		_, refreshErr := engine.Refresh(context.Background(), protocol.IndexRef{ID: first.ID})
		refreshDone <- refreshErr
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh did not reach the slow read")
	}
	assertQuick := func(name string, call func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- call() }()
		select {
		case callErr := <-done:
			if callErr != nil {
				t.Fatalf("%s: %v", name, callErr)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("%s waited for another index refresh", name)
		}
	}
	assertQuick("list", func() error { _, callErr := engine.List(context.Background()); return callErr })
	assertQuick("status", func() error {
		_, callErr := engine.Status(context.Background(), protocol.IndexRef{ID: first.ID})
		return callErr
	})
	assertQuick("other search", func() error {
		result, callErr := engine.Search(context.Background(), protocol.SearchParams{IndexRef: protocol.IndexRef{ID: second.ID}, Query: "ready", Mode: protocol.SearchLexical})
		if callErr == nil && len(result.Hits) == 0 {
			return errors.New("other index returned no lexical hit")
		}
		return callErr
	})
	assertQuick("remove", func() error { return engine.Remove(context.Background(), protocol.IndexRef{ID: first.ID}) })
	select {
	case refreshErr := <-refreshDone:
		if !errors.Is(refreshErr, context.Canceled) {
			t.Fatalf("refresh error = %v", refreshErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled refresh did not stop")
	}
}

func TestLexicalSearchAndListStayAvailableDuringEmbedding(t *testing.T) {
	setTestXDG(t)
	settings := func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: true}} }
	engine, err := Open(nil, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	engine.embedTexts = func(ctx context.Context, texts []string) ([][]float32, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		vectors := make([][]float32, len(texts))
		for index := range vectors {
			vectors[index] = make([]float32, 256)
		}
		return vectors, nil
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sample.go"), "package sample\nfunc LexicalNeedle() string { return \"needle\" }\n")
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("embedding did not start")
	}
	searchDone := make(chan error, 1)
	go func() {
		result, searchErr := engine.Search(context.Background(), protocol.SearchParams{IndexRef: protocol.IndexRef{ID: info.ID}, Query: "LexicalNeedle"})
		if searchErr == nil && len(result.Hits) == 0 {
			searchErr = errors.New("lexical search returned no hit")
		}
		searchDone <- searchErr
	}()
	select {
	case searchErr := <-searchDone:
		if searchErr != nil {
			t.Fatal(searchErr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("search waited for embedding")
	}
	listDone := make(chan error, 1)
	go func() { _, listErr := engine.List(context.Background()); listDone <- listErr }()
	select {
	case listErr := <-listDone:
		if listErr != nil {
			t.Fatal(listErr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("list waited for embedding")
	}
	close(release)
	jobsDone := make(chan struct{})
	go func() { engine.jobs.Wait(); close(jobsDone) }()
	select {
	case <-jobsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("embedding did not finish")
	}
	status, err := engine.Status(context.Background(), protocol.IndexRef{ID: info.ID})
	if err != nil {
		t.Fatal(err)
	}
	if status.Chunks == 0 || status.Vectors != status.Chunks {
		t.Fatalf("status counts = chunks %d vectors %d", status.Chunks, status.Vectors)
	}
}

func TestExistingSearchReturnsCurrentDataAndRefreshesInBackground(t *testing.T) {
	setTestXDG(t)
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	root := t.TempDir()
	file := filepath.Join(root, "sample.go")
	writeTestFile(t, file, "package sample\nconst CurrentNeedle = \"old\"\n")
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, file, "package sample\nconst CurrentNeedle = \"new\"\n")
	started := make(chan struct{})
	engine.readSource = func(ctx context.Context, path string) ([]byte, error) {
		if path == file {
			select {
			case <-started:
			default:
				close(started)
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return readIndexSource(ctx, path)
	}
	result, err := engine.Search(context.Background(), protocol.SearchParams{IndexRef: protocol.IndexRef{ID: info.ID}, Query: "CurrentNeedle", Mode: protocol.SearchLexical, Existing: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) == 0 || !strings.Contains(result.Hits[0].Snippet, "old") {
		t.Fatalf("existing result = %#v", result.Hits)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh was not started")
	}
	operation := engine.operationFor(info.ID)
	operation.cancelActive()
	operation.mu.Lock()
	operation.mu.Unlock()
	if _, err = engine.Search(context.Background(), protocol.SearchParams{IndexRef: protocol.IndexRef{ID: "missing"}, Query: "x", Existing: true}); err == nil {
		t.Fatal("Existing search registered a missing index")
	}
}

func TestInfoReadsStoredCounts(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("stored-counts")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.setMeta(map[string]string{"count_files": "11", "count_chunks": "22", "count_symbols": "33", "count_vectors": "44"}); err != nil {
		t.Fatal(err)
	}
	info, err := store.info(registryEntry{ID: "stored-counts"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Files != 11 || info.Chunks != 22 || info.Symbols != 33 || info.Vectors != 44 {
		t.Fatalf("stored counts = %#v", info)
	}
}

func TestBM25LoadsOnlyRankedChunkRecords(t *testing.T) {
	setTestXDG(t)
	store, err := openStore("selective")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.replaceFile(fileRecord{Path: "good.go", Language: "go"}, extract.Result{Chunks: []extract.Chunk{{Start: 1, End: 1, Text: "needle"}}}); err != nil {
		t.Fatal(err)
	}
	if err = store.update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketChunks).Put([]byte("unrelated"), []byte("not-json")) }); err != nil {
		t.Fatal(err)
	}
	if err = store.setMeta(map[string]string{"bm25_documents": "2", "bm25_total_terms": "2"}); err != nil {
		t.Fatal(err)
	}
	ranked, err := store.bm25("needle", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked) != 1 || ranked[0].chunk.Path != "good.go" {
		t.Fatalf("ranked = %#v", ranked)
	}
}

func TestRemovedIndexCannotBeRecreatedByStoreLookup(t *testing.T) {
	setTestXDG(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sample.go"), "package sample\n")
	engine, err := Open(nil, func() protocol.Settings { return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}} })
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Remove(context.Background(), protocol.IndexRef{ID: info.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.storeFor(info.ID); err == nil {
		t.Fatal("removed index store was recreated")
	}
	if _, statErr := os.Stat(paths.IndexDir(info.ID)); !os.IsNotExist(statErr) {
		t.Fatalf("removed index directory exists: %v", statErr)
	}
}
