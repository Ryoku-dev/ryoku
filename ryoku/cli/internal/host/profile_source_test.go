package host

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func profileSourceFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home", "nero")
	source := filepath.Join(home, ".local", "share", "ryoku", "profiles", "hacker")
	receiptDir := filepath.Join(home, ".local", "state", "ryoku", "store", "profiles")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(receiptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"schema":1}`)
	if err := os.WriteFile(filepath.Join(source, "profile.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(manifest))
	receipt := fmt.Sprintf(`{"category":"profiles","id":"hacker","version":"1.0.0","destination":"ryoku/profiles/hacker","files":[{"destination":"profile.json","sha256":%q}]}`, hash)
	if err := os.WriteFile(filepath.Join(receiptDir, "hacker.json"), []byte(receipt), 0o600); err != nil {
		t.Fatal(err)
	}
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	if err := os.WriteFile(passwd, []byte("nero:x:1000:1000:Nero:"+home+":/bin/bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(group, []byte("nero:x:1000:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := New(Config{PasswdPath: passwd, GroupPath: group, UIDSet: true, UID: 1000})
	return app, source, hash
}

func TestVerifiedProfileSourceRequiresCallerStorePath(t *testing.T) {
	app, source, _ := profileSourceFixture(t)
	if _, err := app.verifiedProfileSource(filepath.Join(filepath.Dir(source), "..", "hacker")); err == nil {
		t.Fatal("source outside the caller Store path accepted")
	}
	if _, err := app.verifiedProfileSource(source); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
}

func TestVerifiedProfileSourceUsesPKEXECUID(t *testing.T) {
	app, source, _ := profileSourceFixture(t)
	app.cfg.UID = 0
	app.cfg.Getenv = func(key string) string {
		if key == "PKEXEC_UID" {
			return "1000"
		}
		return ""
	}
	if _, err := app.verifiedProfileSource(source); err != nil {
		t.Fatalf("pkexec caller source rejected: %v", err)
	}
}

func TestVerifiedProfileSourceRejectsReceiptHashMismatch(t *testing.T) {
	app, source, _ := profileSourceFixture(t)
	if err := os.WriteFile(filepath.Join(source, "profile.json"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := app.verifiedProfileSource(source)
	if err == nil || !strings.Contains(err.Error(), "Reinstall it from the Store") {
		t.Fatalf("hash mismatch error = %v", err)
	}
}

func TestVerifiedProfileSourceRejectsSymlinkedPayload(t *testing.T) {
	app, source, hash := profileSourceFixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte(`{"schema":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(source, "profile.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "profile.json")); err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("fixture hash is empty")
	}
	if _, err := app.verifiedProfileSource(source); err == nil {
		t.Fatal("symlinked payload accepted")
	}
}
