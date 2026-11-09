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
	Separator        string
	Cursor           string
	ToggleKnob       string
	SecretMask       string
}

var unicodeGlyphs = GlyphSet{
	FrameTop: "─", FrameBottom: "─", FrameLeft: "│", FrameRight: "│",
	FrameTopLeft: "╱", FrameTopRight: "╲", FrameBottomLeft: "╲", FrameBottomRight: "╱",
	Notch: "◤", FlakeActive: "▰", FlakeIdle: "▱", FlakeBecoming: "▰▱",
	BladeFill: "━", BladeTrack: "─", BladeEdge: "╸",
	WorkingFrames: []string{"▰ ▱ ▱", "▱ ▰ ▱", "▱ ▱ ▰"},
	Arrow:         "›", Chevron: "▸", Separator: "·", Cursor: "▸", ToggleKnob: "■", SecretMask: "•",
}

var consoleGlyphs = GlyphSet{
	FrameTop: "─", FrameBottom: "─", FrameLeft: "│", FrameRight: "│",
	FrameTopLeft: "┌", FrameTopRight: "┐", FrameBottomLeft: "└", FrameBottomRight: "┘",
	Notch: "▀", FlakeActive: "▬", FlakeIdle: "▬", FlakeBecoming: "▬",
	BladeFill: "─", BladeTrack: "─", BladeEdge: "",
	WorkingFrames: []string{"░", "▒", "▓", "▒"},
	Arrow:         ">", Chevron: "►", Separator: "·", Cursor: "►", ToggleKnob: "■", SecretMask: "•",
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
