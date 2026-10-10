// ryoku-tui is the Ryoku installer: a full-screen Bubble Tea v2 TUI that collects
// the install choices and hands them to the ryoku-install backend. The partition
// step is a guided layout: it creates the ESP and a btrfs root, and lets you set
// the ESP and swap sizes and toggle optional subvolumes (snapshots, /home,
// backups). Snapshots and backups are btrfs subvolumes, not partitions. The live
// system glue (data lists, hardware detection, the backend handoff) lives in
// system.go.
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/x/ansi"
	"github.com/mdp/qrterminal/v3"
	"github.com/sahilm/fuzzy"

	"ryoku-i18n"
	"ryoku-i18n/catalog"
	wm "ryoku-wm"
)

// The failure screen renders this as a scannable QR code so the user can reach
// install help from another device.
const ryokuSupportURL = "https://docs.ryoku.dev"

// Ryoku community links shown on the welcome screen (logos, press 's' for QR).
const (
	discordURL  = "https://discord.gg/8KjBmUEyKA"
	redditURL   = "https://www.reddit.com/r/RyokuArch"
	xURL        = "https://x.com/neur0map"
	iconDiscord = "󰙯" // nf-md-discord
	iconReddit  = "" // nf-fa-reddit
	iconX       = "𝕏"
)

// ───────────────────────── palette ─────────────────────────
var (
	cBg    = lipgloss.Color("#060607")
	cText  = lipgloss.Color("#eae2d5")
	cSub   = lipgloss.Color("#8c857a")
	cDim   = lipgloss.Color("#3a3630")
	cBrand = lipgloss.Color("#c75d2b")
	cBlue  = lipgloss.Color("#8c857a")
	cGreen = lipgloss.Color("#88a57d")
	cYell  = lipgloss.Color("#c79a5b")
	cMauve = lipgloss.Color("#c75d2b")
	cRed   = lipgloss.Color("#b24a38")
)

func sty() lipgloss.Style                 { return lipgloss.NewStyle() }
func fg(c color.Color, s string) string   { return sty().Foreground(c).Render(s) }
func bold(c color.Color, s string) string { return sty().Foreground(c).Bold(true).Render(s) }
func dw(s string) int                     { return lipgloss.Width(s) }

func truncW(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dw(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && dw(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
func padTo(s string, w int) string {
	if d := dw(s); d < w {
		return s + strings.Repeat(" ", w-d)
	}
	return s
}
func padLines(s string, w int) string {
	ls := strings.Split(s, "\n")
	for i := range ls {
		ls[i] = padTo(ls[i], w)
	}
	return strings.Join(ls, "\n")
}
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ───────────────────────── glyphs / fallback ─────────────────────────
// lipgloss v2 (colorprofile) auto-downsamples truecolor → 256/16/none, so colors
// need no manual fallback. Glyphs do: `ascii` swaps box-drawing/blocks/markers to
// plain ASCII for dumb/no-UTF8 consoles; `nerd` gates Private-Use icons.
var ascii bool
var nerd = true
var (
	gCheck   = "✓"
	gPend    = "·"
	gCurStep = "▸ "
	gSelCur  = "▌ "
	gPrompt  = "❯ "
	gCaret   = "▏"
	gMark    = "▲"
	gOn      = "● on "
	gOff     = "○ off"
	gFull    = "█"
	gEmpty   = "░"
	gOKtxt   = "✓"
	gBad     = "✗"
)
var asciiBorder = lipgloss.Border{Top: "-", Bottom: "-", Left: "|", Right: "|", TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+"}

func ruleCh() string {
	if ascii {
		return "-"
	}
	return "─"
}
func border() lipgloss.Border {
	if ascii {
		return asciiBorder
	}
	return lipgloss.NormalBorder()
}
func borderDouble() lipgloss.Border {
	if ascii {
		return asciiBorder
	}
	return lipgloss.DoubleBorder()
}
func initGlyphs() {
	t := os.Getenv("TERM")
	ascii = os.Getenv("RYOKU_ASCII") != "" || t == "dumb" || t == "vt100" || t == ""
	nerd = !ascii && t != "linux" // Linux fb console renders boxes/blocks but not PUA icons
	if !ascii {
		return
	}
	gCheck, gPend, gCurStep = "+", ".", "> "
	gSelCur, gPrompt, gCaret = "> ", "> ", "_"
	gMark, gOn, gOff = "^", "[x]on ", "[ ]off"
	gFull, gEmpty = "#", "-"
	gOKtxt, gBad = "OK", "x"
	spinFrames = []string{"|", "/", "-", "\\"}
}

var gradA = [3]int{0xF2, 0x56, 0x23}
var gradB = [3]int{0xFF, 0xD2, 0x4A}

func gradColor(t float64) color.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	r := int(float64(gradA[0]) + float64(gradB[0]-gradA[0])*t)
	g := int(float64(gradA[1]) + float64(gradB[1]-gradA[1])*t)
	b := int(float64(gradA[2]) + float64(gradB[2]-gradA[2])*t)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

// ───────────────────────── banners ─────────────────────────
var smallRows = []string{
	"█▀▄ █ █ █▀█ █▄▀ █ █",
	"█▀▄ ▀█▀ █ █ █▀▄ █ █",
	"▀ ▀ ░█░ ▀▀▀ ▀ ▀ ▀▀▀",
}

const smallW = 19

func wordmark(dim bool) string { // ASCII-fallback wordmark
	w := []rune("R Y O K U")
	var b strings.Builder
	for i, ch := range w {
		if dim {
			b.WriteString(fg(cDim, string(ch)))
		} else {
			b.WriteString(fg(gradColor(float64(i)/float64(len(w)-1)), string(ch)))
		}
	}
	return b.String()
}

func smallBanner(phase int) []string {
	if ascii {
		return []string{wordmark(false)}
	}
	out := make([]string, 0, 3)
	for _, row := range smallRows {
		var b strings.Builder
		for i, ch := range []rune(row) {
			idx := (i + phase) % smallW
			b.WriteString(fg(gradColor(float64(idx)/float64(smallW-1)), string(ch)))
		}
		out = append(out, b.String())
	}
	return out
}

var bigLetters = [][]string{
	{"████ ", "█  █ ", "████ ", "█ █  ", "█  █ "},
	{"█   █", " █ █ ", "  █  ", "  █  ", "  █  "},
	{"█████", "█   █", "█   █", "█   █", "█████"},
	{"█  █ ", "█ █  ", "██   ", "█ █  ", "█  █ "},
	{"█   █", "█   █", "█   █", "█   █", "█████"},
}

func bigRows() []string {
	rows := make([]string, 5)
	for r := 0; r < 5; r++ {
		parts := make([]string, len(bigLetters))
		for i := range bigLetters {
			parts[i] = bigLetters[i][r]
		}
		rows[r] = strings.Join(parts, " ")
	}
	return rows
}

func bigBanner(reveal float64, phase int) []string {
	if ascii {
		w := []rune("R Y O K U")
		cut := int(reveal * float64(len(w)+1))
		var b strings.Builder
		for i, ch := range w {
			if i <= cut {
				b.WriteString(fg(gradColor(float64(i)/float64(len(w)-1)), string(ch)))
			} else {
				b.WriteString(fg(cDim, string(ch)))
			}
		}
		return []string{"", b.String(), ""}
	}
	rows := bigRows()
	total := dw(rows[0])
	cut := int(reveal * float64(total+3))
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i, ch := range []rune(row) {
			if ch == ' ' {
				b.WriteString(" ")
				continue
			}
			if i <= cut {
				idx := (i + phase) % total
				b.WriteString(fg(gradColor(float64(idx)/float64(total-1)), string(ch)))
			} else {
				b.WriteString(fg(cDim, string(ch)))
			}
		}
		out = append(out, b.String())
	}
	return out
}

// faintBig renders the wordmark as a dim, slowly-shimmering watermark for the
// welcome screen backdrop (the bright text sits in front of it).
func faintBig(phase int) []string {
	if ascii {
		return []string{"", wordmark(true), ""}
	}
	rows := bigRows()
	total := dw(rows[0])
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i, ch := range []rune(row) {
			if ch == ' ' {
				b.WriteString(" ")
				continue
			}
			t := float64((i+phase)%total) / float64(total-1)
			r := int(0x39 + (0x52-0x39)*t)
			g := int(0x2c + (0x3e-0x2c)*t)
			bl := int(0x24 + (0x2a-0x24)*t)
			b.WriteString(fg(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bl)), string(ch)))
		}
		out = append(out, b.String())
	}
	return out
}

// ───────────────────────── picker ─────────────────────────
type item struct{ key, label, hint string }
type itemSource []item

func (s itemSource) String(i int) string { return s[i].label }
func (s itemSource) Len() int            { return len(s) }

type picker struct {
	items     []item
	matches   []int
	cursor    int
	offset    int
	height    int
	query     string
	searching bool
	numbered  bool
	disabled  map[string]bool // item keys that render dimmed and refuse to commit
}

func newPicker(items []item, numbered bool) picker {
	p := picker{items: items, numbered: numbered, height: 8}
	p.refilter()
	return p
}
func (p *picker) refilter() {
	p.matches = p.matches[:0]
	if p.query == "" {
		for i := range p.items {
			p.matches = append(p.matches, i)
		}
	} else {
		for _, r := range fuzzy.FindFrom(p.query, itemSource(p.items)) {
			p.matches = append(p.matches, r.Index)
		}
	}
	if p.cursor >= len(p.matches) {
		p.cursor = max(0, len(p.matches)-1)
	}
	p.fixScroll()
}
func (p *picker) move(d int) {
	if len(p.matches) == 0 {
		return
	}
	p.cursor = clamp(p.cursor+d, 0, len(p.matches)-1)
	p.fixScroll()
}
func (p *picker) fixScroll() {
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+p.height {
		p.offset = p.cursor - p.height + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}
func (p *picker) update(key string) (bool, int) {
	if p.searching {
		switch key {
		case "esc":
			p.searching, p.query = false, ""
			p.refilter()
		case "enter":
			if len(p.matches) > 0 {
				if idx := p.matches[p.cursor]; !p.disabled[p.items[idx].key] {
					return true, idx
				}
			}
		case "backspace":
			if len(p.query) > 0 {
				p.query = p.query[:len(p.query)-1]
				p.refilter()
			}
		case "up", "ctrl+p":
			p.move(-1)
		case "down", "ctrl+n":
			p.move(1)
		case "space":
			p.query += " "
			p.refilter()
		default:
			if r := []rune(key); len(r) == 1 && r[0] >= 0x20 {
				p.query += key
				p.refilter()
			}
		}
		return false, -1
	}
	switch key {
	case "up", "k":
		p.move(-1)
	case "down", "j":
		p.move(1)
	case "pgup", "ctrl+u":
		p.move(-p.height)
	case "pgdown", "ctrl+d":
		p.move(p.height)
	case "home", "g":
		p.cursor = 0
		p.fixScroll()
	case "end", "G":
		p.cursor = max(0, len(p.matches)-1)
		p.fixScroll()
	case "/":
		p.searching = true
	case "enter":
		if len(p.matches) > 0 {
			if idx := p.matches[p.cursor]; !p.disabled[p.items[idx].key] {
				return true, idx
			}
		}
	default:
		if p.numbered && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if n := int(key[0] - '1'); n < len(p.matches) {
				if idx := p.matches[n]; !p.disabled[p.items[idx].key] {
					return true, idx
				}
			}
		}
	}
	return false, -1
}

func (p picker) view(w, phase int) string {
	var b strings.Builder
	end := min(len(p.matches), p.offset+p.height)
	if len(p.matches) == 0 {
		b.WriteString(fg(cSub, i18n.T("no matches")))
	}
	for vi := p.offset; vi < end; vi++ {
		it := p.items[p.matches[vi]]
		sel := vi == p.cursor
		num := ""
		if p.numbered {
			if vi < 9 {
				num = fmt.Sprintf("%d ", vi+1)
			} else {
				num = "  "
			}
		}
		var left string
		dis := p.disabled[it.key]
		switch {
		case dis:
			left = "  " + fg(cDim, num) + fg(cDim, it.label)
		case sel:
			left = bold(gradColor(float64(phase)/float64(smallW-1)), gSelCur) + fg(cDim, num) + bold(cText, it.label)
		default:
			left = "  " + fg(cDim, num) + fg(cSub, it.label)
		}
		gut := 2 + dw(num)
		line := left
		if it.hint != "" {
			if avail := w - gut - dw(it.label) - 1; avail >= 6 {
				h := truncW(it.hint, avail)
				pad := w - gut - dw(it.label) - dw(h)
				if pad < 1 {
					pad = 1
				}
				hc := cDim
				if sel {
					hc = cSub
				}
				line = left + strings.Repeat(" ", pad) + fg(hc, h)
			}
		}
		b.WriteString(line + "\n")
	}
	for i := end - p.offset; i < p.height; i++ {
		b.WriteString("\n")
	}
	if p.searching || p.query != "" {
		b.WriteString("\n" + fg(cBrand, "/"+p.query+"▏") + "  " + fg(cDim, fmt.Sprintf("%d/%d", p.cursor+1, len(p.matches))))
	} else {
		b.WriteString("\n" + fg(cDim, fmt.Sprintf("%d/%d   · / filter", p.cursor+1, len(p.matches))))
	}
	return b.String()
}

// ───────────────────────── steps ─────────────────────────
type kind int

const (
	kSelect kind = iota
	kInput
	kConfirm
	kPartition
	kInfo
	kNotice
	kPass // password + confirm
	kNet  // connectivity / Wi-Fi
	kApps // grouped keep/remove app checklist
)

const minDiskGiB = 32 // installer floor: minRootGiB closure + 1G ESP + swap/snapshot headroom
const minRootGiB = 20 // min root partition (GiB): base+desktop closure plus AUR/snapshot headroom (matches backend ryoku_min_root_gib)
const voidSnapshotNote = "Snapshots are not available on Void Linux, so updates cannot be rolled back."
const alongsideBootGiB = 2 // fixed FAT boot partition (matches backend RYOKU_ALONGSIDE_BOOT_MIB)
// The grid the layout is verified to render every critical element into: the
// destructive-write warning, the target disk, the strategy, and the Yes/No
// buttons (see TestReviewKeepsCriticalContent). Below this the text stops being
// readable, so say so rather than draw a garbled screen. Kept low on purpose: a
// live-ISO user cannot resize a kernel VT, so refusing to draw is a dead end and
// the layout degrades instead (see fitLevels).
const minTermW = 56
const minTermH = 16

// abortWindow is how long a first install-state ctrl+c stays "armed": a second
// ctrl+c within it aborts the install; after it, the warning clears and the count
// restarts. Long enough to be a deliberate double-press, short enough to expire.
const abortWindow = 3 * time.Second

type step struct {
	key, title  string
	desc        []string
	kind        kind
	items       []item
	numbered    bool
	password    bool
	placeholder string
	deflt       string
}

func steps() []step {
	return stepsForVariant(sysVariant())
}

func stepsForVariant(variant string) []step {
	browserDesc := []string{i18n.T("Ryoku ships three browsers; pick the one you want."), i18n.T("It becomes the default and the launcher's browser role.")}
	if isVoidVariant(variant) {
		browserDesc[0] = i18n.T("Ryoku ships Firefox and Chromium on Void; pick the one you want.")
	}
	all := []step{
		{key: "keyboard", title: i18n.T("Keyboard layout"), kind: kSelect, items: keymaps(),
			desc: []string{i18n.T("Type to filter · j/k or ↑↓ to move."), i18n.T("Sets console.keyMap + xkb.layout.")}},
		{key: "locale", title: i18n.T("System locale"), kind: kSelect, items: locales(),
			desc: []string{i18n.T("Language & formats. Sets i18n.defaultLocale.")}},
		{key: "timezone", title: i18n.T("Time zone"), kind: kSelect, items: timezones(),
			desc: []string{i18n.T("Used for the clock & logs. Sets time.timeZone.")}},
		{key: "network", title: i18n.T("Network"), kind: kNet, desc: []string{}},
		{key: "hardware", title: i18n.T("Hardware"), kind: kInfo, desc: []string{}},
		{key: "profile", title: i18n.T("Hardware profile"), kind: kSelect, items: profiles(), numbered: true,
			desc: []string{i18n.T("Confirm or change the suggested profile."), i18n.T("Press 1-4 or pick below.")}},
		{key: "gpu", title: i18n.T("Graphics mode"), kind: kSelect, items: gpuModes(), numbered: true,
			desc: []string{i18n.T("Hybrid GPU (iGPU + NVIDIA) detected."), i18n.T("How should displays & apps use them?")}},
		{key: "compositor", title: i18n.T("Window manager"), kind: kSelect, items: compositorsForVariant(variant), numbered: true,
			desc: []string{i18n.T("The Wayland compositor to run.")}},
		{key: "browser", title: i18n.T("Web browser"), kind: kSelect, items: browsersForVariant(variant), numbered: true,
			desc: browserDesc},
		{key: "login-shell", title: i18n.T("Login shell"), kind: kSelect, items: loginShellsForVariant(variant), numbered: true,
			desc: []string{i18n.T("Choose the shell that opens in terminals."), i18n.T("Shared command-line tools stay available with every choice.")}},
		{key: "apps", title: i18n.T("Apps & tools"), kind: kApps,
			desc: []string{i18n.T("Space toggles · every app Ryoku ships, keep or remove."), i18n.T("Required rows back a desktop feature and cannot be removed.")}},
		{key: "diskpick", title: i18n.T("Target disk"), kind: kSelect, items: disks(), numbered: true,
			desc: []string{i18n.T("Pick the disk to install onto."), i18n.T("Everything after this applies to it.")}},
		{key: "disk", title: i18n.T("Disk strategy"), kind: kSelect, items: diskStrategies(), numbered: true,
			desc: []string{i18n.T("Nothing is erased until you confirm.")}},
		{key: "partitions", title: i18n.T("Disk layout"), kind: kPartition, desc: []string{}},
		{key: "hostname", title: i18n.T("Hostname"), kind: kInput, placeholder: "ryoku", deflt: "ryoku",
			desc: []string{i18n.T("Letters, digits and dashes.")}},
		{key: "username", title: i18n.T("Primary user"), kind: kInput, placeholder: "you", deflt: "you",
			desc: []string{i18n.T("Your login account name. Lowercase.")}},
		{key: "password", title: i18n.T("User password"), kind: kPass,
			desc: []string{i18n.T("Set a password for your account.")}},
		{key: "encryption", title: i18n.T("Disk encryption"), kind: kConfirm,
			desc: []string{i18n.T("Encrypt the root with LUKS?"), i18n.T("Installs the -luks host variant.")}},
		{key: "review", title: i18n.T("Review"), kind: kConfirm,
			desc: []string{i18n.T("Last safe point, nothing written yet.")}},
	}
	// On the graphical relaunch (keymapRelaunch), the keyboard layout is already
	// chosen and cage runs under it; drop the step so the wizard resumes at locale,
	// with the password captured in the user's real layout.
	if os.Getenv("RYOKU_KB_PRESET") != "" {
		all = all[1:]
	}
	if isVoidVariant(variant) {
		all = append([]step{{
			key: "void-notice", title: i18n.T("Ryoku on Void"), kind: kNotice,
			desc: []string{
				i18n.T("This edition runs niri only."),
				i18n.T("Hyprland needs GCC 15; Void ships GCC 14."),
				i18n.T("No snapshots or boot-menu rollback: they rely on pacman."),
				i18n.T("Zen Browser, LocalSend and Voxtype are AUR-only."),
				i18n.T("Also unavailable: asusctl, adw-gtk-theme and gpk."),
				i18n.T("The oh-my-zsh theme is unavailable too."),
				i18n.T("Everything else works the same."),
			},
		}}, all...)
	}
	return all
}

