package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestRelativeAndAbsoluteFormsAgree(t *testing.T) {
	absolute, err := parsePath("M10 10H15V15L10 15ZM20 20C21 20 22 21 22 22S23 24 24 24Q25 24 25 25T26 26Z")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := parsePath("m10 10h5v5l-5 0zm10 10c1 0 2 1 2 2s1 2 2 2q1 0 1 1t1 1z")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(absolute, relative) {
		t.Fatalf("absolute %v\nrelative %v", absolute, relative)
	}
}

func TestSmoothCurvesReflectTheirControlPoint(t *testing.T) {
	segments, err := parsePath("M0 0C0 1 1 2 2 2S4 3 4 4Q5 5 6 4T8 4L9 9T10 10")
	if err != nil {
		t.Fatal(err)
	}
	want := []segment{
		{op: 'M', p: [3]point{{0, 0}}},
		{op: 'C', p: [3]point{{0, 1}, {1, 2}, {2, 2}}},
		{op: 'C', p: [3]point{{3, 2}, {4, 3}, {4, 4}}},
		{op: 'Q', p: [3]point{{5, 5}, {6, 4}}},
		{op: 'Q', p: [3]point{{7, 3}, {8, 4}}},
		{op: 'L', p: [3]point{{9, 9}}},
		// T after a line has no control point to reflect, so it takes the current point
		{op: 'Q', p: [3]point{{9, 9}, {10, 10}}},
	}
	if !reflect.DeepEqual(segments, want) {
		t.Fatalf("got %v\nwant %v", segments, want)
	}
}

func TestCompactNumbers(t *testing.T) {
	segments, err := parsePath("M120-580q0-109 75.5-184.5M.5.5l1e1-1E+1,2-2")
	if err != nil {
		t.Fatal(err)
	}
	want := []segment{
		{op: 'M', p: [3]point{{120, -580}}},
		{op: 'Q', p: [3]point{{120, -689}, {195.5, -764.5}}},
		{op: 'M', p: [3]point{{.5, .5}}},
		{op: 'L', p: [3]point{{10.5, -9.5}}},
		{op: 'L', p: [3]point{{12.5, -11.5}}},
	}
	if !reflect.DeepEqual(segments, want) {
		t.Fatalf("got %v\nwant %v", segments, want)
	}
}

func TestDrawingAfterCloseStartsAtTheSubpathStart(t *testing.T) {
	segments, err := parsePath("M1 1L2 1L2 2ZL3 3")
	if err != nil {
		t.Fatal(err)
	}
	want := []segment{
		{op: 'M', p: [3]point{{1, 1}}},
		{op: 'L', p: [3]point{{2, 1}}},
		{op: 'L', p: [3]point{{2, 2}}},
		{op: 'Z'},
		{op: 'M', p: [3]point{{1, 1}}},
		{op: 'L', p: [3]point{{3, 3}}},
	}
	if !reflect.DeepEqual(segments, want) {
		t.Fatalf("got %v\nwant %v", segments, want)
	}
}

func TestBadPathData(t *testing.T) {
	for _, data := range []string{"L1 1", "M1", "M0 0Z1 1", "M0 0A1 1 0 2 0 1 1", "1 1"} {
		if _, err := parsePath(data); err == nil {
			t.Errorf("%q parsed", data)
		}
	}
}

// onCircle checks every cubic's end and middle lie on the circle and returns the cubics.
func onCircle(t *testing.T, data string, centre point, radius float64) []segment {
	t.Helper()
	segments, err := parsePath(data)
	if err != nil {
		t.Fatal(err)
	}
	curves := segments[1:]
	for _, curve := range curves {
		if curve.op != 'C' {
			t.Fatalf("%q: got %c, want cubics", data, curve.op)
		}
		start := segments[0].p[0]
		middle := point{
			(start.x + 3*curve.p[0].x + 3*curve.p[1].x + curve.p[2].x) / 8,
			(start.y + 3*curve.p[0].y + 3*curve.p[1].y + curve.p[2].y) / 8,
		}
		for _, at := range []point{curve.p[2], middle} {
			if distance := math.Hypot(at.x-centre.x, at.y-centre.y); math.Abs(distance-radius) > radius*1e-3 {
				t.Fatalf("%q: %v is %v from the centre, want %v", data, at, distance, radius)
			}
		}
		segments[0].p[0] = curve.p[2]
	}
	return curves
}

