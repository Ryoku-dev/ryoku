package placement

import (
	"image"
	"math"
	"testing"
	"time"
)

func TestDragThreshold(t *testing.T) {
	th := mustSpec(t).Placement.DragThresholdPx
	if th != 4 {
		t.Fatalf("drag threshold %d, want 4", th)
	}
	var d DragState
	d.Origin = image.Pt(500, 300)
	d.Press(image.Pt(100, 100))
	for _, p := range []image.Point{{101, 101}, {103, 102}, {104, 100}, {100, 96}} {
		if dragging, pos := d.Move(p); dragging || pos != d.Origin {
			t.Fatalf("move to %v within the threshold dragged to %v", p, pos)
		}
	}
	if dragging, pos := d.Move(image.Pt(105, 100)); !dragging || pos != image.Pt(505, 300) {
		t.Fatalf("move past the threshold: dragging=%v pos=%v", dragging, pos)
	}
	if dragging, pos := d.Move(image.Pt(101, 99)); !dragging || pos != image.Pt(501, 299) {
		t.Fatalf("a drag stopped when the pointer came back near the press: %v %v", dragging, pos)
	}
	if !d.Release() {
		t.Fatal("a drag released as a click")
	}
	if d.Release() {
		t.Fatal("a second release reported a drag")
	}
	if dragging, _ := d.Move(image.Pt(300, 300)); dragging {
		t.Fatal("moving with no press dragged")
	}

	var click DragState
	click.Press(image.Pt(10, 10))
	click.Move(image.Pt(12, 13))
	if click.Release() {
		t.Fatal("a 3.6 px wobble became a drag")
	}

	loose := DragState{Threshold: 10}
	loose.Press(image.Pt(0, 0))
	if dragging, _ := loose.Move(image.Pt(8, 0)); dragging {
		t.Fatal("an explicit threshold was ignored")
	}
	if dragging, pos := loose.Move(image.Pt(11, 0)); !dragging || pos != image.Pt(11, 0) {
		t.Fatalf("zero origin drag position %v", pos)
	}
}

func TestFlightPlansDisplayChange(t *testing.T) {
	f := mustSpec(t).Flight
	centre := image.Pt(960, 540)
	for _, c := range []struct {
		from, to image.Point
		want     time.Duration
	}{
		{image.Pt(100, 100), image.Pt(130, 140), time.Duration(f.ClampS[0] * float64(time.Second))},
		{image.Pt(0, 900), image.Pt(900, 900), time.Second},
		{image.Pt(0, 0), image.Pt(3000, 2000), time.Duration(f.ClampS[1] * float64(time.Second))},
	} {
		dur, path := Flight(c.from, c.to, centre, f)
		if math.Abs(float64(dur-c.want)) > float64(time.Millisecond) {
			t.Errorf("%v→%v: %v, want %v", c.from, c.to, dur, c.want)
		}
		if path(0) != c.from || path(1) != c.to || path(-1) != c.from || path(2) != c.to {
			t.Errorf("%v→%v endpoints %v %v", c.from, c.to, path(0), path(1))
		}
	}

	from, to := image.Pt(0, 900), image.Pt(900, 900)
	_, path := Flight(from, to, centre, f)
	mid := path(0.5)
	bow := float64(to.Y - mid.Y)
	if !(bow > 0 && bow <= f.BowMaxPx) {
		t.Fatalf("mid-flight %v is not bowed toward the centre within %v px", mid, f.BowMaxPx)
	}
	prev := path(0).X
	for i := 1; i <= 50; i++ {
		x := path(float64(i) / 50).X
		if x < prev {
			t.Fatalf("flight went backwards at %d/50: %d < %d", i, x, prev)
		}
		prev = x
	}
	if early := path(0.1); early.X > 900/10 {
		t.Fatalf("the eased flight left too fast: %v at 10%%", early)
	}

	dur, still := Flight(to, to, centre, f)
	if dur != 0 || still(0.3) != to {
		t.Fatalf("a flight to the same point took %v", dur)
	}
	_, straight := Flight(image.Pt(960, 100), image.Pt(960, 900), centre, f)
	if p := straight(0.5); p.X != 960 {
		t.Fatalf("a flight through the centre bowed to %v", p)
	}
}
