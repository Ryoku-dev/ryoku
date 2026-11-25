package shell

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"ryoku-rashin/internal/app/look"
)

func TestShortSelectKeyboardSkipsDisabledChoicesAndCommits(t *testing.T) {
	s := newStage(t, image.Pt(960, 600))
	s.open("ask/new")
	var selection look.Select
	var overlays look.Overlays
	var navigation Router
	options := []look.SelectOption{{Label: "Summoner"}, {Label: "Unavailable", Disabled: true}, {Label: "Manager"}, {Label: "Peer"}}
	selected := 3
	s.surface = func(gtx layout.Context) layout.Dimensions {
		navigation.HandleKeys(gtx, &overlays)
		s.shell.Frame(gtx)
		local := gtx
		local.Constraints = layout.Constraints{Min: image.Pt(240, 0), Max: image.Pt(240, 340)}
		offset := op.Offset(image.Pt(320, 200)).Push(gtx.Ops)
		selection.Label = "Reporting manager"
		selection.LayoutOptions(local, s.runtime.Theme(), &overlays, options, selected)
		if value, changed := selection.Changed(); changed {
			selected = value
		}
		offset.Pop()
		overlays.Layout(gtx, s.runtime.Theme())
		return layout.Dimensions{Size: s.size}
	}
	s.frame()
	s.settle()
	s.click("Reporting manager: Peer")
	s.press(key.NameHome, 0)
	s.press(key.NameDownArrow, 0)
	s.press(key.NameReturn, 0)
	s.await("reviewed manager selected by keys", func() bool { return selected == 2 && !selection.Opened(&overlays) })
	s.click("Reporting manager: Manager")
	s.press(key.NameEnd, 0)
	s.press(key.NameEscape, 0)
	s.await("dismissed selection unchanged", func() bool { return !selection.Opened(&overlays) })
	if selected != 2 {
		t.Fatal("Escape committed the highlighted replacement")
	}
}
