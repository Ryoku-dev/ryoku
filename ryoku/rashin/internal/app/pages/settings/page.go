// Package settings is the Settings page: whether Rashin runs, the providers
// Ask routes through, how Ask and the agents behave, what is indexed and how
// the app looks.
package settings

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
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
	header  look.PaneHeader
	scroll  look.Scroller
	index   kit.SectionIndex
	current int
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
	usage         usage
	api           apiPanel

	models      protocol.ModelsResult
	modelsReady bool
	route       look.Select
	habits      kit.Toggle

	harness   look.Select
	approvals look.Select
	askMay    look.Select
	idle      kit.Stepper
	notices   kit.Toggle
	atOnce    kit.Stepper
	install   env.HarnessInstall
	instances map[string]*instanceEditor
	fresh     instanceEditor
	// id of the unfolded harness, or newInstance
	openInstance string

	system   kit.Toggle
	semantic kit.Toggle

	zoom   kit.Stepper
	motion kit.Toggle
}

func New() *Page {
	return &Page{providers: make(map[string]*providerRow), instances: make(map[string]*instanceEditor)}
}

// a name it doesn't know leaves the page where it is
func (p *Page) OpenSection(name string) {
	p.providerOrder, p.usage.asked = nil, false
	sections := map[string]int{
		"rashin": sectionRashin, "about": sectionRashin, "providers": sectionProviders, "api": sectionAPI, "ask": sectionAsk,
		"agent": sectionAgents, "agents": sectionAgents, "teams": sectionTeams, "index": sectionIndex, "appearance": sectionAppearance,
		"keys": sectionKeys, "keyboard": sectionKeys,
	}
	if section, ok := sections[name]; ok {
		p.jump(section)
	}
}

// the question that stands before turning Rashin off
func (p *Page) AskTurnOff() { p.offOpen, p.off.Dialog.Open, p.off.Danger = true, true, true }

func (p *Page) jump(section int) {
	p.current, p.jumped = section, true
	p.scroll.ScrollTo(section)
}

// empty on purpose, settings keeps the list of the destination it was opened from
func (p *Page) List(layout.Context, *env.Env) look.ListDimensions { return look.ListDimensions{} }

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

func sectionLabels() []string {
	return []string{
		i18n.T("Rashin"), i18n.T("Providers"), i18n.T("Local API"), i18n.T("Ask"), i18n.T("Agents"), i18n.T("Teams"), i18n.T("Index"),
		i18n.T("Appearance"), i18n.T("Keyboard"),
	}
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.loadModels(e)
	th := e.Theme
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
	side, column, rail, gap := gtx.Dp(36), gtx.Dp(look.SettingsColumn), gtx.Dp(200), gtx.Dp(34)
	labels := sectionLabels()
	p.follow()
	wide := size.X >= column+rail+gap+2*side
	if wide {
		// index and column are centred together so the column stays in the middle of what's left
		left := (size.X - column - rail - gap) / 2
		look.Place(gtx, image.Rect(left, gtx.Dp(34), left+rail, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return p.index.Layout(gtx, th.Theme, labels, p.current, kit.FrameExpanded)
		})
		p.choose()
		look.Place(gtx, image.Rect(left+rail+gap, 0, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
			return p.layoutSections(gtx, e, 0, column)
		})
		return layout.Dimensions{Size: size}
	}
	width := min(column, size.X-2*side)
	left := (size.X - width) / 2
	chips := 0
	if size.X >= gtx.Dp(520) {
		chips = look.Place(gtx, image.Rect(left, gtx.Dp(13), left+width, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return p.index.Layout(gtx, th.Theme, labels, p.current, kit.FrameMedium)
		}).Size.Y + gtx.Dp(13)
		p.choose()
	}
	look.Place(gtx, image.Rect(0, chips, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
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

func (p *Page) loadModels(e *env.Env) {
	if p.modelsReady {
		return
	}
	p.modelsReady = true
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &p.models, func(err error) {
		if err != nil {
			p.modelsReady = false
		}
	})
}

func set(e *env.Env, section string, values map[string]any) {
	e.SetSettings(map[string]any{section: values}, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}
