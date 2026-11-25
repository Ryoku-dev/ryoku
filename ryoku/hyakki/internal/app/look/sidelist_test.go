package look

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

type sideListFrame struct {
	*motionFrame
	list     SideList
	overlays Overlays
	events   []SideEvent
	focus    bool
}

func newSideListFrame(t *testing.T) *sideListFrame {
	frame := &sideListFrame{motionFrame: newMotionFrame(t)}
	frame.layout()
	return frame
}

func (frame *sideListFrame) layout() {
	groups := []SideGroup{{Title: "Sessions", Rows: []SideRow{
		{ID: "one", Title: "First", Menu: true},
		{ID: "two", Title: "Second", Menu: true},
		{ID: "three", Title: "Third"},
	}}}
	gtx := frame.context()
	if frame.focus {
		frame.list.Focus(gtx)
		frame.focus = false
	}
	frame.list.Layout(gtx, frame.theme, &frame.overlays, groups, "one")
	frame.overlays.Layout(gtx, frame.theme)
	frame.finish(time.Millisecond)
	frame.events = append(frame.events, frame.list.Events()...)
}

func (frame *sideListFrame) take() []SideEvent {
	events := frame.events
	frame.events = nil
	return events
}

func TestSideListArrowsMoveTheCursorAndEnterOpensItsRow(t *testing.T) {
	frame := newSideListFrame(t)
	frame.focus = true
	frame.layout()
	frame.layout()
	press := func(name key.Name) {
		frame.router.Queue(key.Event{Name: name, State: key.Press})
		frame.layout()
	}
	press(key.NameDownArrow)
	press(key.NameDownArrow)
	press(key.NameReturn)
	events := frame.take()
	if len(events) != 1 || events[0].Kind != SideOpen || events[0].ID != "three" {
		t.Fatalf("events after Down, Down, Enter = %#v, want the third row opened", events)
	}
}

func TestSideListSecondaryReleaseAsksForTheMenuOfARowThatHasOne(t *testing.T) {
	frame := newSideListFrame(t)
	// rows follow the group header so find each by walking down the list
	secondary := func(y float32) []SideEvent {
		at := f32.Pt(40, y)
		frame.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonSecondary, Position: at})
		frame.layout()
		if events := frame.take(); len(events) != 0 {
			t.Fatalf("a secondary press alone raised %#v", events)
		}
		frame.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: at})
		frame.layout()
		return frame.take()
	}
	asked := map[string]bool{}
	for y := float32(4); y < 200; y += 8 {
		for _, event := range secondary(y) {
			if event.Kind != SideMenu {
				t.Fatalf("a secondary release raised %#v", event)
			}
			asked[event.ID] = true
		}
	}
	if !asked["one"] || !asked["two"] || asked["three"] {
		t.Fatalf("menus asked for: %v, want the two rows that have one", asked)
	}
}

func TestSideListReportsWhereItsSelectedRowStands(t *testing.T) {
	frame := newSideListFrame(t)
	gtx := frame.context()
	dimensions := frame.list.Layout(gtx, frame.theme, &frame.overlays, []SideGroup{{Rows: []SideRow{
		{ID: "one", Title: "First"}, {ID: "two", Title: "Second"},
	}}}, "two")
	frame.finish(time.Millisecond)
	if !dimensions.HasSelected || dimensions.Key != "two" {
		t.Fatalf("selected row = %#v", dimensions)
	}
	if dimensions.Selected.Empty() || dimensions.Selected.Min.Y == 0 || !dimensions.Selected.In(image.Rect(0, 0, 400, 300)) {
		t.Fatalf("the second row stands at %v, want a place below the first inside the list", dimensions.Selected)
	}
}