func keymaps() []item {
	if r := sysKeymaps(); len(r) > 0 {
		return r
	}
	return []item{
		{"us", i18n.T("US (QWERTY)"), ""}, {"uk", i18n.T("United Kingdom"), ""}, {"de", i18n.T("German"), ""},
		{"fr", i18n.T("French (AZERTY)"), ""}, {"es", i18n.T("Spanish"), ""}, {"it", i18n.T("Italian"), ""},
		{"dvorak", "Dvorak", ""}, {"colemak", "Colemak", ""},
	}
}
func locales() []item {
	if r := sysLocales(); len(r) > 0 {
		return r
	}
	return []item{
		{"en_US.UTF-8", i18n.T("English (US)"), ""}, {"en_GB.UTF-8", i18n.T("English (UK)"), ""},
		{"de_DE.UTF-8", i18n.T("German"), ""}, {"fr_FR.UTF-8", i18n.T("French"), ""}, {"es_ES.UTF-8", i18n.T("Spanish"), ""},
	}
}
func timezones() []item {
	if r := sysTimezones(); len(r) > 0 {
		return r
	}
	return []item{
		{"auto", i18n.T("Detect automatically"), i18n.T("via IP, also sets the clock")},
		{"America/New_York", i18n.T("US Eastern"), ""}, {"America/Los_Angeles", i18n.T("US Pacific"), ""},
		{"Europe/London", i18n.T("UK / Ireland"), ""}, {"Europe/Berlin", i18n.T("Germany"), ""},
		{"Europe/Madrid", i18n.T("Spain"), ""}, {"Asia/Tokyo", i18n.T("Japan"), ""}, {"UTC", "UTC", ""},
	}
}
func profiles() []item {
	all := []item{
		{"amd-nvidia", i18n.T("NVIDIA dGPU (any CPU)"), i18n.T("AMD or Intel CPU with an NVIDIA GPU")},
		{"amd", "amd", i18n.T("AMD CPU and GPU")},
		{"intel", "intel", i18n.T("Intel CPU and GPU")},
		{"vm", "vm", i18n.T("virtual machine")},
	}
	return promote(all, []string{ensureHW().profile})
}
func diskStrategies() []item {
	// Placeholder for the static step build; loadStep swaps in
	// diskStrategiesFor(picked disk) the moment the step is entered.
	return diskStrategiesFor(diskLayout{})
}

// diskStrategiesFor orders the strategies for the disk that was actually
// picked. Alongside leads only when there is something on the disk to keep —
// that is its whole point — and it only names Windows when Windows is really
// there (the old static list promised "keep Windows" on blank disks). On a
// populated disk the non-destructive option stays first so a quick Enter can
// never wipe; a blank disk has nothing to protect, so it gets the single
// whole-disk path with honest wording — nothing is erased by using an empty
// disk.
func diskStrategiesFor(dl diskLayout) []item {
	if len(dl.parts) == 0 {
		return []item{{"whole", i18n.T("Use the whole disk"), i18n.T("blank disk · auto-layout")}}
	}
	whole := item{"whole", i18n.T("Erase whole disk"), i18n.T("wipe & auto-layout")}
	var along item
	switch {
	case dl.probeVerdict == "create-esp":
		// GPT disk with free space but no ESP of its own (e.g. a second data disk):
		// Ryoku makes its own ESP in the free space instead of sending the user to fdisk.
		along = item{"alongside", i18n.T("Install in the free space"), i18n.T("create a new EFI partition + root · keep existing data")}
	case dl.windows:
		along = item{"alongside", i18n.T("Install alongside Windows"), i18n.T("keep Windows · use free space")}
	default:
		along = item{"alongside", i18n.T("Install alongside (keep existing OS)"), i18n.T("shrink a partition · use free space")}
	}
	// A hard blocker (no ESP, no GPT) is shown inline on the dimmed option instead
	// of letting a pick dead-end at the layout step.
	if reason := alongsideBlockReason(dl); reason != "" {
		along.hint = i18n.Tf("unavailable — %s", reason)
	}
	return []item{along, whole}
}

// alongsideBlockReason is why alongside cannot run on dl at all, or "" when it
// can. It mirrors the layout step's hard blocks (a GPT disk with a usable EFI
// system partition) so the strategy list can dim the option with its cause
// rather than accept the pick and strand the user at the layout step.
func alongsideBlockReason(dl diskLayout) string {
	if !dl.gpt {
		return i18n.T("needs a GPT disk")
	}
	switch dl.probeVerdict {
	case "no-esp":
		return i18n.T("no EFI system partition")
	case "error":
		return i18n.T("disk probe failed")
	}
	return ""
}

// WIRE: real list from `lsblk -dpno NAME,SIZE,MODEL,TRAN,ROTA`; size via blockdev.
func disks() []item {
	if r := sysDisks(); len(r) > 0 {
		return r
	}
	return []item{{"/dev/vda", "/dev/vda", i18n.T("virtual disk")}}
}
func diskSizeOf(dev string) int {
	if g := sysDiskSize(dev); g > 0 {
		return g
	}
	return 0
}

// WIRE: real scan via `nmcli dev wifi list` / iwctl.
func ssids() []item {
	if r := sysSSIDs(); len(r) > 0 {
		return r
	}
	return []item{{"", i18n.T("No networks found"), i18n.T("move closer or use ethernet")}}
}

func gpuModes() []item {
	return []item{
		{"offload", i18n.T("Hybrid (recommended)"), i18n.T("iGPU display · dGPU on-demand")},
		{"sync", i18n.T("dGPU performance"), i18n.T("NVIDIA drives everything")},
		{"vfio", i18n.T("iGPU + dGPU for VM"), i18n.T("reserve dGPU for passthrough")},
	}
}

// compositors lists the window managers with a shipped variant package, in
// wm.Providers order. The step auto-skips while the list has one entry.
func compositors() []item {
	return []item{
		{wm.ProviderHyprland, "Hyprland", i18n.T("dynamic tiling, the Ryoku default")},
		{wm.ProviderNiri, "niri", i18n.T("scrollable tiling")},
	}
}

func compositorsForVariant(variant string) []item {
	if !isVoidVariant(variant) {
		return compositors()
	}
	return []item{
		{wm.ProviderHyprland, "Hyprland", i18n.T("Unavailable on Void Linux · Hyprland is not packaged")},
		{wm.ProviderNiri, "niri", i18n.T("scrollable tiling")},
	}
}

// compositorLabel is the display name for a compositor key (Review/rail cells).
func compositorLabel(key string) string {
	for _, c := range compositors() {
		if c.key == key {
			return c.label
		}
	}
	return key
}

// gpuDetails returns the pros and cons shown for the highlighted graphics mode.
// offload uses the iGPU for display and the dGPU on demand, sync drives everything
// from the dGPU, and vfio reserves the dGPU for a virtual machine.
func gpuDetails(key string) []string {
	switch key {
	case "offload":
		return []string{
			fg(cGreen, "+ ") + fg(cText, i18n.T("best battery, the dGPU sleeps until an app needs it")),
			fg(cGreen, "+ ") + fg(cText, i18n.T("dGPU still available for games and VMs")),
			fg(cRed, "- ") + fg(cSub, i18n.T("launch heavy apps with the prime-run wrapper")),
		}
	case "sync":
		return []string{
			fg(cGreen, "+ ") + fg(cText, i18n.T("maximum performance, simplest for gaming")),
			fg(cGreen, "+ ") + fg(cText, i18n.T("best for external displays driven by the dGPU")),
			fg(cRed, "- ") + fg(cSub, i18n.T("dGPU always on, more heat and battery drain")),
		}
	case "vfio":
		return []string{
			fg(cGreen, "+ ") + fg(cText, i18n.T("best battery, full dGPU inside a VM (passthrough)")),
			fg(cRed, "- ") + fg(cSub, i18n.T("no dGPU acceleration on the host desktop")),
			fg(cRed, "- ") + fg(cSub, i18n.T("advanced, needs vfio binding and Looking Glass")),
		}
	}
	return nil
}

// ───────────────────────── partition (guided NixOS layout) ─────────────────────────
// A bar segment (for the disk graph) reuses this; the editor itself is a small set
// of adjustable rows, not raw partitions.
type part struct {
	dev     string
	size    int
	fs      string
	mount   string
	flags   string
	status  string // keep | new | free
	reclaim bool   // leftover ryoku/ryokuboot partition the backend will free (alongside)
	used    int    // GiB in use (existing-content map shading); 0 = solid, no shading
}

func partColor(p part) color.Color {
	switch {
	case p.status == "free", p.status == "reclaim":
		return cDim
	case strings.Contains(p.flags, "esp"):
		return cBlue
	case p.mount == "/":
		return cGreen
	case p.fs == "swap":
		return cMauve
	case p.status == "new":
		return cGreen
	case p.status == "keep":
		return cYell
	}
	return cText
}

type lrow struct{ kind, key, label, sub, tag string } // kind: size|toggle|keep ; tag: required|recommended|optional|keep

func tagStyle(t string) string {
	switch t {
	case "required":
		return fg(cGreen, i18n.T("required"))
	case "recommended":
		return fg(cBlue, i18n.T("recommended"))
	case "optional":
		return fg(cDim, i18n.T("optional"))
	}
	return ""
}

// ───────────────────────── model ─────────────────────────
type frameMsg time.Time

