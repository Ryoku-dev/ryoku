package look

import (
	"testing"
	"time"
)

func TestPickerKeepsNearbyRoutesVisibleWhenSelectedModelOpens(t *testing.T) {
	frame := newMotionFrame(t)
	var picker picker
	picker.open()
	options := []SelectOption{
		{Label: "Auto"},
		{Label: "Fast"},
		{Label: "Smart"},
		{Label: "LFM3 Omni 2B", Detail: "On this computer", Group: "On this computer", Chips: []string{"Light", "Tools"}},
	}
	for range 147 {
		options = append(options, SelectOption{Label: "Another model", Detail: "Provider"})
	}
	picker.Layout(frame.context(), frame.theme, options, 3, true)
	if picker.list.Position.First != 0 {
		t.Fatalf("initial reveal scrolled past the routes to %d", picker.list.Position.First)
	}
	frame.finish(40 * time.Millisecond)
	picker.Layout(frame.context(), frame.theme, options, 3, true)
	if picker.list.Position.First != 0 || picker.list.Position.Count <= 3 {
		t.Fatalf("nearby routes or the selected model are not visible: %+v", picker.list.Position)
	}
}
