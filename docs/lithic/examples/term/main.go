package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/term"
)

type state struct {
	label string
	view  string
}

func main() {
	page := flag.String("page", "foundation", "gallery page: foundation, controls, lists, or overlays")
	width := flag.Int("width", 120, "output width in cells")
	modeName := flag.String("mode", "follow", "palette mode: follow, dark, or light")
	glyphName := flag.String("glyphs", "", "glyph set: console or unicode")
	contrastName := flag.String("contrast", "standard", "palette contrast: standard or high")
	tintName := flag.String("tint", "off", "palette tint: off, low, or medium")
	flag.Parse()

	mode, ok := parseMode(*modeName)
	if !ok {
		fmt.Fprintln(os.Stderr, "mode must be follow, dark, or light")
		os.Exit(2)
	}
	if *glyphName != "" {
		if err := term.SetGlyphMode(term.GlyphMode(*glyphName)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	w := max(40, *width)
	th := term.Theme{Roles: palette.FromPalette(palette.Load(mode), palette.ParseLook(*contrastName, *tintName))}
	var output string
	switch *page {
	case "foundation":
		output = foundation(th, w)
	case "controls":
		output = controls(th, w)
	case "lists":
		output = lists(th, w)
	case "overlays":
		output = overlays(th, w)
	default:
		fmt.Fprintln(os.Stderr, "page must be foundation, controls, lists, or overlays")
		os.Exit(2)
	}
	fmt.Print(output)
}

func parseMode(name string) (palette.Mode, bool) {
	switch name {
	case "follow":
		return palette.Follow, true
	case "dark":
		return palette.Dark, true
	case "light":
		return palette.Light, true
	default:
		return palette.Follow, false
	}
}

func pageHeader(th term.Theme, title, description string) string {
	return lipgloss.NewStyle().Foreground(th.Roles.Fg).Bold(true).Render(title) + "\n" +
		lipgloss.NewStyle().Foreground(th.Roles.Fg2).Render(description) + "\n\n"
}

func specimen(b *strings.Builder, name string, states ...state) {
	b.WriteString(name)
	b.WriteString("\n")
	for _, item := range states {
		b.WriteString("  ")
		b.WriteString(item.label)
		b.WriteString("\n")
		for _, line := range strings.Split(item.view, "\n") {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
}

func foundation(th term.Theme, width int) string {
	var b strings.Builder
	b.WriteString(pageHeader(th, "Lithic terminal foundation", "Marks and progress primitives for terminal surfaces."))
	specimen(&b, "Notch",
		state{"Idle", term.RenderNotch(th, palette.Idle)},
		state{"Alive", term.RenderNotch(th, palette.Alive)},
		state{"Becoming", term.RenderNotch(th, palette.Becoming)},
		state{"Needs you", term.RenderNotch(th, palette.Attention)},
		state{"Paused", term.RenderNotch(th, palette.Paused)},
		state{"Failed", term.RenderNotch(th, palette.Failed)})
	specimen(&b, "Blade",
		state{"Empty", term.RenderBlade(th, 32, 0, palette.Becoming)},
		state{"In progress", term.RenderBlade(th, 32, .46, palette.Becoming)},
		state{"Complete", term.RenderBlade(th, 32, 1, palette.Alive)})
	specimen(&b, "Flake",
		state{"Idle", term.RenderFlake(th, palette.Idle, 0)},
		state{"Alive", term.RenderFlake(th, palette.Alive, 1)},
		state{"Becoming", term.RenderFlake(th, palette.Becoming, .5)},
		state{"Failed", term.RenderFlake(th, palette.Failed, 1)})
	return b.String()
}

func controls(th term.Theme, width int) string {
	var b strings.Builder
	b.WriteString(pageHeader(th, "Lithic terminal controls", "Keyboard controls translated to cells without losing state."))
	specimen(&b, "Plaque",
		state{"Consequential", term.RenderPlaque(th, term.Plaque{Label: "Connect", Kind: term.PlaqueGold})},
		state{"Line", term.RenderPlaque(th, term.Plaque{Label: "Inspect", Kind: term.PlaqueLine})},
		state{"Quiet", term.RenderPlaque(th, term.Plaque{Label: "Cancel", Kind: term.PlaqueQuiet})},
		state{"Selected", term.RenderPlaque(th, term.Plaque{Label: "Console", Kind: term.PlaqueOn})},
		state{"Danger", term.RenderPlaque(th, term.Plaque{Label: "Stop", Kind: term.PlaqueDanger})},
		state{"Focused", term.RenderPlaque(th, term.Plaque{Label: "Connect", Focused: true})},
		state{"Busy", term.RenderPlaque(th, term.Plaque{Label: "Connecting", Kind: term.PlaqueGold, Busy: true})},
		state{"Disabled", term.RenderPlaque(th, term.Plaque{Label: "Connect", Disabled: true})})
	specimen(&b, "Split plaque",
		state{"Closed", term.RenderSplitPlaque(th, term.SplitPlaque{Main: term.Plaque{Label: "Start", Kind: term.PlaqueGold}})},
		state{"Menu focused", term.RenderSplitPlaque(th, term.SplitPlaque{Main: term.Plaque{Label: "Start", Kind: term.PlaqueGold}, Expanded: true, FocusMore: true})})
	specimen(&b, "Text field",
		state{"Placeholder", term.RenderTextField(th, term.TextField{Label: "Server", Placeholder: "prowl", Help: "Fleet name", Width: 28})},
		state{"Focused", term.RenderTextField(th, term.TextField{Label: "Address", Value: "198.51.100.24", Focused: true, Width: 28})},
		state{"Error", term.RenderTextField(th, term.TextField{Label: "Address", Value: "198.51.100", Error: "Enter a complete address", Width: 28})},
		state{"Secret", term.RenderTextField(th, term.TextField{Label: "Access key", Value: "fleet-key", Secret: true, Width: 28})},
		state{"Disabled", term.RenderTextField(th, term.TextField{Label: "Host", Value: "ally", Disabled: true, Width: 28})})
	specimen(&b, "Search",
		state{"Idle", term.RenderSearch(th, term.Search{Placeholder: "Search fleet", Shortcut: "/", Width: 36})},
		state{"Focused", term.RenderSearch(th, term.Search{Value: "fedora", Focused: true, Width: 36})},
		state{"Disabled", term.RenderSearch(th, term.Search{Placeholder: "Search fleet", Disabled: true, Width: 36})})
	specimen(&b, "Toggle",
		state{"Off", term.RenderToggle(th, term.Toggle{Label: "Auto-start"})},
		state{"On", term.RenderToggle(th, term.Toggle{Label: "Auto-start", On: true})},
		state{"Focused", term.RenderToggle(th, term.Toggle{Label: "Auto-start", On: true, Focused: true})},
		state{"Disabled", term.RenderToggle(th, term.Toggle{Label: "Auto-start", Disabled: true})})
	specimen(&b, "Stepper",
		state{"Value", term.RenderStepper(th, term.Stepper{Value: 4, Min: 1, Max: 8, Unit: "cores"})},
		state{"Upper bound", term.RenderStepper(th, term.Stepper{Value: 8, Min: 1, Max: 8, Unit: "cores"})},
		state{"Focused", term.RenderStepper(th, term.Stepper{Value: 4, Min: 1, Max: 8, Unit: "cores", Focused: true})})
	filters := []term.Filter{{Label: "All", Count: 6}, {Label: "Running", Count: 2}, {Label: "Needs you", Count: 1}, {Label: "Stopped", Count: 3}}
	specimen(&b, "Filters",
		state{"Selected", term.RenderFilters(th, term.Filters{Items: filters, Selected: 1})},
		state{"Focused", term.RenderFilters(th, term.Filters{Items: filters, Selected: 2, Focused: true})})
	tabs := []string{"Overview", "Console", "Storage", "Network"}
	specimen(&b, "Tabs",
		state{"Selected", term.RenderTabs(th, term.Tabs{Labels: tabs, Selected: 0, Width: max(1, width-4)})},
		state{"Focused", term.RenderTabs(th, term.Tabs{Labels: tabs, Selected: 1, Focused: true, Width: max(1, width-4)})})
	specimen(&b, "Disclosure",
		state{"Closed", term.RenderDisclosure(th, term.Disclosure{Label: "Show connection details", Content: "ops@192.0.2.18"})},
		state{"Open", term.RenderDisclosure(th, term.Disclosure{Label: "Hide connection details", Content: "ops@192.0.2.18", Open: true, Focused: true})})
	return b.String()
}

func fleetRows(width int) []term.Tablet {
	return []term.Tablet{
		{Name: "ubuntu-24.04", Facts: "ally at 192.0.2.18", Claim: "Running", Proof: "12 min", Tone: palette.Alive, Width: width},
		{Name: "fedora-43", Facts: "prowl at 198.51.100.24", Claim: "Building", Proof: "46%", Tone: palette.Becoming, Progress: .46, Width: width},
		{Name: "windows-11", Facts: "mika-server at 203.0.113.70:2222", Claim: "Needs you", Proof: "Sign in", Tone: palette.Attention, Action: "Review", Width: width},
		{Name: "alpine-instant", Facts: "localhost at 127.0.0.1", Claim: "Stopped", Proof: "Yesterday", Tone: palette.Idle, Width: width},
	}
}

func lists(th term.Theme, width int) string {
	var b strings.Builder
	b.WriteString(pageHeader(th, "Lithic terminal lists and progress", "Fleet rows, state, facts, and work progress."))
	rowWidth := min(78, width-8)
	rows := fleetRows(rowWidth)
	specimen(&b, "Tablet",
		state{"Idle", term.RenderTablet(th, rows[3])},
		state{"Alive", term.RenderTablet(th, rows[0])},
		state{"Becoming", term.RenderTablet(th, rows[1])},
		state{"Focused", term.RenderTablet(th, func() term.Tablet { v := rows[2]; v.Focused = true; return v }())},
		state{"Selected", term.RenderTablet(th, func() term.Tablet { v := rows[0]; v.Selected = true; return v }())},
		state{"Disabled", term.RenderTablet(th, func() term.Tablet { v := rows[3]; v.Disabled = true; return v }())})
	specimen(&b, "Row list", state{"Selected", term.RenderRowList(th, term.RowList{Rows: rows, Selected: 1, Focused: true})})
	specimen(&b, "Group header",
		state{"Needs you", term.RenderGroupHeader(th, term.GroupHeader{Title: "Needs you", Count: 1, Width: 40})},
		state{"Running", term.RenderGroupHeader(th, term.GroupHeader{Title: "Running", Count: 2, Width: 40})})
	specimen(&b, "Empty", state{"No resources", term.RenderEmpty(th, term.Empty{Title: "No servers yet", Body: "Connect a server to add it to the fleet.", Width: 42, Actions: []term.Plaque{{Label: "Connect", Kind: term.PlaqueGold}}})})
	specimen(&b, "Mineral field",
		state{"Alive", term.RenderMineralField(th, term.MineralField{Tone: palette.Alive, Content: "ubuntu-24.04 is running"})},
		state{"Becoming", term.RenderMineralField(th, term.MineralField{Tone: palette.Becoming, Content: "fedora-43 is building"})},
		state{"Failed", term.RenderMineralField(th, term.MineralField{Tone: palette.Failed, Content: "scratch-arch failed to start"})})
	specimen(&b, "Slab", state{"Default", term.RenderSlab(th, term.Slab{Content: "Console router  /dev/ttyUSB0", Width: 42})})
	specimen(&b, "Section", state{"Default", term.RenderSection(th, term.Section{Title: "Connection", Body: "Server identity and routing"})})
	facts := term.Facts{Rows: []term.Fact{{Label: "Server", Value: "ally"}, {Label: "Address", Value: "ops@192.0.2.18", Literal: true}, {Label: "Machine", Value: "ubuntu-24.04"}}}
	specimen(&b, "Facts", state{"Values", term.RenderFacts(th, facts)})
	aside := term.Aside{Title: "Server to connect", Facts: facts, Preview: "ssh ops@192.0.2.18", Width: 44}
	specimen(&b, "Aside", state{"Live summary", term.RenderAside(th, aside)})
	specimen(&b, "Draft columns",
		state{"Compact", term.RenderDraftColumns(th, term.DraftColumns{Form: "Server\nAddress\nAccess key", Aside: &aside, Width: 70})},
		state{"Wide", term.RenderDraftColumns(th, term.DraftColumns{Form: "Server\nAddress\nAccess key", Aside: &aside, Width: max(110, width)})})
	specimen(&b, "Claim",
		state{"Idle", term.RenderClaim(th, term.Claim{Tone: palette.Idle})},
		state{"Alive", term.RenderClaim(th, term.Claim{Tone: palette.Alive, Progress: 1})},
		state{"Becoming", term.RenderClaim(th, term.Claim{Tone: palette.Becoming, Progress: .46})},
		state{"Failed", term.RenderClaim(th, term.Claim{Tone: palette.Failed, Progress: 1})})
	specimen(&b, "Marks", state{"Traits", term.RenderMarks(th, term.Marks{Disposable: true, Instant: true, Sealed: true})})
	specimen(&b, "Sentence", state{"Mixed spans", term.RenderSentence(th, term.Sentence{Spans: []term.Span{{Text: "Connect "}, {Text: "ops@192.0.2.18", Kind: term.SpanLiteral}, {Text: " to "}, {Text: "ally"}}})})
	specimen(&b, "Blade meter",
		state{"Quiet", term.RenderBladeMeter(th, term.BladeMeter{Width: 32, Fraction: .62})},
		state{"Attention", term.RenderBladeMeter(th, term.BladeMeter{Width: 32, Fraction: .93})})
	specimen(&b, "Blade progress",
		state{"Determinate", term.RenderBladeProgress(th, term.BladeProgress{Width: 32, Fraction: .46, Active: true})},
		state{"Indeterminate", term.RenderBladeProgress(th, term.BladeProgress{Width: 32, Indeterminate: true, Frame: 5})},
		state{"Complete", term.RenderBladeProgress(th, term.BladeProgress{Width: 32, Fraction: 1})})
	specimen(&b, "Working",
		state{"Frozen frame one", term.RenderWorking(th, term.Working{Working: false, Frame: 0})},
		state{"Frozen frame two", term.RenderWorking(th, term.Working{Working: false, Frame: 1})})
	return b.String()
}

func overlays(th term.Theme, width int) string {
	var b strings.Builder
	b.WriteString(pageHeader(th, "Lithic terminal overlays", "Menus and protected decisions rendered in place."))
	items := []term.MenuItem{{Label: "Open console", Shortcut: "Enter"}, {Label: "Pin server", Shortcut: "P"}, {Kind: term.MenuItemDivider}, {Label: "Stop server", Kind: term.MenuItemDanger}, {Label: "Passthrough", Disabled: true}}
	specimen(&b, "Menu",
		state{"Open", term.RenderMenu(th, term.Menu{Items: items, Selected: 0, Focused: true, Width: 32})},
		state{"Danger focused", term.RenderMenu(th, term.Menu{Items: items, Selected: 3, Focused: true, Width: 32})})
	options := []string{"ubuntu-24.04", "fedora-43", "windows-11"}
	specimen(&b, "Picker",
		state{"Closed", term.RenderPicker(th, term.Picker{Label: "Machine", Value: "ubuntu-24.04", Options: options, Width: 32})},
		state{"Focused", term.RenderPicker(th, term.Picker{Label: "Machine", Value: "fedora-43", Options: options, Selected: 1, Focused: true, Width: 32})},
		state{"Open", term.RenderPicker(th, term.Picker{Label: "Machine", Value: "fedora-43", Options: options, Selected: 1, Open: true, Focused: true, Width: 32})},
		state{"Disabled", term.RenderPicker(th, term.Picker{Label: "Machine", Value: "windows-11", Options: options, Disabled: true, Width: 32})})
	dialog := term.Dialog{Title: "Choose a console route", Body: "Route ally through /dev/ttyUSB0.", Open: true, Width: 52, Actions: []term.Plaque{{Label: "Cancel", Kind: term.PlaqueQuiet}, {Label: "Route", Kind: term.PlaqueGold}}, FocusAction: 1}
	specimen(&b, "Dialog", state{"Open", term.RenderDialog(th, dialog)})
	specimen(&b, "Confirm",
		state{"Action disabled", term.RenderConfirm(th, term.Confirm{Title: "Stop server", Body: "This stops mika-server at 203.0.113.70:2222.", Verb: "Stop", Kind: term.PlaqueDanger, RequiredName: "mika-server", TypedName: "mika", FocusField: true, Open: true, Width: 58})},
		state{"Action enabled", term.RenderConfirm(th, term.Confirm{Title: "Stop server", Body: "This stops mika-server at 203.0.113.70:2222.", Verb: "Stop", Kind: term.PlaqueDanger, RequiredName: "mika-server", TypedName: "mika-server", Open: true, Width: 58})})
	specimen(&b, "Placard",
		state{"With action", term.RenderPlacard(th, term.Placard{Text: "fedora-43 was pinned", Action: "Undo", Progress: .72, Width: 48})},
		state{"Without action", term.RenderPlacard(th, term.Placard{Text: "Console route saved", Progress: .34, Width: 48})})
	return b.String()
}
