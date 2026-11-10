package main

// ryoku-shell-install: put the Ryoku desktop on an existing Arch machine, no
// ISO. Interactive bubbletea TUI by default; --yes runs the same engine
// headless. --dry-run prints every command instead of running it.

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"ryoku-i18n"
	"ryoku-i18n/catalog"
	installerplan "ryoku-installerplan"
)

const minTermW, minTermH = 80, 24

// maxStepRetries bounds how many times the failed screen re-runs the same
// step: a deterministic failure (a broken script, a missing file) cannot be
// retried into success, and an unbounded retry loop hides that from the user.
const maxStepRetries = 3

type frameMsg time.Time
type scanMsg struct{ f *facts }

type choiceOption struct {
	key         string
	label       string
	detail      string
	recommended bool
}

type planItem struct {
	label   string
	detail  string
	group   string
	on      *bool
	locked  bool // shown but not toggleable (safety gate holds it)
	pick    *string
	options []choiceOption
}

type model struct {
	w, h  int
	frame int
	state string // scan, ack, plan, install, done, failed

	f        *facts
	p        *plan
	items    []planItem
	sel      int
	confirm  bool
	ackInput string // typed acknowledgement on gated distros (manjaro)

	eng     *engine
	events  chan any
	stepIdx int
	logTail []string
	// the tail line is a live progress repaint; the next one replaces it
	tailTransient bool
	failIdx       int
	failMsg       string
	failCount     int  // consecutive failures at failIdx; retry stays offered below the cap
	intAsk        bool // one ctrl+c pressed during install, awaiting the second

	dry                bool
	ref                string
	payload            string
	compositor         string // --compositor pick; "" installs the default variant
	overrides          planOverrides
	editionCompositors editionCompositorPlan
	exitReboot         bool
}

func newTUIModel(dry bool, ref, payload, compositor string, overrides planOverrides, editionCompositors editionCompositorPlan) model {
	return model{
		state: "scan", dry: dry, ref: ref, payload: payload, compositor: compositor,
		overrides: overrides, editionCompositors: editionCompositors,
	}
}

