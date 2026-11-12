package kit

import (
	"image"
	"reflect"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

func uniformHeights(count, height int) []int {
	heights := make([]int, count)
	for index := range heights {
		heights[index] = height
	}
	return heights
}

func TestRailPositions(t *testing.T) {
	const start, tile, gap, foot = 96, 64, 8, 504
	cases := []struct {
		name      string
		heights   []int
		footIndex int
		newSlot   bool
		want      []int
	}{
		{"today", uniformHeights(3, tile), 0, true, []int{96, 240, 440}},
		{"explicit last", uniformHeights(3, tile), 2, true, []int{96, 240, 440}},
		{"no new", uniformHeights(3, tile), 0, false, []int{96, 168, 440}},
		{"long rail", uniformHeights(6, tile), 4, true, []int{96, 240, 312, 384, 368, 440}},
		{"none at foot", uniformHeights(2, tile), 2, true, []int{96, 240}},
		{"one", uniformHeights(1, tile), 0, true, []int{440}},
		{"wrapped label moves the tiles below", []int{64, 80, 64, 64}, 3, false, []int{96, 168, 256, 440}},
		{"wrapped first tile moves New's slot", []int{80, 64, 64}, 0, true, []int{96, 256, 440}},
		{"wrapped tile in the foot group grows upward", []int{64, 64, 80, 64}, 2, false, []int{96, 168, 352, 440}},
	}
	for _, test := range cases {
		got := railPositions(test.heights, test.footIndex, test.newSlot, start, tile, gap, foot)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: railPositions = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestRailTileWidth(t *testing.T) {
	// floor railTileExpanded 64, rail railExpanded 88, pad space s1 4
	const floor, rail, pad = 64, 88, 4
	cases := []struct {
		name   string
		widest int
		width  int
		wraps  bool
	}{
		{"short labels keep the 64 floor", 48, 64, false},
		{"exactly the floor", 56, 64, false},
		{"a longer label widens every tile", 66, 74, false},
		{"exactly the 80 cap", 72, 80, false},
		{"a wider label caps at 80 and wraps", 73, 80, true},
		{"a long translation wraps", 110, 80, true},
	}
	for _, test := range cases {
		width := railTileWidth(test.widest, floor, rail, pad)
		if width != test.width {
			t.Errorf("%s: railTileWidth(%d) = %d, want %d", test.name, test.widest, width, test.width)
		}
		if wraps := test.widest > width-2*pad; wraps != test.wraps {
			t.Errorf("%s: wraps = %v, want %v", test.name, wraps, test.wraps)
		}
	}
}

func TestRailSelectionFillsInPlaceAfterNoSelection(t *testing.T) {
	th := theme.New(palette.Signature(true))
	destinations := []RailDestination{{Label: "Fleet"}, {Label: "Activity"}, {Label: "Settings"}}
	for _, class := range []FrameClass{FrameExpanded, FrameCompact} {
		var rail Rail
		var ops op.Ops
		start := time.Unix(1000, 0)
		size := image.Pt(88, 600)
		if class == FrameCompact {
			size = image.Pt(400, 64)
		}
		frame := func(at time.Duration, selected int) float32 {
			ops.Reset()
			gtx := layout.Context{Ops: &ops, Now: start.Add(at), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(size)}
			rail.Layout(gtx, th, destinations, selected, class)
			return rail.selection.Value(gtx)
		}
		first := frame(0, 0)
		frame(time.Second, -1)
		settled := frame(2*time.Second, 2)
		if settled == first {
			t.Fatalf("class %d: selection did not move to the new destination", class)
		}
		if again := frame(2*time.Second+th.Motion.Select/2, 2); again != settled {
			t.Errorf("class %d: selection after no selection glided: %v then %v", class, settled, again)
		}
		gliding := frame(3*time.Second, 1)
		mid := frame(3*time.Second+th.Motion.Select/2, 1)
		end := frame(4*time.Second, 1)
		if gliding != settled || mid == settled || mid == end {
			t.Errorf("class %d: a change between shown selections should glide: %v %v %v", class, gliding, mid, end)
		}
	}
}
