// Package settings is the Settings page: whether Rashin runs, the providers
// Ask routes through, how Ask and the agents behave, what is indexed and how
// the app looks.
package settings

import (
	"image"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

const (
	sectionRashin = iota
	sectionProviders
	sectionAPI
	sectionAsk
	sectionAgents
	sectionTeams
	sectionIndex
	sectionAppearance
	sectionKeys
	sectionCount
)

// every control applies as it changes, there's nothing to save
type Page struct {
	header look.PaneHeader
	scroll look.Scroller
	// the sections stand in the sidebar's list with a search over them, a picker tops the column while the sidebar can't show them
	sections look.SideList
	find     kit.Search
	index    kit.SectionIndex
	listed   bool
	current  int
	// the section just picked, until the list has moved to it
	jumped bool

	turnOff kit.Plaque
	off     kit.Confirm
	offOpen bool
	log     kit.Plaque

	providers map[string]*providerRow
	open      string
	// provider ids in the order they were sorted into when Settings opened
	providerOrder []string
	usage         env.Usage
	api           apiPanel

	models env.ModelMenu
	route  look.Select
	habits kit.Toggle

	harness   look.Select
	approvals look.Select
	askMay    look.Select
	idle      kit.Stepper
	notices   kit.Toggle
	teams     teamsPanel
	install   env.HarnessInstall
	instances map[string]*instanceEditor
	fresh     instanceEditor
	// id of the unfolded harness, or newInstance
	openInstance string

	system   kit.Toggle
	semantic kit.Toggle

	zoom     kit.Stepper
	motion   kit.Toggle
	colours  kit.Segmented
	contrast kit.Segmented
	tint     kit.Segmented
}

func New() *Page {
	return &Page{providers: make(map[string]*providerRow), instances: make(map[string]*instanceEditor)}
}

// a name it doesn't know leaves the page where it is
func (p *Page) OpenSection(name string) {
	p.providerOrder = nil
	p.usage.Forget()
	// the older names still lead to their section
	aliases := map[string]string{"about": "rashin", "agent": "agents", "keyboard": "keys"}
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	all := sections()
	if at := slices.IndexFunc(all[:], func(item section) bool { return item.name == name }); at >= 0 {
		p.jump(at)
	}
}

// the question that stands before turning Rashin off
func (p *Page) AskTurnOff() { p.offOpen, p.off.Dialog.Open, p.off.Danger = true, true, true }

func (p *Page) jump(section int) {
	p.current, p.jumped = section, true
	p.scroll.ScrollTo(section)
}

func (p *Page) ListShown(shown bool) { p.listed = shown }

// settings lists its own sections while it is open, the New plaque still belongs to where it was opened from
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	th := e.Theme
	all := sections()
	p.find.Submit = true
	p.find.Placeholder, p.find.Hint = i18n.T("Find a setting"), "Ctrl F"
	// down from the search walks the sections it left, asked before the editor takes the key
	for {
		event, ok := gtx.Event(key.Filter{Focus: &p.find.Editor, Name: key.NameDownArrow})
		if !ok {
			break
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press {
			p.sections.Focus(gtx)
		}
	}
	for {
		event, ok := p.find.Editor.Update(gtx)
		if !ok {
			break
		}
		if submitted, ok := event.(widget.SubmitEvent); ok {
			if first := slices.IndexFunc(all[:], func(item section) bool { return item.matches(submitted.Text) }); first >= 0 {
				p.jump(first)
			}
		}
	}
	query := p.find.Text()
	size := gtx.Constraints.Max
	field := look.Place(gtx, image.Rectangle{Max: size}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return p.find.Layout(gtx, th.Theme)
	})
	top := field.Size.Y + gtx.Dp(13)

	rows := make([]look.SideRow, 0, sectionCount)
	for _, item := range all {
		if item.matches(query) {
			rows = append(rows, look.SideRow{ID: item.name, Title: item.label, Icon: item.icon})
		}
	}
	p.sections.Empty = i18n.T("No section matches what you typed.")
	list := gtx
	list.Constraints = layout.Exact(image.Pt(size.X, max(0, size.Y-top)))
	offset := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	dimensions := p.sections.Layout(list, th, e.Overlays, []look.SideGroup{{Rows: rows}}, all[p.current].name)
	offset.Pop()
	for _, event := range p.sections.Events() {
		if at := slices.IndexFunc(all[:], func(item section) bool { return item.name == event.ID }); event.Kind == look.SideOpen && at >= 0 {
			p.jump(at)
		}
	}
	dimensions.Selected = dimensions.Selected.Add(image.Pt(0, top))
	dimensions.Size = size
	return dimensions
}

// settings makes nothing new
func (p *Page) New(*env.Env) {}

func (p *Page) NewLabel() string { return "" }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	if p.api.overlay(gtx, e) {
		return true
	}
	if !p.offOpen {
		return false
	}
	body := i18n.T("Its daemon stops, every agent stops with it, and nothing runs until you turn it on again. Your sessions and settings stay.")
	if p.off.Layout(gtx, e.Theme.Theme, i18n.T("Turn Rashin off?"), body, i18n.T("Turn off"), "") {
		e.Rashin(nil, "off")
	}
	if p.off.Closed() {
		p.offOpen = false
	}
	return p.off.Dialog.Open
}

