package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

func TestWMListIncludesUnavailableReason(t *testing.T) {
	providers := wm.Providers()
	if len(providers) < 2 {
		t.Fatalf("need two window-manager providers, got %d", len(providers))
	}
	unavailablePackage := wmPackage(providers[0])
	availablePackage := wmPackage(providers[1])
	const reason = "This desktop is not packaged for this host yet."

	bin := t.TempDir()
	host := `#!/bin/sh
case "$1:$2:$3" in
  "pkg:available:$RYOKU_TEST_AVAILABLE_PACKAGE") exit 0 ;;
  "pkg:why:$RYOKU_TEST_UNAVAILABLE_PACKAGE") printf '%s\n' "$RYOKU_TEST_UNAVAILABLE_REASON"; exit 0 ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_TEST_AVAILABLE_PACKAGE", availablePackage)
	t.Setenv("RYOKU_TEST_UNAVAILABLE_PACKAGE", unavailablePackage)
	t.Setenv("RYOKU_TEST_UNAVAILABLE_REASON", reason)

	var listErr error
	out := captureStdout(t, func() { listErr = wmList() })
	if listErr != nil {
		t.Fatal(listErr)
	}
	var rows []wmProvider
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("decode wm list: %v\n%s", err, out)
	}

	seenUnavailable := false
	seenAvailable := false
	for _, row := range rows {
		switch row.Package {
		case unavailablePackage:
			seenUnavailable = true
			if row.Available {
				t.Fatalf("unavailable provider %q marked available", row.Name)
			}
			if row.Reason != reason {
				t.Fatalf("unavailable provider reason = %q, want %q", row.Reason, reason)
			}
		case availablePackage:
			seenAvailable = true
			if !row.Available {
				t.Fatalf("available provider %q marked unavailable", row.Name)
			}
			if row.Reason != "" {
				t.Fatalf("available provider reason = %q, want empty", row.Reason)
			}
		}
	}
	if !seenUnavailable || !seenAvailable {
		t.Fatalf("provider rows missing: unavailable=%t available=%t", seenUnavailable, seenAvailable)
	}
}
