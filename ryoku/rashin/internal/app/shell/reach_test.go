package shell

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

var restRoutes = []string{
	"ask/new", "ask/ask-20261110-094200-a17c",
	"agent/new", "agent/agent-20261110-101500-b82d", "agent/agent-working", "agent/agent-done",
	"projects", "projects/hyperion/overview", "projects/hyperion/graph", "projects/hyperion/changes",
	"projects/hyperion/tasks", "projects/hyperion/activity",
	"index",
	"settings/rashin", "settings/providers", "settings/api", "settings/ask", "settings/agents", "settings/index", "settings/appearance",
}

// A control that is drawn but sits under something that takes its presses
// looks alive and does nothing. This walks every page at rest, in each window
// class, and asks what a press on each control would be delivered to.
func TestEveryControlAtRestTakesThePointer(t *testing.T) {
	for _, size := range []image.Point{{1440, 900}, {1000, 720}, {560, 800}} {
		stage := newStage(t, size)
		for _, route := range restRoutes {
			stage.open(route)
			if dead := stage.unreachable(); len(dead) > 0 {
				t.Errorf("%dx%d %s:\n  %s", size.X, size.Y, route, strings.Join(dead, "\n  "))
			}
		}
	}
}

// The composer once lay under an area that waited for dropped files. It
// listened for nothing the pointer does and still took every press.
func TestTheSweepSeesAControlUnderAnAreaThatKeepsItsPresses(t *testing.T) {
	for name, test := range map[string]struct{ passes, dead bool }{
		"an area that keeps presses":    {false, true},
		"a tracker that passes them on": {true, false},
	} {
		stage := &stage{t: t, size: image.Pt(200, 100)}
		var button widget.Clickable
		cover := new(int)
		draw := func() {
			stage.ops.Reset()
			gtx := layout.Context{
				Ops: &stage.ops, Source: stage.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(stage.size),
			}
			button.Layout(gtx, func(layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(80, 30)} })
			if test.passes {
				defer pointer.PassOp{}.Push(gtx.Ops).Pop()
			}
			area := clip.Rect{Max: stage.size}.Push(gtx.Ops)
			event.Op(gtx.Ops, cover)
			gtx.Event(transfer.TargetFilter{Target: cover, Type: "text/uri-list"})
			area.Pop()
			stage.router.Frame(&stage.ops)
		}
		// the router learns what the button listens for from its first frame
		draw()
		draw()
		if dead := stage.unreachable(); (len(dead) > 0) != test.dead {
			t.Errorf("%s: unreachable = %v, want dead %v", name, dead, test.dead)
		}
	}
}
