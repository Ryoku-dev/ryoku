package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	installerplan "ryoku-installerplan"
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

func decodePlanKey(t *testing.T, sequence string) tea.KeyPressMsg {
	t.Helper()
	var decoder uv.EventDecoder
	n, event := decoder.Decode([]byte(sequence))
	if n != len(sequence) {
		t.Fatalf("decoded %d of %d bytes in %q", n, len(sequence), sequence)
	}
	key, ok := event.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("%q decoded as %T, want a key press", sequence, event)
	}
	return tea.KeyPressMsg(key)
}

func planModelForKeys() model {
	f := &facts{distro: archLinux, kbLayout: "us"}
	p := &plan{compositor: "hyprland", browser: "firefox", shell: "fish"}
	items := groupPlanItems(buildItems(f, p))
	return model{state: "plan", f: f, p: p, items: items, sel: firstToggle(items)}
}

func TestPlanArrowEncodingsVisitEveryRow(t *testing.T) {
	for name, arrows := range map[string]struct {
		down string
		up   string
	}{
		"normal cursor":      {down: "\x1b[B", up: "\x1b[A"},
		"application cursor": {down: "\x1bOB", up: "\x1bOA"},
	} {
		t.Run(name, func(t *testing.T) {
			m := planModelForKeys()
			var selectable []int
			for i, item := range m.items {
				if selectablePlanItem(item) {
					selectable = append(selectable, i)
				}
			}
			if m.sel != selectable[0] {
				t.Fatalf("initial focus = %d, want %d", m.sel, selectable[0])
			}
			for _, want := range selectable[1:] {
				next, _ := m.Update(decodePlanKey(t, arrows.down))
				m = next.(model)
				if m.sel != want {
					t.Fatalf("down focus = %d, want %d", m.sel, want)
				}
				if m.confirm {
					t.Fatal("an arrow key opened the install confirmation")
				}
			}
			for i := len(selectable) - 2; i >= 0; i-- {
				next, _ := m.Update(decodePlanKey(t, arrows.up))
				m = next.(model)
				if m.sel != selectable[i] {
					t.Fatalf("up focus = %d, want %d", m.sel, selectable[i])
				}
			}
		})
	}
}

func TestPlanArrowEncodingsChangeFocusedChoice(t *testing.T) {
	for name, arrows := range map[string]struct {
		right string
		left  string
	}{
		"normal cursor":      {right: "\x1b[C", left: "\x1b[D"},
		"application cursor": {right: "\x1bOC", left: "\x1bOD"},
	} {
		t.Run(name, func(t *testing.T) {
			m := planModelForKeys()
			next, _ := m.Update(decodePlanKey(t, "\x1b[B"))
			m = next.(model)
			if m.items[m.sel].label != "Browser" {
				t.Fatalf("focused row = %q, want Browser", m.items[m.sel].label)
			}
			next, _ = m.Update(decodePlanKey(t, arrows.right))
			m = next.(model)
			if m.p.browser != "chromium" {
				t.Fatalf("right selected %q, want chromium", m.p.browser)
			}
			next, _ = m.Update(decodePlanKey(t, arrows.left))
			m = next.(model)
			if m.p.browser != "firefox" {
				t.Fatalf("left selected %q, want firefox", m.p.browser)
			}
		})
	}
}

func TestPlanKittyArrowIgnoresLockState(t *testing.T) {
	m := planModelForKeys()
	next, _ := m.Update(decodePlanKey(t, "\x1b[57353;193u"))
	m = next.(model)
	if m.items[m.sel].label != "Browser" {
		t.Fatalf("Kitty down arrow focused %q, want Browser", m.items[m.sel].label)
	}
	if m.confirm {
		t.Fatal("Kitty arrow opened the install confirmation")
	}
}

