package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A live-ISO user cannot resize a kernel virtual console: the grid is framebuffer
// pixels over console font cell (or, in the kiosk, output pixels over foot's
// cell). Whatever size the installer is handed, it has to live in it.
//
// lipgloss does not help here -- Place returns oversized content unchanged
// (position.go: `if gap <= 0 { return str }`) -- so a frame bigger than the grid
// reaches the VT, which wraps it and shears the layout: the step rail slides off
// the left edge and the footer with the confirm buttons drops off the bottom.
// That was the reported bug on fresh installs.

// gridSizes: the grids a live install actually lands on. 80x24 and 80x25 are the
// VT before KMS hands over a bigger framebuffer, 80x30 is a 640x480 fb with the
// 8x16 default font, 100x30 and 128x48 are common fb/font pairs, and 240x67 is
// 1920x1080 at 8x16.
var gridSizes = [][2]int{{56, 16}, {60, 18}, {70, 20}, {80, 22}, {80, 24}, {80, 25}, {80, 30}, {100, 30}, {128, 48}, {240, 67}}

// stripSGR drops ANSI styling so a test can assert on the text a user reads.
func stripSGR(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc:
			if r == 'm' {
				esc = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wizardAt builds a wizard screen at a given grid: every pick filled in and a
// whole-disk erase, matching the state in the bug report.
func wizardAt(key string, w, h int) model {
	m := reviewModel()
	m.variant, m.netOnline, m.cpuV3 = "plain", true, true
	m.state, m.flow = "wizard", stepsForVariant("plain")
	m.picks["disk"] = "whole" // the destructive path, as in the bug report
	m.diskDev, m.kept, m.diskG = "/dev/nvme0n1", nil, 240
	m.w, m.h = w, h
	m.idx = flowIndex(m.flow, key)
	m.loadStep()
	m.enterPos = 1
	return m
}

// No screen may render outside the grid it was given. This is the invariant that
// keeps the VT from wrapping a frame and shearing the layout.
func TestFrameNeverExceedsGrid(t *testing.T) {
	for _, sz := range gridSizes {
		w, h := sz[0], sz[1]
		for _, s := range steps() {
			m := wizardAt(s.key, w, h)
			gw, gh := frameBox(m.fittedFrame())
			if gw > w || gh > h {
				t.Errorf("step %s at %dx%d rendered %dx%d (over by %+d cols, %+d rows)",
					s.key, w, h, gw, gh, gw-w, gh-h)
			}
		}
	}
}

func TestUnavailableGamingFitsCompactGrids(t *testing.T) {
	for _, sz := range gridSizes {
		w, h := sz[0], sz[1]
		m := wizardAt("gaming", w, h)
		m.picks["gaming"] = "1"
		m.netOnline, m.cpuV3 = false, true
		m.loadStep()
		m.enterPos = 1
		frame := m.fittedFrame()
		gw, gh := frameBox(frame)
		if gw > w || gh > h {
			t.Errorf("offline gaming at %dx%d rendered %dx%d", w, h, gw, gh)
		}
		if !strings.Contains(stripSGR(frame), "offline") {
			t.Errorf("offline gaming at %dx%d dropped its unavailable reason", w, h)
		}
	}
}

// Review is the screen that decides whether a disk gets erased, and it is the
// tallest one. Every grid must still show what is about to be destroyed and the
// buttons that confirm it -- the reported symptom was losing exactly these.
func TestReviewKeepsCriticalContent(t *testing.T) {
	for _, sz := range gridSizes {
		w, h := sz[0], sz[1]
		got := stripSGR(wizardAt("review", w, h).fittedFrame())
		for _, want := range []string{"/dev/nvme0n1", "ERASE whole disk", "Yes", "No"} {
			if !strings.Contains(got, want) {
				t.Errorf("review at %dx%d dropped %q", w, h, want)
			}
		}
	}
}

// fitBlock is the last line of defence, so it has to be exact: truncate to the
// box, and drop rows from the end the caller can afford to lose.
func TestFitBlockClampsBothAxes(t *testing.T) {
	in := "aaaaaaaa\nbbbbbbbb\ncccccccc\ndddddddd"
	if got := fitBlock(in, 3, 2, false); got != "aaa\nbbb" {
		t.Errorf("top-anchored clamp = %q, want %q", got, "aaa\nbbb")
	}
	if got := fitBlock(in, 3, 2, true); got != "ccc\nddd" {
		t.Errorf("bottom-anchored clamp = %q, want %q", got, "ccc\nddd")
	}
	// h == 0 means "width only", which is how the header and footer are clamped.
	if got := fitBlock(in, 4, 0, false); strings.Count(got, "\n") != 3 {
		t.Errorf("h=0 must leave every row: %q", got)
	}
	// A styled row is measured in display cells, not bytes, and keeps its styling.
	styled := fg(cRed, "abcdefgh")
	if got := dw(fitBlock(styled, 4, 0, false)); got != 4 {
		t.Errorf("styled clamp width = %d, want 4", got)
	}
}

// boxLines is what stops one long row from widening a card past its budget: it
// must pad short rows AND truncate long ones. padLines only padded, which is how
// a 50-column card grew to 59 and pushed the frame off the screen.
func TestBoxLinesPadsAndTruncates(t *testing.T) {
	got := strings.Split(boxLines("ab\nabcdefgh", 4), "\n")
	if dw(got[0]) != 4 || dw(got[1]) != 4 {
		t.Fatalf("both rows must be exactly 4 cells: %q", got)
	}
	if !strings.HasPrefix(got[0], "ab") {
		t.Errorf("short row lost its text: %q", got[0])
	}
}

// The rail grows with every install question. On a short grid it has to scroll
// around the current step: a rail taller than the grid pushed the frame past it,
// and the clamp then cut off the card's title and the step being asked.
func TestShortGridScrollsTheRail(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {80, 25}, {100, 24}} {
		for _, key := range []string{"keyboard", "hyakki", "gaming", "administrator"} {
			m := wizardAt(key, sz[0], sz[1])
			got := stripSGR(m.fittedFrame())
			for _, want := range []string{m.cur().title, "install steps", key} {
				if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
					t.Errorf("%s at %dx%d dropped %q", key, sz[0], sz[1], want)
				}
			}
		}
	}
}

// Every step is either on screen or counted in a marker, and the current one is
// always on screen, whichever end of the list it sits at.
func TestWindowRailAccountsForEveryStep(t *testing.T) {
	steps := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	for cur := range steps {
		got := windowRail(steps, cur, 5)
		if len(got) != 5 {
			t.Fatalf("cur %d: %d rows, want 5", cur, len(got))
		}
		seen, counted := 0, 0
		for _, row := range got {
			plain := stripSGR(row)
			if !strings.HasPrefix(plain, "↑ ") && !strings.HasPrefix(plain, "↓ ") {
				seen++
				continue
			}
			var n int
			if _, err := fmt.Sscanf(plain[len("↑ "):], "%d more", &n); err != nil {
				t.Fatalf("cur %d: unreadable marker %q", cur, plain)
			}
			counted += n
		}
		if seen+counted != len(steps) {
			t.Errorf("cur %d: %d shown + %d counted != %d steps: %q", cur, seen, counted, len(steps), got)
		}
		if !slices.Contains(got, steps[cur]) {
			t.Errorf("cur %d: current step scrolled out: %q", cur, got)
		}
	}
	if got := windowRail(steps, 3, 0); len(got) != len(steps) {
		t.Errorf("rows 0 must list every step, got %d", len(got))
	}
}
