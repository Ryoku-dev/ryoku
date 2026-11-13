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
	// Zen is an AUR build and not offered on Void; None is, since it installs
	// nothing for an edition table to withhold.
	if len(browser.items) != 3 {
		t.Fatalf("Void browsers = %v, want firefox, chromium and none", browser.items)
	}
	if got := []string{browser.items[0].key, browser.items[1].key, browser.items[2].key}; !reflect.DeepEqual(got, []string{"firefox", "chromium", "none"}) {
		t.Fatalf("Void browser keys = %v, want firefox, chromium and none", got)
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
	if !strings.Contains(compositor.items[0].hint, "GCC 15") {
		t.Fatalf("disabled compositor has no package-table reason: %q", compositor.items[0].hint)
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
	text := strings.Join(notice.desc, "\n")
	for _, want := range []string{
		"This Void edition is experimental.",
		"Available window managers: niri.",
		"Use ryoku update --system for base-system updates.",
		"XBPS has no update hooks; this keeps updates snapshotted.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Void notice text does not contain %q: %v", want, notice.desc)
		}
	}
	for _, shipped := range []string{"gpk", "oh-my-zsh", "dualsensectl", "ryotunes"} {
		if strings.Contains(strings.ToLower(text), shipped) {
			t.Fatalf("Void notice still lists shipped package %q as unavailable: %v", shipped, notice.desc)
		}
	}
	m := model{variant: variantVoid, flow: voidFlow, w: 112, h: 42, state: "wizard", enterPos: 1}
	rendered := strings.ToLower(stripSGR(m.viewWizard()))
	for _, want := range []string{"experimental", "niri", "ryoku update --system", "snapshotted", "xbps has no update hooks"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Void notice is missing %q:\n%s", want, rendered)
		}
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

func TestFirmwareGateKeepsArchBlockedAndAllowsVoidWholeDisk(t *testing.T) {
	hardwareStep := func(variant string) model {
		flow := stepsForVariant(variant)
		return model{
			variant: variant,
			flow:    flow,
			idx:     flowIndex(flow, "hardware"),
			state:   "wizard",
			hwBIOS:  true,
			picks:   map[string]string{},
		}
	}

	arch := hardwareStep("plain")
	next, _ := arch.onKey("enter")
	if got := next.(model); got.idx != arch.idx {
		t.Fatalf("Arch BIOS advanced from hardware step %d to %d", arch.idx, got.idx)
	}

	void := hardwareStep(variantVoid)
	next, _ = void.onKey("enter")
	if got := next.(model); got.idx <= void.idx {
		t.Fatalf("Void BIOS did not advance from hardware step %d", void.idx)
	}
	body := stripSGR(void.infoBody(100))
	for _, want := range []string{"Legacy BIOS mode detected", "whole disk", "Alongside installs need UEFI and GPT"} {
		if !strings.Contains(body, want) {
			t.Fatalf("Void BIOS hardware note is missing %q:\n%s", want, body)
		}
	}

	choices, disabled := void.diskStrategyChoices(diskLayout{
		parts: []part{{dev: "/dev/vda1", size: 20}},
		gpt:   true,
	})
	if len(choices) != 2 || choices[0].key != "whole" || choices[1].key != "alongside" {
		t.Fatalf("Void BIOS strategy choices = %+v, want whole then visible alongside", choices)
	}
	if disabled["whole"] || !disabled["alongside"] {
		t.Fatalf("Void BIOS disabled strategies = %v, want only alongside disabled", disabled)
	}
	if !strings.Contains(choices[1].hint, "needs UEFI and GPT") {
		t.Fatalf("Void BIOS alongside reason = %q", choices[1].hint)
	}
	if env := void.installEnv(); !envHas(env, "RYOKU_FIRMWARE_MODE=bios") {
		t.Fatalf("Void BIOS firmware mode did not reach backend env: %v", env)
	}
	for _, value := range arch.installEnv() {
		if strings.HasPrefix(value, "RYOKU_FIRMWARE_MODE=") {
			t.Fatalf("Arch firmware handoff changed: %q", value)
		}
	}
}

func TestReviewShowsFirmwareMode(t *testing.T) {
	m := reviewModel()
	m.variant, m.hwBIOS = variantVoid, true
	if body := stripSGR(m.reviewBody(100)); !strings.Contains(body, "firmware") || !strings.Contains(body, "Legacy BIOS") {
		t.Fatalf("Review does not show legacy BIOS mode:\n%s", body)
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
		if row.ID == "localsend" || strings.Contains(strings.ToUpper(row.Sub), "AUR") {
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

func TestCachyOSLayoutMatchesCachyOSInstaller(t *testing.T) {
	cachy := model{variant: variantCachyOS, picks: map[string]string{"disk": "whole"}}
	cachy.resetLayoutChoices()
	if cachy.espG != 4 || cachy.swapG != 0 {
		t.Fatalf("CachyOS layout defaults esp=%dG swap=%dG, want esp=4G swap=0G", cachy.espG, cachy.swapG)
	}
	if _, _, maxESP, _, _ := cachy.rowSpec("esp"); cachy.espG > maxESP {
		t.Fatalf("CachyOS ESP default %dG exceeds the ESP row maximum %dG", cachy.espG, maxESP)
	}

	arch := model{variant: "plain", picks: map[string]string{"disk": "whole"}}
	arch.resetLayoutChoices()
	if arch.espG != 1 || arch.swapG != 16 {
		t.Fatalf("Arch layout defaults changed: esp=%dG swap=%dG", arch.espG, arch.swapG)
	}
}

func TestCachyOSZenHintNamesTheRepo(t *testing.T) {
	hintOf := func(variant string) string {
		for _, b := range browsersForVariant(variant) {
			if b.key == "zen" {
				return b.hint
			}
		}
		t.Fatalf("%s edition offers no Zen browser", variant)
		return ""
	}
	if got := hintOf(variantCachyOS); strings.Contains(got, "AUR") || !strings.Contains(got, "CachyOS repo") {
		t.Fatalf("CachyOS Zen hint = %q, want the CachyOS repo", got)
	}
	if got := hintOf("plain"); !strings.Contains(got, "AUR") {
		t.Fatalf("Arch Zen hint = %q, want the AUR build", got)
	}
}

func TestLocaleAListUsesSupportedSpelling(t *testing.T) {
	got := utf8Locales("C\nC.utf8\nPOSIX\nde_DE.utf8\nen_US.utf8\nja_JP.eucjp\nsr_RS.utf8@latin\nsr_RS@latin\n")
	want := []string{"C.UTF-8", "de_DE.UTF-8", "en_US.UTF-8", "sr_RS.UTF-8@latin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("utf8Locales = %v, want %v", got, want)
	}
}
