// Package theme turns Lithic tokens and palette roles into Gio values.
package theme

import (
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// TextStyle is one entry in the Lithic type scale.
type TextStyle struct {
	Face font.Font
	Size unit.Sp
	Line unit.Sp
}

// Type is the complete Lithic type scale.
type Type struct {
	Name     TextStyle
	Sentence TextStyle
	Section  TextStyle
	Title    TextStyle
	Row      TextStyle
	Body     TextStyle
	Proof    TextStyle
	Label    TextStyle
	Caption  TextStyle
	Group    TextStyle
	Mono     TextStyle
	MonoSm   TextStyle
}

// Cut is the shared 45 degree cut scale.
type Cut struct {
	Token       unit.Dp
	Small       unit.Dp
	Mid         unit.Dp
	Large       unit.Dp
	Notch       unit.Dp
	Menu        unit.Dp
	Overlay     unit.Dp
	Placard     unit.Dp
	Tooltip     unit.Dp
	ToggleTrack unit.Dp
	ToggleKnob  unit.Dp
}

// Space is the non-linear spacing run.
type Space struct {
	S1 unit.Dp
	S2 unit.Dp
	S3 unit.Dp
	S4 unit.Dp
	S5 unit.Dp
	S6 unit.Dp
}

// Motion carries every named Lithic duration.
type Motion struct {
	Press        time.Duration
	HoverIn      time.Duration
	HoverOut     time.Duration
	FocusIn      time.Duration
	FocusOut     time.Duration
	Select       time.Duration
	Field        time.Duration
	Notch        time.Duration
	Unfold       time.Duration
	Navigate     time.Duration
	MenuIn       time.Duration
	MenuOut      time.Duration
	DialogIn     time.Duration
	DialogOut    time.Duration
	TooltipIn    time.Duration
	TooltipDelay time.Duration
	PlacardIn    time.Duration
	PlacardLife  time.Duration
	Wheel        time.Duration
	Working      time.Duration
	Sweep        time.Duration
}

// Density changes row height without shrinking other controls.
type Density uint8

const (
	Comfortable Density = iota
	Compact
)

// Theme is the complete foundation needed by Gio components.
type Theme struct {
	R       palette.Roles
	T       Type
	Cut     Cut
	Sp      Space
	Motion  Motion
	Density Density
	Shaper  *text.Shaper
}

type config struct {
	shaper     *text.Shaper
	collection []font.FontFace
}

// Option customizes theme construction.
type Option func(*config)

// WithShaper supplies a caller-owned shaper.
func WithShaper(shaper *text.Shaper) Option {
	return func(config *config) { config.shaper = shaper }
}

// WithFontCollection adds embedded faces while retaining system font lookup.
func WithFontCollection(collection []font.FontFace) Option {
	return func(config *config) { config.collection = collection }
}

// New creates a theme from palette roles and generated tokens.
func New(roles palette.Roles, options ...Option) *Theme {
	config := config{}
	for _, option := range options {
		option(&config)
	}
	if config.shaper == nil {
		config.shaper = NewShaper(config.collection)
	}
	return &Theme{
		R: roles, T: defaultType(),
		Cut: Cut{
			Token: tokens.CutToken, Small: tokens.CutSmall, Mid: tokens.CutMid,
			Large: tokens.CutLarge, Notch: tokens.CutNotch, Menu: tokens.CutMenu,
			Overlay: tokens.CutOverlay, Placard: tokens.CutPlacard, Tooltip: tokens.CutTooltip,
			ToggleTrack: tokens.CutToggleTrack, ToggleKnob: tokens.CutToggleKnob,
		},
		Sp: Space{S1: tokens.SpaceS1, S2: tokens.SpaceS2, S3: tokens.SpaceS3, S4: tokens.SpaceS4, S5: tokens.SpaceS5, S6: tokens.SpaceS6},
		Motion: Motion{
			Press: tokens.MotionPress, HoverIn: tokens.MotionHoverIn, HoverOut: tokens.MotionHoverOut,
			FocusIn: tokens.MotionFocusIn, FocusOut: tokens.MotionFocusOut, Select: tokens.MotionSelect,
			Field: tokens.MotionField, Notch: tokens.MotionNotch, Unfold: tokens.MotionUnfold,
			Navigate: tokens.MotionNavigate, MenuIn: tokens.MotionMenuIn, MenuOut: tokens.MotionMenuOut,
			DialogIn: tokens.MotionDialogIn, DialogOut: tokens.MotionDialogOut, TooltipIn: tokens.MotionTooltipIn,
			TooltipDelay: tokens.MotionTooltipDelay, PlacardIn: tokens.MotionPlacardIn,
			PlacardLife: tokens.MotionPlacardLife, Wheel: tokens.MotionWheel,
			Working: tokens.MotionWorking, Sweep: tokens.MotionSweep,
		},
		Shaper: config.shaper,
	}
}

// NewShaper creates a system-font shaper with an optional embedded collection.
func NewShaper(collection []font.FontFace) *text.Shaper {
	if len(collection) == 0 {
		return text.NewShaper()
	}
	return text.NewShaper(text.WithCollection(collection))
}

// RowHeight returns the tablet height for the active density.
func (t *Theme) RowHeight() unit.Dp {
	if t.Density == Compact {
		return tokens.SizeTabletHeightCompact
	}
	return tokens.SizeTabletHeight
}

func defaultType() Type {
	style := func(token tokens.Type) TextStyle {
		families := tokens.FontSans
		generic := "sans-serif"
		if token.Family == "mono" {
			families = tokens.FontMono
			generic = "monospace"
		}
		list := append(append([]string(nil), families...), generic)
		return TextStyle{
			Face: font.Font{Typeface: font.Typeface(strings.Join(list, ", ")), Weight: font.Weight(token.Weight - 400)},
			Size: unit.Sp(token.Size), Line: unit.Sp(token.Line),
		}
	}
	return Type{
		Name: style(tokens.TypeName), Sentence: style(tokens.TypeSentence), Section: style(tokens.TypeSection),
		Title: style(tokens.TypeTitle), Row: style(tokens.TypeRow), Body: style(tokens.TypeBody),
		Proof: style(tokens.TypeProof), Label: style(tokens.TypeLabel), Caption: style(tokens.TypeCaption),
		Group: style(tokens.TypeGroup), Mono: style(tokens.TypeMono), MonoSm: style(tokens.TypeMonoSm),
	}
}
