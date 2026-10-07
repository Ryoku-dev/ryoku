package main

// The terminal follows Ryoku's warm paper-and-ink palette. A single coloured
// seal carries the identity while status and structure stay in the ink ramp.

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	cBg        = lipgloss.Color("#16110b")
	cText      = lipgloss.Color("#e6dccb")
	cSub       = lipgloss.Color("#b5aa9c")
	cDim       = lipgloss.Color("#5c5249")
	cLine      = lipgloss.Color("#3b3026")
	cSelection = lipgloss.Color("#2a2118")
	cBrand     = lipgloss.Color("#e2342a")

	// Status remains an ink treatment. The seal is the only coloured emphasis.
	cGreen = cText
	cYell  = cText
	cRed   = cText
)

func sty() lipgloss.Style                 { return lipgloss.NewStyle() }
func fg(c color.Color, s string) string   { return sty().Foreground(c).Render(s) }
func bold(c color.Color, s string) string { return sty().Foreground(c).Bold(true).Render(s) }
func selected(s string) string {
	return sty().Foreground(cText).Background(cSelection).Bold(true).Render(s)
}
func dw(s string) int                     { return lipgloss.Width(s) }

func leftBlock(w int, s string) string {
	return sty().Width(w).Align(lipgloss.Left).Render(s)
}

func truncW(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dw(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && dw(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func padTo(s string, w int) string {
	if d := dw(s); d < w {
		return s + strings.Repeat(" ", w-d)
	}
	return s
}


func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// glyphs with ASCII fallback for dumb/no-UTF8 consoles; colors need no
// fallback (lipgloss v2 downsamples truecolor itself).
var ascii bool
var (
	gCheck  = "✓"
	gBad    = "×"
	gPend   = "·"
	gSel    = "//"
	gOn     = "●"
	gOff    = "○"
	gFull   = "━"
	gEmpty  = "─"
	gWarn   = "!"
	gBullet = "•"
	gSeal   = "力"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}


func ruleCh() string {
	if ascii {
		return "-"
	}
	return "─"
}


func initGlyphs() {
	t := os.Getenv("TERM")
	ascii = os.Getenv("RYOKU_ASCII") != "" || t == "dumb" || t == "vt100" || t == ""
	if !ascii {
		return
	}
	gCheck, gBad, gPend, gSel = "+", "x", ".", "//"
	gOn, gOff = "*", "o"
	gFull, gEmpty, gWarn, gBullet, gSeal = "#", "-", "!", "*", "R"
	spinFrames = []string{"|", "/", "-", "\\"}
}

// The seal is the installer chrome's one coloured mark. Everything around it
// stays paper and ink so the mark remains memorable instead of decorative.
func banner(_ int) string {
	return bold(cBrand, gSeal) + "  " + bold(cText, "RYOKU") + fg(cSub, " / shell installer")
}

func keyHint(k, desc string) string {
	return bold(cText, k) + " " + fg(cSub, desc)
}

func hintSep() string { return fg(cLine, "  /  ") }