func TestPlanIgnoresEnterTextThatIsNotTheEnterKey(t *testing.T) {
	m := planModelForKeys()
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "enter"}))
	if next.(model).confirm {
		t.Fatal("text named enter must not open the install confirmation")
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

func TestPlanExplainsFilteredDNS(t *testing.T) {
	f := &facts{distro: archLinux, filteredDNS: true}
	p := defaultPlan(f)
	got := model{
		w:     180,
		h:     40,
		f:     f,
		p:     p,
		items: groupPlanItems(buildItems(f, p)),
	}.viewPlan()
	for _, want := range []string{
		"DNS for ryoku.dev is being filtered on this network",
		"docs/updates.md#filtered-dns",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plan warning misses %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "repo.ryoku.dev is unreachable") {
		t.Fatalf("filtered DNS fell back to generic unreachable warning:\n%s", got)
	}
}

func TestVoidPlanShowsEditionNotice(t *testing.T) {
	data, err := os.ReadFile("../void/packages/translations.tsv")
	if err != nil {
		t.Fatal(err)
	}
	voidEdition, err := parseEditionCompositors(data)
	if err != nil {
		t.Fatal(err)
	}
	render := func(d *distro, btrfs bool, edition editionCompositorPlan) string {
		f := &facts{
			distro:     d,
			distroName: d.name,
			hostname:   "ryoku",
			currentDM:  "sddm",
			online:     true,
			btrfsRoot:  btrfs,
		}
		p := defaultPlanForEdition(f, edition)
		return model{
			w: 100, h: 40,
			f: f, p: p,
			items:              groupPlanItems(buildItems(f, p, edition)),
			editionCompositors: edition,
		}.viewPlan()
	}

	const updateNotice = "Update the base system with ryoku update --system so it is snapshotted; XBPS has no update hooks."
	notice := voidEditionNotice(&facts{distro: voidLinux, btrfsRoot: true}, voidEdition)
	noticeText := strings.Join(notice, "\n")
	if !strings.Contains(noticeText, updateNotice) {
		t.Fatalf("Void notice text does not contain %q: %v", updateNotice, notice)
	}
	for _, want := range []string{
		"Hyprland unavailable: Ryoku's Hyprland needs GCC 15 and Void still ships GCC 14.",
		"Zen unavailable: Zen ships only as an upstream binary build; Ryoku does not mirror a browser it cannot keep current",
	} {
		if !strings.Contains(noticeText, want) {
			t.Errorf("Void notice missing table reason %q", want)
		}
	}
	voidPlan := render(voidLinux, true, voidEdition)
	for _, want := range []string{
		"Compositors: niri.",
		"Hyprland unavailable:",
		"Zen unavailable:",
		"snapshot rollback: ready",
		"ryoku update --system",
		"snapshotted",
	} {
		if !strings.Contains(voidPlan, want) {
			t.Errorf("Void btrfs plan missing %q", want)
		}
	}
	if got := strings.Count(voidPlan, "Ryoku on Void"); got != 1 {
		t.Errorf("Void plan notice count = %d, want 1", got)
	}
	nonBtrfsPlan := render(voidLinux, false, voidEdition)
	if strings.Contains(nonBtrfsPlan, "Update the base system with ryoku update --system") || !strings.Contains(nonBtrfsPlan, "snapshot rollback: unavailable") {
		t.Fatalf("Void non-btrfs plan has the wrong snapshot guidance:\n%s", nonBtrfsPlan)
	}
	if strings.Contains(voidPlan, "No snapshots or rollback are available.") {
		t.Fatal("Void plan still says snapshots are unavailable")
	}
	archPlan := render(archLinux, true, sourceEditionChoices("Arch"))
	if strings.Contains(archPlan, "Ryoku on Void") || !strings.Contains(archPlan, "snapshot rollback: ready") {
		t.Fatal("Arch plan changed while routing the Void edition notice")
	}
}

func TestExperimentalEditionAndCompositorChoices(t *testing.T) {
	loadEdition := func(name string) editionCompositorPlan {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", name, "packages", "translations.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		edition, err := parseEditionCompositors(data)
		if err != nil {
			t.Fatal(err)
		}
		return edition
	}
	voidNotice := strings.Join(editionNotice(&facts{distro: voidLinux}, loadEdition("void")), "\n")
	if got := strings.Count(voidNotice, "experimental edition"); got != 1 {
		t.Fatalf("Void experimental label count = %d, want 1", got)
	}
	fedoraNotice := strings.Join(editionNotice(&facts{distro: fedoraLinux}, loadEdition("fedora")), "\n")
	if got := strings.Count(fedoraNotice, "experimental edition"); got != 1 {
		t.Fatalf("Fedora experimental label count = %d, want 1", got)
	}

	items := buildItems(&facts{}, &plan{}, sourceEditionChoices("Arch"))
	for _, item := range items {
		if item.label != "Compositor" {
			continue
		}
		keys := make([]string, 0, len(item.options))
		for _, option := range item.options {
			keys = append(keys, option.key)
		}
		if got := strings.Join(keys, ","); got != "hyprland,niri" {
			t.Fatalf("Arch compositor options = %q, want hyprland,niri", got)
		}
		return
	}
	t.Fatal("Compositor choice missing")
}

func TestEditionBrowserChoicesFollowPackageTables(t *testing.T) {
	browserKeys := func(items []planItem) []string {
		for _, item := range items {
			if item.label != "Browser" {
				continue
			}
			keys := make([]string, 0, len(item.options))
			for _, option := range item.options {
				keys = append(keys, option.key)
			}
			return keys
		}
		return nil
	}
	loadUnavailableZenFixture := func(edition string) editionCompositorPlan {
		t.Helper()
		data, _ := unavailableZenTable(t, edition)
		plan, err := parseEditionCompositors(data)
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	p := &plan{compositor: "hyprland", browser: "firefox", shell: "fish"}
	for name, test := range map[string]struct {
		facts   *facts
		edition editionCompositorPlan
		want    string
	}{
		"Fedora fixture": {
			facts:   &facts{distro: fedoraLinux},
			edition: loadUnavailableZenFixture("fedora"),
			want:    "firefox,chromium,none",
		},
		"Void fixture": {
			facts:   &facts{distro: voidLinux},
			edition: loadUnavailableZenFixture("void"),
			want:    "firefox,chromium,none",
		},
		"Arch": {
			facts: &facts{distro: archLinux},
			want:  "firefox,chromium,zen,none",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var items []planItem
			if len(test.edition.choices.AvailableIDs(installerplan.Compositor)) == 0 {
				items = buildItems(test.facts, p)
			} else {
				items = buildItems(test.facts, p, test.edition)
			}
			if got := strings.Join(browserKeys(items), ","); got != test.want {
				t.Fatalf("browser choices = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFedoraPlanNeverClaimsSnapshotRollback(t *testing.T) {
	f := &facts{
		distro:     fedoraLinux,
		distroName: "Fedora Linux 44",
		hostname:   "fedora",
		currentDM:  "gdm.service",
		btrfsRoot:  true,
		online:     true,
	}
	data, err := os.ReadFile("../fedora/packages/translations.tsv")
	if err != nil {
		t.Fatal(err)
	}
	edition, err := parseEditionCompositors(data)
	if err != nil {
		t.Fatal(err)
	}
	p := defaultPlan(f, edition.choices.AvailableIDs(installerplan.Compositor))
	view := model{
		w: 100, h: 40,
		f: f, p: p,
		items:              groupPlanItems(buildItems(f, p, edition)),
		editionCompositors: edition,
	}.viewPlan()
	if !strings.Contains(view, "snapshot rollback: unavailable") {
		t.Fatalf("Fedora plan did not report snapshot rollback unavailable:\n%s", view)
	}
	if strings.Contains(view, "snapshot rollback: ready") {
		t.Fatalf("Fedora plan falsely reported snapshot rollback ready:\n%s", view)
	}
}

func TestFedoraPlanKeepsNouveauAndSkipsAUR(t *testing.T) {
	f := &facts{distro: fedoraLinux, hasNvidia: true}
	p := defaultPlan(f)
	if p.nvidia {
		t.Fatal("Fedora must not schedule proprietary NVIDIA driver changes")
	}
	if p.aur {
		t.Fatal("Fedora must not schedule the Arch-only AUR step")
	}
	for _, item := range buildItems(f, p) {
		if item.label == "NVIDIA proprietary drivers" || item.label == "AUR extras" {
			t.Fatalf("Fedora plan exposed Arch-only item %q", item.label)
		}
	}
}

func TestScreensFitMinimumTerminal(t *testing.T) {
	f := &facts{
		distroName:  "Arch Linux",
		hostname:    "ryoku",
		currentDM:   "sddm",
		online:      true,
		btrfsRoot:   true,
		hasNvidia:   true,
		otherNet:    []string{"iwd.service"},
		rivalPkgs:   []string{"other-shell"},
		softUnits:   []string{"waybar.service"},
		omarchyRepo: true,
		monOutputs:  []niriOutput{{}},
		monSource:   "saved setup",
		kbLayout:    "us",
	}
	p := defaultPlan(f)
	items := groupPlanItems(buildItems(f, p))
	steps := make([]estep, 15)
	for i := range steps {
		steps[i] = estep{title: "Installing a bounded step title"}
	}
	e := &engine{
		f: f, p: p, steps: steps,
		backupDir:   "/home/test/.local/state/ryoku/shell-install/backup-20261006-230000",
		logPath:     "/home/test/.local/state/ryoku/shell-install.log",
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
