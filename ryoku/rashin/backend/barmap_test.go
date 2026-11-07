package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// barTestEnv points the shells index at a temp state home and neutralises PATH
// so the section never queries a live shell.
func barTestEnv(t *testing.T, shells string) {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv("PATH", "") // loadQsbarCatalog finds no ryoku-shell
	if shells == "" {
		return
	}
	path := shellsIndexPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(shells), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBarSectionListsShellsAndCommands(t *testing.T) {
	barTestEnv(t, `{"schema":1,"active":"orpheus","fallback":"orpheus","items":[
	  {"id":"orpheus","category":"subshells","source":"shipped","name":"Orpheus",
	   "settings":{"ipc":"orpheus","call":"settings","args":[]}},
	  {"id":"qsbar","category":"barstyles","source":"store","name":"QS Bar",
	   "settings":{"ipc":"qsbar","call":"settings","args":[""]}}
	]}`)
	body := barSectionBody()
	for _, want := range []string{
		vaultBarFenceBegin,
		vaultBarFenceEnd,
		"## Bar and dock",
		"| orpheus | Orpheus | subshells | shipped | yes | `ryoku-shell ipc call orpheus settings` |",
		"| qsbar | QS Bar | barstyles | store | no | `ryoku-shell ipc call qsbar settings \"\"` |",
		"ryoku-shell barstyle <id>",
		"ryoku-shell ipc call qsbar move <id> <left|center|right> <placement>",
		"ryoku-shell dock show|hide",
		"QS Bar is not the active bar style",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("bar section missing %q\n---\n%s", want, body)
		}
	}
}

func TestBarSectionDegradesWithoutShellsIndex(t *testing.T) {
	barTestEnv(t, "")
	body := barSectionBody()
	if !strings.Contains(body, vaultBarFenceBegin) || !strings.Contains(body, "## Bar and dock") {
		t.Fatal("section should still render its heading and fence")
	}
	if !strings.Contains(body, "shells index not found") {
		t.Fatalf("section should note the missing shells index:\n%s", body)
	}
}

// fakeQsbarShell puts a ryoku-shell on PATH that answers the catalogue call and
// records that it ran.
func fakeQsbarShell(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ran := filepath.Join(dir, "ran")
	script := "#!/bin/sh\necho \"$*\" >> \"" + ran + "\"\n" +
		"echo '[{\"id\":\"clock\",\"label\":\"Clock\",\"kind\":\"builtin\",\"shown\":true,\"section\":\"right\"}]'\n"
	if err := os.WriteFile(filepath.Join(dir, "ryoku-shell"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return ran
}

// Another active item has no qsbar target, so the section never asks for one.
func TestBarSectionSkipsTheCatalogueUnderAnotherItem(t *testing.T) {
	barTestEnv(t, `{"schema":1,"active":"orpheus","items":[{"id":"orpheus","category":"subshells","source":"shipped","name":"Orpheus"}]}`)
	ran := fakeQsbarShell(t)
	body := barSectionBody()
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("the catalogue was queried while Orpheus is active")
	}
	if !strings.Contains(body, "QS Bar is not the active bar style") {
		t.Fatalf("section should say QS Bar is inactive:\n%s", body)
	}
}

func TestBarSectionListsTheLiveCatalogueWhileQsbarIsActive(t *testing.T) {
	barTestEnv(t, `{"schema":1,"active":"qsbar","items":[{"id":"qsbar","category":"barstyles","source":"store","name":"QS Bar"}]}`)
	ran := fakeQsbarShell(t)
	body := barSectionBody()
	raw, err := os.ReadFile(ran)
	if err != nil || strings.TrimSpace(string(raw)) != "ipc call qsbar catalog" {
		t.Fatalf("catalogue call = %q (%v)", raw, err)
	}
	if !strings.Contains(body, "| clock | Clock | builtin | true | right | (none) |") {
		t.Fatalf("section should list the live catalogue:\n%s", body)
	}
}

func TestSettingKeys(t *testing.T) {
	if got := settingKeys(nil); got != "(none)" {
		t.Fatalf("nil -> %q", got)
	}
	if got := settingKeys([]barSetting{{Key: "a"}, {Key: ""}, {Key: "b"}}); got != "a, b" {
		t.Fatalf("keys -> %q", got)
	}
}
