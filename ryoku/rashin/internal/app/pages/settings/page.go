// Package settings is the Settings page: whether Rashin runs, the providers
// Ask routes through, how Ask and the agents behave, what is indexed and how
// the app looks.
package settings

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

const (
	sectionRashin = iota
	sectionProviders
	sectionAsk
	sectionAgents
	sectionIndex
	sectionAppearance
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

	models      protocol.ModelsResult
	modelsReady bool
	route       look.Select
	habits      kit.Toggle

	harness   look.Select
	approvals look.Select
	idle      kit.Stepper
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
	sections := map[string]int{
		"rashin": sectionRashin, "about": sectionRashin, "providers": sectionProviders, "ask": sectionAsk,
		"agent": sectionAgents, "agents": sectionAgents, "index": sectionIndex, "appearance": sectionAppearance,
	}
	if section, ok := sections[name]; ok {
		p.jump(section)
	}
}

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
	if !p.offOpen {
		return false
	}
	body := i18n.T("Its daemon stops, every agent stops with it, and nothing runs until you turn it on again. Your sessions and settings stay.")
	if p.off.Layout(gtx, e.Theme.Theme, i18n.T("Turn Rashin off?"), body, i18n.T("Turn off"), "") {
		env.Spawn("rashin", "off")
	}
	if p.off.Closed() {
		p.offOpen = false
	}
	return p.off.Dialog.Open
}

func sectionLabels() []string {
	return []string{i18n.T("Rashin"), i18n.T("Providers"), i18n.T("Ask"), i18n.T("Agents"), i18n.T("Index"), i18n.T("Appearance")}
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
		look.Place(gtx, image.Rect(left+rail+gap, 0, left+rail+gap+column, size.Y), func(gtx layout.Context) layout.Dimensions {
			return p.layoutSections(gtx, e)
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
	look.Place(gtx, image.Rect(left, chips, left+width, size.Y), func(gtx layout.Context) layout.Dimensions {
		return p.layoutSections(gtx, e)
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

func (p *Page) layoutSections(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	return p.scroll.Layout(gtx, th, sectionCount, func(gtx layout.Context, index int) layout.Dimensions {
		top, bottom := th.Sp.S5, th.Sp.S1
		if index == sectionCount-1 {
			// room under the last section so it can still reach the top
			bottom = th.Sp.S6
		}
		return layout.Inset{Top: top, Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			switch index {
			case sectionRashin:
				return p.layoutRashin(gtx, e)
			case sectionProviders:
				return p.layoutProviders(gtx, e)
			case sectionAsk:
				return p.layoutAsk(gtx, e)
			case sectionAgents:
				return p.layoutAgents(gtx, e)
			case sectionIndex:
				return p.layoutIndex(gtx, e)
			default:
				return p.layoutAppearance(gtx, e)
			}
		})
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
