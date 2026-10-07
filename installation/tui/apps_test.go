package main

import (
	"strings"
	"testing"
)

// The browser, login-shell, and apps steps are the installer's product choices:
// one browser, one account shell, and a keep/remove table whose required rows
// refuse to toggle. Every losing package reaches the backend in one drop list.

func TestProductStepsExistInOrder(t *testing.T) {
	f := steps()
	b := flowIndex(f, "browser")
	s := flowIndex(f, "login-shell")
	a := flowIndex(f, "apps")
	r := flowIndex(f, "review")
	if b < 0 || s < 0 || a < 0 || r < 0 {
		t.Fatalf("browser/login-shell/apps/review steps missing: %d %d %d %d", b, s, a, r)
	}
	if s != b+1 || !(s < a && a < r) {
		t.Fatalf("steps out of order: browser=%d login-shell=%d apps=%d review=%d", b, s, a, r)
	}
	if f[b].kind != kSelect || f[s].kind != kSelect || f[a].kind != kApps {
		t.Fatalf("wrong kinds: browser=%v login-shell=%v apps=%v", f[b].kind, f[s].kind, f[a].kind)
	}
	if !f[b].numbered || !f[s].numbered {
		t.Fatal("browser and login-shell choices must be numbered")
	}
	m := newModel()
	if m.picks["browser"] != "firefox" || m.picks["login-shell"] != "fish" {
		t.Fatalf("product defaults = browser %q, shell %q; want firefox, fish",
			m.picks["browser"], m.picks["login-shell"])
	}
}

func TestBrowserListOrderAndRecommendation(t *testing.T) {
	bs := browsers()
	want := []string{"firefox", "chromium", "zen"}
	if len(bs) != len(want) {
		t.Fatalf("Ryoku ships %d browsers, got %d", len(want), len(bs))
	}
	for i, key := range want {
		if bs[i].key != key {
			t.Errorf("browser %d = %q, want %q", i, bs[i].key, key)
		}
	}
	if !strings.Contains(bs[0].hint, "Recommended") {
		t.Errorf("Firefox description does not mark it Recommended: %q", bs[0].hint)
	}
	if strings.Contains(bs[2].hint, "default") {
		t.Errorf("Zen description still claims to be the default: %q", bs[2].hint)
	}
}

func TestLoginShellListOrderAndAppOwnership(t *testing.T) {
	ss := loginShells()
	want := []string{"fish", "zsh", "bash"}
	if len(ss) != len(want) {
		t.Fatalf("Ryoku offers %d login shells, got %d", len(want), len(ss))
	}
	for i, key := range want {
		if ss[i].key != key {
			t.Errorf("login shell %d = %q, want %q", i, ss[i].key, key)
		}
	}
	if !strings.Contains(ss[0].hint, "Recommended") {
		t.Errorf("Fish description does not mark it Recommended: %q", ss[0].hint)
	}
	for _, r := range appRows() {
		if r.ID == "shell" || r.ID == "bash-edit" {
			t.Errorf("shell-owned app row %q remains in Apps & tools", r.ID)
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

// installEnv is the handoff contract: both choices always carry their picks,
// and RYOKU_DROP_PACKAGES names deselected apps and every losing product stack.
func TestInstallEnvCarriesProductChoicesAndDrops(t *testing.T) {
	envHas := func(env []string, want string) bool {
		for _, e := range env {
			if e == want {
				return true
			}
		}
		return false
	}
	envValue := func(env []string, key string) (string, bool) {
		prefix := key + "="
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				return strings.TrimPrefix(e, prefix), true
			}
		}
		return "", false
	}
	asSet := func(csv string) map[string]bool {
		set := map[string]bool{}
		for _, value := range strings.Split(csv, ",") {
			if value != "" {
				set[value] = true
			}
		}
		return set
	}

	m := newModel()
	m.diskDev, m.pwHash = "/dev/vda", "x"
	m.picks["disk"] = "whole"
	env := m.installEnv()
	if !envHas(env, "RYOKU_BROWSER=firefox") || !envHas(env, "RYOKU_LOGIN_SHELL=fish") {
		t.Fatalf("default browser/shell missing from %v", env)
	}
	drops, ok := envValue(env, "RYOKU_DROP_PACKAGES")
	wantDefault := "chromium,zen-browser-bin,zsh,zsh-autosuggestions,zsh-history-substring-search,zsh-syntax-highlighting,ryoku-oh-my-zsh,blesh"
	if !ok || drops != wantDefault {
		t.Fatalf("default drop list = %q (ok=%v), want %q", drops, ok, wantDefault)
	}

	for _, tc := range []struct {
		shell string
		drop  []string
		keep  []string
	}{
		{"fish", []string{"zsh", "zsh-autosuggestions", "zsh-history-substring-search", "zsh-syntax-highlighting", "ryoku-oh-my-zsh", "blesh"}, []string{"fish"}},
		{"zsh", []string{"fish", "blesh"}, []string{"zsh", "zsh-autosuggestions", "zsh-history-substring-search", "zsh-syntax-highlighting", "ryoku-oh-my-zsh"}},
		{"bash", []string{"fish", "zsh", "zsh-autosuggestions", "zsh-history-substring-search", "zsh-syntax-highlighting", "ryoku-oh-my-zsh"}, []string{"blesh"}},
	} {
		m.picks["login-shell"] = tc.shell
		env = m.installEnv()
		if !envHas(env, "RYOKU_LOGIN_SHELL="+tc.shell) {
			t.Errorf("%s: chosen shell did not reach env", tc.shell)
		}
		csv, exists := envValue(env, "RYOKU_DROP_PACKAGES")
		if !exists {
			t.Fatalf("%s: no drop list", tc.shell)
		}
		set := asSet(csv)
		for _, pkg := range tc.drop {
			if !set[pkg] {
				t.Errorf("%s: drop list %q missing %s", tc.shell, csv, pkg)
			}
		}
		for _, pkg := range tc.keep {
			if set[pkg] {
				t.Errorf("%s: selected stack package %s appears in %q", tc.shell, pkg, csv)
			}
		}
		if set["bash"] {
			t.Errorf("%s: bash itself must never be dropped", tc.shell)
		}
	}

	m.picks["browser"] = "chromium"
	m.picks["login-shell"] = "zsh"
	m.keep = appDefaults()
	m.keep["docker"] = false
	m.keep["flatpak"] = false
	env = m.installEnv()
	if !envHas(env, "RYOKU_BROWSER=chromium") {
		t.Error("the chosen browser did not reach the env")
	}
	csv, ok := envValue(env, "RYOKU_DROP_PACKAGES")
	if !ok {
		t.Fatal("deselections must emit a drop list")
	}
	set := asSet(csv)
	for _, want := range []string{"docker", "flatpak", "firefox", "zen-browser-bin", "fish", "blesh"} {
		if !set[want] {
			t.Errorf("drop list %q missing %s", csv, want)
		}
	}
	for _, keep := range []string{"chromium", "zsh", "zsh-autosuggestions", "zsh-history-substring-search", "zsh-syntax-highlighting", "ryoku-oh-my-zsh"} {
		if set[keep] {
			t.Errorf("chosen product package %s appears in %q", keep, csv)
		}
	}
}

func TestReviewNamesBrowserAndLoginShell(t *testing.T) {
	m := reviewModel()
	m.picks["browser"] = "chromium"
	m.picks["login-shell"] = "zsh"
	body := stripSGR(m.reviewBody(100))
	for _, want := range []string{"browser", "Chromium", "shell", "Zsh"} {
		if !strings.Contains(body, want) {
			t.Errorf("Review does not show %q:\n%s", want, body)
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
