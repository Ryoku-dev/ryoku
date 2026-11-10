package shell

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

var shellKeys = []event.Filter{
	key.Filter{Name: "K", Required: key.ModShortcut},
	key.Filter{Name: key.NameEscape},
}

// Router owns tab selection and the shell's Escape stack.
type Router struct {
	Tab         string
	Session     string
	NeedsYou    int
	PaletteOpen bool
	invalidate  func()
	paletteTag  any
	tabs        [5]kit.Plaque
}

func NewRouter(tab, session string, invalidate func()) *Router {
	if !validTab(tab) {
		tab = protocol.TabAsk
	}
	return &Router{Tab: tab, Session: session, invalidate: invalidate}
}

func (r *Router) Navigate(tab, session string) {
	if validTab(tab) {
		r.Tab = tab
	}
	if session != "" {
		r.Session = session
	}
	if r.invalidate != nil {
		r.invalidate()
	}
}

func (r *Router) OpenPalette() {
	r.PaletteOpen = true
	if r.invalidate != nil {
		r.invalidate()
	}
}

func (r *Router) Dismiss(overlays *look.Overlays) bool {
	if overlays != nil {
		if tag, ok := overlays.Top(); ok {
			overlays.DismissTop()
			if tag == r.paletteTag {
				r.PaletteOpen = false
			}
			if r.invalidate != nil {
				r.invalidate()
			}
			return true
		}
	}
	if r.PaletteOpen {
		r.PaletteOpen = false
		if r.invalidate != nil {
			r.invalidate()
		}
		return true
	}
	return false
}

func (r *Router) HandleKeys(gtx layout.Context, overlays *look.Overlays) {
	for {
		raw, ok := gtx.Event(shellKeys...)
		if !ok {
			return
		}
		e, ok := raw.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == "K" && e.Modifiers == key.ModShortcut:
			r.OpenPalette()
		case e.Name == key.NameEscape:
			r.Dismiss(overlays)
		}
	}
}

func (r *Router) LayoutRail(gtx layout.Context, th *look.Theme, class kit.FrameClass) layout.Dimensions {
	ids := []string{protocol.TabAsk, protocol.TabAgent, protocol.TabProjects, protocol.TabIndex, protocol.TabSettings}
	labels := []string{i18n.T("Ask"), i18n.T("Agents"), i18n.T("Projects"), i18n.T("Index"), i18n.T("Settings")}
	children := make([]layout.FlexChild, len(ids))
	for index := range ids {
		index := index
		button := &r.tabs[index]
		button.Label = labels[index]
		button.Kind = kit.PlaqueQuiet
		if r.Tab == ids[index] {
			button.Kind = kit.PlaqueOn
		}
		if button.Clicked(gtx) {
			r.Navigate(ids[index], "")
		}
		layoutButton := func(gtx layout.Context) layout.Dimensions {
			if index != 1 || r.NeedsYou <= 0 {
				return button.Layout(gtx, th.Theme)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return button.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Amber, i18n.Tf("%d", r.NeedsYou))),
			)
		}
		widget := func(gtx layout.Context) layout.Dimensions {
			if class == kit.FrameCompact {
				return layout.Inset{Left: 4, Right: 4}.Layout(gtx, layoutButton)
			}
			return layout.Inset{Top: 8, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layoutButton(gtx)
			})
		}
		if class == kit.FrameCompact {
			children[index] = layout.Flexed(1, widget)
		} else {
			children[index] = layout.Rigid(widget)
		}
	}
	if class == kit.FrameCompact {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func validTab(tab string) bool {
	switch tab {
	case protocol.TabAsk, protocol.TabAgent, protocol.TabProjects, protocol.TabIndex, protocol.TabSettings:
		return true
	default:
		return false
	}
}
