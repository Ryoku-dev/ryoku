package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	wm "ryoku-wm"
)

func TestWMListDoesNotMarkDeadForcedProviderActive(t *testing.T) {
	providers := wm.Providers()
	if len(providers) == 0 {
		t.Fatal("no window-manager providers registered")
	}
	forced := providers[0]
	t.Setenv("RYOKU_WM", forced)
	// Every live handle would outrank the forced name; the seam owns the list.
	for _, h := range wm.SessionHandles() {
		t.Setenv(h, "")
	}
	t.Setenv("PATH", t.TempDir())

	var listErr error
	out := captureStdout(t, func() { listErr = wmList() })
	if listErr != nil {
		t.Fatal(listErr)
	}
	var rows []wmProvider
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("decode wm list: %v\n%s", err, out)
	}
	for _, row := range rows {
		if row.Name != forced {
			continue
		}
		if row.Active {
			t.Fatalf("dead forced provider %q marked active", forced)
		}
		return
	}
	t.Fatalf("forced provider %q missing from wm list", forced)
}

func TestApplyDesktopReturnsReloadError(t *testing.T) {
	binDir := t.TempDir()
	const provider = "reload-fails"
	script := `#!/bin/sh
case "$1" in
caps)
  printf '%s\n' '{"name":"reload-fails","supports":["configReload"]}'
  ;;
apply)
  printf '%s\n' '{"reloadNeeded":true}'
  ;;
act)
  printf '%s\n' 'reload rejected' >&2
  exit 19
  ;;
*)
  exit 2
  ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "ryoku-wm-"+provider), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("RYOKU_WM", provider)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	wmOnce = sync.Once{}
	wmClient = nil
	t.Cleanup(func() {
		wmOnce = sync.Once{}
		wmClient = nil
	})

	err := applyDesktop()
	if err == nil {
		t.Fatal("applyDesktop swallowed the reload failure")
	}
	if !strings.Contains(err.Error(), "reload rejected") {
		t.Fatalf("applyDesktop error %q does not include the provider reload error", err)
	}
}
