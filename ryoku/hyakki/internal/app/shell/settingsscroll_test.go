package shell

import (
	"image"
	goruntime "runtime"
	"slices"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

// A wheel run down the whole of Settings and back. Each section used to be one
// row of the scroll list, so a frame laid out all of Teams or Memory around
// the screenful shown, copied the whole store a dozen times over and drew the
// sidebar's icons again: some 1,500 allocations and 370 KB a frame, and
// scrolling fast lagged.
func TestScrollingSettingsLaysOutOnlyWhatIsShown(t *testing.T) {
	size := image.Pt(1612, 1043)
	stage := newStage(t, size)
	stage.open("settings")
	at := f32.Pt(float32(size.X/2), float32(size.Y/2))
	var before, after goruntime.MemStats
	var allocations, bytes []uint64
	// how far above and under the pane anything was laid out; a node's top is where it was drawn, its bottom can run to the end of the list
	above, under, far := 0, 0, false
	for _, step := range []float32{40, -40} {
		for range 120 {
			stage.router.Queue(pointer.Event{
				Kind: pointer.Scroll, Source: pointer.Mouse, PointerID: 1, Position: at,
				Scroll: f32.Pt(0, step), Time: stage.now.Sub(storyboardEpoch),
			})
			goruntime.ReadMemStats(&before)
			stage.frame()
			goruntime.ReadMemStats(&after)
			allocations = append(allocations, after.Mallocs-before.Mallocs)
			bytes = append(bytes, after.TotalAlloc-before.TotalAlloc)
			for _, node := range stage.nodes() {
				if bounds := node.Desc.Bounds; !(image.Rectangle{Max: size}).In(bounds) {
					above, under = max(above, -bounds.Min.Y), max(under, bounds.Min.Y-size.Y)
				}
			}
			if zoom, ok := stage.named("Zoom"); ok && zoom.Desc.Bounds.Min.Y >= 0 && zoom.Desc.Bounds.Min.Y < size.Y {
				far = true
			}
		}
	}
	if !far {
		t.Fatal("the wheel never brought Appearance, past every long section, into view")
	}
	// the tallest block, Teams' Sandbox, is under three screens; more around the pane is a whole section laid out
	if above > 3*size.Y || under > 3*size.Y {
		t.Errorf("a frame laid out %d px above the pane and %d px under it", above, under)
	}
	slices.Sort(allocations)
	slices.Sort(bytes)
	t.Logf("above %d, under %d, median %d allocations and %d bytes", above, under, allocations[len(allocations)/2], bytes[len(bytes)/2])
	if median := allocations[len(allocations)/2]; median > 900 {
		t.Errorf("a scrolled frame allocates %d times, the budget is 900", median)
	}
	if median := bytes[len(bytes)/2]; median > 150_000 {
		t.Errorf("a scrolled frame allocates %d bytes, the budget is 150000", median)
	}
}
