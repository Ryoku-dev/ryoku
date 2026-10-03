package main

import (
	"strings"
	"testing"
)

// The browser and apps steps are the installer's product choices: exactly the
// three shipped browsers, one pick; a keep/remove table whose required rows
// refuse to toggle and whose drops reach the backend as one drop list.

func TestAppsStepsExistInOrder(t *testing.T) {
	f := steps()
	b, a, r := flowIndex(f, "browser"), flowIndex(f, "apps"), flowIndex(f, "review")
	if b < 0 || a < 0 || r < 0 {
		t.Fatalf("browser/apps/review steps missing: %d %d %d", b, a, r)
	}
	if !(b < a && a < r) {
		t.Fatalf("steps out of order: browser=%d apps=%d review=%d", b, a, r)
	}
	if f[b].kind != kSelect || f[a].kind != kApps {
		t.Fatalf("wrong kinds: browser=%v apps=%v", f[b].kind, f[a].kind)
	}
}

func TestBrowserListIsExactlyThree(t *testing.T) {
	bs := browsers()
	if len(bs) != 3 {
		t.Fatalf("Ryoku ships three browsers, got %d", len(bs))
	}
	seen := map[string]bool{}
	for _, b := range bs {
		if seen[b.key] {
			t.Fatalf("duplicate browser key %q", b.key)
		}
		seen[b.key] = true
	}
	for _, want := range []string{"zen", "chromium", "firefox"} {
		if !seen[want] {
			t.Errorf("missing browser %q", want)
		}
	}
}

func TestRequiredRowsAreDefaultOn(t *testing.T) {
	for _, r := range appRows() {
		if r.Req != "" && !r.Def {
			t.Errorf("%s: required rows must ship on by default", r.ID)
		}
	}
}

func TestAppsToggleSkipsRequired(t *testing.T) {
	m := newModel()
	m.state, m.flow = "wizard", steps()
	m.idx = flowIndex(m.flow, "apps")
	m.loadStep()
	if m.keep == nil {
		t.Fatal("loadStep did not seed the keep map")
	}
	m.alsel = appsRowIdx(m.appsRows(), 0) // the first app row is a required one
	m.appsKey("space")
	if !m.keep[m.appsRows()[m.alsel].id] {
		t.Fatal("a required row toggled off")
	}
	if m.inputErr == "" {
		t.Error("refusing the toggle must explain itself")
	}
	// Walk to the first removable row; a clean toggle clears the notice.
	rows := m.appsRows()
	for i, r := range rows {
		if r.kind == "app" {
			if ar, _ := appRowByID(r.id); ar.Req == "" {
				m.alsel = i
				break
			}
		}
	}
	id := rows[m.alsel].id
	m.appsKey("space")
	if m.keep[id] {
		t.Fatalf("row %s did not toggle off", id)
	}
	if m.inputErr != "" {
		t.Errorf("clean toggle left an error: %q", m.inputErr)
	}
}

func TestDeselectDoesNotTouchRequiredPackages(t *testing.T) {
	keep := appDefaults()
	for _, r := range appRows() {
		keep[r.ID] = false // user tries to drop everything
	}
	d := deselectedPkgs(keep)
	set := map[string]bool{}
	for _, p := range d {
		set[p] = true
	}
	for _, r := range appRows() {
		if r.Req == "" {
			continue
		}
		for _, p := range r.Pkgs {
			if set[p] {
				t.Errorf("required package %s (%s) appears in the drop list", p, r.ID)
			}
		}
	}
}

func TestAppsResetRestoresDefaults(t *testing.T) {
	m := newModel()
	m.state, m.flow = "wizard", steps()
	m.idx = flowIndex(m.flow, "apps")
	m.loadStep()
	m.keep["yazi"] = false
	m.keep["docker"] = false
	m.appsKey("a")
	if !m.keep["yazi"] || !m.keep["docker"] {
		t.Fatal("'a' must restore the shipped defaults")
	}
	if got := appsSummary(m.keep); got != "all" {
		t.Errorf("appsSummary after reset = %q, want %q", got, "all")
	}
	m.keep["docker"] = false
	if got := appsSummary(m.keep); !strings.Contains(got, "docker") {
		t.Errorf("appsSummary after drop = %q, must name docker", got)
	}
}

// installEnv is the handoff contract: RYOKU_BROWSER always carries the pick,
// and RYOKU_DROP_PACKAGES names the deselected apps plus the lost browsers.
func TestInstallEnvCarriesBrowserAndDrops(t *testing.T) {
	envHas := func(env []string, want string) bool {
		for _, e := range env {
			if e == want {
				return true
			}
		}
		return false
	}
	envHasPrefix := func(env []string, pfx string) (string, bool) {
		for _, e := range env {
			if strings.HasPrefix(e, pfx) {
				return e, true
			}
		}
		return "", false
	}

	// A model that never visited the steps: shipped defaults flow through.
	m := newModel()
	m.diskDev, m.pwHash = "/dev/vda", "x"
	m.picks["disk"] = "whole"
	env := m.installEnv()
	if !envHas(env, "RYOKU_BROWSER=zen") {
		t.Fatalf("browser default missing from %v", env)
	}
	// One-browser policy: even untouched, the two losing repo browsers are
	// dropped; no app package may be listed (the apps step was never visited).
	drops, ok := envHasPrefix(env, "RYOKU_DROP_PACKAGES=")
	if !ok || drops != "RYOKU_DROP_PACKAGES=chromium,firefox" {
		t.Fatalf("default drop list = %q (ok=%v), want only chromium+firefox", drops, ok)
	}

	// Chromium picked, two optional apps removed.
	m.picks["browser"] = "chromium"
	m.keep = appDefaults()
	m.keep["docker"] = false
	m.keep["flatpak"] = false
	env = m.installEnv()
	if !envHas(env, "RYOKU_BROWSER=chromium") {
		t.Error("the chosen browser did not reach the env")
	}
	drops, ok = envHasPrefix(env, "RYOKU_DROP_PACKAGES=")
	if !ok {
		t.Fatal("deselections must emit a drop list")
	}
	for _, want := range []string{"docker", "flatpak", "firefox", "zen-browser-bin"} {
		if !strings.Contains(drops, want) {
			t.Errorf("drop list %q missing %s", drops, want)
		}
	}
	if strings.Contains(drops, "chromium") {
		t.Error("the picked browser must never be in the drop list")
	}
	for _, r := range appRows() {
		if r.Req == "" {
			continue
		}
		for _, p := range r.Pkgs {
			if strings.Contains(drops, p) {
				t.Errorf("required package %s leaked into the drop list", p)
			}
		}
	}
}

// The apps step must Tab forward with the summary recorded, and the whole
// checklist must render without error on a tiny grid (the VT invariant).
func TestAppsStepAdvancesAndRendersSmall(t *testing.T) {
	for _, sz := range gridSizes {
		m := wizardAt("apps", sz[0], sz[1])
		gw, gh := frameBox(m.fittedFrame())
		if gw > sz[0] || gh > sz[1] {
			t.Errorf("apps step at %dx%d rendered %dx%d", sz[0], sz[1], gw, gh)
		}
	}
	m := wizardAt("apps", 112, 42)
	before := m.idx
	m.appsKey("tab")
	if m.idx <= before {
		t.Fatal("tab did not advance from the apps step")
	}
	if m.picks["apps"] == "" {
		t.Error("tab did not record the apps summary pick")
	}
}
