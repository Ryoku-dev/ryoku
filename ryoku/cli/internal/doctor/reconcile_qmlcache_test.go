package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func qmlCacheFixture(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	return filepath.Join(cache, "quickshell", "qmlcache")
}

func writeCacheUnit(t *testing.T, dir, name string, size int, atime, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, atime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestQMLCacheMissingDir(t *testing.T) {
	dir := qmlCacheFixture(t)
	for _, checkOnly := range []bool{true, false} {
		if got := reconcileQMLCache(checkOnly); got.status != recOK {
			t.Fatalf("checkOnly=%v: missing cache should be ok, got %v (%s)", checkOnly, got.status, got.detail)
		}
	}
	if exists(dir) {
		t.Error("the doctor created the QML cache directory")
	}
}

func TestQMLCachePrunesOnlyUnused(t *testing.T) {
	dir := qmlCacheFixture(t)
	now := time.Now()
	old := now.Add(-40 * 24 * time.Hour)
	stale := writeCacheUnit(t, dir, "stale.qmlc", 3<<20, old, old)
	staleJS := writeCacheUnit(t, dir, "stale.jsc", 1<<20, old, old)
	read := writeCacheUnit(t, dir, "read.qmlc", 2<<20, now, old)
	written := writeCacheUnit(t, dir, "written.qmlc", 1<<20, old, now.Add(-time.Hour))

	got := reconcileQMLCache(true)
	if got.status != recWouldFix {
		t.Fatalf("check mode should report the unused files, got %v (%s)", got.status, got.detail)
	}
	if !strings.Contains(got.detail, "7 MiB") || !strings.Contains(got.detail, "2 files (4 MiB)") {
		t.Errorf("check mode should report the cache size and the reclaimable part, got %q", got.detail)
	}
	for _, p := range []string{stale, staleJS, read, written} {
		if !exists(p) {
			t.Fatalf("check mode removed %s", p)
		}
	}

	got = reconcileQMLCache(false)
	if got.status != recFixed {
		t.Fatalf("fix mode should prune, got %v (%s)", got.status, got.detail)
	}
	if !strings.Contains(got.detail, "removed 2 QML cache files (4 MiB)") || !strings.Contains(got.detail, "3 MiB remain") {
		t.Errorf("fix mode should report what it freed and what remains, got %q", got.detail)
	}
	for _, p := range []string{stale, staleJS} {
		if exists(p) {
			t.Errorf("unused %s was kept", filepath.Base(p))
		}
	}
	for _, p := range []string{read, written} {
		if !exists(p) {
			t.Errorf("recently used %s was removed", filepath.Base(p))
		}
	}

	if got := reconcileQMLCache(false); got.status != recOK {
		t.Errorf("second pass should be a no-op, got %v (%s)", got.status, got.detail)
	}
}

func TestQMLCacheLeavesNonFilesAlone(t *testing.T) {
	dir := qmlCacheFixture(t)
	old := time.Now().Add(-40 * 24 * time.Hour)
	outside := writeCacheUnit(t, t.TempDir(), "keep.qmlc", 1, old, old)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.qmlc")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if got := reconcileQMLCache(false); got.status != recOK {
		t.Fatalf("a cache with no regular files has nothing to prune, got %v (%s)", got.status, got.detail)
	}
	if !exists(link) || !exists(outside) {
		t.Error("the symlink or its target was removed")
	}
}
