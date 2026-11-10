package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/vault"
)

func TestVaultReconcilerRespectsUnwireOptOut(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	share := filepath.Join(root, "share")
	for _, dir := range []string{
		home,
		filepath.Join(home, ".claude"),
		filepath.Join(share, "skills", "ryoku"),
		filepath.Join(share, "wiki"),
		filepath.Join(root, "runtime"),
		filepath.Join(root, "base"),
		filepath.Join(root, "bin"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(share, "skills", "ryoku", "SKILL.md"), []byte("---\nname: ryoku\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(share, "wiki", "README.md"), []byte("# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("RYOKU_WM", "")
	t.Setenv("RYOKU_VAULT_SHARE", share)
	t.Setenv("RYOKU_VAULT_VERSION", "test")
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(root, "base"))
	t.Setenv("PATH", filepath.Join(root, "bin"))

	if err := vault.Unwire(); err != nil {
		t.Fatal(err)
	}
	result := reconcileVault(false)
	if result.status != recFixed {
		t.Fatalf("reconcile status = %v, detail = %q", result.status, result.detail)
	}
	if !vault.IsOptedOut() {
		t.Fatal("reconciler cleared the wiring opt-out")
	}
	if vault.Wired() {
		t.Fatal("reconciler wired an opted-out harness")
	}
	if result := reconcileVault(true); result.status != recOK {
		t.Fatalf("check-only status = %v, detail = %q", result.status, result.detail)
	}
}