type section struct {
	// as OpenSection and the routes name it
	name  string
	label string
	icon  *widget.Icon
	// words beside its name that find it, translated as one line
	keywords string
}

func sections() [sectionCount]section {
	return [sectionCount]section{
		{"rashin", i18n.T("Rashin"), look.Icon.Thought, i18n.T("on off turn daemon log about version")},
		{"providers", i18n.T("Providers"), look.Icon.Graph, i18n.T("models routing route keys usage spend free paid local ollama llama.cpp address")},
		{"api", i18n.T("Local API"), look.Icon.Link, i18n.T("address port key token exports openai")},
		{"ask", i18n.T("Ask"), look.Icon.Ask, i18n.T("answers model memory habits shell approvals changes")},
		{"agents", i18n.T("Agents"), look.Icon.Agents, i18n.T("harness harnesses omp approvals notices idle")},
		{"teams", i18n.T("Teams"), look.Icon.Teams, i18n.T("ledger budget spend members sandbox mail sign-in login harness hidden secrets")},
		{"index", i18n.T("Index"), look.Icon.Index, i18n.T("search meaning semantic system folders hide code")},
		{"appearance", i18n.T("Appearance"), look.Icon.Show, i18n.T("theme colours colors dark light contrast tint zoom motion")},
		{"keys", i18n.T("Keyboard"), look.Icon.Key, i18n.T("keys shortcuts keybinds")},
	}
}

// every word typed has to start a word of the section's name or keywords
func (item section) matches(query string) bool {
	words := strings.Fields(strings.ToLower(item.label + " " + item.keywords))
	for _, typed := range strings.Fields(strings.ToLower(query)) {
		if !slices.ContainsFunc(words, func(word string) bool { return strings.HasPrefix(word, typed) }) {
			return false
		}
	}
	return true
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	for {
		event, ok := gtx.Event(key.Filter{Name: "F", Required: key.ModShortcut})
		if !ok {
			break
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press && p.listed {
			p.find.Focus(gtx)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			scrolled := p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0
			return p.header.Layout(gtx, th, e.Overlays, look.PaneHeaderContent{Title: i18n.T("Settings"), Scrolled: scrolled})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutBody(gtx, e) }),
	)
}

func (p *Page) layoutBody(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	side, column := gtx.Dp(36), gtx.Dp(look.SettingsColumn)
	p.follow()
	width := min(column, size.X-2*side)
	left := (size.X - width) / 2
	top := 0
	// the sections stand in the sidebar while it is open, otherwise a picker tops the column
	if !p.listed {
		all := sections()
		labels := make([]string, 0, sectionCount)
		for _, item := range all {
			labels = append(labels, item.label)
		}
		top = look.Place(gtx, image.Rect(left, gtx.Dp(13), left+width, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return p.index.Layout(gtx, th.Theme, labels, p.current, kit.FrameMedium)
		}).Size.Y + gtx.Dp(13)
		p.choose()
	}
	look.Place(gtx, image.Rect(0, top, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
		return p.layoutSections(gtx, e, left, width)
	})
	return layout.Dimensions{Size: size}
}

// follows the section at the top of the column, unless one was just picked and the list is still on its way
func (p *Page) follow() {
	first := p.scroll.Position.First
	if p.jumped {
		p.jumped = first != p.current
		return
	}
	p.current = min(max(first, 0), sectionCount-1)
}

func (p *Page) choose() {
	if selected, ok := p.index.Selected(); ok && selected < sectionCount {
		p.jump(selected)
	}
}

// the list takes the pane out to its edge so the wheel works beside the column and the thumb stands at the edge
func (p *Page) layoutSections(gtx layout.Context, e *env.Env, inset, width int) layout.Dimensions {
	th := e.Theme
	return p.scroll.Layout(gtx, th, sectionCount, func(gtx layout.Context, index int) layout.Dimensions {
		top, bottom := th.Sp.S5, th.Sp.S1
		if index == sectionCount-1 {
			// room under the last section so it can still reach the top
			bottom = th.Sp.S6
		}
		full := gtx.Constraints.Max.X
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = min(gtx.Constraints.Min.X, width), width
		defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
		dimensions := layout.Inset{Top: top, Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			switch index {
			case sectionRashin:
				return p.layoutRashin(gtx, e)
			case sectionProviders:
				return p.layoutProviders(gtx, e)
			case sectionAPI:
				return p.layoutAPI(gtx, e)
			case sectionAsk:
				return p.layoutAsk(gtx, e)
			case sectionAgents:
				return p.layoutAgents(gtx, e)
			case sectionTeams:
				return p.layoutTeams(gtx, e)
			case sectionIndex:
				return p.layoutIndex(gtx, e)
			case sectionAppearance:
				return p.layoutAppearance(gtx, e)
			default:
				return p.layoutKeys(gtx, e)
			}
		})
		return layout.Dimensions{Size: image.Pt(full, dimensions.Size.Y)}
	})
}

func set(e *env.Env, section string, values map[string]any) {
	e.SetSettings(map[string]any{section: values}, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}
