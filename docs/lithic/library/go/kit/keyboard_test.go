package kit

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

type confirmFixture struct {
	confirm   Confirm
	opener    Plaque
	typed     string
	confirmed int
	harness   *keyHarness
}

func newConfirmFixture(t *testing.T, typed string) *confirmFixture {
	th := theme.New(palette.Signature(true))
	f := &confirmFixture{typed: typed}
	f.harness = newKeyHarness(t, func(gtx layout.Context) {
		f.opener.Label = "Remove scratch-arch"
		f.opener.Layout(gtx, th)
		if f.confirm.Layout(gtx, th, "Remove scratch-arch", "This deletes scratch-arch and its disk.", "Remove", f.typed) {
			f.confirmed++
		}
	})
	f.harness.settle()
	f.harness.frame(func(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &f.opener.Clickable}) })
	f.confirm.Dialog.Opener = &f.opener.Clickable
	f.confirm.Dialog.Open = true
	f.harness.settle()
	return f
}

func TestConfirmOpensOnCancelSoReturnCancels(t *testing.T) {
	f := newConfirmFixture(t, "")
	if !f.harness.focused(&f.confirm.cancel.Clickable) {
		t.Fatal("a fresh Confirm did not put focus on Cancel")
	}
	f.harness.press(key.NameReturn)
	if f.confirmed != 0 || f.confirm.Dialog.Open {
		t.Fatalf("Return on a fresh Confirm: confirmed %d, open %v; want it cancelled", f.confirmed, f.confirm.Dialog.Open)
	}
	if !f.harness.focused(&f.opener.Clickable) {
		t.Error("cancelling did not return focus to the opener")
	}
}

func TestConfirmTabThenReturnConfirms(t *testing.T) {
	f := newConfirmFixture(t, "")
	f.harness.router.MoveFocus(key.FocusForward)
	f.harness.settle()
	f.harness.press(key.NameReturn)
	if f.confirmed != 1 {
		t.Fatalf("Tab then Return confirmed %d times, want 1", f.confirmed)
	}
}

func TestConfirmWithTypedNameFocusesTheField(t *testing.T) {
	f := newConfirmFixture(t, "scratch-arch")
	if !f.harness.focused(&f.confirm.field.Editor) {
		t.Fatal("a Confirm that needs the typed name did not focus its field")
	}
	f.harness.press(key.NameEscape)
	if f.confirm.Dialog.Open || f.confirmed != 0 {
		t.Fatal("Escape did not close the Confirm without confirming")
	}
	if !f.harness.focused(&f.opener.Clickable) {
		t.Error("Escape did not return focus to the opener")
	}
}

func TestDialogKeepsFocusItsContentTakes(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var dialog Dialog
	var first Plaque
	var field TextField
	opened := false
	harness := newKeyHarness(t, func(gtx layout.Context) {
		if dialog.Open && !opened {
			opened = true
			field.Focus(gtx)
		}
		dialog.Layout(gtx, th, "Rename", func(gtx layout.Context) layout.Dimensions {
			first.Label = "Suggest a name"
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return first.Layout(gtx, th) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return field.Layout(gtx, th) }),
			)
		})
	})
	harness.settle()
	dialog.Open = true
	harness.settle()
	if !harness.focused(&field.Editor) {
		t.Fatal("the dialog took focus from the field its content focused on open")
	}
}

func TestPlaqueAnswersReturnAndKeypadEnter(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var plaque Plaque
	clicks := 0
	harness := newKeyHarness(t, func(gtx layout.Context) {
		plaque.Label = "Connect"
		if plaque.Clicked(gtx) {
			clicks++
		}
		plaque.Layout(gtx, th)
	})
	harness.settle()
	harness.frame(func(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &plaque.Clickable}) })
	harness.settle()
	harness.press(key.NameReturn)
	if clicks != 1 {
		t.Fatalf("Return pressed the plaque %d times, want 1", clicks)
	}
	harness.press(key.NameEnter)
	if clicks != 2 {
		t.Fatalf("keypad Enter pressed the plaque %d times in all, want 2", clicks)
	}
	harness.press(key.NameSpace)
	if clicks != 3 {
		t.Fatalf("Space pressed the plaque %d times in all, want 3", clicks)
	}
}

func TestTabletReturnRunsThePrimaryAction(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var tablet Tablet
	selects, actions := 0, 0
	harness := newKeyHarness(t, func(gtx layout.Context) {
		if tablet.Clicked(gtx) {
			selects++
		}
		if tablet.DoubleClicked(gtx) {
			actions++
		}
		tablet.Layout(gtx, th, TabletContent{Title: "ubuntu-24.04", Progress: -1}, false, false)
	})
	harness.settle()
	harness.frame(func(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &tablet.Clickable}) })
	harness.settle()
	harness.press(key.NameSpace)
	if selects != 1 || actions != 0 {
		t.Fatalf("Space: %d selections and %d actions, want 1 and 0", selects, actions)
	}
	harness.press(key.NameReturn)
	if selects != 2 || actions != 1 {
		t.Fatalf("Return: %d selections and %d actions in all, want 2 and 1", selects, actions)
	}
}

