package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayHomeSeedKeepModifiedAndReset(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	payload := filepath.Join(root, "payload")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(payload, "home", ".config", "app.conf"), "home=@HOME@ user=@USER@ id=@ID@\n")
	writeTestFile(t, filepath.Join(payload, "home", "notes", "start.txt"), "seed\n")
	m := &Manifest{ID: "hacker", Version: "1.0.0", Expand: []string{".config/app.conf"}, SeedOnce: []string{"notes/start.txt"}, UserData: []string{"work"}}
	v := Vars{Home: home, User: "hacker", ID: "hacker"}
	receipt, changes, err := LayHome(home, filepath.Join(payload, "home"), m, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("changes = %#v", changes)
	}
	managed := filepath.Join(home, ".config", "app.conf")
	seed := filepath.Join(home, "notes", "start.txt")
	if err := os.WriteFile(managed, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seed, []byte("my notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, changes, err := LayHome(home, filepath.Join(payload, "home"), m, v, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !hasChange(changes, ".config/app.conf", "keep-user-modified") {
		t.Fatalf("missing keep change: %#v", changes)
	}
	if body, _ := os.ReadFile(seed); string(body) != "my notes\n" {
		t.Fatal("seedOnce was overwritten")
	}
	unmanaged := filepath.Join(home, "work", "report.txt")
	writeTestFile(t, unmanaged, "keep\n")
	changes, err = ResetHome(home, filepath.Join(payload, "home"), m, v, next, false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasChange(changes, ".config/app.conf", "restore") {
		t.Fatalf("reset changes = %#v", changes)
	}
	if body, _ := os.ReadFile(managed); string(body) != "home="+home+" user=hacker id=hacker\n" {
		t.Fatalf("managed body = %q", body)
	}
	if body, _ := os.ReadFile(seed); string(body) != "my notes\n" {
		t.Fatal("reset touched seedOnce")
	}
	if body, _ := os.ReadFile(unmanaged); string(body) != "keep\n" {
		t.Fatal("reset touched user data")
	}
}

func TestLayHomeRefusesSymlinkPathComponent(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	outside := filepath.Join(root, "outside")
	payload := filepath.Join(root, "payload")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(payload, "home", ".config", "app.conf"), "managed\n")
	if _, _, err := LayHome(home, filepath.Join(payload, "home"), &Manifest{ID: "hacker", Version: "1.0.0"}, Vars{Home: home}, nil); err == nil {
		t.Fatal("LayHome followed a symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "app.conf")); !os.IsNotExist(err) {
		t.Fatal("wrote outside home")
	}
}

func hasChange(changes []Change, path, action string) bool {
	for _, change := range changes {
		if change.Path == path && change.Action == action {
			return true
		}
	}
	return false
}
