package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func stepByKey(t *testing.T, flow []step, key string) step {
	t.Helper()
	i := flowIndex(flow, key)
	if i < 0 {
		t.Fatalf("step %q is missing", key)
	}
	return flow[i]
}

func TestVariantMarkerRoutesWizard(t *testing.T) {
	old := variantMarkerPath
	variantMarkerPath = filepath.Join(t.TempDir(), "variant")
	t.Cleanup(func() { variantMarkerPath = old })

	if err := os.WriteFile(variantMarkerPath, []byte("void\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := sysVariant(); got != variantVoid {
		t.Fatalf("sysVariant() = %q, want %q", got, variantVoid)
	}

	flow := steps()
	browser := stepByKey(t, flow, "browser")
	if len(browser.items) != 2 {
		t.Fatalf("Void browsers = %v, want firefox and chromium", browser.items)
	}
	if got := []string{browser.items[0].key, browser.items[1].key}; !reflect.DeepEqual(got, []string{"firefox", "chromium"}) {
		t.Fatalf("Void browser keys = %v, want firefox and chromium", got)
	}
	compositor := stepByKey(t, flow, "compositor")
	m := model{variant: variantVoid, flow: flow, idx: flowIndex(flow, "compositor"), picks: map[string]string{"compositor": wm.ProviderNiri}}
	m.loadStep()
	if len(compositor.items) != 2 || !m.pick.disabled[wm.ProviderHyprland] {
		t.Fatalf("Void compositor choices = %v, disabled = %v", compositor.items, m.pick.disabled)
	}
	selected := m.pick.items[m.pick.matches[m.pick.cursor]].key
	if selected != wm.ProviderNiri {
		t.Fatalf("Void compositor default = %q, want %q", selected, wm.ProviderNiri)
	}
	if !strings.Contains(compositor.items[0].hint, "not packaged") {
		t.Fatalf("disabled compositor has no reason: %q", compositor.items[0].hint)
	}
}

func TestVoidNoticeFollowsWelcome(t *testing.T) {
	t.Setenv("RYOKU_KB_PRESET", "")
	keys := func(flow []step) []string {
		out := make([]string, len(flow))
		for i, s := range flow {
			out[i] = s.key
		}
		return out
	}

	defaultFlow := stepsForVariant("plain")
	voidFlow := stepsForVariant(variantVoid)
	defaultKeys := keys(defaultFlow)
	voidKeys := keys(voidFlow)
	if flowIndex(defaultFlow, "void-notice") >= 0 {
		t.Fatalf("default flow includes the Void notice: %v", defaultKeys)
	}
	if len(voidKeys) != len(defaultKeys)+1 || voidKeys[0] != "void-notice" || !reflect.DeepEqual(voidKeys[1:], defaultKeys) {
		t.Fatalf("Void flow = %v, want void-notice followed by %v", voidKeys, defaultKeys)
	}

	notice := stepByKey(t, voidFlow, "void-notice")
	const updateNotice = "Update the base system with ryoku update --system so it is snapshotted; XBPS has no update hooks."
	if !strings.Contains(strings.Join(notice.desc, "\n"), updateNotice) {
		t.Fatalf("Void notice text does not contain %q: %v", updateNotice, notice.desc)
	}
	m := model{variant: variantVoid, flow: voidFlow, w: 112, h: 42, state: "wizard", enterPos: 1}
	rendered := strings.ToLower(stripSGR(m.viewWizard()))
	for _, want := range []string{"niri", "ryoku update --system", "snapshotted", "xbps has no update hooks", "aur-only"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Void notice is missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "no snapshots") {
		t.Fatalf("Void notice still says snapshots are unavailable:\n%s", rendered)
	}
	m.back()
	if m.state != "welcome" {
		t.Fatalf("back from Void notice entered %q, want welcome", m.state)
	}
	m.state = "wizard"
	m.advance()
	if got := m.cur().key; got != defaultKeys[0] {
		t.Fatalf("continue from Void notice entered %q, want %q", got, defaultKeys[0])
	}
}

func TestWelcomeFrameNamesLiveVariant(t *testing.T) {
	render := func(variant string) string {
		m := model{variant: variant, w: 112, h: 42}
		return stripSGR(m.welcomeFrame())
	}
	arch := render("")
	if !strings.Contains(arch, "the live Arch image now, and this installer turns it into") || strings.Contains(arch, "the live Void image") {
		t.Fatalf("default welcome does not retain Arch wording:\n%s", arch)
	}
	void := render(variantVoid)
	if !strings.Contains(void, "the live Void image now, and this installer turns it into") || strings.Contains(void, "the live Arch image") {
		t.Fatalf("Void welcome does not use Void wording:\n%s", void)
	}
}

func TestVoidOffersOnlyPackagedProductChoices(t *testing.T) {
	flow := stepsForVariant(variantVoid)
	if got, want := stepByKey(t, flow, "profile").items, stepByKey(t, stepsForVariant("plain"), "profile").items; !reflect.DeepEqual(got, want) {
		t.Fatalf("Void hardware profiles changed: got %v, want %v", got, want)
	}
	shells := stepByKey(t, flow, "login-shell").items
	var shellKeys []string
	for _, shell := range shells {
		shellKeys = append(shellKeys, shell.key)
	}
	if want := []string{"fish", "zsh", "bash"}; !reflect.DeepEqual(shellKeys, want) {
		t.Fatalf("Void login shells = %v, want %v", shellKeys, want)
	}

	for _, s := range flow {
		switch s.key {
		case "kernel", "variant", "aur":
			t.Errorf("Void flow includes Arch-only step %q", s.key)
		}
		for _, item := range s.items {
			if strings.Contains(strings.ToUpper(item.hint), "AUR") {
				t.Errorf("Void step %q offers AUR choice %q", s.key, item.key)
			}
		}
	}
	for _, row := range appRowsForVariant(variantVoid) {
		if row.ID == "localsend" || row.ID == "voxtype" || strings.Contains(strings.ToUpper(row.Sub), "AUR") {
			t.Errorf("Void app picker offers unavailable row %+v", row)
		}
	}
}

func TestVoidUsesSnapshotsAndKeepsArchDefaults(t *testing.T) {
	voidModel := model{variant: variantVoid, picks: map[string]string{"disk": "whole"}}
	voidModel.resetLayoutChoices()
	if !voidModel.snapshots {
		t.Fatal("Void layout does not enable snapshots by default")
	}
	var voidSnapshotRow *lrow
	for _, row := range voidModel.layoutRows() {
		if row.key == "snap" {
			copy := row
			voidSnapshotRow = &copy
			break
		}
	}
	if voidSnapshotRow == nil {
		t.Fatal("Void layout does not offer the snapshot toggle")
	}

	archModel := model{variant: "plain", picks: map[string]string{"disk": "whole"}}
	archModel.resetLayoutChoices()
	if !archModel.snapshots {
		t.Fatal("Arch snapshot default changed")
	}
	var archSnapshotRow *lrow
	for _, row := range archModel.layoutRows() {
		if row.key == "snap" {
			copy := row
			archSnapshotRow = &copy
			break
		}
	}
	if archSnapshotRow == nil || !reflect.DeepEqual(*voidSnapshotRow, *archSnapshotRow) {
		t.Fatalf("Void snapshot row = %+v, want Arch row %+v", voidSnapshotRow, archSnapshotRow)
	}
	if !reflect.DeepEqual(browsersForVariant("plain"), browsers()) || !reflect.DeepEqual(compositorsForVariant("plain"), compositors()) || !reflect.DeepEqual(appRowsForVariant("plain"), appRows()) {
		t.Fatal("Arch choices changed while routing the Void variant")
	}

	picks := map[string]string{"hostname": "ryoku", "username": "you", "profile": "desktop", "encryption": "off", "timezone": "UTC"}
	voidDone := stripSGR((model{variant: variantVoid, picks: picks}).viewDone())
	archDone := stripSGR((model{variant: "plain", picks: picks}).viewDone())
	wantNext := "snapshots and rollback from the Limine boot menu"
	if !strings.Contains(voidDone, wantNext) || !strings.Contains(archDone, wantNext) {
		t.Fatalf("done summary diverged: Void=%q Arch=%q", voidDone, archDone)
	}
}

func TestVoidOfflineRepositoryHandoff(t *testing.T) {
	oldArch, oldVoid := archOfflineRepoPath, voidOfflineRepoPath
	archOfflineRepoPath = filepath.Join(t.TempDir(), "arch")
	voidOfflineRepoPath = filepath.Join(t.TempDir(), "void")
	t.Cleanup(func() {
		archOfflineRepoPath, voidOfflineRepoPath = oldArch, oldVoid
	})
	if err := os.MkdirAll(voidOfflineRepoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(voidOfflineRepoPath, "x86_64-repodata"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := model{variant: variantVoid, diskDev: "/dev/vda", pwHash: "x", snapshots: true, sepHome: true, picks: map[string]string{
		"disk": "whole", "compositor": wm.ProviderNiri, "browser": "firefox", "login-shell": "fish",
	}}
	if envHas(m.installEnv(), "RYOKU_ONLINE=0") {
		t.Fatal("an index without any XBPS packages was accepted as an offline repository")
	}
	if err := os.WriteFile(filepath.Join(voidOfflineRepoPath, "ryoku-desktop-1_1.x86_64.xbps"), []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := m.installEnv()
	if !envHas(env, "RYOKU_ONLINE=0") || !envHas(env, "RYOKU_OFFLINE_REPO="+voidOfflineRepoPath) {
		t.Fatalf("Void offline repository did not reach backend env: %v", env)
	}
	if !envHas(env, "RYOKU_SUBVOL_SNAPSHOTS=1") {
		t.Fatalf("Void snapshot default did not reach backend env: %v", env)
	}
	if err := os.MkdirAll(archOfflineRepoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archOfflineRepoPath, "offline.db"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	archModel := m
	archModel.variant = "plain"
	archEnv := archModel.installEnv()
	if !envHas(archEnv, "RYOKU_ONLINE=0") || !envHas(archEnv, "RYOKU_OFFLINE_REPO="+archOfflineRepoPath) {
		t.Fatalf("Arch offline repository handoff changed: %v", archEnv)
	}
}
