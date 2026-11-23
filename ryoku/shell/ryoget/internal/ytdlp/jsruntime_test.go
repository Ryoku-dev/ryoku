package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func freshJSCache(t *testing.T, runtimes ...string) {
	t.Helper()
	old := jsRuntimes
	jsRuntimes = runtimes
	jsMu.Lock()
	jsCache = map[string][]string{}
	jsMu.Unlock()
	t.Cleanup(func() {
		jsRuntimes = old
		jsMu.Lock()
		jsCache = map[string][]string{}
		jsMu.Unlock()
	})
}

func fakeRuntime(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return p
}

func TestJSRuntimePassedWhenKnown(t *testing.T) {
	freshJSCache(t, "ryoget-no-such-runtime", "ryoget-test-js")
	rt := fakeRuntime(t, "ryoget-test-js")
	env := stubEnv(t, "video.json")
	dir := t.TempDir()
	args, count := filepath.Join(dir, "args"), filepath.Join(dir, "help")
	t.Setenv("STUB_ARGS", args)
	t.Setenv("STUB_HELP", "1")
	t.Setenv("STUB_HELP_COUNT", count)
	for range 2 {
		if _, err := Resolve(context.Background(), env, "https://vimeo.test/123"); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(args)
	if !strings.Contains(string(got), "--js-runtimes\nryoget-test-js:"+rt+"\n") {
		t.Fatalf("args %q lack the runtime", got)
	}
	if calls, _ := os.ReadFile(count); strings.Count(string(calls), "x") != 1 {
		t.Fatalf("--help ran %d times, want once per run", strings.Count(string(calls), "x"))
	}
}

func TestJSRuntimeSkippedForOldYTDLP(t *testing.T) {
	freshJSCache(t, "ryoget-test-js")
	fakeRuntime(t, "ryoget-test-js")
	env := stubEnv(t, "video.json")
	args := filepath.Join(t.TempDir(), "args")
	t.Setenv("STUB_ARGS", args)
	if _, err := Resolve(context.Background(), env, "https://vimeo.test/123"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(args); strings.Contains(string(got), "--js-runtimes") {
		t.Fatalf("an old yt-dlp got %q", got)
	}
}

func TestJSRuntimeNeedsAnInstalledOne(t *testing.T) {
	freshJSCache(t, "ryoget-no-such-runtime")
	t.Setenv("STUB_HELP", "1")
	if got := jsRuntimeArgs(context.Background(), stubEnv(t, "").YTDLP); got != nil {
		t.Fatalf("args = %v", got)
	}
}