type model struct {
	w, h     int
	fit      fitLevel // how much chrome the current grid can afford; View() sets it
	flow     []step
	idx      int
	pick     picker
	input    string
	yes      bool
	inputErr string
	encStage int    // 0 ask · 1 passphrase · 2 confirm
	pass1    string // first passphrase entry (mock; never persisted)
	encErr   string
	picks    map[string]string
	variant  string

	state string // intro|transition|wizard|install|done|failed
	phase int
	frame int
	help  bool

	hwOK         bool   // hardware detected & classified
	hwHybrid     bool   // hybrid iGPU+dGPU present → ask GPU mode
	hwBIOS       bool   // legacy BIOS boot → hard-block past the hardware step (backend is UEFI-only)
	hwSecureBoot bool   // Secure Boot on → block Review (Limine is unsigned)
	diskHint     string // set when no installable disk was found (VMD / generic message)
	doneSel      int    // reboot / poweroff / shell
	exitAction   string // done screen choice: "reboot" | "poweroff" | "" (exit to shell)

	diskDev                                      string // chosen target disk
	diskTotal                                    int    // its size in GiB (layout math)
	diskBytes                                    int64  // its exact size in bytes (dual-unit display)
	netOnline                                    bool
	netStage                                     int    // 0 pick SSID · 1 Wi-Fi password (offline only)
	pwStage                                      int    // 0 enter · 1 confirm
	pw1                                          string // first password entry (mock; never persisted)
	pwErr                                        string
	netErr                                       string // Wi-Fi connect failure (offline flow)
	failStep                                     string
	logPath                                      string
	qrStr                                        string
	pwHash                                       string
	luksPass                                     string
	netSSID                                      string
	istream                                      *installStream
	hwCPU, hwGPU, hwMem, hwFW, hwDisk, hwProfile string

	showSocialQR             bool
	qrDiscord, qrReddit, qrX string

	introSpr           harmonica.Spring
	introPos, introVel float64
	introHold          int
	transSpr           harmonica.Spring
	transPos, transVel float64
	enterSpr           harmonica.Spring
	enterPos, enterVel float64
	progSpr            harmonica.Spring
	progress, progVel  float64
	installAt          int
	installLog         []string // granular, scrolling command output

	// guided partition layout
	diskG int
	// existing is the disk's actual current partition table, loaded when the
	// partition step runs regardless of strategy. Review's wipe gate uses
	// len(existing) > 0 to decide whether to require the typed "ERASE"
	// acknowledgement before launching a whole-disk install.
	existing []part
	kept     []part // existing partitions kept on an alongside install
	// reclaim holds leftover ryoku/ryokuboot partitions from a prior failed run.
	// Under alongside the backend frees them (RYOKU_RECLAIM_LEFTOVERS) after the
	// typed-ERASE ack, so reclaimG counts toward the usable free figure.
	reclaim []part
	// retried is set by the failed screen's "r": the next backend run has to be
	// allowed to clean up the debris the failed one left (see installEnv).
	retried                    bool
	reclaimG                   int
	gpt                        bool   // target disk has a GPT label (alongside requires it)
	bitlocker                  bool   // target disk carries a BitLocker partition (review warning)
	freeG                      int    // largest contiguous free region (GiB) for alongside (excludes reclaimG)
	regionStart, regionEnd     int64  // that region's first/last sector, from the probe (exported at install)
	probeVerdict, probeMessage string // alongside probe verdict + human cause (rendered as the block reason)
	espKind                    string // existing ESP kind
	existingBoot               string // existing OS EFI binary, or "none"
	espCount                   int
	espFreeKiB                 int64
	// carve (in-installer resize): resizeParts is the backend's per-partition
	// shrinkability report; when non-empty on an alongside disk the layout page
	// offers the carve picker. carvePart indexes the chosen partition to shrink
	// (-1 = use the existing gap instead), and carveTakeMiB is how much space to
	// arrow out of it for Ryoku. Exported as RYOKU_RESIZE_PART/TAKE_MIB.
	resizeParts                 []resizePart
	carvePart                   int
	carveTakeMiB                int64
	espG, swapG                 int
	snapshots, sepHome, backups bool
	lsel                        int
	sAnim, sVel                 float64
	sSpr                        harmonica.Spring
	// apps step: the keep/remove map over apps.go's rows (nil until the step
	// first loads, then seeded with the shipped defaults), the selected row,
	// and the scroll offset into the flattened row list.
	keep  map[string]bool
	alsel int
	aoff  int
	// wipeStage gates the Review->install transition for a whole-disk wipe on a
	// populated disk: 0 = idle, 1 = user typing "ERASE", 2 = confirmed. installEnv
	// emits RYOKU_WIPE_CONFIRMED=1 only when wipeStage == 2.
	wipeStage  int
	eraseInput string
	// abortArmed/abortAt gate the install-state ctrl+c: a single press only arms a
	// warning; a second press within abortWindow kills the backend and quits, so a
	// stray ctrl+c can't SIGPIPE-kill the backend mid-write and half-write the disk.
	abortArmed bool
	abortAt    time.Time
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// installSteps names the backend's install phases for the progress panel. It is
// a function, not a package-level var, so the names resolve in the language
// main() picked (a var initializer would run before i18n.Use).
func installSteps() []string {
	return []string{
		i18n.T("Partitioning the disk"), i18n.T("Creating filesystems and btrfs subvolumes"),
		i18n.T("Mounting the target"), i18n.T("Installing the base system"),
		i18n.T("Configuring the system"), i18n.T("Installing the Limine bootloader"),
	}
}

func newModel() model {
	variant := sysVariant()
	m := model{
		variant:  variant,
		flow:     stepsForVariant(variant),
		picks:    map[string]string{},
		state:    "intro",
		introSpr: harmonica.NewSpring(harmonica.FPS(30), 3.4, 1.0),
		transSpr: harmonica.NewSpring(harmonica.FPS(30), 5.5, 0.85),
		enterSpr: harmonica.NewSpring(harmonica.FPS(30), 7.0, 0.9),
		progSpr:  harmonica.NewSpring(harmonica.FPS(30), 6.0, 0.7),
		sSpr:     harmonica.NewSpring(harmonica.FPS(30), 9.0, 0.8),
		enterPos: 1,
	}
	if kb := os.Getenv("RYOKU_KB_PRESET"); kb != "" {
		m.picks["keyboard"] = kb
	}
	hw := ensureHW()
	m.hwOK, m.hwHybrid = hw.ok, hw.hybrid
	m.hwBIOS, m.hwSecureBoot = hw.bios, hw.secureBoot
	m.hwCPU, m.hwGPU, m.hwMem = hw.cpu, hw.gpu, hw.mem
	m.hwFW, m.hwDisk, m.hwProfile = hw.fw, hw.disk, hw.profile
	if d := sysDisks(); len(d) > 0 {
		m.diskDev, m.diskTotal, m.diskBytes = d[0].key, sysDiskSize(d[0].key), sysDiskBytes(d[0].key)
	} else {
		m.diskHint = diskHint()
	}
	// Defaults must flow even when a choice step is skipped programmatically.
	m.picks["compositor"] = wm.Providers()[0]
	if isVoidVariant(variant) {
		m.picks["compositor"] = wm.ProviderNiri
	}
	m.picks["browser"] = "firefox"
	m.picks["login-shell"] = "fish"
	m.netOnline = netOnline()
	m.loadStep()
	return m
}

func (m *model) cur() step { return m.flow[m.idx] }

func (m *model) resetLayoutChoices() {
	m.espG, m.swapG = 1, 16
	m.snapshots, m.sepHome, m.backups = !isVoidVariant(m.variant), true, false
}

func (m *model) loadStep() {
	m.enterPos, m.enterVel = 0, 0
	s := m.cur()
	switch s.kind {
	case kSelect:
		items := s.items
		var disabled map[string]bool
		if s.key == "disk" {
			// strategies depend on what is actually on the picked disk; the static
			// step list was built before any disk was chosen. A hard-blocked
			// alongside is offered dimmed and non-committable, with its cause inline.
			dl := sysDiskLayout(m.diskDev)
			items = diskStrategiesFor(dl)
			if alongsideBlockReason(dl) != "" {
				disabled = map[string]bool{"alongside": true}
			}
		}
		if s.key == "compositor" && isVoidVariant(m.variant) {
			disabled = map[string]bool{wm.ProviderHyprland: true}
		}
		if s.key == "locale" {
			// keep the keyboard's own country's locales in front so the intended
			// pick leads, not the Belarusian be_BY that shares the "be" prefix.
			items = promoteKbLocales(items, m.picks["keyboard"])
		}
		m.pick = newPicker(items, s.numbered)
		m.pick.disabled = disabled
		if s.key == "compositor" && isVoidVariant(m.variant) {
			for pos, idx := range m.pick.matches {
				if m.pick.items[idx].key == wm.ProviderNiri {
					m.pick.cursor = pos
					break
				}
			}
		}
		m.pick.height = m.listRows()
	case kPartition:
		m.diskG = m.diskTotal
		m.resetLayoutChoices()
		m.lsel, m.sAnim = 0, 0
		dl := sysDiskLayout(m.diskDev) // real partitions, used by alongside layout AND the wipe gate
		m.existing = dl.parts
		m.gpt, m.bitlocker = dl.gpt, dl.bitlocker
		m.resizeParts, m.carvePart, m.carveTakeMiB = nil, -1, 0
		if m.picks["disk"] == "alongside" {
			// Kept = every real partition (the existing OS stays); reclaim = the
			// probe's verified failed-install debris, whose GiB the backend frees
			// and folds into usable space.
			m.kept, m.freeG = dl.parts, dl.freeG
			m.reclaim, m.reclaimG = nil, 0
			m.regionStart, m.regionEnd = dl.regionStart, dl.regionEnd
			m.probeVerdict, m.probeMessage = dl.probeVerdict, dl.probeMessage
			m.espKind, m.existingBoot = dl.espKind, dl.existingBoot
			m.espCount, m.espFreeKiB = dl.espCount, dl.espFreeKiB
			for _, p := range dl.leftovers {
				m.reclaim = append(m.reclaim, p)
				m.reclaimG += p.size
			}
			// The resize probe drives the carve picker. When neither the existing gap
			// nor the reclaimable debris covers the install, pre-select the largest
			// partition we can shrink so carve is the obvious path, not a dead end.
			m.resizeParts = probeResize(m.diskDev)
			if m.freeG+m.reclaimG < minRootGiB+alongsideBootGiB {
				m.selectDefaultCarve()
			}
		} else {
			m.kept, m.reclaim, m.reclaimG, m.freeG = nil, nil, 0, 0
			m.regionStart, m.regionEnd = 0, 0
			m.probeVerdict, m.probeMessage = "", ""
			m.espKind, m.existingBoot, m.espFreeKiB = "", "", 0
		}
		m.clampSwapToLayout() // keep default swap within the layout (backend-consistent)
	case kPass:
		m.pwStage, m.pw1, m.pwErr, m.input = 0, "", "", ""
	case kNet:
		m.netOnline = netOnline()
		m.netStage, m.input = 0, ""
		// No Wi-Fi scan on a bundled image: it needs no network, netBody says so,
		// and scanning here only makes the step wait on the radio.
		if !m.netOnline && !m.hasOfflineRepo() {
			m.pick = newPicker(ssids(), true)
			m.pick.height = 5
		}
	case kApps:
		// Seed once; back/forward keeps the user's edits, and the row list is
		// static so the cursor and scroll offset survive a revisit too.
		if m.keep == nil {
			m.keep = appDefaultsForVariant(m.variant)
			m.alsel, m.aoff = appsRowIdx(m.appsRows(), 0), 0
		}
	case kInput:
		// returning to a text step (esc back, or Review edit) shows what was
		// entered before, not a blank field.
		m.input, m.inputErr = m.picks[s.key], ""
	default:
		m.input, m.yes = "", false
		m.inputErr, m.encStage, m.pass1, m.encErr = "", 0, "", ""
		m.wipeStage, m.eraseInput = 0, ""
		if s.key == "review" {
			m.netOnline = netOnline() // re-probe: Review needs it only on a non-offline image
		}
	}
}

// validation mirrors installer/script/ryoku-install (valid_hostname / valid_username)
var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
var userRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

func validInput(key, v string) (bool, string) {
	switch key {
	case "hostname":
		if hostRe.MatchString(v) {
			return true, ""
		}
		return false, i18n.T("letters, digits, dashes, no leading or trailing dash")
	case "username":
		if userRe.MatchString(v) {
			return true, ""
		}
		return false, i18n.T("lowercase; start with a letter or _")
	}
	return true, ""
}

func (m model) tickCmd() tea.Cmd {
	d := 80 * time.Millisecond
	fast := m.state == "intro" || m.state == "transition" || m.state == "install" ||
		(m.state == "wizard" && (m.enterPos < 0.99 || m.cur().kind == kPartition))
	if fast {
		d = 33 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func (m model) Init() tea.Cmd { return m.tickCmd() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.cur().kind == kSelect {
			m.pick.height = m.listRows()
		}
		return m, nil
	case frameMsg:
		m.frame++
		m.phase = (m.phase + 1) % smallW
		switch m.state {
		case "intro":
			m.introPos, m.introVel = m.introSpr.Update(m.introPos, m.introVel, 1.0)
			if m.introPos > 0.995 {
				if m.introHold++; m.introHold > 165 { // hold the brand ~5s longer
					m.state, m.transPos, m.transVel, m.enterPos = "transition", 0, 0, 1
				}
			}
		case "transition":
			m.transPos, m.transVel = m.transSpr.Update(m.transPos, m.transVel, 1.0)
			if m.transPos > 0.99 {
				m.state = "welcome"
			}
		case "wizard":
			m.enterPos, m.enterVel = m.enterSpr.Update(m.enterPos, m.enterVel, 1.0)
			if m.cur().kind == kPartition {
				rows := m.layoutRows()
				tgt := m.sAnim
				if r := rows[m.lsel]; r.kind == "size" {
					if v, _, mx, _, _ := m.rowSpec(r.key); mx > 0 {
						tgt = float64(v) / float64(mx)
					}
				}
				m.sAnim, m.sVel = m.sSpr.Update(m.sAnim, m.sVel, tgt)
			}
		case "install":
			m.progress, m.progVel = m.progSpr.Update(m.progress, m.progVel, float64(m.installAt)/float64(len(installSteps())))
		}
		return m, m.tickCmd()
	case installStepMsg:
		m.installAt = int(msg)
		if m.istream != nil {
			return m, m.istream.wait()
		}
		return m, nil
	case installLineMsg:
		m.installLog = append(m.installLog, string(msg))
		if m.istream != nil {
			return m, m.istream.wait()
		}
		return m, nil
	case installDoneMsg:
		if msg.err != nil {
			m.failInstall(installSteps()[clamp(m.installAt, 0, len(installSteps())-1)])
		} else {
			m.installAt, m.progress, m.state = len(installSteps()), 1, "done"
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.onKey(msg.String())
	case tea.MouseWheelMsg: // scroll wheel moves the active selection
		switch msg.Button {
		case tea.MouseWheelUp:
			return m.onKey("up")
		case tea.MouseWheelDown:
			return m.onKey("down")
		}
	}
	return m, nil
}

func (m model) onKey(k string) (tea.Model, tea.Cmd) {
	if k == "ctrl+c" {
		if m.state == "install" {
			// A single ctrl+c would SIGPIPE-kill the backend mid-write with no trap
			// (bash EXIT traps do not run on untrapped fatal signals), leaving a
			// half-written disk. Require a confirming second press within the window;
			// the footer shows the warning. On confirm, kill the backend group and
			// wait for it to reap before quitting so nothing scribbles after we exit.
			if m.abortArmed && time.Since(m.abortAt) <= abortWindow {
				m.istream.kill()
				return m, tea.Quit
			}
			m.abortArmed, m.abortAt = true, time.Now()
			return m, nil
		}
		return m, tea.Quit
	}
	if m.help { // help overlay open
		if k == "?" || k == "esc" || k == "q" || k == "enter" {
			m.help = false
		}
		return m, nil
	}
	switch m.state {
	case "intro":
		m.state, m.transPos, m.transVel, m.enterPos = "transition", 0, 0, 1
		return m, m.tickCmd()
	case "install":
		return m, nil
	case "done":
		switch k {
		case "up", "k":
			m.doneSel = clamp(m.doneSel-1, 0, 2)
		case "down", "j":
			m.doneSel = clamp(m.doneSel+1, 0, 2)
		case "1":
			m.doneSel = 0
		case "2":
			m.doneSel = 1
		case "3":
			m.doneSel = 2
		case "enter":
			m.exitAction = []string{"reboot", "poweroff", "shell"}[m.doneSel]
			return m, tea.Quit
		case "q":
			return m, tea.Quit
		}
		return m, nil
	case "failed":
		switch k {
		case "r":
			m.state, m.installAt, m.installLog = "install", 0, nil
			m.abortArmed = false
			m.retried = true
			m.progress, m.progVel = 0, 0
			return m, tea.Batch(m.tickCmd(), m.startInstall())
		case "q":
			return m, tea.Quit
		}
		return m, nil
	case "welcome":
		if m.showSocialQR {
			if k == "s" || k == "esc" || k == "enter" || k == "q" {
				m.showSocialQR = false
			}
			return m, nil
		}
		switch k {
		case "enter":
			m.state, m.enterPos, m.enterVel = "wizard", 0, 0
			return m, m.tickCmd()
		case "s":
			m.ensureSocialQR()
			m.showSocialQR = true
		case "q":
			return m, tea.Quit
		}
		return m, nil
	case "transition":
		m.state, m.transPos = "welcome", 1
		return m, nil
	}

	s := m.cur()
	typing := s.kind == kInput || (s.kind == kSelect && m.pick.searching) ||
		(s.kind == kConfirm && s.key == "encryption" && m.encStage > 0) ||
		(s.kind == kConfirm && s.key == "review" && m.wipeStage > 0) ||
		s.kind == kPass || (s.kind == kNet && m.netStage == 1)
	if k == "?" && !typing {
		m.help = true
		return m, nil
	}
	if k == "q" && !typing {
		return m, tea.Quit
	}
	if k == "esc" && !typing {
		m.back()
		return m, nil
	}

	switch s.kind {
	case kNotice:
		if k == "enter" {
			m.advance()
		}
	case kSelect:
		if done, sel := m.pick.update(k); done {
			// Commit from the picker's own list, never s.items: loadStep swaps
			// per-disk items into the picker only (the step keeps its static
			// placeholder), so the two can diverge - selecting "alongside" used
			// to store "whole" and the review offered to erase the disk.
			m.picks[s.key] = m.pick.items[sel].key
			if s.key == "diskpick" { // WIRE: real device + size from lsblk/blockdev
				m.diskDev = m.pick.items[sel].key
				m.diskTotal = diskSizeOf(m.diskDev)
				m.diskBytes = sysDiskBytes(m.diskDev)
			}
			if s.key == "keyboard" {
				if keymapRelaunch(m.picks["keyboard"]) {
					return m, tea.Quit // session relaunches cage under the chosen layout
				}
				applyKeymap(m.picks["keyboard"])
			}
			// The locale IS the language choice: there is no second question to
			// ask. Switching here retranslates the rest of the wizard at once,
			// and steps() is rebuilt because its titles and descriptions were
			// resolved in the previous language.
			if s.key == "locale" {
				if code := i18n.Use(m.picks["locale"]); code != "" {
					m.flow = steps()
				}
			}
			m.advance()
		}
	case kPass:
		switch k {
		case "esc":
			if m.pwStage == 1 {
				m.pwStage, m.input, m.pwErr = 0, "", ""
			} else {
				m.back()
			}
		case "enter":
			if m.pwStage == 0 {
				if m.input == "" {
					m.pwErr = i18n.T("password cannot be empty")
				} else {
					m.pw1, m.input, m.pwStage, m.pwErr = m.input, "", 1, ""
				}
			} else if m.input != m.pw1 {
				m.pw1, m.input, m.pwStage, m.pwErr = "", "", 0, i18n.T("did not match, try again")
			} else if h, err := hashPassword(m.pw1); err != nil {
				// The hash is computed in-process (password.go), so the only way
				// here is a kernel that cannot produce random bytes for the salt.
				// Report it and keep the user on this screen rather than handing
				// the backend an empty RYOKU_PASSWORD_HASH that only dies at
				// preflight, after the whole wizard has been walked.
				m.pw1, m.input, m.pwStage, m.pwErr = "", "", 0, i18n.Tf("could not hash the password: %s", err)
			} else {
				m.pwHash = h
				m.picks["password"], m.pw1, m.input, m.pwStage = "set", "", "", 0
				m.advance()
			}
		default:
			m.editInput(k, &m.input)
		}
	case kNet:
		// An offline image is as ready as a connected one, so it takes the same
		// path. netBody already renders the "no network needed, enter to continue"
		// card for it; gating this on netOnline alone meant enter did nothing and
		// the user was stuck at a Wi-Fi picker the card never mentioned.
		if m.netOnline || m.hasOfflineRepo() {
			if k == "enter" {
				m.picks["network"] = "online"
				if m.hasOfflineRepo() {
					m.picks["network"] = "offline"
				}
				m.advance()
			}
		} else if m.netStage == 1 { // Wi-Fi passphrase entry (offline flow)
			switch k {
			case "esc":
				m.netStage, m.input, m.netErr = 0, "", ""
			case "enter":
				// wifiConnect's result was ignored and netOnline forced true, so a
				// wrong passphrase silently marched on to install with no network.
				// Require a real connection AND a live probe; otherwise stay here.
				if wifiConnect(m.netSSID, m.input) && netOnline() {
					m.netOnline, m.picks["network"], m.input, m.netErr = true, "wifi", "", ""
					m.advance()
				} else {
					m.input, m.netErr = "", i18n.T("could not connect (wrong passphrase?)")
				}
			default:
				m.netErr = ""
				m.editInput(k, &m.input)
			}
		} else if k == "r" && !m.pick.searching {
			m.netOnline = netOnline()
			if !m.netOnline {
				m.pick = newPicker(ssids(), true)
				m.pick.height = 5
			}
		} else if done, sel := m.pick.update(k); done {
			// Resolve from the picker's OWN items: re-running ssids() here indexed a
			// freshly rescanned (possibly reordered/shorter) list with the stale
			// picker index, connecting to the wrong network.
			if sel < len(m.pick.items) {
				m.netSSID = m.pick.items[sel].key
			}
			m.netStage, m.input, m.netErr = 1, "", ""
		}
	case kInput:
		switch k {
		case "esc":
			// input steps set `typing`, so the global esc->back above is skipped.
			// Handle it here (as kPass/kNet do) so the footer's "esc back" works
			// and hostname/username are not one-way dead ends.
			m.inputErr, m.input = "", ""
			m.back()
		case "enter":
			v := m.input
			if v == "" {
				v = s.deflt
			}
			if ok, msg := validInput(s.key, v); !ok {
				m.inputErr = msg // block advance on invalid input
				return m, nil
			}
			m.inputErr, m.picks[s.key] = "", v
			m.advance()
		default:
			m.inputErr = ""
			m.editInput(k, &m.input)
		}
	case kConfirm:
		if s.key == "encryption" && m.encStage > 0 { // passphrase entry
			if k == "esc" {
				m.encStage, m.input, m.encErr = 0, "", ""
				return m, nil
			}
			m.editInput(k, &m.input)
			if k == "enter" {
				if m.encStage == 1 {
					if m.input == "" {
						m.encErr = i18n.T("passphrase cannot be empty")
					} else {
						m.pass1, m.input, m.encStage, m.encErr = m.input, "", 2, ""
					}
				} else { // confirm
					if m.input != m.pass1 {
						m.pass1, m.input, m.encStage, m.encErr = "", "", 1, i18n.T("did not match, try again")
					} else {
						m.luksPass = m.pass1
						m.picks["encryption"], m.pass1, m.input, m.encStage = "LUKS", "", "", 0
						m.advance()
					}
				}
			}
			return m, nil
		}

		// Review wipe-confirm sub-stage: when whole is picked on a populated
		// disk, m.yes + Enter transitions to a typed "ERASE" prompt instead of
		// starting the install. Each keystroke builds m.eraseInput; Enter when
		// eraseInput == "ERASE" sets wipeStage = 2 and launches the backend
		// (installEnv then emits RYOKU_WIPE_CONFIRMED=1). Esc cancels the prompt.
		if s.key == "review" && m.wipeStage == 1 {
			switch k {
			case "esc":
				m.wipeStage, m.eraseInput = 0, ""
			case "backspace":
				if n := len(m.eraseInput); n > 0 {
					m.eraseInput = m.eraseInput[:n-1]
				}
			case "enter":
				if strings.EqualFold(m.eraseInput, "ERASE") && m.reviewBlockReason() == "" {
					m.wipeStage = 2
					m.state, m.installAt, m.installLog = "install", 0, nil
					m.progress, m.progVel = 0, 0
					return m, tea.Batch(m.tickCmd(), m.startInstall())
				}
			default:
				if r := []rune(k); len(r) == 1 && r[0] >= 0x20 {
					m.eraseInput += k
				}
			}
			return m, nil
		}
		switch k {
		case "left", "right", "h", "l", "tab":
			m.yes = !m.yes
		case "y":
			m.yes = true
		case "n":
			m.yes = false
		case "enter":
			if s.key == "encryption" {
				if m.yes {
					m.encStage, m.input, m.encErr = 1, "", "" // collect a passphrase
				} else {
					m.picks["encryption"] = "none"
					m.advance()
				}
			} else if m.yes {
				// Defense in depth: never start the install without a committed
				// disk strategy. partReady gates Tab past partitions, so this
				// should be unreachable; if it ever is, refuse to launch rather
				// than ship an empty RYOKU_DISK_STRATEGY (backend fails closed).
				strat := m.picks["disk"]
				if strat != "whole" && strat != "alongside" {
					return m, nil
				}
				// Real-hardware gates: Secure Boot (Limine unsigned) and an offline
				// live system (no offline package source) are hard blocks. reviewBody
				// shows the reason; refuse to launch rather than fail mid-install.
				if s.key == "review" && m.reviewBlockReason() != "" {
					return m, nil
				}
				// A destructive step needs the typed "ERASE" acknowledgement before
				// any backend command runs: a whole-disk wipe on a populated disk, or
				// an alongside install that must free leftover ryoku/ryokuboot
				// partitions. Enter from Yes enters that sub-stage instead of launching.
				if m.needsEraseAck() {
					m.wipeStage, m.eraseInput = 1, ""
					return m, nil
				}
				m.state, m.installAt, m.installLog = "install", 0, nil
				m.progress, m.progVel = 0, 0
				return m, tea.Batch(m.tickCmd(), m.startInstall())
			} else {
				// Enter on the default No is a no-op: it used to quit and silently
				// discard the whole session. Only an explicit Yes proceeds; esc/q
				// still leave.
				return m, nil
			}
		default:
			if s.key == "review" && len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
				m.jumpToActive(int(k[0] - '1')) // edit a step from Review
			}
		}
	case kApps:
		m.appsKey(k)
	case kPartition:
		m.partKey(k)
	case kInfo:
		if k == "enter" && !m.hwBIOS { // BIOS is a hard block: no UEFI, no install
			m.advance()
		}
	}
	return m, nil
}

func (m *model) editInput(k string, dst *string) {
	switch k {
	case "backspace":
		if len(*dst) > 0 {
			*dst = (*dst)[:len(*dst)-1]
		}
	case "enter":
	case "space":
		// bubbletea v2 delivers the space bar as "space", not " ". Without this
		// Wi-Fi passphrases and user/LUKS passwords silently drop their spaces
		// (a post-install lockout). hostname/username still reject spaces via
		// validInput's visible check, so appending here is safe for them too.
		*dst += " "
	default:
		if r := []rune(k); len(r) == 1 && r[0] >= 0x20 {
			*dst += k
		}
	}
}

func (m *model) advance() {
	for m.idx < len(m.flow)-1 {
		m.idx++
		if m.stepActive(m.idx) {
			m.loadStep()
			return
		}
	}
}

func (m *model) back() {
	if m.idx == 0 && m.cur().kind == kNotice {
		m.state = "welcome"
		return
	}
	for m.idx > 0 {
		m.idx--
		if m.stepActive(m.idx) {
			m.loadStep()
			return
		}
	}
}

// jumpToActive jumps to the n-th active step (0-based), used from Review to edit.
func (m *model) jumpToActive(n int) {
	c := 0
	for i := range m.flow {
		if !m.stepActive(i) {
			continue
		}
		if c == n {
			m.idx = i
			m.loadStep()
			return
		}
		c++
	}
}

// stepActive lets steps be skipped: the GPU-mode screen only shows on a hybrid
// iGPU+dGPU machine.
func (m model) stepActive(i int) bool {
	if m.flow[i].key == "gpu" {
		p := m.picks["profile"]
		return m.hwHybrid && (p == "amd-nvidia" || p == "intel-nvidia")
	}
	if m.flow[i].key == "compositor" {
		return len(m.flow[i].items) > 1 // one provider: nothing to choose, skip
	}
	return true
}

// failInstall writes the captured install output to a stable log path and renders
// a QR to the support URL so the user can read it or share it.
func (m *model) failInstall(step string) {
	m.state, m.failStep = "failed", step
	m.logPath = "/var/log/ryoku-install.log"
	var b strings.Builder
	for _, l := range m.installLog {
		b.WriteString(l + "\n")
	}
	if err := os.WriteFile(m.logPath, []byte(b.String()), 0o644); err != nil {
		dir := filepath.Join(os.TempDir(), "ryoku-install")
		_ = os.MkdirAll(dir, 0o755)
		m.logPath = filepath.Join(dir, "install.log")
		_ = os.WriteFile(m.logPath, []byte(b.String()), 0o644)
	}
	var q strings.Builder
	qrterminal.GenerateHalfBlock(ryokuSupportURL, qrterminal.L, &q)
	m.qrStr = q.String()
}

// ───────────────────────── guided layout logic ─────────────────────────
func (m model) keptG() int {
	n := 0
	for _, k := range m.kept {
		n += k.size
	}
	return n
}

// diskPopulated reports whether the target disk currently holds any partition.
// The wipe-confirm gate uses this to decide whether the user must type "ERASE"
// on Review before a whole-disk install starts. Populated is read once into
// m.existing when the partition step loads, so this is a cheap field check.
func (m model) diskPopulated() bool { return len(m.existing) > 0 }

// freeAlongside is the usable free space (GiB) for an alongside install. When
// carving, that space is the gap the backend will open by shrinking the chosen
// partition (the take), which everything downstream — availRoot, swapCeil, the
// readiness gate — then treats exactly like a pre-existing region. Otherwise it
// is the detected free region plus any leftover ryoku/ryokuboot partitions the
// backend reclaims before it measures (matching the backend's own order).
func (m model) freeAlongside() int {
	if m.carving() {
		return m.carveTakeG()
	}
	return m.freeG + m.reclaimG
}

// ── carve (in-installer resize) ──────────────────────────────────────────────
// Arrow steps for scrubbing the take: a fine 1 GiB nudge, a coarse 10 GiB jump.
const carveStepMiB = 1024
const carveBigStepMiB = 10 * 1024

func clampI64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// carveFloorMiB is the least we can take: Ryoku's own minimum (the 2 GiB boot
// partition plus the root floor). Below this the carved gap can't hold an install.
func (m model) carveFloorMiB() int64 { return int64(alongsideBootGiB+minRootGiB) * 1024 }

// carveCeilMiB is the most we can take from p: its headroom above the smallest
// the filesystem can shrink to (size − min).
func (m model) carveCeilMiB(p resizePart) int64 { return p.sizeMiB - p.minMiB }

// carveDefaultMiB is the take we pre-select: half the partition's headroom, capped
// at 64 GiB, never below Ryoku's floor.
func (m model) carveDefaultMiB(p resizePart) int64 {
	def := m.carveCeilMiB(p) / 2
	if def > 64*1024 {
		def = 64 * 1024
	}
	return clampI64(def, m.carveFloorMiB(), m.carveCeilMiB(p))
}

// carveablePart reports whether p can be shrunk enough to fit Ryoku: the probe
// says the filesystem may shrink, and its headroom clears our floor.
func (m model) carveablePart(p resizePart) bool {
	return p.shrinkable && m.carveCeilMiB(p) >= m.carveFloorMiB()
}

// carveUI reports whether the layout page shows the carve picker: an alongside
// install with a shrinkability report to drive it. Verified leftovers no longer
// suppress it -- they render as their own freed segments while the carve picker
// still shrinks a living partition beside them.
func (m model) carveUI() bool {
	return m.picks["disk"] == "alongside" && len(m.resizeParts) > 0
}

// carving reports whether a partition is currently selected to be shrunk.
func (m model) carving() bool {
	return m.carveUI() && m.carvePart >= 0 && m.carvePart < len(m.resizeParts)
}

func (m model) carveTakeG() int { return int(m.carveTakeMiB / 1024) }

// hasCarveable reports whether any listed partition can be carved.
func (m model) hasCarveable() bool {
	for _, p := range m.resizeParts {
		if m.carveablePart(p) {
			return true
		}
	}
	return false
}

// selectDefaultCarve pre-selects the largest carveable partition (with its default
// take), used when there is no free region big enough to install into.
func (m *model) selectDefaultCarve() {
	best := -1
	for i, p := range m.resizeParts {
		if m.carveablePart(p) && (best < 0 || p.sizeMiB > m.resizeParts[best].sizeMiB) {
			best = i
		}
	}
	if best >= 0 {
		m.carvePart = best
		m.carveTakeMiB = m.carveDefaultMiB(m.resizeParts[best])
	}
}

// carveScrub selects the carve target at resizeParts[i] (defaulting its take on
// first touch) and moves the take by deltaMiB, clamped to [floor, headroom]. A
// zero delta just selects. Swap is re-clamped because the root pool moved.
func (m *model) carveScrub(i int, deltaMiB int64) {
	if i < 0 || i >= len(m.resizeParts) {
		return
	}
	if m.carvePart != i {
		m.carvePart = i
		m.carveTakeMiB = m.carveDefaultMiB(m.resizeParts[i])
	}
	m.carveTakeMiB = clampI64(m.carveTakeMiB+deltaMiB, m.carveFloorMiB(), m.carveCeilMiB(m.resizeParts[i]))
	m.clampSwapToLayout()
}

// carveIndex reads the resizeParts index out of a "carveN" layout-row key.
func carveIndex(key string) int {
	var i int
	fmt.Sscanf(key, "carve%d", &i)
	return i
}

// needsEraseAck reports whether Review must demand the typed "ERASE"
// acknowledgement before launching: a whole-disk wipe on a populated disk, or an
// alongside install that must free leftover Ryoku partitions. Both are
// destructive, so both gate on the same confirmation.
func (m model) needsEraseAck() bool {
	switch m.picks["disk"] {
	case "whole":
		return m.diskPopulated()
	case "alongside":
		return len(m.reclaim) > 0
	}
	return false
}

func (m model) espMode() string {
	if m.picks["disk"] != "alongside" {
		return "auto"
	}
	if m.probeVerdict == "create-esp" {
		return "dedicated" // no existing ESP; Ryoku creates its own in the free space
	}
	if m.espFreeKiB < 0 {
		return "auto"
	}
	if m.espFreeKiB >= 8192 {
		return "shared"
	}
	return "dedicated"
}

// availRoot is the new root size before subtracting swap. Alongside always
// reserves the fixed 2 GiB FAT boot partition.
// The swapfile is carved from root, so usable root is availRoot - swap.
func (m model) availRoot() int {
	var a int
	if m.picks["disk"] == "alongside" {
		a = m.freeAlongside() - alongsideBootGiB // free region + reclaimable Ryoku parts, minus the boot partition
	} else {
		a = m.diskG - m.keptG() - m.espG
	}
	if a < 0 {
		a = 0
	}
	return a
}

// swapCeil caps the swapfile size: at most 64 GiB, and always leaving at least
// minRootGiB of usable root (both strategies) so swap can never starve the
// system partition. Mirrors the backend's root floor.
func (m model) swapCeil() int {
	mx := m.availRoot() - minRootGiB
	if mx > 64 {
		mx = 64
	}
	if mx < 0 {
		mx = 0
	}
	return mx
}

// clampSwapToLayout pins the default swap into what the current layout can give
// it (root floor + swapfile must both fit), so partReady's free-space gate
// matches the backend's ryoku_min_root_gib and Tab never advances a layout the
// backend would reject mid-install. Called on partition load; a no-op on a
// roomy disk.
func (m *model) clampSwapToLayout() { m.swapG = clamp(m.swapG, 0, m.swapCeil()) }

func (m model) layoutRows() []lrow {
	var rows []lrow
	if m.carveUI() {
		// The existing partitions show in the content map above; the chooser here
		// is a radio group: use the existing gap (when it's big enough) or carve a
		// shrinkable partition. Non-shrinkable partitions surface as dimmed reasons
		// in the body, not as rows.
		if m.freeG >= minRootGiB+alongsideBootGiB {
			rows = append(rows, lrow{"region", "region", i18n.T("Use free space"), i18n.Tf("%d GiB is already free", m.freeG), ""})
		}
		nCarve := 0
		for _, p := range m.resizeParts {
			if m.carveablePart(p) {
				nCarve++
			}
		}
		for i, p := range m.resizeParts {
			if m.carveablePart(p) {
				// One candidate is the common case and needs no qualifier; with
				// several, each row names the partition it takes the space from.
				label := i18n.T("Space for Ryoku")
				if nCarve > 1 {
					label += " · " + strings.TrimPrefix(p.dev, "/dev/")
				}
				rows = append(rows, lrow{"carve", fmt.Sprintf("carve%d", i), label, "", ""})
			}
		}
	} else {
		for i, k := range m.kept {
			rows = append(rows, lrow{"keep", fmt.Sprintf("keep%d", i), k.dev, "", "keep"})
		}
	}
	// Verified leftovers show as reclaimed (freed), never kept, so the user sees
	// they will be removed and their space folded into the new root -- alongside
	// the carve picker or the kept list, whichever the disk offers.
	for i, r := range m.reclaim {
		rows = append(rows, lrow{"reclaim", fmt.Sprintf("reclaim%d", i), r.dev, i18n.T("previous Ryoku, will be freed"), "reclaim"})
	}
	if m.picks["disk"] != "alongside" {
		rows = append(rows, lrow{"size", "esp", i18n.T("ESP size"), "/boot · fat32", "required"}) // alongside boot is fixed at 2 GiB
	}
	rows = append(rows, lrow{"size", "swap", i18n.T("Swap (swapfile)"), i18n.T("@swap · 0 = none · carved from root"), "optional"})
	if !isVoidVariant(m.variant) {
		rows = append(rows, lrow{"toggle", "snap", i18n.T("Snapshots & rollback"), "@snapshots → /.snapshots", "recommended"})
	}
	rows = append(rows,
		lrow{"toggle", "home", i18n.T("Separate /home"), "@home → /home", "optional"},
		lrow{"toggle", "backups", i18n.T("Backups"), "@backups → /.backups", "optional"},
	)
	return rows
}

func (m model) rowSpec(key string) (int, int, int, int, int) {
	switch key {
	case "esp":
		return m.espG, 1, 4, 1, 1
	case "swap":
		return m.swapG, 0, m.swapCeil(), 2, 8
	}
	return 0, 0, 1, 1, 1
}
func (m *model) setRow(key string, v int) {
	_, mn, mx, _, _ := m.rowSpec(key)
	v = clamp(v, mn, mx)
	switch key {
	case "esp":
		m.espG = v
		m.swapG = clamp(m.swapG, 0, m.swapCeil()) // ESP eats space; keep swap in range
	case "swap":
		m.swapG = v
	}
}
func (m *model) toggle(key string) {
	switch key {
	case "snap":
		m.snapshots = !m.snapshots
	case "home":
		m.sepHome = !m.sepHome
	case "backups":
		m.backups = !m.backups
	}
}
func (m model) toggleOn(key string) bool {
	switch key {
	case "snap":
		return m.snapshots
	case "home":
		return m.sepHome
	case "backups":
		return m.backups
	}
	return false
}
func (m model) keepIndex(key string) int {
	var i int
	fmt.Sscanf(key, "keep%d", &i)
	return i
}
func (m model) reclaimIndex(key string) int {
	var i int
	fmt.Sscanf(key, "reclaim%d", &i)
	return i
}

func (m *model) partKey(k string) {
	rows := m.layoutRows()
	r := rows[m.lsel]
	switch k {
	case "up", "k":
		m.lsel = clamp(m.lsel-1, 0, len(rows)-1)
	case "down", "j":
		m.lsel = clamp(m.lsel+1, 0, len(rows)-1)
	case "left", "h":
		if r.kind == "size" {
			v, _, _, st, _ := m.rowSpec(r.key)
			m.setRow(r.key, v-st)
		} else if r.kind == "carve" {
			m.carveScrub(carveIndex(r.key), -carveStepMiB)
		}
	case "right", "l":
		if r.kind == "size" {
			v, _, _, st, _ := m.rowSpec(r.key)
			m.setRow(r.key, v+st)
		} else if r.kind == "carve" {
			m.carveScrub(carveIndex(r.key), +carveStepMiB)
		}
	case "shift+left", "H":
		if r.kind == "size" {
			v, _, _, _, bg := m.rowSpec(r.key)
			m.setRow(r.key, v-bg)
		} else if r.kind == "carve" {
			m.carveScrub(carveIndex(r.key), -carveBigStepMiB)
		}
	case "shift+right", "L":
		if r.kind == "size" {
			v, _, _, _, bg := m.rowSpec(r.key)
			m.setRow(r.key, v+bg)
		} else if r.kind == "carve" {
			m.carveScrub(carveIndex(r.key), +carveBigStepMiB)
		}
	case "enter", "space":
		switch r.kind {
		case "toggle":
			m.toggle(r.key)
		case "carve":
			m.carveScrub(carveIndex(r.key), 0) // select this partition, keep its take
		case "region":
			m.carvePart = -1 // fall back to the existing gap
			m.clampSwapToLayout()
		}
	case "a": // reset the editable sizes and toggles to recommended
		m.resetLayoutChoices()
	case "tab":
		if !m.partReady() {
			return
		}
		m.picks["partitions"] = m.layoutSummary()
		m.advance()
	}
}

// partBlockReason says why the chosen layout can't install yet, or "" when it can,
// so a blocked Tab on the partition step can explain itself instead of doing nothing.
func (m model) partBlockReason() string {
	if m.diskG < minDiskGiB {
		return i18n.Tf("Disk is %dG; Ryoku needs at least %dG. Press esc to pick another.", m.diskG, minDiskGiB)
	}
	switch m.picks["disk"] {
	case "whole":
		return ""
	case "alongside":
		if !m.gpt {
			// The backend's alongside path is GPT-only (it appends a partition and
			// reads GPT partlabels); an MBR disk with free space would pass the TUI
			// and die at backend stage 1. Fail here with the same guidance.
			return i18n.T("alongside needs a GPT disk; press esc and choose 'Erase whole disk'.")
		}
		// The probe knows the exact hard blocker (no Windows ESP, unreadable table)
		// that a generic free-space message would hide; surface it verbatim. These
		// are reclaim-independent, unlike the free-space gate below (which folds in
		// reclaimable ryoku/ryokuboot leftovers the backend frees before measuring).
		if m.probeVerdict == "no-esp" || m.probeVerdict == "error" {
			return m.probeMessage
		}
		// When carving, freeAlongside() is the take — clamped to at least Ryoku's
		// floor — so this gate passes. It only bites with no usable gap and nothing
		// selected to carve.
		if free, need := m.freeAlongside(), minRootGiB+alongsideBootGiB; free < need {
			if m.carveUI() {
				if m.hasCarveable() {
					return i18n.T("Not enough free space; select a partition below and use ←/→ to set how much space Ryoku gets.")
				}
				// No gap AND nothing shrinkable: the per-partition reasons show below.
				return i18n.T("No free space, and no partition here can be shrunk safely (see reasons below). Press esc to pick another disk or 'Erase whole disk'.")
			}
			return i18n.Tf("Only %dG free; alongside needs %dG (a %dG root plus a %dG boot partition). Shrink an existing partition first, or press esc and choose 'Erase whole disk'.", free, need, minRootGiB, alongsideBootGiB)
		}
		return ""
	default:
		return i18n.T("Choose a disk strategy first (press esc).")
	}
}

// partReady reports whether the chosen layout can be installed.
func (m model) partReady() bool { return m.partBlockReason() == "" }

// ───────────────────────── apps (keep/remove checklist) ─────────────────────────
// arow is one display line of the apps step: a group header or a keep/remove
// row for one appRow id. The cursor only ever lands on app rows.
type arow struct {
	kind string // hdr|app
	id   string // appRow id when kind == app
	text string // group label when kind == hdr
}

func (m model) appsRows() []arow {
	var out []arow
	for _, g := range appGroupsForVariant(m.variant) {
		if g[0].Group != "" {
			out = append(out, arow{kind: "hdr", text: g[0].Group})
		}
		for _, r := range g {
			out = append(out, arow{kind: "app", id: r.ID})
		}
	}
	return out
}

// appsRowIdx maps a cursor position to its row, skipping headers: it walks
// up from n to the next app row, then falls back to the last app row, so the
// end/G keys can never park on a header or past the list.
func appsRowIdx(rows []arow, n int) int {
	last := -1
	for i, r := range rows {
		if r.kind != "app" {
			continue
		}
		if i >= n {
			return i
		}
		last = i
	}
	return max(last, 0)
}

func (m *model) appsMove(d int) {
	rows := m.appsRows()
	n := m.alsel
	for {
		n += d
		if n < 0 || n >= len(rows) {
			return
		}
		if rows[n].kind == "app" {
			m.alsel = n
			return
		}
	}
}

func (m *model) appsToggle(id string) {
	r, ok := appRowByIDForVariant(m.variant, id)
	if !ok {
		return
	}
	if r.Req != "" {
		// Refuse with the reason, not silence: the row is marked REQUIRED for
		// a named desktop feature, and the user deserves to read which.
		m.inputErr = i18n.Tf("required: %s", r.Req)
		return
	}
	m.keep[id] = !m.keep[id]
	m.inputErr = ""
}

func (m *model) appsKey(k string) {
	rows := m.appsRows()
	switch k {
	case "up", "k":
		m.appsMove(-1)
	case "down", "j":
		m.appsMove(1)
	case "pgup", "ctrl+u":
		m.appsMove(-m.listRows())
	case "pgdown", "ctrl+d":
		m.appsMove(m.listRows())
	case "home", "g":
		m.alsel = appsRowIdx(rows, 0)
	case "end", "G":
		m.alsel = appsRowIdx(rows, len(rows)-1)
	case "enter", "space":
		m.appsToggle(rows[m.alsel].id)
	case "a": // back to the shipped defaults
		m.keep, m.inputErr = appDefaultsForVariant(m.variant), ""
	case "tab":
		m.picks["apps"] = appsSummaryForVariant(m.variant, m.keep)
		m.advance()
	}
	m.appsFixScroll()
}

// appsFixScroll keeps the cursor inside the visible window, oldest-style:
// pull the offset up when the cursor escapes above, slide it down below.
func (m *model) appsFixScroll() {
	vis := m.listRows() - 2 // the step keeps two intro lines above the list
	if vis < 3 {
		vis = 3
	}
	if m.alsel < m.aoff {
		m.aoff = m.alsel
	}
	if m.alsel >= m.aoff+vis {
		m.aoff = m.alsel - vis + 1
	}
	if m.aoff < 0 {
		m.aoff = 0
	}
}

func (m model) appsBody(inner int) string {
	var b strings.Builder
	rows := m.appsRows()
	vis := m.listRows() - 2
	if vis < 3 {
		vis = 3
	}
	end := min(m.aoff+vis, len(rows))
	labelW := 18
	for _, r := range appRowsForVariant(m.variant) {
		if lw := dw(r.Name); lw > labelW {
			labelW = lw
		}
	}
	for i := m.aoff; i < end; i++ {
		r := rows[i]
		prefix := "  "
		if i == m.alsel {
			prefix = bold(gradColor(float64(m.phase)/float64(smallW-1)), gSelCur)
		}
		if r.kind == "hdr" {
			b.WriteString(fg(cSub, strings.ToUpper(r.text)) + "\n")
			continue
		}
		ar, _ := appRowByIDForVariant(m.variant, r.id)
		on := m.keep[r.id]
		var box, name, tag string
		switch {
		case ar.Req != "":
			box = fg(cGreen, "[■] ")
			name = bold(cText, ar.Name)
			tag = fg(cYell, i18n.T("REQUIRED"))
		case on:
			box = fg(cGreen, "[x] ")
			name = fg(cText, ar.Name)
			tag = fg(cDim, i18n.T("keep"))
		default:
			box = fg(cDim, "[ ] ")
			name = fg(cSub, ar.Name)
			tag = fg(cRed, i18n.T("remove"))
		}
		gut := 2 + 4 + labelW + 1 + dw(tag)
		sub := ""
		if avail := inner - gut; avail >= 8 {
			text := ar.Sub
			if text == "" {
				text = ar.Req
			}
			sub = "  " + fg(cDim, truncW(text, avail))
		}
		b.WriteString(prefix + box + padTo(name, labelW) + " " + padTo(tag, dw(i18n.T("REQUIRED"))) + sub + "\n")
	}
	if m.inputErr != "" {
		b.WriteString(fg(cYell, "⚠ "+m.inputErr) + "\n")
	}
	dropped := 0
	for _, r := range appRowsForVariant(m.variant) {
		if r.Req == "" && !m.keep[r.ID] {
			dropped++
		}
	}
	removable := 0
	for _, r := range appRowsForVariant(m.variant) {
		if r.Req == "" {
			removable++
		}
	}
	b.WriteString("\n" + fg(cDim, i18n.Tf("%d of %d apps removed", dropped, removable)))
	return strings.TrimRight(b.String(), "\n")
}

// reviewBlockReason reports why the install cannot start from Review, or "" when
// it can. Secure Boot (Limine is unsigned) is a hard block. Connectivity is only
// a block when this ISO has no baked package closure: an offline ISO carries the
// whole set in a file:// repo and installs with the network down, which is the
// whole point of baking it. Gating on netOnline alone refused those installs at
// the last step with "installs are online-only" even though the handoff right
// below (system.go: RYOKU_ONLINE=0) was already set up to do it. Fail here
// honestly, not mid-install.
func (m model) reviewBlockReason() string {
	if m.hwSecureBoot {
		return i18n.T("Secure Boot is enabled -- disable Secure Boot in firmware setup (Limine is unsigned), then reboot the installer.")
	}
	if !m.netOnline && !m.hasOfflineRepo() {
		return i18n.T("No internet connection, and this image has no offline package set. Go back to the Network step to connect.")
	}
	return ""
}

func (m model) layoutSummary() string {
	n := 3 // @, @log and @pkg always (what the backend actually creates)
	if m.sepHome {
		n++
	}
	if m.snapshots {
		n++
	}
	if m.backups {
		n++
	}
	if m.swapG > 0 {
		n++ // the swapfile lives in its own @swap
	}
	if len(m.kept) == 0 {
		return i18n.Tf("wiped · btrfs %dsv", n)
	}
	return i18n.Tf("alongside · btrfs %dsv", n)
}

// layoutSegs builds the disk-bar segments: kept + (new boot/ESP) + root + free.
func (m model) layoutSegs() []part {
	segs := append([]part(nil), m.kept...)
	bootG, bootDev, bootFlags := m.espG, "ESP", "esp"
	if m.picks["disk"] == "alongside" {
		bootG = alongsideBootGiB
		if m.espMode() == "dedicated" {
			bootDev, bootFlags = "Ryoku ESP", "esp"
		} else {
			bootDev, bootFlags = "boot", "xbootldr"
		}
	}
	segs = append(segs, part{dev: bootDev, size: bootG, fs: "vfat", mount: "/boot", flags: bootFlags, status: "new"})
	rootUsable := m.availRoot() - m.swapG
	if rootUsable < 0 {
		rootUsable = 0
	}
	segs = append(segs, part{dev: "root", size: rootUsable, fs: "btrfs", mount: "/", flags: "-", status: "new"})
	if m.swapG > 0 {
		segs = append(segs, part{dev: "swap", size: m.swapG, fs: "swap", mount: "[SWAP]", flags: "swap", status: "new"})
	}
	return segs
}

// gibRound rounds a MiB count to the nearest whole GiB (bar segments are integers).
func gibRound(mib int64) int { return int((mib + 512) / 1024) }

// ryokuSegs are the segments Ryoku drops into a gapG-GiB gap: the 2 GiB boot
// partition, the btrfs root, and a swap partition when one is configured.
func (m model) ryokuSegs(gapG int) []part {
	flags := "xbootldr"
	if m.espMode() == "dedicated" {
		flags = "esp"
	}
	segs := []part{{dev: "boot", size: alongsideBootGiB, fs: "vfat", mount: "/boot", flags: flags, status: "new"}}
	root := gapG - alongsideBootGiB - m.swapG
	if root < 0 {
		root = 0
	}
	segs = append(segs, part{dev: "root", size: root, fs: "btrfs", mount: "/", flags: "-", status: "new"})
	if m.swapG > 0 {
		segs = append(segs, part{dev: "swap", size: m.swapG, fs: "swap", mount: "[SWAP]", flags: "swap", status: "new"})
	}
	return segs
}

// existingSegs is the disk's current contents as bar segments: one per partition
// (shaded by usage) plus the trailing free region, from the resize probe.
func (m model) existingSegs() []part {
	var segs []part
	for _, p := range m.resizeParts {
		flags := "-"
		if p.fs == "vfat" || p.fs == "fat32" {
			flags = "esp"
		}
		segs = append(segs, part{
			dev: p.name(), size: gibRound(p.sizeMiB), fs: p.fs,
			used: gibRound(p.usedMiB), flags: flags, status: "keep",
		})
	}
	// Verified leftovers show as their own dimmed freed segments, kept distinct
	// from the living partitions above and from the trailing free region.
	for _, r := range m.reclaim {
		segs = append(segs, part{dev: r.dev, size: r.size, status: "reclaim"})
	}
	if m.freeG > 0 {
		segs = append(segs, part{dev: "free", size: m.freeG, status: "free"})
	}
	return segs
}

// mapSegs is what the content bar draws on an alongside carve disk: the existing
// layout by default, morphing live into the post-carve preview as you scrub — the
// chosen partition shrinks and Ryoku's segments grow in the freed space.
func (m model) mapSegs() []part {
	base := m.existingSegs()
	if !m.carving() {
		return base
	}
	take := m.carveTakeG()
	var out []part
	for i, s := range base {
		if i == m.carvePart {
			s.size -= take
			if s.size < 0 {
				s.size = 0
			}
			out = append(out, s)
			out = append(out, m.ryokuSegs(take)...)
		} else {
			out = append(out, s)
		}
	}
	return out
}

// selSeg maps the selected editable row to a disk-bar segment (for the ▲ marker).
func (m model) selSeg() int {
	rows := m.layoutRows()
	if m.lsel >= len(rows) {
		return -1
	}
	r := rows[m.lsel]
	segs := m.layoutSegs()
	switch {
	case r.kind == "keep":
		return m.keepIndex(r.key) // kept come first in segs
	case r.key == "esp":
		for i, s := range segs {
			if strings.Contains(s.flags, "esp") && s.status == "new" {
				return i
			}
		}
	case r.key == "swap":
		for i, s := range segs {
			if s.fs == "swap" {
				return i
			}
		}
	}
	return -1
}

// ───────────────────────── sizing ─────────────────────────
//
// A live-ISO user cannot resize their terminal. On the console path the grid is
// framebuffer pixels over console font cell; on the kiosk path it is output
// pixels over foot's cell. Either way the installer is handed a size and has to
// live in it, so every screen is built to a budget and then clamped to it.
//
// fitLevel is how much chrome the current grid can afford. View() renders at the
// richest level that fits and degrades only as far as it must, so a roomy
// terminal still gets the full banner and the step rail.
type fitLevel struct {
	noTagline   bool // drop "for the sake of power & beauty"
	smallHeader bool // one-line wordmark instead of the three-row block banner
	tightCard   bool // no vertical card padding, no spacer rows, no key hints
	noRail      bool // drop the install-steps rail and give the width to the card
}

// fitLevels: richest first, height chrome only. fittedFrame walks these until the
// frame fits. The rail is NOT in this chain -- it costs columns, not rows, so a
// merely short terminal must not lose it (see railFits).
var fitLevels = []fitLevel{
	{},
	{noTagline: true},
	{noTagline: true, smallHeader: true},
	{noTagline: true, smallHeader: true, tightCard: true},
}

const railW = 22 // rail inner 18 + padding 2 + border 2

// railFits: the rail plus its gutter plus a card wide enough to hold the widest
// review row ("⚠ this writes the layout below to /dev/nvme0n1", 46 cells) and the
// card's own border and padding. Below that the rail's 24 columns are worth more
// to the card than to the step list.
func (m model) railFits() bool { return m.w >= railW+2+46+6 }

// innerW is the card's content width: whatever is left after the card's own
// border and padding, and the rail when it is showing.
func (m model) innerW() int {
	chrome := 6 // card border (2) + horizontal padding (4)
	if !m.fit.noRail {
		chrome += railW + 2 // rail plus the two-column gutter
	}
	return clamp(m.w-chrome, 24, 72)
}

// listRows sizes a picker to the rows actually left for it, so a short grid gets
// a short list instead of a list that pushes the footer off the screen.
func (m model) listRows() int {
	chrome := 14 // header 6 + footer 2 + card 4 + title 2
	if m.fit.smallHeader {
		chrome -= 4
	} else if m.fit.noTagline {
		chrome--
	}
	if m.fit.tightCard {
		chrome -= 2
	}
	return clamp(m.h-chrome, 3, 16)
}

// frameBox measures a rendered block in display cells: the widest line, and the
// number of lines. This is the yardstick every fit decision uses.
func frameBox(s string) (w, h int) {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for _, l := range ls {
		if d := dw(l); d > w {
			w = d
		}
	}
	return w, len(ls)
}

// fitBlock forces a block inside w×h. Lines are truncated ANSI-aware so a styled
// row cannot bleed colour, and rows past the budget are dropped: from the top
// when bottom is set (a confirm screen keeps its buttons reachable), otherwise
// from the bottom. lipgloss.Place does NOT truncate -- it returns oversized
// content unchanged (position.go: `if gap <= 0 { return str }`) -- so without
// this the VT wraps the overflow and shears the layout.
func fitBlock(s string, w, h int, bottom bool) string {
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		if dw(l) > w {
			ls[i] = ansi.Truncate(l, w, "")
		}
	}
	if h > 0 && len(ls) > h {
		if bottom {
			ls = ls[len(ls)-h:]
		} else {
			ls = ls[:h]
		}
	}
	return strings.Join(ls, "\n")
}

// boxLines pads every line out to w AND truncates the ones past it, so a card
// built from this content is exactly w wide. padLines alone only pads, which let
// one long row widen a card past its budget and push the whole frame off-screen.
func boxLines(s string, w int) string {
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		if dw(l) > w {
			l = ansi.Truncate(l, w, "")
		}
		ls[i] = padTo(l, w)
	}
	return strings.Join(ls, "\n")
}