func TestEscapeInsideADialog(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var dialog Dialog
	var search Search
	var picker Picker
	selected := 1
	harness := newKeyHarness(t, func(gtx layout.Context) {
		dialog.Layout(gtx, th, "Fleet view", func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return search.Layout(gtx, th) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims := picker.Layout(gtx, th, "Group by", []string{"Nothing", "Server", "State"}, selected)
					if index, ok := picker.Changed(gtx); ok {
						selected = index
					}
					return dims
				}),
			)
		})
	})
	open := func() {
		dialog.Open = true
		harness.settle()
	}

	harness.settle()
	open()
	harness.frame(func(gtx layout.Context) { search.Focus(gtx) })
	search.SetText("ally")
	harness.settle()
	harness.press(key.NameEscape)
	if search.Text() != "" || !dialog.Open {
		t.Fatalf("Escape in a Search with text: text %q, dialog open %v; want it cleared and the dialog open", search.Text(), dialog.Open)
	}
	harness.press(key.NameEscape)
	if dialog.Open {
		t.Fatal("Escape in an empty Search did not reach the dialog")
	}

	harness.settle()
	open()
	harness.frame(func(gtx layout.Context) { picker.Focus(gtx) })
	harness.settle()
	harness.press(key.NameDownArrow)
	if !picker.Open() {
		t.Fatal("Down did not open the picker inside the dialog")
	}
	harness.press(key.NameEscape)
	if picker.Open() || !dialog.Open || selected != 1 {
		t.Fatalf("Escape in an open picker: picker open %v, dialog open %v, value %d; want only the picker closed", picker.Open(), dialog.Open, selected)
	}
	harness.press(key.NameEscape)
	if dialog.Open {
		t.Fatal("Escape on a closed picker did not reach the dialog")
	}
}

func TestEmptySearchInAPopoverLetsEscapeClose(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var popover Popover
	var anchor Plaque
	var search Search
	harness := newKeyHarness(t, func(gtx layout.Context) {
		popover.Opener = &anchor.Clickable
		popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			anchor.Label = "Filter"
			return anchor.Layout(gtx, th)
		}, func(gtx layout.Context) layout.Dimensions { return search.Layout(gtx, th) })
	})
	harness.settle()
	popover.Open = true
	harness.settle()
	harness.frame(func(gtx layout.Context) { search.Focus(gtx) })
	harness.settle()
	if !harness.focused(&search.Editor) {
		t.Fatal("the popover pulled focus away from its Search")
	}
	harness.press(key.NameEscape)
	if popover.Open {
		t.Fatal("Escape in an empty Search did not close its popover")
	}
	if !harness.focused(&anchor.Clickable) {
		t.Error("closing the popover did not return focus to its opener")
	}
}

func TestPickerInsideAPopover(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var outer Popover
	var anchor Plaque
	var picker Picker
	selected := 0
	harness := newKeyHarness(t, func(gtx layout.Context) {
		outer.Opener = &anchor.Clickable
		outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			anchor.Label = "View options"
			return anchor.Layout(gtx, th)
		}, func(gtx layout.Context) layout.Dimensions {
			dims := picker.Layout(gtx, th, "Sort by", []string{"Name", "State", "Server"}, selected)
			if index, ok := picker.Changed(gtx); ok {
				selected = index
			}
			return dims
		})
	})
	harness.settle()
	outer.Open = true
	harness.settle()
	harness.frame(func(gtx layout.Context) { picker.Focus(gtx) })
	harness.settle()
	harness.press(key.NameDownArrow)
	if !picker.Open() || !outer.Open {
		t.Fatalf("opening a picker inside a popover: picker open %v, outer open %v; want both", picker.Open(), outer.Open)
	}
	if !harness.focused(&picker.menu.clicks[0].Clickable) {
		t.Fatal("the outer popover pulled focus back from the inner menu")
	}
	harness.press(key.NameDownArrow, key.NameDownArrow, key.NameReturn)
	if selected != 2 || picker.Open() || !outer.Open {
		t.Fatalf("Down, Down, Return chose %d (picker open %v, outer open %v); want 2 with only the picker closed", selected, picker.Open(), outer.Open)
	}
	if !harness.focused(&picker.button.Clickable) {
		t.Error("choosing did not return focus to the picker inside the popover")
	}
	harness.press(key.NameEscape)
	if outer.Open {
		t.Error("Escape on the closed picker did not close the outer popover")
	}
}
