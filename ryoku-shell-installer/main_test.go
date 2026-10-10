package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGroupPlanItems(t *testing.T) {
	on := true
	var items []planItem
	for _, l := range []string{
		"Resume the previous run",
		"NVIDIA proprietary drivers", "Switch login to SDDM", "Ryoku greeter theme",
		"Switch to NetworkManager", "Remove rival shells", "Disable conflicting daemons",
		"Retire the Omarchy repo", "Carry over monitor layout",
		"AUR extras", "Developer toolchain",
	} {
		items = append(items, planItem{label: l, on: &on})
	}
	got := groupPlanItems(items)
	headers := 0
	for _, it := range got {
		if it.on == nil {
			headers++
		}
	}
	if headers != 3 {
		t.Fatalf("want 3 section headers over %d toggles, got %d", len(items), headers)
	}
	if got[0].on == nil {
		t.Fatal("the resume row stays on top, before any header")
	}
	if firstToggle(got) != 0 {
		t.Fatalf("first toggle should be the resume row, got %d", firstToggle(got))
	}

	short := groupPlanItems(items[:6])
	if len(short) != 6 {
		t.Fatalf("short plans stay flat, got %d rows", len(short))
	}
}

func TestAzertyPlanItems(t *testing.T) {
	find := func(items []planItem, label string) *planItem {
		for i := range items {
			if items[i].label == label {
				return &items[i]
			}
		}
		return nil
	}
	p := &plan{}
	items := buildItems(&facts{kbLayout: "us"}, p)
	fr := find(items, "AZERTY keyboard (French)")
	be := find(items, "AZERTY keyboard (Belgian)")
	if fr == nil || be == nil {
		t.Fatal("AZERTY toggles missing on a plain us layout")
	}
	if fr.on != &p.azertyFR || be.on != &p.azertyBE {
		t.Fatal("AZERTY toggles wired to the wrong plan fields")
	}
	if find(buildItems(&facts{kbLayout: "de"}, &plan{}), "AZERTY keyboard (French)") != nil {
		t.Fatal("AZERTY toggles must stay hidden when a layout was salvaged")
	}
	if find(buildItems(&facts{}, &plan{}), "AZERTY keyboard (Belgian)") == nil {
		t.Fatal("AZERTY toggles missing when no layout was salvaged")
	}

	// the two choices are exclusive: the one just switched on wins.
	*fr.on = true
	p.azertyExclusive(fr.on)
	*be.on = true
	p.azertyExclusive(be.on)
	if p.azertyFR || !p.azertyBE {
		t.Fatalf("want fr=false be=true after toggling both, got fr=%v be=%v", p.azertyFR, p.azertyBE)
	}
	*be.on = false
	p.azertyExclusive(be.on)
	if p.azertyFR || p.azertyBE {
		t.Fatal("switching a toggle off must not resurrect the other")
	}
}

func TestCleanTermLine(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "installing gpk\n", "installing gpk"},
		{"curl progress keeps the last repaint", "  0     0    0\r 12 12.2M   12  1.54M\n", " 12 12.2M   12  1.54M"},
		{"trailing crlf", "done\r\n", "done"},
		{"tabs spaced", "a\tb", "a  b"},
		{"control bytes dropped", "ok\x1b[1m!\x07", "ok[1m!"},
		{"only a bare repaint", "\r", ""},
	}
	for _, c := range cases {
		if got := cleanTermLine(c.in); got != c.want {
			t.Errorf("%s: cleanTermLine(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestVoidPlanShowsEditionNotice(t *testing.T) {
	render := func(d *distro, btrfs bool) string {
		f := &facts{
			distro:     d,
			distroName: d.name,
			hostname:   "ryoku",
			currentDM:  "sddm",
			online:     true,
			btrfsRoot:  btrfs,
		}
		p := defaultPlan(f)
		return model{
			w: 100, h: 40,
			f: f, p: p,
			items: groupPlanItems(buildItems(f, p)),
		}.viewPlan()
	}

	const updateNotice = "Update the base system with ryoku update --system so it is snapshotted; XBPS has no update hooks."
	notice := voidEditionNotice(&facts{distro: voidLinux, btrfsRoot: true})
	if !strings.Contains(strings.Join(notice, "\n"), updateNotice) {
		t.Fatalf("Void notice text does not contain %q: %v", updateNotice, notice)
	}
	voidPlan := render(voidLinux, true)
	for _, want := range []string{"niri only", "Unavailable: Zen Browser", "snapshot rollback: ready", "ryoku update --system", "snapshotted"} {
		if !strings.Contains(voidPlan, want) {
			t.Errorf("Void btrfs plan missing %q", want)
		}
	}
	if got := strings.Count(voidPlan, "Ryoku on Void"); got != 1 {
		t.Errorf("Void plan notice count = %d, want 1", got)
	}
	nonBtrfsPlan := render(voidLinux, false)
	if strings.Contains(nonBtrfsPlan, "Update the base system with ryoku update --system") || !strings.Contains(nonBtrfsPlan, "snapshot rollback: unavailable") {
		t.Fatalf("Void non-btrfs plan has the wrong snapshot guidance:\n%s", nonBtrfsPlan)
	}
	if strings.Contains(voidPlan, "No snapshots or rollback are available.") {
		t.Fatal("Void plan still says snapshots are unavailable")
	}
	archPlan := render(archLinux, true)
	if strings.Contains(archPlan, "Ryoku on Void") || !strings.Contains(archPlan, "snapshot rollback: ready") {
		t.Fatal("Arch plan changed while routing the Void edition notice")
	}
}

func TestScreensFitMinimumTerminal(t *testing.T) {
	f := &facts{
		distroName: "Arch Linux",
		hostname: "ryoku",
		currentDM: "sddm",
		online: true,
		btrfsRoot: true,
		hasNvidia: true,
		otherNet: []string{"iwd.service"},
		rivalPkgs: []string{"other-shell"},
		softUnits: []string{"waybar.service"},
		omarchyRepo: true,
		monOutputs: []niriOutput{{}},
		monSource: "saved setup",
		kbLayout: "us",
	}
	p := defaultPlan(f)
	items := groupPlanItems(buildItems(f, p))
	steps := make([]estep, 15)
	for i := range steps {
		steps[i] = estep{title: "Installing a bounded step title"}
	}
	e := &engine{
		f: f, p: p, steps: steps,
		backupDir: "/home/test/.local/state/ryoku/shell-install/backup-20261006-230000",
		logPath: "/home/test/.local/state/ryoku/shell-install.log",
		prevBackups: 1,
	}
	base := model{
		w: minTermW, h: minTermH,
		f: f, p: p, items: items, eng: e,
		stepIdx: 7, failIdx: 7, failMsg: "the package transaction failed",
		logTail: []string{"one bounded log line", "two bounded log lines", "three bounded log lines", "four bounded log lines", "five bounded log lines", "six bounded log lines", "seven bounded log lines"},
	}
	check := func(name, view string) {
		t.Helper()
		if width := lipgloss.Width(view); width > minTermW {
			t.Errorf("%s width = %d, want <= %d", name, width, minTermW)
		}
		if height := lipgloss.Height(view); height > minTermH-2 {
			t.Errorf("%s height = %d, want <= %d with footer room", name, height, minTermH-2)
		}
	}
	check("scan", base.viewScan())
	check("ack", base.viewAck())
	check("install", base.viewInstall())
	check("done", base.viewDone())
	check("failed", base.viewFailed())
	for i, item := range items {
		if !selectablePlanItem(item) {
			continue
		}
		base.sel = i
		base.confirm = true
		check("plan", base.viewPlan())
	}
}