// ───────────────────────── view ─────────────────────────
func (m model) View() tea.View {
	if m.w == 0 {
		return tea.NewView("")
	}
	if m.w < minTermW || m.h < minTermH {
		// A console this small cannot show the destructive-write warning legibly, and
		// on a live VT there is no window to drag bigger -- so name the real levers
		// (a smaller console font, or installing from the shell) instead of asking
		// for a resize the user cannot perform.
		msg := lipgloss.JoinVertical(lipgloss.Center,
			bold(cYell, i18n.T("This screen is too small for the installer")), "",
			fg(cText, i18n.Tf("Ryoku needs at least %d × %d; this console is %d × %d.", minTermW, minTermH, m.w, m.h)), "",
			fg(cSub, i18n.T("On a terminal: make the window bigger.")),
			fg(cSub, i18n.T("On the console: switch to a smaller font, e.g.")),
			fg(cBlue, "setfont ter-v16n"), "",
			fg(cSub, i18n.T("Or install from the shell with:")+" ryoku-install"))
		v := tea.NewView(fitBlock(lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, msg), m.w, m.h, false))
		v.AltScreen, v.BackgroundColor, v.ForegroundColor = true, cBg, cText
		return v
	}
	if m.help {
		foot := lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("?", i18n.T("close"))+fg(cDim, "    ")+keyHint("esc", i18n.T("close")))
		v := tea.NewView(m.frameWithFooter(m.helpBody(), "\n"+foot))
		v.AltScreen, v.BackgroundColor, v.ForegroundColor = true, cBg, cText
		return v
	}
	frame := m.fittedFrame()
	v := tea.NewView(frame)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = cBg
	v.ForegroundColor = cText
	v.WindowTitle = i18n.T("Ryoku installer")
	return v
}

