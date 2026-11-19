package fs

import (
	"context"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
)

func TestHashlineRejectsInventedTagAndOverlaps(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/f.go": "a\nb\nc\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	ctx := context.Background()
	tag := fileTag("a\nb\nc\n")
	patch := "[f.go#" + tag + "]\nPUT 2.=2:\n+B\n"
	result := (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if !result.IsError || !strings.Contains(result.Text, "not read in this session") {
		t.Fatalf("invented tag: %#v", result)
	}
	(readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "f.go"})}, env)
	patch += "CUT 2.=3\n"
	result = (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if !result.IsError || !strings.Contains(result.Text, "overlaps") || fs.writes != 0 {
		t.Fatalf("overlapping patch: %#v", result)
	}
}

func TestHashlineMoveAndRemove(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/a.go": "one\n", "/work/b.go": "two\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	ctx := context.Background()
	for _, p := range []string{"a.go", "b.go"} {
		result := (readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": p})}, env)
		if result.IsError {
			t.Fatal(result.Text)
		}
	}
	patch := "[a.go#" + fileTag("one\n") + "]\nPUT 1.=1:\n+ONE\nMV c.go\n" +
		"[b.go#" + fileTag("two\n") + "]\nREM\n"
	result := (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(fs.files["/work/c.go"]) != "ONE\n" || len(fs.files) != 1 || result.Diff == nil {
		t.Fatalf("move/removal: %#v, files %v", result, fs.files)
	}
}

func TestHashlineRegisterPersistsAndNoChangeIsRefused(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/a.go": "a\nb\n", "/work/b.go": "x\ny\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	ctx := context.Background()
	for _, p := range []string{"a.go", "b.go"} {
		(readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": p})}, env)
	}
	patch := "[a.go#" + fileTag("a\nb\n") + "]\nCUT 2.=2 @saved\n"
	result := (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError {
		t.Fatal(result.Text)
	}
	patch = "[b.go#" + fileTag("x\ny\n") + "]\nPUT >2 @saved\n"
	result = (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(fs.files["/work/b.go"]) != "x\ny\nb\n" {
		t.Fatalf("register paste: %#v, file %q", result, fs.files["/work/b.go"])
	}
	patch = "[b.go#" + fileTag("x\ny\nb\n") + "]\nPUT 3.=3:\n+b\n"
	result = (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if !result.IsError || !strings.Contains(result.Text, "unchanged") {
		t.Fatalf("no-change patch: %#v", result)
	}
}

func TestHashlineXXHash32Tags(t *testing.T) {
	if got := fileTag(""); got != "5D05" {
		t.Fatalf("empty file hash = %s, want 5D05", got)
	}
	if got := fileTag("abc"); got != "53FF" {
		t.Fatalf("abc hash = %s, want 53FF", got)
	}
	if fileTag("abc \t\n") != fileTag("abc\n") {
		t.Fatal("trailing spaces must not change a hashline tag")
	}
}