func (m model) tickCmd() tea.Cmd {
	d := 250 * time.Millisecond
	if m.state == "scan" || m.state == "install" {
		d = 90 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func scanCmd() tea.Msg { return scanMsg{f: detect()} }

func (m model) Init() tea.Cmd {
	return tea.Batch(m.tickCmd(), func() tea.Msg { return scanCmd() })
}

func (m model) waitEv() tea.Cmd {
	ch := m.events
	return func() tea.Msg { return <-ch }
}

func buildItems(f *facts, p *plan, editionPlans ...editionCompositorPlan) []planItem {
	// labels stay English here: groupPlanItems matches them against planGroups
	// to insert section headers, so they are compared, not just shown. viewPlan
	// translates them at the render site. Details are display-only -> wrapped.
	edition := sourceEditionChoices("Arch")
	if len(editionPlans) > 0 {
		edition = editionPlans[0]
	}
	p.edition = edition.choices
	for id, selected := range edition.choices.DefaultComponents() {
		if p.components[id] == nil {
			p.setComponent(id, selected)
		}
	}
	options := func(kind string) []choiceOption {
		available := edition.choices.Choices(kind)
		out := make([]choiceOption, 0, len(available))
		for _, item := range available {
			if !item.Available {
				continue
			}
			choice := item.Choice
			label := i18n.T(choice.Label)
			if choice.Experimental {
				label = i18n.Tf("%s (experimental)", label)
			}
			recommended := choice.ID == installerplan.FreshDefault(kind)
			if kind == installerplan.Compositor {
				recommended = len(out) == 0
			}
			out = append(out, choiceOption{
				key: choice.ID, label: label, detail: i18n.T(choice.ShellDetail), recommended: recommended,
			})
		}
		return out
	}
	var it = []planItem{
		{
			label:   i18n.T("Compositor"),
			pick:    &p.compositor,
			options: options(installerplan.Compositor),
		},
		{
			label:   i18n.T("Browser"),
			pick:    &p.browser,
			options: options(installerplan.Browser),
		},
		{
			label:   i18n.T("Login shell"),
			pick:    &p.shell,
			options: options(installerplan.LoginShell),
		},
		{
			label:   i18n.T("File manager"),
			pick:    &p.fileManager,
			options: options(installerplan.FileManager),
		},
	}
	if availability, ok := edition.choices.Component("rashin"); ok && availability.Available {
		it = append(it, planItem{
			label:  i18n.T("Rashin"),
			detail: i18n.T("Optional AI for asking about your system and running coding agents; off until you turn it on."),
			on:     p.components["rashin"], locked: availability.Locked,
		})
	}
	if availability, ok := edition.choices.GamingComponent(); ok {
		component := availability.Component
		detail := i18n.T(component.Detail)
		if component.ID == "cachyos-layer" {
			detail = i18n.T("requires a network connection and x86-64-v3; installs the CachyOS kernel, tuning, sched-ext, Steam and Proton-CachyOS; the Arch kernel stays as a boot fallback")
		}
		it = append(it, planItem{
			label: i18n.T(component.Label), detail: detail,
			on: p.components[component.ID], locked: availability.Locked,
		})
	}
	for _, availability := range edition.choices.Components() {
		component := availability.Component
		if !availability.Available || component.Step != installerplan.ComponentApps || component.Group == devToolchainGroup {
			continue
		}
		it = append(it, planItem{
			label: i18n.T(component.Label), detail: i18n.T(component.Detail),
			group: i18n.T(component.GroupLabel), on: p.components[component.ID], locked: availability.Locked,
		})
	}
	if f.prevRun != nil {
		it = append(it, planItem{
			label:  "Resume the previous run",
			detail: i18n.Tf("%d step(s) already finished last time; keeps that run's backup dir and skips them (toggle off to redo everything)", len(f.prevRun.Completed)),
			on:     &p.resume,
		})
	}
	if f.hasNvidia && (f.distro == nil || f.distro.id != "fedora") {
		d := i18n.T("installs the proprietary driver, blacklists nouveau, rebuilds the initramfs")
		if f.nouveauLive {
			d = i18n.T("the proprietary driver replaces nouveau after a reboot")
		}
		locked := false
		switch {
		case f.secureBoot && !f.sbctlSigned:
			d = i18n.T("held off: Secure Boot rejects unsigned DKMS modules (black screen at boot); sign with sbctl or disable Secure Boot, then re-run")
			locked = true
		case f.secureBoot && f.sbctlSigned:
			d += "; " + i18n.T("Secure Boot is on, sbctl found: make sure its hook signs DKMS modules")
		}
		it = append(it, planItem{label: "NVIDIA proprietary drivers", detail: d, on: &p.nvidia, locked: locked})
	}
	if dm := f.otherDM(); dm != "" {
		d := i18n.Tf("disables %s and enables SDDM (at reboot)", dm)
		if len(f.desktops) > 0 {
			d += "; " + i18n.Tf("%s stays installed and selectable at login", strings.Join(f.desktops, ", "))
		}
		it = append(it, planItem{label: "Switch login to SDDM", detail: d, on: &p.switchDM})
	} else if f.currentDM == "" {
		it = append(it, planItem{label: "Enable SDDM login", detail: i18n.T("no display manager found; toggle off to keep starting Hyprland by hand"), on: &p.switchDM})
	}
	gd := i18n.T("points the SDDM login screen at the Ryoku qylock greeter")
	if f.kdeSddmConf {
		gd = i18n.T("keeps KDE's current login theme by default; toggle on to select the Ryoku greeter instead")
	}
	it = append(it, planItem{label: "Ryoku greeter theme", detail: gd, on: &p.greeter})
	// only offered when nothing better was salvaged: an existing fr/be/de/...
	// setup already carries its own layout into keyboard.lua.
	if f.kbLayout == "" || f.kbLayout == "us" {
		it = append(it, planItem{
			label:  "AZERTY keyboard (French)",
			detail: i18n.T("sets layout fr for Hyprland, the console (KEYMAP=fr), and the SDDM login screen; turns the Belgian toggle off"),
			on:     &p.azertyFR,
		})
		it = append(it, planItem{
			label:  "AZERTY keyboard (Belgian)",
			detail: i18n.T("sets layout be for Hyprland, the console (KEYMAP=be-latin1), and the SDDM login screen; turns the French toggle off"),
			on:     &p.azertyBE,
		})
	}
	if len(f.otherNet) > 0 {
		it = append(it, planItem{label: "Switch to NetworkManager", detail: i18n.Tf("disables %s (at reboot)", strings.Join(f.otherNet, ", ")), on: &p.switchNet})
	}
	if len(f.rivalPkgs) > 0 {
		it = append(it, planItem{label: "Remove rival shells", detail: i18n.Tf("uninstalls %s", strings.Join(f.rivalPkgs, ", ")), on: &p.rivals})
	}
	if len(f.softUnits) > 0 {
		it = append(it, planItem{label: "Disable conflicting daemons", detail: i18n.Tf("disables %s", strings.Join(f.softUnits, ", ")), on: &p.softOff})
	}
	if f.omarchyRepo || f.omarchyMirror || len(f.omarchyGuards) > 0 {
		it = append(it, planItem{label: "Retire the Omarchy repo", detail: i18n.T("drops [omarchy] from pacman.conf, restores a standard Arch mirrorlist, removes omarchy-keyring, retires the pacman guard hook that blocks -Syu"), on: &p.omarchy})
	}
	if len(f.monOutputs) > 0 {
		it = append(it, planItem{label: "Carry over monitor layout", detail: i18n.Tf("pins %d output(s) from your %s setup (rotation, scale, position) into monitors_user.lua", len(f.monOutputs), f.monSource), on: &p.monPins})
	}
	// awww is retired: the wallpaper daemon is ryogami, a hard ryoku-desktop
	// depend the packages step pulls, not an AUR build.
	if f.distro == nil || f.distro.id != "fedora" {
		aurDetail := i18n.T("Bibata cursor and the selected AUR app components")
		if p.browser == "zen" {
			aurDetail = i18n.T("required for Zen; also builds the selected AUR app components and the Bibata cursor")
		}
		it = append(it, planItem{label: "AUR extras", detail: aurDetail, on: &p.aur, locked: p.browser == "zen"})
	}
	it = append(it, planItem{label: "Developer toolchain", detail: i18n.T("go, rust, node and python; Ryoku recovery uses go"), on: &p.devtools})
	return it
}

// section headers keep a crowded plan readable; below ~10 toggles the flat
// list reads fine and headers would only add noise.
var planGroups = []struct {
	title  string
	labels []string
}{
	{"session & hardware", []string{"NVIDIA proprietary drivers", "Switch login to SDDM", "Enable SDDM login", "Ryoku greeter theme", "AZERTY keyboard (French)", "AZERTY keyboard (Belgian)", "Switch to NetworkManager"}},
	{"migration & cleanup", []string{"Remove rival shells", "Disable conflicting daemons", "Retire the Omarchy repo", "Carry over monitor layout"}},
	{"extras", []string{"AUR extras", "Developer toolchain"}},
}

// groupPlanItems inserts non-selectable header rows between sections once the
// toggle list grows past ten entries.
func groupPlanItems(items []planItem) []planItem {
	if len(items) <= 10 {
		return items
	}
	group := map[string]string{}
	for _, g := range planGroups {
		for _, l := range g.labels {
			group[l] = g.title
		}
	}
	var out []planItem
	last := ""
	for _, it := range items {
		g := it.group
		if g == "" {
			g = group[it.label]
		}
		if g != "" && g != last {
			out = append(out, planItem{label: g})
			last = g
		}
		out = append(out, it)
	}
	return out
}

func selectablePlanItem(it planItem) bool {
	return it.on != nil || it.pick != nil
}

// firstToggle keeps its historical name, but choices are selectable rows too.
func firstToggle(items []planItem) int {
	for i, it := range items {
		if selectablePlanItem(it) {
			return i
		}
	}
	return 0
}

func cyclePlanChoice(it planItem, delta int) {
	if it.pick == nil || len(it.options) == 0 {
		return
	}
	at := 0
	for i, option := range it.options {
		if option.key == *it.pick {
			at = i
			break
		}
	}
	at = (at + delta + len(it.options)) % len(it.options)
	*it.pick = it.options[at].key
}

func (m *model) syncPlanConstraints() {
	if m.f != nil && m.f.distro != nil && m.f.distro.id == "fedora" {
		m.p.aur = false
	} else if m.p.browser == "zen" {
		m.p.aur = true
	}
	for i := range m.items {
		if m.items[i].label != "AUR extras" {
			continue
		}
		m.items[i].locked = m.p.browser == "zen"
		if m.items[i].locked {
			m.items[i].detail = i18n.T("required for Zen; also builds the selected AUR app components and the Bibata cursor")
		} else {
			m.items[i].detail = i18n.T("Bibata cursor and the selected AUR app components")
		}
	}
}

func (m *model) startInstall() tea.Cmd {
	m.eng = newEngine(m.f, m.p, m.dry, m.ref, m.payload)
	m.events = m.eng.runFrom(0)
	m.stepIdx, m.logTail = 0, nil
	m.state = "install"
	return tea.Batch(m.tickCmd(), m.waitEv())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case frameMsg:
		m.frame++
		if m.state == "scan" || m.state == "install" {
			return m, m.tickCmd()
		}
		return m, nil
	case scanMsg:
		m.f = msg.f
		m.p = defaultPlanForEdition(m.f, m.editionCompositors)
		if m.compositor != "" {
			m.p.compositor = m.compositor
		}
		applyPlanOverrides(m.p, m.overrides)
		if m.f.distro != nil && m.f.distro.id == "fedora" {
			m.p.aur = false
		}
		m.items = groupPlanItems(buildItems(m.f, m.p, m.editionCompositors))
		m.sel = firstToggle(m.items)
		if needsManjaroAck(m.f) {
			m.state = "ack"
		} else {
			m.state = "plan"
		}
		return m, nil
	case evStep:
		m.stepIdx = msg.idx
		m.tailTransient = false
		// full clear per step: anything a child wrote straight to /dev/tty (a
		// stray password prompt, a curses fragment) would otherwise sit over
		// the frame until that exact region happens to repaint.
		return m, tea.Batch(m.waitEv(), tea.ClearScreen)
	case evLine:
		if msg.transient && m.tailTransient && len(m.logTail) > 0 {
			m.logTail[len(m.logTail)-1] = msg.line
		} else {
			m.logTail = append(m.logTail, msg.line)
			if len(m.logTail) > 400 {
				m.logTail = m.logTail[len(m.logTail)-400:]
			}
		}
		m.tailTransient = msg.transient
		return m, m.waitEv()
	case evDone:
		if msg.err != nil {
			if msg.idx == m.failIdx {
				m.failCount++
			} else {
				m.failIdx, m.failCount = msg.idx, 1
			}
			m.failMsg = msg.err.Error()
			m.state = "failed"
		} else {
			m.state = "done"
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	activeMods := key.Mod &^ (tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock)
	plain := activeMods == 0
	textInput := activeMods == 0 || activeMods == tea.ModShift
	if activeMods == tea.ModCtrl && key.Code == 'c' {
		// quitting mid-install abandons a live root pacman transaction; make
		// that a deliberate double-press, not a slip.
		if m.state == "install" && !m.intAsk {
			m.intAsk = true
			return m, nil
		}
		return m, tea.Quit
	}
	if m.state == "install" && m.intAsk {
		m.intAsk = false
	}
	switch m.state {
	case "ack":
		switch {
		case plain && key.Code == tea.KeyEscape:
			return m, tea.Quit
		case plain && key.Code == tea.KeyEnter:
			if strings.EqualFold(strings.TrimSpace(m.ackInput), "manjaro") {
				m.state = "plan"
			}
		case plain && key.Code == tea.KeyBackspace:
			if len(m.ackInput) > 0 {
				m.ackInput = m.ackInput[:len(m.ackInput)-1]
			}
		case textInput && len(key.Text) == 1 && len(m.ackInput) < 16:
			m.ackInput += key.Text
		}
		return m, nil
	case "plan":
		if m.confirm {
			switch {
			case plain && key.Code == tea.KeyEnter || (textInput && strings.EqualFold(key.Text, "y")):
				m.confirm = false
				return m, m.startInstall()
			case plain && key.Code == tea.KeyEscape || (textInput && strings.EqualFold(key.Text, "n")):
				m.confirm = false
			}
			return m, nil
		}
		switch {
		case plain && key.Code == 'q':
			return m, tea.Quit
		case plain && (key.Code == 'j' || key.Code == tea.KeyDown):
			for i := m.sel + 1; i < len(m.items); i++ {
				if selectablePlanItem(m.items[i]) {
					m.sel = i
					break
				}
			}
		case plain && (key.Code == 'k' || key.Code == tea.KeyUp):
			for i := m.sel - 1; i >= 0; i-- {
				if selectablePlanItem(m.items[i]) {
					m.sel = i
					break
				}
			}
		case plain && (key.Code == 'h' || key.Code == tea.KeyLeft):
			cyclePlanChoice(m.items[m.sel], -1)
			m.syncPlanConstraints()
		case plain && (key.Code == 'l' || key.Code == tea.KeyRight):
			cyclePlanChoice(m.items[m.sel], 1)
			m.syncPlanConstraints()
		case plain && key.Code == tea.KeySpace:
			if len(m.items) > 0 && m.items[m.sel].on != nil && !m.items[m.sel].locked {
				*m.items[m.sel].on = !*m.items[m.sel].on
				m.p.azertyExclusive(m.items[m.sel].on)
			}
		case plain && key.Code == tea.KeyEnter:
			m.confirm = true
		}
	case "done":
		switch {
		case plain && key.Code == 'r':
			m.exitReboot = true
			return m, tea.Quit
		case plain && (key.Code == 'q' || key.Code == tea.KeyEnter):
			return m, tea.Quit
		}
	case "failed":
		switch {
		case plain && key.Code == 'r':
			if m.failCount >= maxStepRetries {
				return m, nil
			}
			m.events = m.eng.runFrom(m.failIdx)
			m.state = "install"
			return m, tea.Batch(m.tickCmd(), m.waitEv())
		case plain && key.Code == 'q':
			return m, tea.Quit
		}
	}
	return m, nil
}

// ---- views ----

func (m model) View() tea.View {
	if m.w == 0 {
		return tea.NewView("")
	}
	if m.w < minTermW || m.h < minTermH {
		msg := lipgloss.JoinVertical(lipgloss.Center,
			bold(cText, i18n.T("Please enlarge your terminal")), "",
			fg(cText, i18n.Tf("The Ryoku shell installer needs at least %d x %d.", minTermW, minTermH)),
			fg(cSub, i18n.Tf("Current size: %d x %d.", m.w, m.h)))
		v := tea.NewView(lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, msg))
		v.AltScreen, v.BackgroundColor, v.ForegroundColor = true, cBg, cText
		return v
	}
	var body string
	switch m.state {
	case "scan":
		body = m.viewScan()
	case "ack":
		body = m.viewAck()
	case "plan":
		body = m.viewPlan()
	case "install":
		body = m.viewInstall()
	case "done":
		body = m.viewDone()
	case "failed":
		body = m.viewFailed()
	}
	frame := lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Top, body)
	if foot := m.footer(); foot != "" {
		lines := strings.Split(frame, "\n")
		if len(lines) >= 2 {
			foot = truncW(foot, m.w-4)
			lines[len(lines)-2] = lipgloss.PlaceHorizontal(m.w, lipgloss.Center, foot)
		}
		frame = strings.Join(lines, "\n")
	}
	v := tea.NewView(frame)
	v.AltScreen = true
	v.BackgroundColor = cBg
	v.ForegroundColor = cText
	v.WindowTitle = i18n.T("Ryoku shell installer")
	return v
}

func (m model) footer() string {
	switch m.state {
	case "ack":
		return keyHint(i18n.T("type manjaro + enter"), i18n.T("accept the risk")) + hintSep() + keyHint("esc", i18n.T("quit"))
	case "plan":
		if m.confirm {
			return keyHint("y", i18n.T("install")) + hintSep() + keyHint("n", i18n.T("back"))
		}
		hints := keyHint("up/down", i18n.T("move"))
		if len(m.items) > 0 && m.items[m.sel].pick != nil {
			hints += hintSep() + keyHint("left/right", i18n.T("choose"))
		} else {
			hints += hintSep() + keyHint("space", i18n.T("toggle"))
		}
		return hints + hintSep() + keyHint("enter", i18n.T("install")) + hintSep() + keyHint("q", i18n.T("quit"))
	case "install":
		if m.intAsk {
			return bold(cText, i18n.T("a package transaction may be running; press ctrl+c again to abandon"))
		}
		return fg(cDim, i18n.T("installing, do not interrupt")) + hintSep() + fg(cDim, i18n.T("a bounded live log is shown above"))
	case "done":
		return keyHint("r", i18n.T("reboot now")) + hintSep() + keyHint("q", i18n.T("quit"))
	case "failed":
		if m.failCount >= maxStepRetries {
			return fg(cDim, i18n.Tf("this step failed %d times; fix the cause or roll back", m.failCount)) + hintSep() + keyHint("q", i18n.T("quit"))
		}
		return keyHint("r", i18n.T("retry failed step")) + hintSep() + keyHint("q", i18n.T("quit"))
	}
	return ""
}

func (m model) logPath() string {
	if m.eng != nil {
		return m.eng.logPath
	}
	return ""
}

func (m model) displayPath(path string) string {
	if m.f != nil && m.f.homeDir != "" {
		if rel, ok := strings.CutPrefix(path, m.f.homeDir); ok {
			return "~" + rel
		}
	}
	return path
}

func (m model) header(sub string) string {
	tag := banner(0)
	if m.dry {
		tag += fg(cSub, "  "+i18n.T("[dry run]"))
	}
	iw := clamp(m.w-8, 64, 104)
	return leftBlock(iw, tag+"\n"+fg(cLine, strings.Repeat(ruleCh(), iw))+"\n"+sub)
}

func (m model) viewScan() string {
	sp := spinFrames[m.frame%len(spinFrames)]
	return m.header("\n" + fg(cText, sp+"  "+i18n.T("Inspecting this machine")))
}

// viewAck is the Manjaro gate: a hard warning that must be typed through.
// The repository follows Arch current, so explicit consent prevents a partial
// upgrade on a delayed package base from looking like a safe install.
func (m model) viewAck() string {
	iw := clamp(m.w-8, 64, 104)
	var b strings.Builder
	b.WriteString(bold(cText, gWarn+" "+i18n.Tf("Manjaro detected: %s", m.f.distroName)) + "\n\n")
	b.WriteString(sty().Foreground(cText).Width(iw).Render(i18n.T("The [ryoku] repository follows Arch current. Manjaro stable can trail it by several weeks, leaving the Qt stack partly upgraded and the desktop unable to start.")) + "\n\n")
	b.WriteString(fg(cText, i18n.T("This setup is unsupported; breakage lands on you.")) + "\n")
	b.WriteString(fg(cSub, i18n.T("Type manjaro and press enter to accept the risk.")) + "\n\n")
	b.WriteString(fg(cSub, gSel+" ") + fg(cText, m.ackInput) + fg(cSub, "_"))
	return m.header(b.String())
}

func choiceOptionText(option choiceOption) string {
	label := option.label
	if option.recommended {
		label += " " + i18n.T("(Recommended)")
	}
	return label
}

func choiceColumns(items []planItem) (int, []int) {
	labelW := 0
	var optionW []int
	for _, item := range items {
		if item.pick == nil {
			continue
		}
		if w := dw(item.label); w > labelW {
			labelW = w
		}
		for i, option := range item.options {
			for len(optionW) <= i {
				optionW = append(optionW, 0)
			}
			if w := dw(choiceOptionText(option)) + 2; w > optionW[i] {
				optionW[i] = w
			}
		}
	}
	return labelW, optionW
}

func renderChoiceRow(it planItem, focused bool, labelW int, optionW []int, width int) string {
	lead := "  "
	if focused {
		lead = bold(cText, gSel)
	}
	line := lead + " " + fg(cSub, padTo(it.label, labelW)) + " "
	for i, option := range it.options {
		cell := padTo(" "+choiceOptionText(option), optionW[i])
		if option.key == *it.pick {
			line += selected(cell)
		} else {
			line += fg(cSub, cell)
		}
	}
	return truncW(line, width)
}

func choiceDetail(it planItem) string {
	if it.pick == nil {
		return ""
	}
	for _, option := range it.options {
		if option.key == *it.pick {
			return option.detail
		}
	}
	return ""
}

func truncWords(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if dw(s) <= width {
		return s
	}
	limit := width - 1
	var out string
	for _, word := range strings.Fields(s) {
		next := word
		if out != "" {
			next = out + " " + word
		}
		if dw(next) > limit {
			break
		}
		out = next
	}
	if out == "" {
		return "…"
	}
	return out + "…"
}

func wrapWords(s string, width int) []string {
	if width <= 0 || s == "" {
		return nil
	}
	var lines []string
	var line string
	for _, word := range strings.Fields(s) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if line != "" && dw(next) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func labeledRow(label, value string, labelW int) string {
	return fg(cSub, padTo(label, labelW)) + fg(cText, value)
}

func bulletRow(label, value string, labelW int) string {
	return fg(cSub, padTo(gBullet, 3)) + labeledRow(label, value, labelW)
}

func unavailableChoiceNotices(plan editionCompositorPlan, kind string) []string {
	var notices []string
	for _, availability := range plan.choices.Choices(kind) {
		if !availability.Available {
			notices = append(notices, i18n.Tf("%s unavailable: %s", availability.Choice.Label, availability.Reason))
		}
	}
	return notices
}

func voidEditionNotice(f *facts, plan editionCompositorPlan) []string {
	if f == nil || f.distro == nil || f.distro.id != "void" {
		return nil
	}
	notice := []string{
		i18n.T("Ryoku on Void (experimental edition)"),
		i18n.Tf("Compositors: %s.", strings.Join(plan.choices.AvailableIDs(installerplan.Compositor), ", ")),
	}
	notice = append(notice, unavailableChoiceNotices(plan, installerplan.Compositor)...)
	notice = append(notice, unavailableChoiceNotices(plan, installerplan.Browser)...)
	notice = append(notice, unavailableChoiceNotices(plan, installerplan.LoginShell)...)
	notice = append(notice, unavailableChoiceNotices(plan, installerplan.FileManager)...)
	if f.btrfsRoot {
		notice = append(notice, i18n.T("Update the base system with ryoku update --system so it is snapshotted; XBPS has no update hooks."))
	}
	return notice
}

func editionNotice(f *facts, compositors editionCompositorPlan) []string {
	if f == nil || f.distro == nil {
		return nil
	}
	if f.distro.id == "void" {
		return voidEditionNotice(f, compositors)
	}
	if f.distro.id != "fedora" {
		return nil
	}
	notice := []string{
		i18n.T("Ryoku on Fedora (experimental edition)"),
		i18n.Tf("Compositors: %s.", strings.Join(compositors.choices.AvailableIDs(installerplan.Compositor), ", ")),
	}
	notice = append(notice, unavailableChoiceNotices(compositors, installerplan.Compositor)...)
	notice = append(notice, unavailableChoiceNotices(compositors, installerplan.Browser)...)
	notice = append(notice, unavailableChoiceNotices(compositors, installerplan.LoginShell)...)
	return append(notice, unavailableChoiceNotices(compositors, installerplan.FileManager)...)
}

func (m model) viewPlan() string {
	f := m.f
	iw := clamp(m.w-8, 64, 104)
	dm := f.currentDM
	if dm == "" {
		dm = i18n.T("none")
	}
	machine := truncW(i18n.Tf("%s / %s / %s", f.hostname, f.distroName, f.gpuSummary()), iw)
	safety := i18n.Tf("login: %s / configs backed up / snapshot rollback: %s", dm, i18n.T("unavailable"))
	if f.btrfsRoot && (f.distro == nil || f.distro.id != "fedora") {
		safety = i18n.Tf("login: %s / configs backed up / snapshot rollback: %s", dm, i18n.T("ready"))
	}

	var choices, rest []struct {
		index int
		item  planItem
	}
	for i, it := range m.items {
		if it.pick != nil {
			choices = append(choices, struct {
				index int
				item  planItem
			}{i, it})
		} else {
			rest = append(rest, struct {
				index int
				item  planItem
			}{i, it})
		}
	}

	choiceLabelW, optionW := choiceColumns(m.items)
	choiceDetailIndent := 4 + choiceLabelW
	notice := editionNotice(f, m.editionCompositors)
	var b strings.Builder
	b.WriteString(bold(cText, i18n.Tf("Install plan for %s", f.hostname)) + "\n")
	b.WriteString(fg(cText, machine) + "\n")
	b.WriteString(fg(cSub, truncWords(safety, iw)) + "\n\n")
	for _, row := range choices {
		focused := row.index == m.sel
		b.WriteString(renderChoiceRow(row.item, focused, choiceLabelW, optionW, iw) + "\n")
		if focused {
			detail := choiceDetail(row.item)
			b.WriteString(strings.Repeat(" ", choiceDetailIndent) + fg(cSub, detail) + "\n")
		}
	}
	b.WriteString(fg(cLine, strings.Repeat(ruleCh(), iw)) + "\n")

	selectedRest := 0
	for i, row := range rest {
		if row.index == m.sel {
			selectedRest = i
			break
		}
	}
	visibleRows := clamp(m.h-17, 5, 7)
	if len(notice) > 0 {
		visibleRows = clamp(m.h-22, 2, 7)
	}
	start, end := stepWindow(len(rest), selectedRest, visibleRows)
	for _, row := range rest[start:end] {
		it := row.item
		if !selectablePlanItem(it) {
			b.WriteString(strings.Repeat(" ", 9) + fg(cDim, it.label) + "\n")
			continue
		}
		focused := row.index == m.sel
		lead := "  "
		if focused {
			lead = bold(cText, gSel)
		}
		mark, word := gOn, i18n.T("on")
		stateColor := cText
		if !*it.on {
			mark, word = gOff, i18n.T("off")
			stateColor = cDim
		}
		state := fg(stateColor, padTo(mark, 2)+padTo(word, 4))
		suffix := ""
		if it.locked {
			suffix = " " + fg(cDim, i18n.T("(required)"))
		}
		label := fg(cText, it.label)
		if focused {
			label = bold(cText, it.label)
		}
		b.WriteString(truncW(lead+" "+state+label+suffix, iw) + "\n")
		if focused {
			b.WriteString(strings.Repeat(" ", 9) + fg(cSub, truncWords(it.detail, iw-9)) + "\n")
		}
	}
	if len(rest) > end-start {
		b.WriteString(fg(cDim, i18n.Tf("   showing %d-%d of %d plan rows", start+1, end, len(rest))) + "\n")
	}
	if f.filteredDNS {
		b.WriteString(fg(cText, gWarn+" "+i18n.T("DNS for ryoku.dev is being filtered on this network; see https://github.com/Ryoku-dev/ryoku/blob/main/docs/updates.md#filtered-dns")) + "\n")
	} else if !f.online {
		b.WriteString(fg(cText, gWarn+" "+i18n.T("repo.ryoku.dev is unreachable; installation needs network")) + "\n")
	}
	if len(notice) > 0 {
		b.WriteString("\n")
		for i, line := range notice {
			if i == 0 {
				b.WriteString(bold(cText, line) + "\n")
			} else {
				b.WriteString(fg(cSub, line) + "\n")
			}
		}
	}
	if m.confirm {
		b.WriteString("\n" + selected(" "+i18n.T("Install the Ryoku desktop with these choices?")+" "))
	}
	return m.header(strings.TrimRight(b.String(), "\n"))
}

func stepWindow(total, current, count int) (int, int) {
	if total <= count {
		return 0, total
	}
	start := current - count/2
	if start < 0 {
		start = 0
	}
	if start+count > total {
		start = total - count
	}
	return start, start + count
}

func (m model) viewInstall() string {
	iw := clamp(m.w-8, 64, 104)
	total := len(m.eng.steps)
	done := clamp(m.stepIdx, 0, total)
	bw := clamp(iw-12, 30, 72)
	fill := 0
	if total > 0 {
		fill = clamp(int(float64(done)/float64(total)*float64(bw)), 0, bw)
	}
	bar := fg(cText, strings.Repeat(gFull, fill)) + fg(cLine, strings.Repeat(gEmpty, bw-fill)) +
		fg(cSub, fmt.Sprintf(" %2d/%d", done, total))

	var b strings.Builder
	b.WriteString(bold(cText, i18n.T("Installing the Ryoku desktop")) + "\n\n")
	b.WriteString(bar + "\n")
	doneStatus, nowStatus, nextStatus := i18n.T("done"), i18n.T("now"), i18n.T("next")
	statusW := dw(doneStatus)
	if w := dw(nowStatus); w > statusW {
		statusW = w
	}
	if w := dw(nextStatus); w > statusW {
		statusW = w
	}
	statusW++
	start, end := stepWindow(total, m.stepIdx, 5)
	for i := start; i < end; i++ {
		s := m.eng.steps[i]
		lead := "  "
		mark, status := gPend, nextStatus
		lineColor := cDim
		switch {
		case i < m.stepIdx:
			mark, status, lineColor = gCheck, doneStatus, cSub
		case i == m.stepIdx:
			lead = gSel
			mark, status, lineColor = spinFrames[m.frame%len(spinFrames)], nowStatus, cText
		}
		line := lead + " " + padTo(mark, 3) + padTo(status, statusW) + s.title
		if i == m.stepIdx {
			b.WriteString(bold(lineColor, line) + "\n")
		} else {
			b.WriteString(fg(lineColor, line) + "\n")
		}
	}
	b.WriteString(fg(cLine, strings.Repeat(ruleCh(), iw)) + "\n")
	b.WriteString(fg(cSub, i18n.T("Recent install log")) + "\n")
	logRows := clamp(m.h-18, 3, 6)
	tail := m.logTail
	if len(tail) > logRows {
		tail = tail[len(tail)-logRows:]
	}
	for _, line := range tail {
		b.WriteString(fg(cDim, truncW(line, iw)) + "\n")
	}
	for i := len(tail); i < logRows; i++ {
		b.WriteString("\n")
	}
	return m.header(strings.TrimRight(b.String(), "\n"))
}

func (m model) viewDone() string {
	iw := clamp(m.w-8, 64, 104)
	labelW := 19
	valueW := iw - 3 - labelW
	var b strings.Builder
	b.WriteString(bold(cText, gCheck+" "+i18n.T("The Ryoku desktop is installed")) + "\n\n")
	b.WriteString(fg(cText, i18n.T("Reboot to land in the Ryoku greeter and your new session.")) + "\n\n")
	b.WriteString(bulletRow(i18n.T("updates forever:"), "ryoku update", labelW) + "\n")
	b.WriteString(bulletRow(i18n.T("health checks:"), "ryoku doctor", labelW) + "\n")
	if m.eng != nil && m.eng.backupDir != "" {
		label := i18n.T("your old configs:")
		if m.eng.prevBackups > 0 {
			label = i18n.T("this run's backup:")
		}
		b.WriteString(bulletRow(label, truncW(m.displayPath(m.eng.backupDir), valueW), labelW) + "\n")
		indent := strings.Repeat(" ", 3+labelW)
		b.WriteString(indent + fg(cSub, i18n.T("restore.sh inside undoes this run's changes")) + "\n")
		if m.eng.prevBackups > 0 {
			for _, line := range wrapWords(i18n.T("earlier backups sit alongside; the oldest holds your pre-Ryoku configs"), valueW) {
				b.WriteString(indent + fg(cSub, line) + "\n")
			}
		}
	}
	b.WriteString(bulletRow(i18n.T("install log:"), truncW(m.displayPath(m.logPath()), valueW), labelW) + "\n")
	first := wrapWords(i18n.T("Super+Space launcher / Super+, settings / Super+K keybinds"), valueW)
	if len(first) > 0 {
		b.WriteString(bulletRow(i18n.T("first steps:"), first[0], labelW))
		indent := strings.Repeat(" ", 3+labelW)
		for _, line := range first[1:] {
			b.WriteString("\n" + indent + fg(cText, line))
		}
	}
	return m.header(b.String())
}

func (m model) viewFailed() string {
	iw := clamp(m.w-8, 64, 104)
	labelW := 14
	valueW := iw - labelW
	step := "?"
	if m.eng != nil && m.failIdx < len(m.eng.steps) {
		step = m.eng.steps[m.failIdx].title
	}
	var b strings.Builder
	b.WriteString(bold(cText, gBad+" "+i18n.T("Install failed")) + "\n\n")
	b.WriteString(labeledRow(i18n.T("Step:"), truncWords(step, valueW), labelW) + "\n")
	b.WriteString(labeledRow(i18n.T("Error:"), truncWords(m.failMsg, valueW), labelW) + "\n\n")
	b.WriteString(labeledRow(i18n.T("Recent log:"), "", labelW) + "\n")
	logRows := clamp(m.h-18, 3, 6)
	tail := m.logTail
	if len(tail) > logRows {
		tail = tail[len(tail)-logRows:]
	}
	indent := strings.Repeat(" ", labelW)
	for _, line := range tail {
		b.WriteString(indent + fg(cDim, truncW(line, valueW)) + "\n")
	}
	b.WriteString("\n" + labeledRow(i18n.T("Full log:"), truncW(m.displayPath(m.logPath()), valueW), labelW) + "\n")
	b.WriteString(labeledRow(i18n.T("Retry:"), i18n.T("completed steps stay; retry resumes at the failed one"), labelW) + "\n")
	if m.eng != nil && m.eng.backupDir != "" {
		rollback := "bash " + m.displayPath(m.eng.backupDir) + "/restore.sh"
		b.WriteString(labeledRow(i18n.T("Roll back:"), truncW(rollback, valueW), labelW) + "\n")
	}
	return m.header(strings.TrimRight(b.String(), "\n"))
}

// ---- headless (--yes) ----

func runHeadless(dry bool, ref, payload, compositor string, overrides planOverrides, editionCompositors editionCompositorPlan) int {
	fmt.Println(bold(cBrand, "ryoku-shell-install") + fg(cSub, " "+i18n.T("(headless)")))
	f := detect()
	if needsManjaroAck(f) {
		fmt.Println(bold(cYell, i18n.T("refusing to install on Manjaro non-interactively.")))
		fmt.Println(i18n.T("the [ryoku] repo is built against Arch current; Manjaro stable trails it by weeks and partial upgrades can break the Qt stack."))
		fmt.Println(i18n.T("run without --yes to read the warning, or set RYOKU_ALLOW_MANJARO=1 to accept the risk here."))
		return 1
	}
	p := defaultPlanForEdition(f, editionCompositors)
	if compositor != "" {
		p.compositor = compositor
	}
	applyPlanOverrides(p, overrides)
	if f.distro != nil && f.distro.id == "fedora" {
		p.aur = false
	}
	fmt.Println(i18n.Tf("system: %s | gpu: %s | dm: %s", f.distroName, f.gpuSummary(), f.currentDM))
	if len(f.riceFound) > 0 {
		fmt.Println(i18n.Tf("rice found: %s (daemons replaced, configs ride the backup)", strings.Join(f.riceFound, ", ")))
	}
	if f.prevRun != nil {
		fmt.Println(i18n.Tf("resuming the interrupted previous run: %d step(s) already done", len(f.prevRun.Completed)))
	}
	// A machine-readable plan dump keeps unattended runs auditable.
	selectedApps := selectedComponentIDs(p, installerplan.ComponentApps)
	selectedGaming := selectedComponentIDs(p, installerplan.StepGaming)
	fmt.Printf("plan: compositor=%s browser=%s shell=%s file-manager=%s rashin=%v ryoku-apps=%s gaming=%s cachyos-layer=%v components=%s without=%s nvidia=%v sddm=%v greeter-theme=%v networkmanager=%v remove-shells=%v aur=%v devtools=%v omarchy-cleanup=%v monitor-pins=%v azerty-fr=%v azerty-be=%v\n",
		p.compositor, p.browser, p.shell, p.fileManager, p.componentOn("rashin"),
		strings.Join(selectedComponentGroupIDs(p, "ryoku-apps"), ","), strings.Join(selectedGaming, ","),
		p.componentOn("cachyos-layer"), strings.Join(selectedApps, ","), strings.Join(deselectedComponentIDs(p), ","),
		p.nvidia, p.switchDM, p.greeter, p.switchNet, p.rivals, p.aur, p.devtools,
		p.omarchy, p.monPins, p.azertyFR, p.azertyBE)
	if notice := editionNotice(f, editionCompositors); len(notice) > 0 {
		fmt.Println()
		for _, line := range notice {
			fmt.Println(line)
		}
	}
	e := newEngine(f, p, dry, ref, payload)
	ev := e.runFrom(0)
	for msg := range ev {
		switch msg := msg.(type) {
		case evStep:
			fmt.Println(bold(cBrand, fmt.Sprintf("==> [%d/%d] %s", msg.idx+1, len(e.steps), msg.title)))
		case evLine:
			if msg.transient {
				continue // progress repaints are UI-only; headless logs stay line-per-event
			}
			fmt.Println("    " + msg.line)
		case evDone:
			if msg.err != nil {
				fmt.Println(bold(cRed, i18n.Tf("install failed: %s", msg.err.Error())))
				fmt.Println(i18n.Tf("log: %s", e.logPath))
				return 1
			}
			fmt.Println(bold(cGreen, i18n.T("the Ryoku desktop is installed; reboot to use it")))
			if e.backupDir != "" {
				fmt.Println(i18n.Tf("old configs: %s (restore.sh inside)", e.backupDir))
			}
			return 0
		}
	}
	return 1
}

func selectedComponentIDs(p *plan, step installerplan.ComponentStep) []string {
	var ids []string
	for _, availability := range p.choiceEdition().Components() {
		component := availability.Component
		if availability.Available && component.Step == step && p.componentOn(component.ID) {
			ids = append(ids, component.ID)
		}
	}
	return ids
}

func selectedComponentGroupIDs(p *plan, group string) []string {
	var ids []string
	for _, availability := range p.choiceEdition().Components() {
		component := availability.Component
		if availability.Available && component.Group == group && p.componentOn(component.ID) {
			ids = append(ids, component.ID)
		}
	}
	return ids
}

func deselectedComponentIDs(p *plan) []string {
	var ids []string
	for _, availability := range p.choiceEdition().Components() {
		component := availability.Component
		if availability.Available && component.DefaultOn && component.Group != devToolchainGroup && !p.componentOn(component.ID) {
			ids = append(ids, component.ID)
		}
	}
	return ids
}

// ---- entry ----

func die(msg string) {
	fmt.Fprintln(os.Stderr, "ryoku-shell-install: "+msg)
	os.Exit(1)
}

func primeSudo() {
	if exec.Command("sudo", "-n", "true").Run() == nil {
		go sudoKeepalive()
		return
	}
	fmt.Println(i18n.T("ryoku-shell-install needs sudo for system changes; asking once up front."))
	c := exec.Command("sudo", "-v")
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		c.Stdin, c.Stdout, c.Stderr = tty, tty, tty
	} else {
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	if err := c.Run(); err != nil {
		die(i18n.T("sudo authentication failed"))
	}
	go sudoKeepalive()
}

func sudoKeepalive() {
	// 20s survives sudoers with timestamp_timeout down to half a minute; a
	// lapsed credential turns every nested sudo (yay, makepkg, the driver
	// scripts) into a password prompt written straight over the TUI.
	for range time.Tick(20 * time.Second) {
		_ = exec.Command("sudo", "-n", "-v").Run()
	}
}

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	// this binary lands on a stranger's distro with no Ryoku catalog on disk,
	// so the catalogs are compiled in; point the loader at them before Use.
	i18n.SetFS(catalog.FS)
	i18n.Use("")
	rashinDefault, err := envToggle("RYOKU_RASHIN", false)
	if err != nil {
		die(err.Error())
	}
	cachyosDefault, err := envToggle("RYOKU_CACHYOS_LAYER", false)
	if err != nil {
		die(err.Error())
	}
	gamingDefault, err := envToggle("RYOKU_GAMING", false)
	if err != nil {
		die(err.Error())
	}

	yes := flag.Bool("yes", false, i18n.T("run non-interactively with the default plan"))
	dry := flag.Bool("dry-run", false, i18n.T("print every command instead of running it"))
	uninstall := flag.Bool("uninstall", false, i18n.T("remove the ryoku packages and restore the backup chain"))
	ref := flag.String("ref", envOr("RYOKU_SHELL_REF", "main"), i18n.T("ryoku-arch git ref for the payload"))
	payload := flag.String("payload", os.Getenv("RYOKU_SHELL_PAYLOAD"), i18n.T("use a local ryoku-arch checkout as the payload"))
	compositor := flag.String("compositor", "", i18n.T("window manager to install (default: first available)"))
	browser := flag.String("browser", envOr("RYOKU_BROWSER", installerplan.FreshDefault(installerplan.Browser)), i18n.T("browser to install"))
	shell := flag.String("shell", os.Getenv("RYOKU_LOGIN_SHELL"), i18n.T("login shell"))
	fileManager := flag.String("file-manager", os.Getenv("RYOKU_FILE_MANAGER"), i18n.T("file manager to install"))
	rashin := flag.Bool("rashin", rashinDefault, i18n.T("install Ryoku's Rashin AI agent"))
	gaming := flag.Bool("gaming", gamingDefault, i18n.T("install this edition's Steam gaming bundle on CachyOS or Void"))
	cachyosLayer := flag.Bool("cachyos-layer", cachyosDefault, i18n.T("install Gaming with CachyOS on Arch (network and x86-64-v3 required; Steam + Proton-CachyOS)"))
	without := flag.String("without", os.Getenv("RYOKU_WITHOUT"), i18n.T("comma-separated optional components to leave out"))
	flag.Parse()
	withoutIDs, err := parseWithout(*without)
	if err != nil {
		die(err.Error())
	}
	overrides := planOverrides{
		browser: *browser, shell: *shell, fileManager: *fileManager,
		rashin: *rashin, gaming: *gaming, cachyosLayer: *cachyosLayer, without: withoutIDs,
	}

	initGlyphs()

	if os.Geteuid() == 0 {
		die(i18n.T("run as your normal user, not root; sudo is used where needed"))
	}
	d := detectHostDistro()
	if d == nil {
		die(i18n.T("unsupported distribution: Ryoku installs on Arch-based, Debian-based, Void and Fedora systems"))
	}
	if out("uname", "-m") != "x86_64" {
		die(i18n.T("Ryoku ships x86_64 builds only"))
	}
	if d.id == "fedora" {
		if _, err := os.Stat("/run/ostree-booted"); err == nil {
			die(i18n.T("rpm-ostree systems are not supported; use a mutable Fedora Workstation or Server install"))
		}
	}
	if !supportedInitBooted(d) {
		die(i18n.T("unsupported init: Arch, Debian and Fedora installs need systemd; Void installs need runit"))
	}

	if *uninstall {
		if !*dry {
			primeSudo()
		}
		os.Exit(runUninstall(*yes, *dry))
	}

	client := fedoraReleaseHTTPClient()
	if d.id == "fedora" {
		if err := fedoraReleaseGate(client, *ref, "/etc/os-release"); err != nil {
			die(err.Error())
		}
		if os.Getenv("RYOKU_RPM_REPO") != "" {
			fmt.Println(i18n.T("Fedora release gate: skipped by RYOKU_RPM_REPO."))
		} else if release, err := fedoraReleaseVersion("/etc/os-release"); err == nil {
			fmt.Println(i18n.Tf("Fedora release gate: packages are published for Fedora %s.", release))
		}
	}
	editionCompositors, err := loadEditionCompositors(client, d, *ref, *payload)
	if err != nil {
		die(err.Error())
	}
	if err := validateEditionFlags(overrides, editionCompositors); err != nil {
		die(err.Error())
	}
	comp := chooseCompositor(*compositor, editionCompositors)
	if !*dry {
		primeSudo()
	}
	if *yes {
		os.Exit(runHeadless(*dry, *ref, *payload, comp, overrides, editionCompositors))
	}
	if !stdoutIsTTY() {
		die(i18n.T("unable to run interactively; re-run with --yes for the default plan"))
	}

	fm, err := tea.NewProgram(newTUIModel(*dry, *ref, *payload, comp, overrides, editionCompositors)).Run()
	if err != nil {
		die(err.Error())
	}
	if m, ok := fm.(model); ok && m.exitReboot {
		if hostInit() == initRunit {
			_ = exec.Command("loginctl", "reboot").Run()
		} else {
			_ = exec.Command("systemctl", "reboot").Run()
		}
	}
}

// needsManjaroAck: Manjaro gets a hard warning with typed consent (TUI) or a
// refusal (--yes); RYOKU_ALLOW_MANJARO=1 waves both through.
func needsManjaroAck(f *facts) bool {
	return f.distroID == "manjaro" && os.Getenv("RYOKU_ALLOW_MANJARO") != "1"
}

// chooseCompositor validates a --compositor pick against the shared definition
// and the edition's package table, returning "" for the table's first choice.
func chooseCompositor(choice string, edition editionCompositorPlan) string {
	if choice == "" {
		return ""
	}
	if _, ok := installerplan.ChoiceByID(installerplan.Compositor, choice); !ok {
		die(i18n.Tf("unknown compositor %q; choose one of: %s", choice,
			strings.Join(installerplan.IDs(installerplan.Compositor), ", ")))
	}
	if err := validateEditionChoice(installerplan.Compositor, choice, edition); err != nil {
		die(err.Error())
	}
	return choice
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envToggle(name string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback, nil
	}
	switch value {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, fmt.Errorf(i18n.T("%s must be 0 or 1"), name)
	}
}

func parseWithout(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(value, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, errors.New(i18n.T("--without must be a comma-separated list of component names"))
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}