// fittedFrame renders the current screen at the richest chrome level that fits
// the grid, degrading only as far as it must: the tagline goes first, then the
// three-row block banner, then the spacer rows and key hints, then the step rail.
// A roomy terminal never leaves level 0 and looks exactly as before.
//
// This is what makes an 80-column boot console usable instead of sheared. The
// live VT is commonly 80 columns until KMS hands over a bigger framebuffer, and
// the user has no way to resize it, so the installer adapts instead of asking.
func (m model) fittedFrame() string {
	build := func() string {
		switch m.state {
		case "intro":
			return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, m.viewIntro())
		case "transition":
			a := frameLines(lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, m.viewIntro()), m.h)
			b := frameLines(m.welcomeFrame(), m.h)
			off := clamp(int(m.transPos*float64(m.h)), 0, m.h)
			return strings.Join(append(a, b...)[off:off+m.h], "\n")
		case "welcome":
			if m.showSocialQR {
				return m.welcomeQR()
			}
			return m.welcomeFrame()
		case "install", "done", "failed":
			return m.frameWithFooter(m.viewCentered(), "")
		}
		return m.frameWizard()
	}
	var frame string
	noRail := !m.railFits()
	for i, f := range fitLevels {
		f.noRail = noRail
		m.fit = f
		frame = build()
		if w, h := frameBox(frame); (w <= m.w && h <= m.h) || i == len(fitLevels)-1 {
			break
		}
	}
	// Last resort: even the leanest level can be too tall for a truly tiny grid, and
	// a frame wider or taller than the terminal is what wraps and shears the whole
	// layout. Clamp unconditionally so that can never reach the screen, keeping the
	// bottom of a confirm screen so its Yes/No buttons stay reachable.
	return fitBlock(frame, m.w, m.h, m.onConfirmScreen())
}

func frameLines(s string, h int) []string {
	ls := strings.Split(s, "\n")
	for len(ls) < h {
		ls = append(ls, "")
	}
	return ls[:h]
}

// onConfirmScreen reports whether the body ends in Yes/No buttons, which decides
// which end of an over-tall card survives clamping. cur() indexes flow directly,
// so this stays bounds-safe: View() also runs on screens reached before (and
// after) the wizard, and a panic here would strand the installer.
func (m model) onConfirmScreen() bool {
	return m.idx >= 0 && m.idx < len(m.flow) && m.flow[m.idx].kind == kConfirm
}

func (m model) frameWizard() string { return m.frameWithFooter(m.viewWizard(), m.footer()) }

func (m model) frameWithFooter(body, footer string) string {
	hdr := m.header()
	foot := footer
	if foot == "" {
		switch m.state {
		case "install":
			msg := fg(cDim, i18n.T("installing…  ·  ctrl+c to abort"))
			if m.abortArmed && time.Since(m.abortAt) <= abortWindow {
				msg = bold(cRed, i18n.T("press ctrl+c again to abort -- leaves a half-written disk"))
			}
			foot = lipgloss.PlaceHorizontal(m.w, lipgloss.Center, msg)
		case "done":
			foot = lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("↑↓", i18n.T("choose"))+fg(cDim, "    ")+keyHint("enter", i18n.T("confirm"))+fg(cDim, "    ")+keyHint("q", i18n.T("quit")))
		case "failed":
			foot = lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("r", i18n.T("retry"))+fg(cDim, "    ")+keyHint("q", i18n.T("quit")))
		}
	}
	// Width is clamped here so no row can ever be wider than the grid. Height is
	// deliberately NOT clamped: fittedFrame measures the natural height to pick a
	// chrome level, and clamping here would make every level look like it fits.
	hdr = fitBlock(hdr, m.w, 0, false)
	foot = fitBlock(foot, m.w, 0, false)
	body = fitBlock(body, m.w, 0, false)
	bodyH := m.h - lipgloss.Height(hdr) - lipgloss.Height(foot)
	if bodyH < 1 {
		bodyH = 1
	}
	mid := lipgloss.Place(m.w, bodyH, lipgloss.Center, lipgloss.Center, body)
	return lipgloss.JoinVertical(lipgloss.Left, hdr, mid, foot)
}

