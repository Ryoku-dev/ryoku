package main

import (
	"encoding/json"
	"fmt"
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

// stubReloadProvider forces a fake provider whose caps claim configReload when
// reload is true and whose act exits actExit, logging each act to the returned file
func stubReloadProvider(t *testing.T, reload bool, actExit int) string {
	t.Helper()
	bin := t.TempDir()
	acts := filepath.Join(t.TempDir(), "acts")
	supports := `[]`
	if reload {
		supports = `["configReload"]`
	}
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
caps) printf '%%s\n' '{"name":"stub","supports":%s}' ;;
act) printf '%%s\n' "$2" >> %q; printf '%%s\n' 'reload rejected' >&2; exit %d ;;
*) exit 2 ;;
esac
`, supports, acts, actExit)
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-stub"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_WM", "stub")
	wmOnce = sync.Once{}
	wmClient = nil
	t.Cleanup(func() {
		wmOnce = sync.Once{}
		wmClient = nil
	})
	return acts
}

func TestApplyAnimPresetRestoresFileWhenReloadFails(t *testing.T) {
	for name, before := range map[string]string{"present": "dusky\n", "absent": ""} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if before != "" {
				if err := atomicWrite(animPresetPath(), []byte(before), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			acts := stubReloadProvider(t, true, 19)

			err := applyAnimPreset("bounce")
			if err == nil || !strings.Contains(err.Error(), "reload rejected") {
				t.Fatalf("applyAnimPreset error = %v, want the provider reload error", err)
			}
			got, rerr := os.ReadFile(animPresetPath())
			if before == "" {
				if !os.IsNotExist(rerr) {
					t.Fatalf("anim-preset exists after failed reload: %q %v", got, rerr)
				}
			} else if string(got) != before {
				t.Fatalf("anim-preset = %q after failed reload, want %q", got, before)
			}
			if log, _ := os.ReadFile(acts); strings.Count(string(log), "config.reload") != 2 {
				t.Fatalf("reloads = %q, want the refused one and one on the restored file", log)
			}

			stubReloadProvider(t, true, 0)
			if err := applyAnimPreset("bounce"); err != nil {
				t.Fatal(err)
			}
			if got := currentAnimPreset(); got != "bounce" {
				t.Fatalf("currentAnimPreset() = %q after retry, want bounce", got)
			}
		})
	}
}

func TestApplyAnimPresetWithoutReloadIsSuccess(t *testing.T) {
	for name, stub := range map[string]func(t *testing.T){
		"unsupported": func(t *testing.T) { stubReloadProvider(t, false, 19) },
		"no provider": func(t *testing.T) {
			// no session and no provider binary on PATH, so detection names none
			t.Setenv("PATH", t.TempDir())
			wmOnce = sync.Once{}
			wmClient = nil
			t.Cleanup(func() {
				wmOnce = sync.Once{}
				wmClient = nil
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			stub(t)
			if err := applyAnimPreset("fade"); err != nil {
				t.Fatal(err)
			}
			if got := currentAnimPreset(); got != "fade" {
				t.Fatalf("currentAnimPreset() = %q, want fade", got)
			}
		})
	}
}
