package term

import (
	"fmt"
	"os"
	"strings"
)

// GlyphMode selects the terminal's available glyph repertoire.
type GlyphMode string

const (
	GlyphUnicode GlyphMode = "unicode"
	GlyphConsole GlyphMode = "console"
)

// GlyphSet contains every structural mark used by terminal components.
type GlyphSet struct {
	FrameTop         string
	FrameBottom      string
	FrameLeft        string
	FrameRight       string
	FrameTopLeft     string
	FrameTopRight    string
	FrameBottomLeft  string
	FrameBottomRight string
	// A stone is a filled plane whose cut shows in a third-height cap above
	// and below it, the cap's ends sloped like the chamfer.
	StoneTop         string
	StoneBottom      string
	StoneTopLeft     string
	StoneTopRight    string
	StoneBottomLeft  string
	StoneBottomRight string
	Rule             string
	Notch            string
	FlakeActive      string
	FlakeIdle        string
	FlakeBecoming    string
	BladeFill        string
	BladeTrack       string
	BladeEdge        string
	WorkingFrames    []string
	Arrow            string
	Chevron          string
	Cursor           string
	ToggleKnob       string
	SecretMask       string
}

// The frame's edges hug the outer side of their cells (eighth blocks), so the
// corner diagonals meet them; a mid-cell line never could.
var unicodeGlyphs = GlyphSet{
	FrameTop: "▔", FrameBottom: "▁", FrameLeft: "▏", FrameRight: "▕",
	FrameTopLeft: "╱", FrameTopRight: "╲", FrameBottomLeft: "╲", FrameBottomRight: "╱",
	StoneTop: "\U0001FB2D", StoneBottom: "\U0001FB02",
	StoneTopLeft: "\U0001FB48", StoneTopRight: "\U0001FB3D",
	StoneBottomLeft: "\U0001FB63", StoneBottomRight: "\U0001FB58",
	Rule:  "─",
	Notch: "◤", FlakeActive: "▰", FlakeIdle: "▱", FlakeBecoming: "▰▱",
	BladeFill: "━", BladeTrack: "─", BladeEdge: "╸",
	WorkingFrames: []string{"▰ ▱ ▱", "▱ ▰ ▱", "▱ ▱ ▰"},
	Arrow:         "›", Chevron: "▸", Cursor: "▸", ToggleKnob: "■", SecretMask: "•",
}

var consoleGlyphs = GlyphSet{
	FrameTop: "─", FrameBottom: "─", FrameLeft: "│", FrameRight: "│",
	FrameTopLeft: "┌", FrameTopRight: "┐", FrameBottomLeft: "└", FrameBottomRight: "┘",
	StoneTop: "▄", StoneBottom: "▀", StoneTopLeft: "▄", StoneTopRight: "▄", StoneBottomLeft: "▀", StoneBottomRight: "▀",
	Rule:  "─",
	Notch: "▀", FlakeActive: "▬", FlakeIdle: "▬", FlakeBecoming: "▬",
	BladeFill: "─", BladeTrack: "─", BladeEdge: "",
	WorkingFrames: []string{"░", "▒", "▓", "▒"},
	Arrow:         ">", Chevron: "►", Cursor: "►", ToggleKnob: "■", SecretMask: "•",
}

var activeGlyphs = detectGlyphs()

func detectGlyphs() GlyphSet {
	switch strings.ToLower(os.Getenv("LITHIC_GLYPHS")) {
	case string(GlyphConsole):
		return consoleGlyphs
	case string(GlyphUnicode):
		return unicodeGlyphs
	}
	if os.Getenv("TERM") == "linux" {
		return consoleGlyphs
	}
	return unicodeGlyphs
}

// SetGlyphMode overrides automatic detection before components are rendered.
func SetGlyphMode(mode GlyphMode) error {
	switch mode {
	case GlyphUnicode:
		activeGlyphs = unicodeGlyphs
	case GlyphConsole:
		activeGlyphs = consoleGlyphs
	default:
		return fmt.Errorf("unknown Lithic glyph mode %q", mode)
	}
	return nil
}

// Glyphs returns a copy of the selected glyph set.
func Glyphs() GlyphSet {
	g := activeGlyphs
	g.WorkingFrames = append([]string(nil), activeGlyphs.WorkingFrames...)
	return g
}

func glyphs() GlyphSet { return activeGlyphs }