// header is the wordmark block. On a short grid it collapses to a single line so
// the four rows of banner and tagline go to the body instead of pushing the
// footer -- which carries the confirm buttons -- off the bottom of the screen.
func (m model) header() string {
	var b string
	if m.fit.smallHeader {
		b = wordmark(false)
		if m.state == "wizard" {
			b += "  " + m.headerProgress()
		}
		return lipgloss.PlaceHorizontal(m.w, lipgloss.Center, fitBlock(b, m.w, 1, false))
	}
	b = lipgloss.JoinVertical(lipgloss.Center, smallBanner(m.phase)...)
	if !m.fit.noTagline {
		b = lipgloss.JoinVertical(lipgloss.Center, b, fg(cSub, i18n.T("for the sake of power & beauty")))
	}
	if m.state == "wizard" {
		b = lipgloss.JoinVertical(lipgloss.Center, b, m.headerProgress())
	}
	return lipgloss.PlaceHorizontal(m.w, lipgloss.Center, fitBlock(b, m.w, 0, false)) + "\n"
}

func (m model) headerProgress() string {
	total, cur := m.activeTotal(), m.activePos()
	w := 28
	filled := clamp((cur+1)*w/total, 0, w)
	bar := fg(cBrand, strings.Repeat(gFull, filled)) + fg(cDim, strings.Repeat(gEmpty, w-filled))
	return bar + fg(cDim, fmt.Sprintf("  %d/%d", cur+1, total))
}

func (m *model) ensureSocialQR() {
	if m.qrDiscord != "" {
		return
	}
	gen := func(u string) string {
		var b strings.Builder
		qrterminal.GenerateHalfBlock(u, qrterminal.L, &b)
		return b.String()
	}
	m.qrDiscord, m.qrReddit, m.qrX = gen(discordURL), gen(redditURL), gen(xURL)
}

func (m model) welcomeFrame() string {
	logo := lipgloss.JoinVertical(lipgloss.Center, faintBig(m.phase)...)
	liveImageLine := i18n.T("the live Arch image now, and this installer turns it into")
	if isVoidVariant(m.variant) {
		liveImageLine = i18n.T("the live Void image now, and this installer turns it into")
	}
	card := sty().Border(border()).BorderForeground(cBlue).Padding(1, 3).Render(
		bold(cBrand, i18n.T("Thank you for installing Ryoku")) + "\n\n" +
			fg(cText, i18n.T("There is no try-before-you-install demo. You are running")) + "\n" +
			fg(cText, liveImageLine) + "\n" +
			fg(cText, i18n.T("Ryoku in a single pass. Nothing is written until you")) + "\n" +
			fg(cText, i18n.T("confirm on the review screen.")))
	di, ri := iconDiscord+" ", iconReddit+" "
	if !nerd { // Private-Use logo glyphs won't render; drop to labels
		di, ri = "", ""
	}
	socials := fg(cBlue, di+"Discord") + fg(cDim, "      ") +
		fg(cBrand, ri+"r/RyokuArch") + fg(cDim, "      ") +
		fg(cText, iconX+" @neur0map")
	block := lipgloss.JoinVertical(lipgloss.Center, logo, "", "", card, "", socials)
	foot := lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("enter", i18n.T("continue"))+fg(cDim, "      ")+keyHint("s", i18n.T("socials & QR")))
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.Place(m.w, m.h-2, lipgloss.Center, lipgloss.Center, block), "\n"+foot)
}

func (m model) welcomeQR() string {
	if ascii { // QR needs block glyphs, fall back to a plain link list
		block := lipgloss.JoinVertical(lipgloss.Center, bold(cBrand, i18n.T("Join the Ryoku community")), "",
			fg(cBlue, "Discord  ")+fg(cText, discordURL),
			fg(cBrand, "Reddit   ")+fg(cText, redditURL),
			fg(cText, "X        ")+fg(cText, xURL))
		foot := lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("s / esc", i18n.T("back")))
		return lipgloss.JoinVertical(lipgloss.Left, lipgloss.Place(m.w, m.h-2, lipgloss.Center, lipgloss.Center, block), "\n"+foot)
	}
	qst := sty().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#ffffff"))
	tile := func(icon, label, url, q string) string {
		var b strings.Builder
		for _, ln := range strings.Split(strings.TrimRight(q, "\n"), "\n") {
			b.WriteString(qst.Render(ln) + "\n")
		}
		head := label
		if nerd {
			head = icon + " " + label
		}
		return lipgloss.JoinVertical(lipgloss.Center, bold(cText, head), "",
			strings.TrimRight(b.String(), "\n"), "", fg(cDim, url))
	}
	d := tile(iconDiscord, "Discord", discordURL, m.qrDiscord)
	r := tile(iconReddit, "Reddit", redditURL, m.qrReddit)
	x := tile(iconX, "X", xURL, m.qrX)
	var row string
	if m.w >= 88 {
		row = lipgloss.JoinHorizontal(lipgloss.Top, d, "   ", r, "   ", x)
	} else {
		row = lipgloss.JoinVertical(lipgloss.Center, d, "", r, "", x)
	}
	block := lipgloss.JoinVertical(lipgloss.Center, bold(cBrand, i18n.T("Join the Ryoku community")), "", row)
	foot := lipgloss.PlaceHorizontal(m.w, lipgloss.Center, keyHint("s / esc", i18n.T("back")))
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.Place(m.w, m.h-2, lipgloss.Center, lipgloss.Center, block), "\n"+foot)
}

func (m model) viewIntro() string {
	banner := lipgloss.JoinVertical(lipgloss.Center, bigBanner(m.introPos, m.phase)...)
	tag := ""
	if m.introPos > 0.45 {
		tag = fg(cSub, i18n.T("for the sake of power & beauty"))
	}
	bw := 34
	fill := clamp(int(m.introPos*float64(bw)), 0, bw)
	bar := fg(cBrand, strings.Repeat(gFull, fill)) + fg(cDim, strings.Repeat(gEmpty, bw-fill))
	status := fg(cSub, i18n.T("preparing installer")+strings.Repeat(".", (m.frame/4)%4))
	if m.introPos > 0.995 {
		status = fg(cGreen, i18n.T("ready"))
	}
	return lipgloss.JoinVertical(lipgloss.Center, banner, "", tag, "", "", bar, "", status)
}

func (m model) viewWizard() string {
	s := m.cur()
	inner := m.innerW()

	var c strings.Builder
	c.WriteString(bold(cBrand, s.title) + "\n\n")
	switch {
	case s.kind == kNotice:
		for _, d := range s.desc {
			c.WriteString(fg(cSub, truncW(d, inner)) + "\n")
		}
	case s.kind == kConfirm && s.key == "review":
		// The step numbers live in the rail, so the hint only promises them when the
		// rail is actually on screen; a tight grid drops the hint altogether.
		hint := "\n" + fg(cDim, i18n.T("press 1-9 to edit a step (numbered in the rail)"))
		switch {
		case m.fit.tightCard:
			hint = ""
		case m.fit.noRail:
			hint = "\n" + fg(cDim, i18n.T("press 1-9 to edit a step"))
		}
		c.WriteString(m.reviewBody(inner) + "\n\n" + m.confirmButtons() + hint)
	case s.kind == kPartition:
		c.WriteString(m.partBody(inner))
	case s.kind == kApps:
		for _, d := range s.desc {
			c.WriteString(fg(cSub, truncW(d, inner)) + "\n")
		}
		c.WriteString("\n" + m.appsBody(inner))
	case s.kind == kInfo:
		c.WriteString(m.infoBody(inner))
	case s.kind == kNet:
		c.WriteString(m.netBody(inner))
	case s.kind == kPass:
		c.WriteString(m.passBody(inner))
	case s.kind == kSelect && s.key == "gpu":
		for _, d := range s.desc {
			c.WriteString(fg(cSub, truncW(d, inner)) + "\n")
		}
		c.WriteString("\n" + m.pick.view(inner, m.phase) + "\n")
		if len(m.pick.matches) > 0 {
			for _, ln := range gpuDetails(m.pick.items[m.pick.matches[m.pick.cursor]].key) {
				c.WriteString(ln + "\n")
			}
		}
		c.WriteString(fg(cDim, i18n.T("↺ changeable later, re-run the GPU setup after install")))
	default:
		for _, d := range s.desc {
			c.WriteString(fg(cSub, truncW(d, inner)) + "\n")
		}
		if s.key == "diskpick" && m.diskHint != "" {
			c.WriteString("\n" + sty().Foreground(cYell).Width(inner).Render("⚠ "+m.diskHint) + "\n")
		}
		c.WriteString("\n")
		switch s.kind {
		case kSelect:
			c.WriteString(m.pick.view(inner, m.phase))
		case kInput:
			c.WriteString(inputBox(m.input, s.placeholder, s.password) + "\n")
			if m.inputErr != "" {
				c.WriteString(fg(cRed, "⚠ "+m.inputErr) + "\n")
			} else {
				c.WriteString(fg(cDim, i18n.T("enter to accept")) + "\n")
			}
		case kConfirm:
			if s.key == "encryption" && m.encStage > 0 {
				label := i18n.T("Set a LUKS passphrase:")
				if m.encStage == 2 {
					label = i18n.T("Confirm passphrase:")
				}
				c.WriteString(fg(cText, label) + "\n" + inputBox(m.input, "", true) + "\n")
				if m.encErr != "" {
					c.WriteString(fg(cRed, "⚠ "+m.encErr) + "\n")
				}
				c.WriteString(fg(cDim, i18n.T("enter to continue · esc cancel")))
			} else {
				c.WriteString(m.confirmButtons())
			}
		}
	}

	// boxLines, not padLines: a row longer than the budget used to widen the card,
	// push the frame past the grid, and let the VT wrap it -- which is what sheared
	// the layout and pushed the rail off the left edge of the screen.
	lines := strings.Split(boxLines(c.String(), inner), "\n")
	show := clamp(int(m.enterPos*float64(len(lines))+0.5), 0, len(lines))
	for i := show; i < len(lines); i++ {
		lines[i] = strings.Repeat(" ", inner)
	}
	pad := 1
	if m.fit.tightCard {
		pad = 0
	}
	card := sty().Border(border()).BorderForeground(cBlue).Padding(pad, 2).Render(strings.Join(lines, "\n"))
	if m.fit.noRail {
		return card
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, m.rail(), "  ", card)
}

// ───────────────────────── guided partition view ─────────────────────────
// humanSize renders a byte count in BOTH units the partition UI needs: binary
// GiB (what the layout math uses) and decimal GB (what the drive is sold as),
// e.g. "465.7 GiB (500 GB)", so a 500 GB drive never looks like it shrank.
func humanSize(bytes int64) string {
	if bytes <= 0 {
		return i18n.T("unknown size")
	}
	return fmt.Sprintf("%.1f GiB (%.0f GB)", float64(bytes)/(1<<30), float64(bytes)/1e9)
}

func (m model) partBody(inner int) string {
	var b strings.Builder
	size := humanSize(m.diskBytes)
	if m.diskBytes <= 0 {
		size = fmt.Sprintf("%d GiB", m.diskG)
	}
	b.WriteString(fg(cSub, fmt.Sprintf("%s · %s · UEFI · ", m.diskDev, size)) +
		fg(cText, "Limine") + "\n")
	segs, sel := m.layoutSegs(), m.selSeg()
	barLabel := ""
	if m.carveUI() {
		// Show the disk's real contents; it morphs into the post-carve preview live.
		segs, sel = m.mapSegs(), -1
		barLabel = i18n.T("on this disk now")
		if m.carving() {
			rp := m.resizeParts[m.carvePart]
			barLabel = i18n.Tf("%s (%s) shrinks · the freed space becomes Ryoku", strings.TrimPrefix(rp.dev, "/dev/"), rp.name())
		}
	}
	b.WriteString(m.diskBar(segs, inner, sel) + "\n")
	if barLabel != "" {
		b.WriteString(fg(cDim, barLabel) + "\n")
	}
	b.WriteString("\n")

	if r := m.partBlockReason(); r != "" {
		b.WriteString(bold(cRed, "⚠ "+r) + "\n")
		if m.picks["disk"] == "alongside" {
			b.WriteString(fg(cDim, "  "+i18n.T("dual-boot guide:")+" docs.ryoku.dev/docs/dual-boot") + "\n")
		}
		b.WriteString("\n")
	}

	rows := m.layoutRows()
	// One grid for the whole block: a 20-col label, a 5-col state mark (size
	// rows carry a blank spacer), then content - every slider bracket opens at
	// the same column and the row block reads as one table.
	// rowLabelW is the label column, tagW the tag column. Both default to their
	// original literals (20 / 11) and widen to the measured widest item so a
	// longer translation is not truncated (labelStyled) or misaligned (padTo).
	rowLabelW := 20
	tagW := 11
	for _, r := range rows {
		if lw := dw(r.label); lw > rowLabelW {
			rowLabelW = lw
		}
		if tw := dw(tagStyle(r.tag)); tw > tagW {
			tagW = tw
		}
	}
	markCell := func(on bool) string {
		if on {
			return padTo(fg(cGreen, gOn), 5)
		}
		return padTo(fg(cDim, gOff), 5)
	}
	knobW := clamp(inner-46-(rowLabelW-20)-(tagW-11), 8, 20)
	for i, r := range rows {
		sel := i == m.lsel
		prefix := "  "
		if sel {
			prefix = bold(gradColor(float64(m.phase)/float64(smallW-1)), gSelCur)
		}
		switch r.kind {
		case "keep":
			p := m.kept[m.keepIndex(r.key)]
			sw := sty().Foreground(partColor(p)).Render(gFull + gFull)
			info := fmt.Sprintf("%4dG %-5s", p.size, p.fs)
			b.WriteString(prefix + sw + " " + labelStyled(sel, r.label, rowLabelW) + " " + fg(cText, info) + " " + fg(cYell, i18n.T("keep")) + fg(cDim, i18n.T(" · kept")) + "\n")
		case "reclaim":
			p := m.reclaim[m.reclaimIndex(r.key)]
			sw := sty().Foreground(cDim).Render(gFull + gFull)
			info := fmt.Sprintf("%4dG %-5s", p.size, p.fs)
			b.WriteString(prefix + sw + " " + labelStyled(sel, r.label, rowLabelW) + " " + fg(cText, info) + " " + fg(cRed, i18n.T("reclaim")) + fg(cDim, i18n.T(" · freed")) + "\n")
		case "size":
			v, _, mx, _, _ := m.rowSpec(r.key)
			frac := 0.0
			if mx > 0 {
				frac = float64(v) / float64(mx)
			}
			if sel {
				frac = m.sAnim
			}
			fill := clamp(int(frac*float64(knobW)+0.5), 0, knobW)
			knob := fg(cBrand, strings.Repeat(gFull, fill)) + fg(cDim, strings.Repeat(gEmpty, knobW-fill))
			val := fmt.Sprintf("%dG", v)
			if r.key == "swap" && v == 0 {
				val = i18n.T("none")
			}
			b.WriteString(prefix + "   " + labelStyled(sel, r.label, rowLabelW) + " " + strings.Repeat(" ", 5) + " [" + knob + "] " + padTo(bold(cText, val), 6) + "  " + tagStyle(r.tag) + "\n")
		case "region":
			b.WriteString(prefix + "   " + labelStyled(sel, r.label, rowLabelW) + " " + markCell(!m.carving()) + " " + fg(cDim, r.sub) + "\n")
		case "carve":
			rp := m.resizeParts[carveIndex(r.key)]
			active := m.carving() && carveIndex(r.key) == m.carvePart
			line := prefix + "   " + labelStyled(sel, r.label, rowLabelW) + " " + markCell(active)
			if active {
				floor, ceil := m.carveFloorMiB(), m.carveCeilMiB(rp)
				frac := 0.0
				if ceil > floor {
					frac = float64(m.carveTakeMiB-floor) / float64(ceil-floor)
				}
				fill := clamp(int(frac*float64(knobW)+0.5), 0, knobW)
				knob := fg(cBrand, strings.Repeat(gFull, fill)) + fg(cDim, strings.Repeat(gEmpty, knobW-fill))
				line += " [" + knob + "] " + bold(cText, humanSize(m.carveTakeMiB<<20))
			} else {
				line += " " + fg(cDim, i18n.T("←/→ how much space Ryoku gets"))
			}
			b.WriteString(line + "\n")
		default: // toggle
			used := 2 + 3 + rowLabelW + 1 + 5 + 1 + tagW + 2
			sub := truncW(r.sub, max(0, inner-used))
			b.WriteString(prefix + "   " + labelStyled(sel, r.label, rowLabelW) + " " + markCell(m.toggleOn(r.key)) + " " + padTo(tagStyle(r.tag), tagW) + "  " + fg(cDim, sub) + "\n")
		}
	}
	if isVoidVariant(m.variant) {
		b.WriteString("   " + fg(cYell, truncW(i18n.T(voidSnapshotNote), max(0, inner-3))) + "\n")
	}
	// Honesty: the partitions we cannot carve show dimmed, each with the probe's
	// reason, so a disk that can't be carved says exactly why rather than just
	// omitting options.
	if m.carveUI() {
		for _, rp := range m.resizeParts {
			if m.carveablePart(rp) {
				continue
			}
			reason := rp.reason
			if reason == "" {
				reason = i18n.T("cannot be shrunk")
			}
			b.WriteString("     " + fg(cDim, gBad+" "+rp.name()+" — "+reason) + "\n")
		}
	}
	rootUsable := m.availRoot() - m.swapG
	if rootUsable < 0 {
		rootUsable = 0
	}
	note := fg(cSub, i18n.Tf("root %dG", rootUsable))
	if m.swapG > 0 {
		note += fg(cDim, i18n.Tf("  ·  swap %dG", m.swapG))
	}
	note += fg(cDim, i18n.T("  ·  @ /, @log and @pkg always included"))
	b.WriteString("\n" + note)
	return strings.TrimRight(b.String(), "\n")
}

func (m model) netBody(inner int) string {
	if m.hasOfflineRepo() {
		return strings.Join([]string{
			fg(cGreen, gOKtxt+" "+i18n.T("Offline")) + fg(cSub, i18n.T("   bundled image")), "",
			fg(cSub, i18n.T("The whole system is bundled on this image, so no network")),
			fg(cSub, i18n.T("connection is needed. You're good to go.")),
			"", fg(cDim, i18n.T("enter to continue · esc back")),
		}, "\n")
	}
	if m.netOnline {
		return strings.Join([]string{
			fg(cGreen, gOKtxt+" "+i18n.T("Connected")) + fg(cSub, "   "+netInterface()), "",
			fg(cSub, i18n.T("An internet connection is required to download and build")),
			fg(cSub, i18n.T("the system. You're good to go.")),
			"", fg(cDim, i18n.T("enter to continue · esc back")),
		}, "\n")
	}
	if m.netStage == 1 {
		b := fg(cRed, gBad+" "+i18n.T("Not connected")) + "\n\n" +
			fg(cText, i18n.T("Wi-Fi password:")) + "\n" + inputBox(m.input, "", true) + "\n"
		if m.netErr != "" {
			b += fg(cRed, "⚠ "+m.netErr) + "\n"
		}
		return b + "\n" + fg(cDim, i18n.T("enter to connect · esc back"))
	}
	return fg(cRed, gBad+" "+i18n.T("Not connected")) + fg(cSub, i18n.T("   pick a Wi-Fi network, or plug in ethernet and press r")) + "\n\n" + m.pick.view(inner, m.phase) + "\n" + fg(cDim, i18n.T("r to rescan"))
}