func TestArcSweepFollowsSVG(t *testing.T) {
	// y grows downward, so sweep 1 runs clockwise on screen: left to right over the top
	over := onCircle(t, "M0 0A10 10 0 0 1 20 0", point{10, 0}, 10)
	if len(over) != 2 || over[0].p[2].y > -9.99 {
		t.Fatalf("sweep 1 went %v", over)
	}
	under := onCircle(t, "M0 0a10 10 0 0 0 20 0", point{10, 0}, 10)
	if len(under) != 2 || under[0].p[2].y < 9.99 {
		t.Fatalf("sweep 0 went %v", under)
	}
}

func TestArcLargeFlagAndCompactFlags(t *testing.T) {
	// clockwise from the top of a circle centred at 0,10 to its right side
	small := onCircle(t, "M0 0A10 10 0 0 1 10 10", point{0, 10}, 10)
	if len(small) != 1 {
		t.Fatalf("quarter arc took %d cubics", len(small))
	}
	// clockwise from the left of a circle centred at 10,0 round to its bottom
	large := onCircle(t, "M0 0a10 10 0 1110 10", point{10, 0}, 10)
	if len(large) != 3 {
		t.Fatalf("three quarter arc took %d cubics", len(large))
	}
	if end := large[len(large)-1].p[2]; end != (point{10, 10}) {
		t.Fatalf("arc ended at %v", end)
	}
}

func TestArcRadiiGrowToReachTheEnd(t *testing.T) {
	onCircle(t, "M0 0A1 1 0 0 1 20 0", point{10, 0}, 10)
}

func TestArcRotatedEllipse(t *testing.T) {
	// an ellipse with rx 20 and ry 10 turned 90 degrees is tall: its half from top to bottom bulges by 10
	segments, err := parsePath("M0 -20A20 10 90 0 1 0 20")
	if err != nil {
		t.Fatal(err)
	}
	widest := 0.0
	for _, curve := range segments[1:] {
		widest = math.Max(widest, curve.p[2].x)
	}
	if math.Abs(widest-10) > 1e-9 {
		t.Fatalf("rotated arc reached x %v, want 10", widest)
	}
}

func TestDegenerateArcs(t *testing.T) {
	segments, err := parsePath("M0 0A0 5 0 0 1 4 4A5 5 0 0 1 4 4")
	if err != nil {
		t.Fatal(err)
	}
	want := []segment{{op: 'M'}, {op: 'L', p: [3]point{{4, 4}}}}
	if !reflect.DeepEqual(segments, want) {
		t.Fatalf("got %v", segments)
	}
}

func TestDocumentRefusesWhatIconVGWouldDrop(t *testing.T) {
	for _, svg := range []string{
		`<svg viewBox="0 0 24 24"><path d="M0 0h1v1z" transform="scale(2)"/></svg>`,
		`<svg viewBox="0 0 24 24"><path d="M0 0h1v1z" fill-rule="evenodd"/></svg>`,
		`<svg viewBox="0 0 24 24"><circle r="4"/></svg>`,
		`<svg viewBox="0 0 24 12"><path d="M0 0h1v1z"/></svg>`,
	} {
		if _, err := parseDocument([]byte(svg)); err == nil {
			t.Errorf("accepted %s", svg)
		}
	}
	doc, err := parseDocument([]byte(`<svg xmlns="http://www.w3.org/2000/svg" height="24" width="24"><path d="M0 0h1v1z"/></svg>`))
	if err != nil || doc.viewBox != [4]float64{0, 0, 24, 24} || len(doc.paths) != 1 {
		t.Fatalf("got %v, %v", doc, err)
	}
}

func TestListFile(t *testing.T) {
	entries, err := readList("../../symbols/list.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range entries {
		if strings.Contains(goName(item.name), "_") {
			t.Errorf("%s spells %s", item.name, goName(item.name))
		}
	}
}
