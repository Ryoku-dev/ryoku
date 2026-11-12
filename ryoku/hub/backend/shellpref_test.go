package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureShellLoadersPreservesUserContent(t *testing.T) {
	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	profile := filepath.Join(home, ".bash_profile")
	if err := os.WriteFile(bashrc, []byte("alias mine='yes'\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("export MINE=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rewrote, err := ensureShellLoaders(home, "bash")
	if err != nil || len(rewrote) != 2 {
		t.Fatalf("ensureShellLoaders = %d rewritten, %v", len(rewrote), err)
	}
	for _, path := range []string{bashrc, profile} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !strings.HasSuffix(text, bashLoader+"\n") {
			t.Errorf("loader not last in %s: %q", path, text)
		}
	}
	b, _ := os.ReadFile(bashrc)
	if !strings.Contains(string(b), "alias mine='yes'") {
		t.Fatal("existing bashrc content was lost")
	}
	rewrote, err = ensureShellLoaders(home, "bash")
	if err != nil || len(rewrote) != 0 {
		t.Fatalf("second ensure = %d rewritten, %v, want unchanged", len(rewrote), err)
	}
}

func TestShellChoiceValidation(t *testing.T) {
	dir := t.TempDir()
	fish := filepath.Join(dir, "fish")
	if err := os.WriteFile(fish, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	shells := filepath.Join(dir, "shells")
	if err := os.WriteFile(shells, []byte(fish+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	choices := map[string]string{"fish": fish, "bash": filepath.Join(dir, "bash")}
	if got, err := validateShellChoice("fish", choices, shells); err != nil || got != fish {
		t.Fatalf("fish = %q, %v", got, err)
	}
	if _, err := validateShellChoice("bash", choices, shells); err == nil {
		t.Fatal("missing bash accepted")
	}
	if _, err := validateShellChoice("sh", choices, shells); err == nil {
		t.Fatal("unknown shell accepted")
	}
}

func TestZshPromptDefaultsAndPersists(t *testing.T) {
	home := t.TempDir()
	if got, err := zshPrompt(home); err != nil || got != "starship" {
		t.Fatalf("default zsh prompt = %q, %v", got, err)
	}
	if err := setZshPrompt(home, "oh-my-zsh"); err != nil {
		t.Fatal(err)
	}
	if got, err := zshPrompt(home); err != nil || got != "oh-my-zsh" {
		t.Fatalf("saved zsh prompt = %q, %v", got, err)
	}
	if err := setZshPrompt(home, "invalid"); err == nil {
		t.Fatal("invalid zsh prompt accepted")
	}
}

// the picker writes under $HOME, so a private HOME keeps it off the account's real home
func TestShellPromptWritesUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runShellPref([]string{"prompt", "oh-my-zsh"}); err != nil {
		t.Fatal(err)
	}
	if got, err := zshPrompt(home); err != nil || got != "oh-my-zsh" {
		t.Fatalf("prompt under $HOME = %q, %v", got, err)
	}
}

func TestSyncSessionShellUpdatesLaunchEnvironments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old := runSessionCommand
	defer func() { runSessionCommand = old }()
	var calls []string
	runSessionCommand = func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	syncSessionShell("/usr/bin/zsh")
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"ryoku-host svc env SHELL=/usr/bin/zsh",
		"ryoku reload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("session calls missing %q:\n%s", want, joined)
		}
	}
	// SHELL is persisted into desktop.env for the provider to emit into env config.
	d, _ := readJSONMap(desktopStorePath())["desktop"].(map[string]any)
	env, _ := d["env"].([]any)
	found := false
	for _, e := range env {
		if m, ok := e.(map[string]any); ok && m["key"] == "SHELL" && m["value"] == "/usr/bin/zsh" {
			found = true
		}
	}
	if !found {
		t.Errorf("SHELL not persisted into desktop.env: %v", d["env"])
	}
}

// a dismissed prompt or failed usermod leaves the rc files as they were (absent stays absent)
func TestShellSetRestoresLoadersWhenEscalationFails(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "pkexec"), []byte("#!/bin/sh\nexit 126\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, key := range []string{"bash", "zsh"} {
		home := t.TempDir()
		bashrc := filepath.Join(home, ".bashrc")
		if err := os.WriteFile(bashrc, []byte("alias mine='yes'\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := applyShellChoice(home, key); err == nil {
			t.Fatalf("%s: set succeeded with a failing escalation", key)
		}
		st, err := os.Stat(bashrc)
		if err != nil || readFileString(bashrc) != "alias mine='yes'\n" || st.Mode().Perm() != 0o640 {
			t.Fatalf("%s: .bashrc changed by a failed set: %q %v", key, readFileString(bashrc), err)
		}
		for _, name := range []string{".bash_profile", ".zshrc"} {
			if fileExists(filepath.Join(home, name)) {
				t.Fatalf("%s: %s left behind by a failed set", key, name)
			}
		}
	}
}

// bash writes two rc files; a failure on the second must not leave the first rewritten or a tmp stub
func TestEnsureShellLoadersRestoresOnPartialFailure(t *testing.T) {
	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("alias mine='yes'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// the second file's tmp write lands on /dev/full, so it fails mid-write after .bashrc was rewritten
	if err := os.Symlink("/dev/full", filepath.Join(home, ".bash_profile.ryoku-tmp")); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureShellLoaders(home, "bash"); err == nil {
		t.Fatal("ensureShellLoaders succeeded with a full disk")
	}
	if fileExists(filepath.Join(home, ".bash_profile")) {
		t.Fatal(".bash_profile left behind by a failed write")
	}
	if got := readFileString(bashrc); got != "alias mine='yes'\n" {
		t.Fatalf(".bashrc = %q after a failed set", got)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ryoku-tmp") {
			t.Fatalf("tmp stub %s left behind", e.Name())
		}
	}
}

// a refused symlinked rc file stays a symlink while the file rewritten before it goes back
func TestEnsureShellLoadersKeepsRefusedSymlink(t *testing.T) {
	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	target := filepath.Join(home, "dotfiles-profile")
	if err := os.WriteFile(bashrc, []byte("alias mine='yes'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("export MINE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, ".bash_profile")); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureShellLoaders(home, "bash"); err == nil {
		t.Fatal("symlinked .bash_profile was replaced")
	}
	if st, err := os.Lstat(filepath.Join(home, ".bash_profile")); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".bash_profile is no longer a symlink: %v", err)
	}
	if got := readFileString(bashrc); got != "alias mine='yes'\n" {
		t.Fatalf(".bashrc = %q after a refused set", got)
	}
}