func strength(s string) string {
	var lower, upper, digit, sym bool
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			sym = true
		}
	}
	classes := 0
	for _, b := range []bool{lower, upper, digit, sym} {
		if b {
			classes++
		}
	}
	switch {
	case len(s) >= 12 && classes >= 3:
		return fg(cGreen, i18n.T("strong"))
	case len(s) >= 8 && classes >= 2:
		return fg(cYell, i18n.T("ok"))
	default:
		return fg(cRed, i18n.T("weak"))
	}
}

func (m model) passBody(inner int) string {
	label := i18n.T("Set a password:")
	if m.pwStage == 1 {
		label = i18n.T("Confirm password:")
	}
	b := fg(cText, label) + "\n" + inputBox(m.input, "", true) + "\n"
	if m.pwStage == 0 && m.input != "" {
		b += fg(cSub, i18n.T("strength: ")) + strength(m.input) + "\n"
	} else {
		b += "\n"
	}
	if m.pwErr != "" {
		b += fg(cRed, "⚠ "+m.pwErr) + "\n"
	}
	b += "\n" + fg(cDim, i18n.T("used at login; change it anytime · esc back"))
	return b
}

func labelStyled(sel bool, s string, w int) string {
	s = truncW(s, w)
	if sel {
		return padTo(bold(cText, s), w)
	}
	return padTo(fg(cSub, s), w)
}

// diskBar: proportional 2-row graph; selIdx<0 means no selection marker.
func (m model) diskBar(parts []part, w, selIdx int) string {
	total := 0
	for _, p := range parts {
		total += p.size
	}
	if total <= 0 {
		total = 1
	}
	widths := make([]int, len(parts))
	rema := make([]float64, len(parts))
	used := 0
	for i, p := range parts {
		f := float64(p.size) / float64(total) * float64(w)
		iw := int(f)
		if p.size > 0 && iw < 1 {
			iw = 1
		}
		widths[i], rema[i] = iw, f-float64(int(f))
		used += iw
	}
	for used < w {
		mi, mx := -1, -1.0
		for i := range rema {
			if rema[i] > mx {
				mx, mi = rema[i], i
			}
		}
		if mi < 0 {
			break
		}
		widths[mi]++
		rema[mi] = -1
		used++
	}
	for used > w {
		mi := 0
		for i := range widths {
			if widths[i] > widths[mi] {
				mi = i
			}
		}
		if widths[mi] <= 0 {
			break
		}
		widths[mi]--
		used--
	}
	var bar strings.Builder
	start, selStart := 0, 0
	for i, p := range parts {
		s := sty().Foreground(partColor(p))
		if i == selIdx {
			selStart = start
		} else if selIdx >= 0 {
			s = s.Faint(true)
		}
		// On the read-only content map (no selection marker) a partition with known
		// usage shades its used portion solid and its free tail faint, so you can
		// see at a glance how full each one is. New/free segments render solid.
		if selIdx < 0 && p.used > 0 && p.used < p.size && widths[i] >= 2 {
			uw := clamp(int(float64(p.used)/float64(p.size)*float64(widths[i])+0.5), 1, widths[i]-1)
			bar.WriteString(s.Render(strings.Repeat(gFull, uw)))
			bar.WriteString(s.Faint(true).Render(strings.Repeat(gFull, widths[i]-uw)))
		} else {
			bar.WriteString(s.Render(strings.Repeat(gFull, widths[i])))
		}
		start += widths[i]
	}
	row := bar.String()
	out := row + "\n" + row
	if selIdx >= 0 && selIdx < len(parts) {
		mk := []rune(strings.Repeat(" ", w))
		if c := selStart + widths[selIdx]/2; c >= 0 && c < w {
			mk[c] = []rune(gMark)[0]
		}
		out += "\n" + fg(cBrand, string(mk))
	}
	return out
}

// infoBody is the hardware-detection card built from detectHardware(). It also
// surfaces the firmware hard-stops (BIOS boot, Secure Boot) enforced elsewhere.
func (m model) infoBody(inner int) string {
	var lines []string
	if !m.hwOK {
		lines = []string{
			bold(cYell, i18n.T("⚠  Could not fully classify this hardware")), "",
			fg(cText, i18n.T("That is fine. Ryoku runs on any x86_64 UEFI machine and")),
			fg(cText, i18n.T("loads open kernel drivers (amdgpu, i915, nouveau) for")),
			fg(cText, i18n.T("unknown GPUs. You can tune drivers after install.")),
			"",
			fg(cSub, i18n.T("Firmware ")) + m.fwCell(),
			fg(cSub, "GPU      ") + fg(cText, def(m.hwGPU, i18n.T("unclassified"))),
			"",
			fg(cSub, i18n.T("Suggested profile  ")) + bold(cBrand, "vm") + fg(cDim, i18n.T("  (safe generic, pick yours next)")),
		}
	} else {
		hybrid := ""
		if m.hwHybrid {
			hybrid = fg(cYell, i18n.T("  hybrid"))
		}
		lines = []string{
			bold(cBrand, i18n.T("Detected hardware")), "",
			fg(cSub, "CPU      ") + fg(cText, def(m.hwCPU, i18n.T("unknown"))),
			fg(cSub, "GPU      ") + fg(cText, def(m.hwGPU, i18n.T("unknown"))) + hybrid,
			fg(cSub, i18n.T("Memory   ")) + fg(cText, def(m.hwMem, i18n.T("unknown"))),
			fg(cSub, i18n.T("Firmware ")) + m.fwCell(),
			fg(cSub, i18n.T("Disk     ")) + fg(cText, def(m.hwDisk, m.diskDev)),
			"",
			fg(cSub, i18n.T("Suggested profile  ")) + bold(cBrand, def(m.hwProfile, "vm")),
		}
	}
	if g := m.hwGateLines(); len(g) > 0 {
		lines = append(lines, "")
		lines = append(lines, g...)
		lines = append(lines, "", fg(cDim, i18n.T("resolve the above in firmware, then reboot · esc to go back")))
	} else {
		lines = append(lines, "", fg(cDim, i18n.T("enter to continue, esc to go back")))
	}
	return strings.Join(lines, "\n")
}

// fwCell colors the firmware summary red when the machine booted in BIOS mode (a
// hard block), green for UEFI.
func (m model) fwCell() string {
	if m.hwBIOS {
		return fg(cRed, m.hwFW)
	}
	return fg(cGreen, m.hwFW)
}

// hwGateLines are the hard-stop firmware warnings shown on the hardware card and
// enforced elsewhere: BIOS boot (backend is UEFI-only, blocks the hardware step)
// and Secure Boot (Limine is unsigned, blocks Review). Empty when firmware is OK.
func (m model) hwGateLines() []string {
	var out []string
	if m.hwBIOS {
		out = append(out,
			bold(cRed, i18n.T("⚠ Booted in BIOS / legacy mode -- Ryoku installs UEFI-only.")),
			fg(cText, i18n.T("  Reboot, enter firmware setup, disable CSM / Legacy boot and")),
			fg(cText, i18n.T("  enable UEFI boot mode, then start the installer again.")),
		)
	}
	if m.hwSecureBoot {
		out = append(out,
			bold(cRed, i18n.T("⚠ Secure Boot is enabled -- Limine is unsigned.")),
			fg(cText, i18n.T("  Disable Secure Boot in firmware setup, then reboot the installer.")),
		)
	}
	return out
}

func (m model) helpBody() string {
	return strings.Join([]string{
		bold(cBrand, i18n.T("Keys")), "",
		fg(cSub, i18n.T("move       ")) + fg(cText, "↑↓ / j k"),
		fg(cSub, i18n.T("filter     ")) + fg(cText, i18n.T("/  then type   (long lists)")),
		fg(cSub, i18n.T("quick pick ")) + fg(cText, i18n.T("1-9   (numbered menus)")),
		fg(cSub, i18n.T("adjust     ")) + fg(cText, i18n.T("←/→ · shift = ±big   (sliders)")),
		fg(cSub, i18n.T("toggle     ")) + fg(cText, i18n.T("space / enter   (on/off rows)")),
		fg(cSub, i18n.T("partitions ")) + fg(cText, i18n.T("←/→ size · space toggle · tab done")),
		fg(cSub, i18n.T("confirm    ")) + fg(cText, "enter        ") + fg(cSub, i18n.T("back ")) + fg(cText, "esc"),
		fg(cSub, i18n.T("quit       ")) + fg(cText, "q  /  ctrl+c"),
		"", fg(cDim, i18n.T("Nothing is written until the final Review step.")),
	}, "\n")
}

// Nerd-font icons per step (gated on `nerd`; plain markers otherwise).
var stepIcon = map[string]string{
	"keyboard": "", "locale": "", "timezone": "", "network": "",
	"hardware": "", "profile": "", "gpu": "", "diskpick": "",
	"disk": "", "partitions": "", "hostname": "", "username": "",
	"password": "", "encryption": "", "review": "",
}

func (m model) activeTotal() int {
	n := 0
	for i := range m.flow {
		if m.stepActive(i) {
			n++
		}
	}
	return n
}
func (m model) activePos() int { // 0-based index among active steps
	n := 0
	for i := 0; i < m.idx; i++ {
		if m.stepActive(i) {
			n++
		}
	}
	return n
}

func (m model) rail() string {
	const inner = 18
	num := m.cur().key == "review" // number steps so 1-9 can jump to edit them
	lines := []string{fg(cSub, i18n.T("install steps")), ""}
	n := 0
	for i, s := range m.flow {
		if !m.stepActive(i) {
			continue
		}
		n++
		g := ""
		if nerd {
			g = stepIcon[s.key]
		}
		lead := func(c color.Color, fallback string) string {
			p := ""
			if num {
				p = fg(cDim, fmt.Sprintf("%d ", n))
			}
			if g != "" {
				return p + fg(c, g+" ")
			}
			return p + fg(c, fallback)
		}
		switch {
		case i < m.idx:
			l := lead(cGreen, gCheck+" ") + fg(cText, s.key)
			if v := m.picks[s.key]; v != "" && !num {
				l += " " + fg(cDim, truncW(v, 14-len(s.key)))
			}
			lines = append(lines, l)
		case i == m.idx && (m.state == "wizard" || m.state == "transition"):
			lines = append(lines, lead(cBrand, gCurStep)+bold(cBrand, s.key))
		default:
			lines = append(lines, lead(cDim, gPend+" ")+fg(cDim, s.key))
		}
	}
	return sty().Border(border()).BorderForeground(cDim).Padding(0, 1).
		Render(padLines(strings.Join(lines, "\n"), inner))
}

func inputBox(val, placeholder string, pw bool) string {
	shown := val
	if pw {
		shown = strings.Repeat("•", len(val))
	}
	caret := sty().Foreground(cBrand).Render(gCaret)
	if val == "" && placeholder != "" {
		return fg(cBrand, gPrompt) + fg(cDim, placeholder) + caret
	}
	return fg(cBrand, gPrompt) + fg(cText, shown) + caret
}

func (m model) confirmButtons() string {
	yes, no := sty().Padding(0, 3), sty().Padding(0, 3)
	if m.yes {
		yes = yes.Background(cBrand).Foreground(cBg).Bold(true)
		no = no.Foreground(cSub)
	} else {
		no = no.Background(cBrand).Foreground(cBg).Bold(true)
		yes = yes.Foreground(cSub)
	}
	btns := lipgloss.JoinHorizontal(lipgloss.Top, yes.Render(i18n.T("Yes")), "  ", no.Render(i18n.T("No")))
	if m.fit.tightCard {
		return btns + "\n" + fg(cDim, i18n.T("←/→ · enter"))
	}
	return btns + "\n\n" + fg(cDim, i18n.T("←/→ or y/n · enter"))
}

func (m model) reviewBody(w int) string {
	row := func(k, v string) string { return fg(cSub, fmt.Sprintf("%-11s", k)) + fg(cText, truncW(v, w-12)) }
	subs := "@ @log @pkg"
	if m.sepHome {
		subs += " @home"
	}
	if m.snapshots {
		subs += " @snapshots"
	}
	if m.backups {
		subs += " @backups"
	}
	if m.swapG > 0 {
		subs += " @swap"
	}
	swap := fmt.Sprintf("%dG", m.swapG)
	if m.swapG == 0 {
		swap = i18n.T("none")
	}
	esp, bootKind := fmt.Sprintf("%dG", m.espG), "ESP"
	if m.picks["disk"] == "alongside" {
		bootKind = "boot"
		esp = fmt.Sprintf("%dG %s", alongsideBootGiB, map[string]string{
			"shared": "XBOOTLDR", "dedicated": i18n.T("dedicated ESP"), "auto": "boot",
		}[m.espMode()])
	}
	// strategy: render in red+bold for "whole" so the wipe is undeniable on
	// Review before the user moves to Yes. Alongside renders green to mark it
	// as the non-destructive path. Anything else (should never reach Review
	// thanks to partReady) is shown bold-red so the user can spot it.
	strat := m.picks["disk"]
	var stratCell string
	switch strat {
	case "whole":
		stratCell = bold(cRed, i18n.T("ERASE whole disk"))
	case "alongside":
		if m.probeVerdict == "create-esp" {
			stratCell = fg(cGreen, i18n.T("install into free space (new ESP)"))
		} else {
			stratCell = fg(cGreen, i18n.T("alongside (keep existing OS)"))
		}
	default:
		stratCell = bold(cRed, i18n.T("unset (refused)"))
	}
	lines := []string{
		bold(cBrand, i18n.T("Review, then confirm to install")), "",
		fg(cRed, i18n.Tf("⚠ this writes the layout below to %s", m.diskDev)), "",
		row(i18n.T("keyboard"), m.picks["keyboard"]), row(i18n.T("locale"), m.picks["locale"]),
		row(i18n.T("time zone"), m.picks["timezone"]), row(i18n.T("profile"), m.picks["profile"]),
		row(i18n.T("wm"), compositorLabel(m.picks["compositor"])), row(i18n.T("browser"), browserLabel(m.picks["browser"])),
		row(i18n.T("shell"), loginShellLabel(m.picks["login-shell"])), row(i18n.T("apps"), m.appsReviewCell()),
		row(i18n.T("disk"), m.diskDev),
		fg(cSub, fmt.Sprintf("%-11s", i18n.T("strategy"))) + stratCell,
		row(i18n.T("hostname"), m.picks["hostname"]),
		row(i18n.T("user"), m.picks["username"]), row(i18n.T("password"), m.picks["password"]),
		row(i18n.T("encryption"), m.picks["encryption"]), "",
		fg(cSub, i18n.T("layout     ")) + fg(cText, i18n.Tf("%s %s · root %dG · swap %s", bootKind, esp, m.availRoot()-m.swapG, swap)),
		fg(cSub, i18n.T("subvols    ")) + fg(cText, subs),
	}
	if len(m.kept) > 0 {
		lines = append(lines, fg(cSub, i18n.T("kept       "))+fg(cYell, i18n.Tf("%d existing partition(s)", len(m.kept))))
	}
	if strat == "alongside" && m.probeVerdict == "create-esp" {
		// No existing ESP: Ryoku creates its own in the free space (dedicated mode).
		lines = append(lines,
			fg(cSub, i18n.T("boot       "))+fg(cText, i18n.T("new 2 GiB EFI partition + root created in the free space; existing partitions are untouched")))
	} else if strat == "alongside" {
		existing := map[string]string{"windows": "Windows", "ryoku": "Ryoku", "linux": "Linux"}[m.espKind]
		if existing == "" {
			existing = i18n.T("existing OS")
		}
		if m.espMode() == "dedicated" {
			lines = append(lines,
				fg(cSub, i18n.T("boot       "))+fg(cText, i18n.Tf("dedicated Ryoku ESP; existing %s ESP is untouched", existing)))
		} else {
			boot := i18n.Tf("shared existing ESP (%s, backed up first)", m.espKind)
			if m.existingBoot == "none" {
				lines = append(lines, fg(cSub, i18n.T("boot       "))+fg(cText, boot),
					fg(cYell, "           "+i18n.T("existing system stays available through the firmware menu only (no chainload entry found)")))
			} else {
				lines = append(lines, fg(cSub, i18n.T("boot       "))+fg(cText, i18n.Tf("%s + %s (existing) entry in the boot menu", boot, existing)))
			}
			if m.espCount > 1 {
				lines = append(lines,
					fg(cYell, "           "+i18n.Tf("%d EFI partitions found; Ryoku shares the %s ESP", m.espCount, m.espKind)))
			}
		}
	}
	if len(m.reclaim) > 0 {
		lines = append(lines, fg(cSub, i18n.T("reclaim    "))+fg(cRed, i18n.Tf("%d leftover partition(s) (%dG) from a failed prior install", len(m.reclaim), m.reclaimG)))
	}
	// Typed-ERASE sub-stage: a destructive step (whole-disk wipe, or freeing the
	// leftover Ryoku partitions on alongside) blocks the install handoff behind a
	// loud red confirmation. Both reuse wipeStage; only the wording differs so the
	// user knows exactly what is being destroyed.
	switch {
	case strat == "whole" && m.diskPopulated():
		names := make([]string, 0, len(m.existing))
		for _, p := range m.existing {
			names = append(names, p.dev)
		}
		lines = append(lines,
			"",
			bold(cRed, i18n.Tf("⚠ ERASING %d existing partition(s): %s", len(m.existing), truncW(strings.Join(names, ", "), w-30))),
		)
		lines = append(lines, m.eraseAckLines()...)
	case strat == "alongside" && len(m.reclaim) > 0:
		lines = append(lines,
			"",
			bold(cRed, i18n.Tf("⚠ reclaiming %d leftover partition(s) (%dG) from a failed prior install", len(m.reclaim), m.reclaimG)),
		)
		lines = append(lines, m.eraseAckLines()...)
	}
	// BitLocker (alongside): a non-blocking heads-up. A locked Windows volume will
	// demand its recovery key when booted through the Ryoku menu until BitLocker is
	// suspended or re-sealed.
	if strat == "alongside" && m.bitlocker {
		lines = append(lines,
			"",
			fg(cYell, i18n.T("⚠ BitLocker: booting Windows via the Ryoku menu demands the recovery key")),
			fg(cDim, i18n.T("  until you suspend it in Windows (or boot Windows via the firmware menu);")),
			fg(cDim, "  "+i18n.T("see")+" docs/installation-hardware.md"),
		)
	}
	if r := m.reviewBlockReason(); r != "" {
		lines = append(lines, "", bold(cRed, "⚠ "+r))
	}
	// On a grid too short for the whole card, the blank spacer rows are the first
	// thing to go: losing them costs readability, losing the bottom of this list
	// costs the user the confirm buttons.
	if m.fit.tightCard {
		kept := lines[:0]
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				kept = append(kept, l)
			}
		}
		lines = kept
	}
	return strings.Join(lines, "\n")
}

