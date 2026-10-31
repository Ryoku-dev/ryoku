package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func zshrcFixture(t *testing.T, shell string) (home, template string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	stubAccountShell(t, shell)
	template = filepath.Join(t.TempDir(), "zshrc")
	oldTemplate := zshrcTemplatePath
	zshrcTemplatePath = template
	t.Cleanup(func() { zshrcTemplatePath = oldTemplate })
	return home, template
}

func TestReconcileZshrcSeedsMissingFile(t *testing.T) {
	home, template := zshrcFixture(t, "/usr/bin/zsh")
	want := "[ -r \"$HOME/.config/zsh/ryoku.zsh\" ] && . \"$HOME/.config/zsh/ryoku.zsh\"\n"
	if err := os.WriteFile(template, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".zshrc")

	if got := reconcileZshrc(true); got.status != recWouldFix {
		t.Fatalf("check-only status = %s, want todo", got.status.label())
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("check-only created .zshrc: %v", err)
	}
	if got := reconcileZshrc(false); got.status != recFixed {
		t.Fatalf("repair status = %s, detail = %q", got.status.label(), got.detail)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf(".zshrc = %q, want %q", body, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf(".zshrc mode = %o, want 644", info.Mode().Perm())
	}
	if got := reconcileZshrc(false); got.status != recOK {
		t.Fatalf("second repair status = %s, want ok", got.status.label())
	}
}

func TestReconcileZshrcLeavesExistingFileAlone(t *testing.T) {
	home, template := zshrcFixture(t, "/usr/bin/zsh")
	if err := os.WriteFile(template, []byte("shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(path, []byte("custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := reconcileZshrc(false); got.status != recOK {
		t.Fatalf("status = %s, want ok", got.status.label())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "custom\n" {
		t.Fatalf("existing .zshrc was overwritten: %q", body)
	}
}

func TestReconcileZshrcSkipsOtherShellsAndMissingTemplate(t *testing.T) {
	home, _ := zshrcFixture(t, "/usr/bin/fish")
	if got := reconcileZshrc(false); got.status != recOK {
		t.Fatalf("fish status = %s, want ok", got.status.label())
	}
	if _, err := os.Lstat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("fish account got .zshrc: %v", err)
	}

	stubAccountShell(t, "/usr/bin/zsh")
	if got := reconcileZshrc(false); got.status != recWarn {
		t.Fatalf("missing-template status = %s, want warning", got.status.label())
	}
}
