package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The first-turn note is a contract with the user's own agent persona: absent
// keeps the Needle, an explicit "" injects nothing, and a path replaces the
// text framed the same way. A file that cannot be read must not strip the
// machine guidance silently.
func TestIntroPreamble(t *testing.T) {
	if got := (Config{}).IntroPreamble(); got != needleIdentity {
		t.Fatalf("unset intro = %q, want the Needle identity", got)
	}

	off := ""
	if got := (Config{Intro: &off}).IntroPreamble(); got != "" {
		t.Fatalf("empty intro = %q, want no preamble", got)
	}

	dir := t.TempDir()
	persona := filepath.Join(dir, "SOUL.md")
	if err := os.WriteFile(persona, []byte("You are Null.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (Config{Intro: &persona}).IntroPreamble()
	if got != identityNote("You are Null.") || stripIdentityPreamble(got+"hi") != "hi" {
		t.Fatalf("path intro = %q, want the file text framed like the Needle note", got)
	}

	// A ~/ path resolves against the home directory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ryoku", "persona.txt"), []byte("Null"), 0o644); err != nil {
		t.Fatal(err)
	}
	tilde := "~/.ryoku/persona.txt"
	if got := (Config{Intro: &tilde}).IntroPreamble(); !strings.Contains(got, "Null") {
		t.Fatalf("~/ intro = %q, want the file text", got)
	}

	missing := filepath.Join(dir, "nope.md")
	if got := (Config{Intro: &missing}).IntroPreamble(); got != needleIdentity {
		t.Fatalf("unreadable intro file = %q, want the default", got)
	}
}