// eraseAckLines renders the typed-ERASE prompt shared by the whole-disk wipe and
// the alongside reclaim confirmations: a live input box at wipeStage 1, otherwise
// a hint to switch to Yes.
func (m model) eraseAckLines() []string {
	if m.wipeStage == 1 {
		return []string{"", fg(cYell, i18n.T("type ERASE then press enter to confirm  ·  esc cancels")), inputBox(m.eraseInput, "ERASE", false)}
	}
	return []string{fg(cDim, i18n.T("switch to Yes and press enter; you will be asked to type ERASE"))}
}

// stepLog is sample command output per step, used by the snapshot layout preview.
// Live installs stream the real backend output instead.
func (m model) stepLog(i int) []string {
	d := m.diskDev
	switch i {
	case 0:
		return []string{"sgdisk --zap-all " + d, "parted -s " + d + " mklabel gpt"}
	case 1:
		return []string{"mkfs.vfat -F32 -n BOOT " + d + "p1", "mkfs.btrfs -f -L ryoku " + d + "p2", "btrfs subvolume create @ @home @log @pkg @snapshots"}
	case 2:
		return []string{"mount -o subvol=@,compress=zstd,noatime " + d + "p2 /mnt", "mount " + d + "p1 /mnt/boot"}
	case 3:
		return []string{"pacstrap -K /mnt base linux linux-firmware ...", "genfstab -U /mnt >> /mnt/etc/fstab"}
	case 4:
		return []string{"arch-chroot /mnt (locale, timezone, hostname, user)", "mkinitcpio -P"}
	case 5:
		return []string{"limine install + Ryoku branding", "wrote /boot/limine.conf"}
	}
	return nil
}

func (m model) viewCentered() string {
	switch m.state {
	case "done":
		return m.viewDone()
	case "failed":
		return m.viewFailed()
	}
	// Responsive panel: wide enough for real log lines; long lines truncate cleanly.
	iw := clamp(m.w-12, 56, 100) // inner content width
	bw := clamp(iw-10, 30, 70)
	logRows := clamp(m.h-18, 5, 14)

	fill := clamp(int(m.progress*float64(bw)), 0, bw)
	bar := fg(cBrand, strings.Repeat(gFull, fill)) + fg(cDim, strings.Repeat(gEmpty, bw-fill)) +
		fg(cSub, fmt.Sprintf(" %3.0f%%", m.progress*100))

	var b strings.Builder
	b.WriteString(bold(cBrand, i18n.T("Installing Ryoku")) + "\n\n")
	b.WriteString(bar + "\n\n")
	names := installSteps()
	for i := range names {
		switch {
		case i < m.installAt:
			b.WriteString(fg(cGreen, gCheck+" ") + fg(cSub, names[i]) + "\n")
		case i == m.installAt:
			b.WriteString(fg(cBrand, spinFrames[m.frame%len(spinFrames)]) + " " + fg(cText, names[i]) + "\n")
		default:
			b.WriteString(fg(cDim, gPend+" "+names[i]) + "\n")
		}
	}
	b.WriteString(fg(cDim, strings.Repeat(ruleCh(), iw)) + "\n")
	tail := m.installLog
	if len(tail) > logRows {
		tail = tail[len(tail)-logRows:]
	}
	for _, ln := range tail {
		b.WriteString(fg(cDim, truncW(ln, iw)) + "\n")
	}
	for i := len(tail); i < logRows; i++ { // fixed-height log pane (no jitter)
		b.WriteString("\n")
	}
	return sty().Border(border()).BorderForeground(cBrand).Padding(1, 2).
		Render(padLines(strings.TrimRight(b.String(), "\n"), iw))
}

func (m model) viewDone() string {
	user := m.picks["username"]
	if user == "" {
		user = "you"
	}
	next := i18n.T("snapshots and rollback from the Limine boot menu")
	if isVoidVariant(m.variant) {
		next = i18n.T(voidSnapshotNote)
	}
	card := sty().Border(borderDouble()).BorderForeground(cGreen).Padding(1, 3).Align(lipgloss.Center).
		Render(bold(cGreen, gCheck+"  "+i18n.T("Ryoku installed")) + "\n\n" +
			fg(cText, m.picks["hostname"]+" · "+m.picks["username"]+" · "+m.picks["profile"]) + "\n" +
			fg(cSub, i18n.T("encryption: ")+m.picks["encryption"]+" · "+m.picks["timezone"]) + "\n\n" +
			fg(cSub, i18n.T("what's next")) + "\n" +
			fg(cDim, i18n.Tf("log in as %s  ·  your configs live in ~/.config", user)) + "\n" +
			fg(cDim, next))
	// WIRE: doneSel 0 → systemctl reboot · 1 → systemctl poweroff · 2 → exit to a shell
	opts := []struct{ label, hint string }{
		{i18n.T("Reboot now"), i18n.T("recommended")},
		{i18n.T("Power off"), ""},
		{i18n.T("Exit to a shell"), i18n.T("poke around first")},
	}
	var b strings.Builder
	for i, o := range opts {
		if i == m.doneSel {
			b.WriteString(bold(gradColor(float64(m.phase)/float64(smallW-1)), gPrompt) + bold(cText, o.label))
		} else {
			b.WriteString("  " + fg(cSub, o.label))
		}
		if o.hint != "" {
			b.WriteString("  " + fg(cDim, o.hint))
		}
		b.WriteString("\n")
	}
	return lipgloss.JoinVertical(lipgloss.Center, card, "", strings.TrimRight(b.String(), "\n"))
}

func (m model) viewFailed() string {
	// QR rendered black-on-white so it scans on the dark theme background.
	qst := sty().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#ffffff"))
	var qb strings.Builder
	for _, ln := range strings.Split(strings.TrimRight(m.qrStr, "\n"), "\n") {
		qb.WriteString(qst.Render(ln) + "\n")
	}
	tail := i18n.T("Scan for help (and to share this log):")
	if ascii {
		tail = i18n.T("For help, visit:")
	}
	card := sty().Border(border()).BorderForeground(cRed).Padding(1, 3).
		Render(bold(cRed, gBad+"  "+i18n.T("Installation failed")) + "\n\n" +
			fg(cSub, i18n.T("step  ")) + fg(cText, m.failStep) + "\n" +
			fg(cSub, i18n.T("log   ")) + fg(cText, m.logPath) + "\n\n" +
			fg(cSub, tail))
	if ascii { // QR needs block glyphs
		return lipgloss.JoinVertical(lipgloss.Center, card, "", fg(cBlue, ryokuSupportURL))
	}
	return lipgloss.JoinVertical(lipgloss.Center, card, "", strings.TrimRight(qb.String(), "\n"), "", fg(cDim, ryokuSupportURL))
}

func (m model) footer() string {
	s := m.cur()
	var parts []string
	switch {
	case s.kind == kNotice:
		parts = []string{keyHint("enter", i18n.T("continue")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
	case s.kind == kPartition:
		switch {
		case m.layoutRows()[m.lsel].kind == "keep", m.layoutRows()[m.lsel].kind == "reclaim":
			parts = []string{keyHint("↑↓", i18n.T("move")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back"))}
		case m.layoutRows()[m.lsel].kind == "carve":
			// The ±big hint tracks the handler: it only fires once this row is the
			// active carve target, so advertise it only then.
			parts = []string{keyHint("←/→", i18n.T("Ryoku's space"))}
			if m.carving() && carveIndex(m.layoutRows()[m.lsel].key) == m.carvePart {
				parts = append(parts, keyHint("shift", i18n.T("±big")))
			}
			parts = append(parts, keyHint("↑↓", i18n.T("move")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back")))
		case m.layoutRows()[m.lsel].kind == "region":
			parts = []string{keyHint("enter", i18n.T("use")), keyHint("↑↓", i18n.T("move")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back"))}
		case m.layoutRows()[m.lsel].kind == "size":
			parts = []string{keyHint("←/→", i18n.T("adjust")), keyHint("shift", i18n.T("±big")), keyHint("↑↓", i18n.T("move")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back"))}
		default:
			parts = []string{keyHint("space", i18n.T("toggle")), keyHint("↑↓", i18n.T("move")), keyHint("a", i18n.T("reset")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back"))}
		}
	case s.kind == kApps:
		parts = []string{keyHint("↑↓", i18n.T("move")), keyHint("space", i18n.T("keep/remove")), keyHint("a", i18n.T("reset")), keyHint("tab", i18n.T("done")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
	case s.kind == kInfo:
		if m.hwBIOS { // BIOS is a hard block; there is no "continue" to offer
			parts = []string{keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
		} else {
			parts = []string{keyHint("enter", i18n.T("continue")), keyHint("esc", i18n.T("back")), keyHint("?", i18n.T("help")), keyHint("q", i18n.T("quit"))}
		}
	case s.kind == kPass:
		parts = []string{keyHint("type", i18n.T("password")), keyHint("enter", i18n.T("continue")), keyHint("esc", i18n.T("back"))}
	case s.kind == kNet:
		switch {
		case m.netOnline || m.hasOfflineRepo():
			parts = []string{keyHint("enter", i18n.T("continue")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
		case m.netStage == 1:
			parts = []string{keyHint("type", i18n.T("password")), keyHint("enter", i18n.T("connect")), keyHint("esc", i18n.T("back"))}
		default:
			parts = []string{keyHint("↑↓", i18n.T("move")), keyHint("enter", i18n.T("connect")), keyHint("r", i18n.T("rescan")), keyHint("esc", i18n.T("back"))}
		}
	case s.kind == kSelect:
		parts = []string{keyHint("↑↓/jk", i18n.T("move")), keyHint("/", i18n.T("filter"))}
		if s.numbered {
			parts = append(parts, keyHint("1-9", i18n.T("quick")))
		}
		parts = append(parts, keyHint("enter", i18n.T("select")), keyHint("?", i18n.T("help")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit")))
	case s.kind == kInput:
		parts = []string{keyHint("type", i18n.T("edit")), keyHint("enter", i18n.T("accept")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
	case s.kind == kConfirm && s.key == "encryption" && m.encStage > 0:
		parts = []string{keyHint("type", i18n.T("passphrase")), keyHint("enter", i18n.T("next")), keyHint("esc", i18n.T("cancel"))}
	case s.kind == kConfirm:
		parts = []string{keyHint("←/→", i18n.T("choose")), keyHint("enter", i18n.T("confirm")), keyHint("esc", i18n.T("back")), keyHint("q", i18n.T("quit"))}
	}
	bar := strings.Join(parts, fg(cDim, "  ·  ")) // step count now lives in the header progress bar
	return "\n" + lipgloss.PlaceHorizontal(m.w, lipgloss.Center, bar)
}

func keyHint(k, desc string) string { return bold(cBrand, k) + " " + fg(cSub, desc) }

func main() {
	// The ISO carries no /usr/share/ryoku/i18n, so the catalogs are compiled in;
	// Use("") resolves RYOKU_LANG, then the desktop shell.json, then LANG, and
	// falls back to English. Done before newModel() so every step, list and label
	// is built in the resolved language.
	i18n.SetFS(catalog.FS)
	i18n.Use("")
	initGlyphs()
	if len(os.Args) > 1 && os.Args[1] == "snapshot" {
		snapshot()
		return
	}
	fm, err := tea.NewProgram(newModel()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if m, ok := fm.(model); ok {
		applyExit(m.exitAction)
	}
}

func snapshot() {
	picks := map[string]string{"keyboard": "us", "locale": "en_US.UTF-8", "timezone": "Europe/Madrid",
		"profile": "amd-nvidia", "gpu": "offload", "compositor": "niri", "browser": "zen",
		"disk": "alongside", "hostname": "ryoku", "username": "carlos", "encryption": "LUKS"}
	picks["browser"], picks["login-shell"] = "firefox", "fish"
	mk := func() model { m := newModel(); m.w, m.h, m.enterPos, m.state = 112, 42, 1, "wizard"; return m }
	at := func(m *model, key string) { m.idx = flowIndex(m.flow, key) }
	sep := strings.Repeat("─", 112)
	render := func(m model) string {
		if m.state == "done" || m.state == "failed" || m.state == "install" {
			return m.frameWithFooter(m.viewCentered(), "")
		}
		return m.frameWizard()
	}
	show := func(t string, m model) { fmt.Println("### " + t + " ###\n" + render(m) + "\n" + sep) }

	wm := mk()
	wm.state = "welcome"
	fmt.Println("### welcome ###\n" + wm.welcomeFrame() + "\n" + sep)
	wm = mk()
	wm.state = "welcome"
	wm.ensureSocialQR()
	fmt.Println("### welcome: social QR ###\n" + wm.welcomeQR() + "\n" + sep)

	m := mk()
	at(&m, "network") // network (online)
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("network: connected", m)

	m = mk()
	at(&m, "network")
	m.picks, m.netOnline = picks, false // network offline → Wi-Fi list
	m.loadStep()
	m.enterPos = 1
	show("network: offline (Wi-Fi)", m)

	m = mk()
	at(&m, "hardware") // hardware detected
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("hardware: detected", m)

	m = mk()
	at(&m, "hardware")
	m.picks, m.hwOK, m.hwHybrid = picks, false, false
	m.loadStep()
	m.enterPos = 1
	show("hardware: not detected (graceful fallback)", m)

	m = mk()
	at(&m, "gpu") // graphics mode (hybrid)
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("graphics mode (hybrid GPU)", m)

	m = mk()
	at(&m, "compositor") // window-manager picker
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("window manager (the shipped providers)", m)

	m = mk()
	at(&m, "browser") // browser picker: exactly the three shipped browsers
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("web browser", m)

	m = mk()
	at(&m, "login-shell")
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("login shell", m)

	m = mk()
	at(&m, "apps") // apps keep/remove checklist, shipped defaults
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("apps: keep/remove (defaults)", m)

	m = mk()
	at(&m, "apps") // a curated slim install: the gaming, VM and sharing rows off
	m.picks = picks
	m.loadStep()
	for _, id := range []string{"gamescope", "gamemode", "mangohud", "controllers", "vm", "docker", "flatpak", "yazi", "localsend"} {
		m.keep[id] = false
	}
	for i, r := range m.appsRows() { // park the cursor on a removed row
		if r.kind == "app" && r.id == "gamescope" {
			m.alsel = i
		}
	}
	m.appsFixScroll()
	m.enterPos = 1
	show("apps: gaming + VM bundles removed", m)

	m = mk()
	at(&m, "diskpick") // target-disk picker
	m.picks = picks
	m.loadStep()
	m.enterPos = 1
	show("target disk", m)

	m = mk()
	at(&m, "partitions")
	m.picks = picks
	m.picks["disk"] = "whole"
	m.loadStep()
	m.enterPos, m.lsel = 1, 0
	show("partitions: required and optional", m)

	// ── alongside on a populated pure-Ryoku disk (the reported 1 TB scenario) ──
	// Fixture: 953869 MiB GPT, 1 GiB ESP (BOOT), 930.5 GiB btrfs (ryoku); the
	// shared Ryoku ESP had no chainloadable binary (existing_boot none).
	const btrfsMiB = 952832 // 930.5 GiB
	ryokuParts := []resizePart{
		{dev: "/dev/loop0p1", index: 1, fs: "vfat", label: "BOOT", sizeMiB: 1024, usedMiB: 60, minMiB: -1, shrinkable: false, reason: "boot partition (ESP): kept as is"},
		{dev: "/dev/loop0p2", index: 2, fs: "btrfs", label: "ryoku", sizeMiB: btrfsMiB, usedMiB: 2048, minMiB: 2048, shrinkable: true, reason: "single-device btrfs"},
	}
	ryokuKept := []part{
		{dev: "EFI System", size: 1, fs: "fat32", flags: "esp", status: "keep"},
		{dev: "ryoku", size: 931, fs: "btrfs", status: "keep"},
	}
	alongPicks := map[string]string{}
	for k, v := range picks {
		alongPicks[k] = v
	}
	alongPicks["disk"] = "alongside"

	m = mk()
	at(&m, "disk") // disk strategy
	dl := diskLayout{parts: ryokuKept, gpt: true, espKind: "ryoku", probeVerdict: "ok"}
	m.pick = newPicker(diskStrategiesFor(dl), true)
	m.pick.height, m.picks, m.enterPos = m.listRows(), alongPicks, 1
	show("alongside: strategy on a 931.5 GiB pure-Ryoku disk", m)

	carve := func() model {
		c := mk()
		c.idx, c.picks = flowIndex(c.flow, "partitions"), alongPicks
		c.diskDev, c.diskTotal, c.diskG = "/dev/loop0", 931, 931
		c.diskBytes = 953869 * 1024 * 1024
		c.gpt, c.espKind, c.existingBoot, c.probeVerdict, c.espFreeKiB = true, "ryoku", "none", "ok", 8192
		c.kept, c.freeG, c.resizeParts = ryokuKept, 0, ryokuParts
		c.espG, c.swapG = 1, 16
		c.snapshots, c.sepHome, c.backups = true, true, false
		c.carvePart, c.carveTakeMiB = 1, 120*1024
		c.clampSwapToLayout()
		c.enterPos, c.lsel = 1, 0
		return c
	}

	show("alongside: layout carving 120 GiB from ryoku", carve())

	m = carve()
	m.partKey("shift+right") // +10 GiB big step
	show("alongside: layout with Shift big-step hint (130 GiB)", m)

	m = carve()
	at(&m, "review")
	m.netOnline, m.hwSecureBoot = true, false
	show("alongside: review (shared ESP, no erase)", m)

	m = carve()
	at(&m, "review")
	m.netOnline, m.hwSecureBoot, m.espFreeKiB = true, false, 6144
	m.espKind, m.existingBoot = "windows", "/EFI/Microsoft/Boot/bootmgfw.efi"
	show("alongside: review (dedicated ESP, existing ESP untouched)", m)

	m = carve()
	at(&m, "review")
	m.netOnline, m.hwSecureBoot = true, false
	m.reclaim = []part{
		{dev: "previous Ryoku", size: 6, fs: "ryoku", status: "reclaim", reclaim: true},
		{dev: "previous Ryoku", size: 1, fs: "ryokuboot", status: "reclaim", reclaim: true},
	}
	m.reclaimG = 7
	show("alongside: review with reclaimed leftovers", m)

	m = mk()
	at(&m, "password") // user password
	m.picks = picks
	m.loadStep()
	m.enterPos, m.input = 1, "Hunter2!"
	show("user password (+ strength)", m)

	m = mk()
	m.picks, m.state, m.doneSel = picks, "done", 0
	show("done: reboot / poweroff / shell", m)

	m = mk()
	m.picks, m.state, m.installAt, m.progress = picks, "install", 5, 0.62
	for i := 0; i < 5; i++ {
		m.installLog = append(m.installLog, m.stepLog(i)...)
	}
	show("install: wide scrolling log", m)

	m = mk()
	m.picks, m.installAt = picks, 3
	for i := 0; i < 4; i++ {
		m.installLog = append(m.installLog, m.stepLog(i)...)
	}
	m.failInstall("Installing the base system")
	fmt.Println("### failed: fail-safe log + QR ###\n" + render(m))
}
