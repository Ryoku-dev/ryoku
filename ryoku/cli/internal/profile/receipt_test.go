package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemReceiptsMissingRootAndAtomicMode(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	receipts, err := ReadSystemReceipts(root)
	if err != nil || len(receipts) != 0 {
		t.Fatalf("receipts=%v err=%v", receipts, err)
	}
	r := &SystemReceipt{Schema: 1}
	r.Profile.ID = "hacker"
	r.Account.Name = "hacker"
	if err := WriteSystemReceipt(root, r); err != nil {
		t.Fatal(err)
	}
	path := SystemReceiptPath(root, "hacker", "hacker")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	loaded, err := ReadSystemReceipt(root, "hacker", "hacker")
	if err != nil || loaded.Profile.ID != "hacker" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
}

func TestAccountReceiptMode(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	r := &AccountReceipt{Schema: 1}
	r.Profile.ID = "hacker"
	if err := WriteAccountReceipt(home, r); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(AccountReceiptPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
