package kit

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// ChordField shows a keyboard shortcut as keycaps and records a new one when
// activated. It never talks to the compositor; a host holds a shortcut
// inhibitor while Recording reports true.
type ChordField struct {
	Click
	Chord            string
	Label            string
	Placeholder      string
	Conflict         string
	Preview          State
	PreviewRecording bool

	keyTag        int
	recording     bool
	recorded      string
	recordedReady bool
	cleared       bool
	hover, focus  anim.Tween
}

// Recording reports whether the field is waiting for a shortcut.
func (f *ChordField) Recording() bool { return f.recording }

// Recorded reports and clears the most recent recorded chord. The field does
// not change Chord itself.
func (f *ChordField) Recorded() (string, bool) {
	chord, ok := f.recorded, f.recordedReady
	f.recorded, f.recordedReady = "", false
	return chord, ok
}

// Cleared reports and clears a request to remove the shortcut.
func (f *ChordField) Cleared() bool {
	cleared := f.cleared
	f.cleared = false
	return cleared
}

// Focus moves keyboard focus to the field.
func (f *ChordField) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &f.Clickable})
}

func (f *ChordField) stop(gtx layout.Context) {
	f.recording = false
	gtx.Execute(key.FocusCmd{Tag: &f.Clickable})
	gtx.Execute(op.InvalidateCmd{})
}

func (f *ChordField) update(gtx layout.Context) {
	if f.Clicked(gtx) && !f.recording && gtx.Source.Enabled() {
		f.recording = true
		gtx.Execute(key.FocusCmd{Tag: &f.keyTag})
	}
	if !f.recording {
		return
	}
	modifiers := key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper
	for {
		value, ok := gtx.Event(
			key.FocusFilter{Target: &f.keyTag},
			key.Filter{Focus: &f.keyTag, Optional: modifiers},
			key.Filter{Focus: &f.keyTag, Name: key.NameTab, Required: key.ModSuper, Optional: modifiers},
			key.Filter{Focus: &f.keyTag, Name: key.NameTab, Required: key.ModCtrl, Optional: modifiers},
			key.Filter{Focus: &f.keyTag, Name: key.NameTab, Required: key.ModAlt, Optional: modifiers},
		)
		if !ok {
			break
		}
		switch event := value.(type) {
		case key.FocusEvent:
			if !event.Focus && f.recording {
				f.recording = false
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.Event:
			if event.State != key.Press || !f.recording {
				continue
			}
			action, chord := chordFromKey(event.Name, event.Modifiers)
			switch action {
			case chordCancel:
				f.stop(gtx)
			case chordClear:
				f.cleared = true
				f.stop(gtx)
			case chordRecord:
				f.recorded, f.recordedReady = chord, true
				f.stop(gtx)
			}
		}
	}
}

// Layout draws and operates the field.
func (f *ChordField) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	f.update(gtx)
	recording := f.recording || f.PreviewRecording
	placeholder := f.Placeholder
	if placeholder == "" {
		placeholder = "Not set"
	}
	caps := chordKeycaps(f.Chord)

	width := gtx.Constraints.Max.X
	if width <= 0 {
		width = gtx.Dp(unit.Dp(tokens.SizeTextFieldDefaultWidth))
	}
	size := gtx.Constraints.Constrain(image.Pt(width, gtx.Dp(unit.Dp(tokens.SizeTextFieldHeight))))
	cut := gtx.Dp(th.Cut.Mid)
	original := gtx
	dims := f.Click.Layout(exactContext(gtx, size), func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		name := f.Label
		if name == "" {
			name = "Shortcut"
		}
		semantic.LabelOp(name).Add(gtx.Ops)
		switch {
		case recording:
			semantic.DescriptionOp("Press a shortcut").Add(gtx.Ops)
		case len(caps) > 0:
			semantic.DescriptionOp(strings.Join(caps, " ")).Add(gtx.Ops)
		default:
			semantic.DescriptionOp(placeholder).Add(gtx.Ops)
		}
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		hover := previewTransition(original, &f.hover, f.Preview, StateHover, f.Hovered() && !recording, th.Motion.HoverIn, th.Motion.HoverOut)
		if f.Preview == StatePressed {
			hover = 1
		}
		line := float32(0)
		if recording {
			line = 1
		}
		bounds := image.Rectangle{Max: size}
		drawFieldSurface(gtx, th, bounds, cut, palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*hover), line, f.Conflict != "")

		inset := gtx.Dp(unit.Dp(tokens.SizeTextFieldInset))
		inner := exactContext(gtx, image.Pt(max(0, size.X-2*inset), size.Y))
		offset := op.Offset(image.Pt(inset, 0)).Push(gtx.Ops)
		switch {
		case recording:
			centerY(inner, rawText(th, th.T.Body, disabledInk(gtx, th.R.Fg2), "Press a shortcut"))
		case len(caps) == 0:
			centerY(inner, rawText(th, th.T.Body, disabledInk(gtx, th.R.Fg3), placeholder))
		default:
			centerY(inner, func(gtx layout.Context) layout.Dimensions { return layoutKeycaps(gtx, th, caps) })
		}
		offset.Pop()
		return layout.Dimensions{Size: size}
	})
	if !recording {
		focus := previewTransition(original, &f.focus, f.Preview, StateFocus, original.Focused(&f.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
		FocusRingAlpha(original, th, image.Rectangle{Max: dims.Size}, cut, focus)
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	keyArea := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &f.keyTag)
	keyArea.Pop()
	pass.Pop()

	if f.Conflict == "" {
		return dims
	}
	gap := gtx.Dp(unit.Dp(tokens.SizeTextFieldHelpGap))
	offset := op.Offset(image.Pt(0, dims.Size.Y+gap)).Push(gtx.Ops)
	lineGTX := gtx
	lineGTX.Constraints = layout.Constraints{Max: image.Pt(dims.Size.X, gtx.Constraints.Max.Y)}
	conflict := Label{Style: th.T.Caption, Color: disabledInk(gtx, th.R.Jasper)}.Layout(lineGTX, th, f.Conflict)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(dims.Size.X, dims.Size.Y+gap+conflict.Size.Y), Baseline: dims.Baseline}
}

func layoutKeycaps(gtx layout.Context, th *theme.Theme, caps []string) layout.Dimensions {
	padX, padY := gtx.Dp(th.Sp.S2), gtx.Dp(th.Sp.S1)
	gap := gtx.Dp(th.Sp.S1)
	cut := gtx.Dp(th.Cut.Token)
	hairline := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
	height := gtx.Sp(th.T.Label.Line) + 2*padY
	x := 0
	for index, face := range caps {
		if index > 0 {
			x += gap
		}
		record := op.Record(gtx.Ops)
		textGTX := gtx
		textGTX.Constraints = layout.Constraints{Max: image.Pt(max(0, gtx.Constraints.Max.X-x-2*padX), gtx.Sp(th.T.Label.Line))}
		text := rawText(th, th.T.Label, disabledInk(gtx, th.R.Fg), face)(textGTX)
		call := record.Stop()
		capWidth := text.Size.X + 2*padX
		if x+capWidth > gtx.Constraints.Max.X {
			break
		}
		bounds := image.Rect(x, 0, x+capWidth, height)
		FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, th.R.Field))
		StrokeCut(gtx.Ops, bounds, cut, hairline, disabledInk(gtx, th.R.Edge))
		offset := op.Offset(image.Pt(x+padX, padY)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		x += capWidth
	}
	return layout.Dimensions{Size: image.Pt(x, height)}
}

type chordAction uint8

const (
	chordIgnore chordAction = iota
	chordCancel
	chordClear
	chordRecord
)

// chordFromKey turns one key press into a recorder action and, for a record,
// the canonical chord: Super, Ctrl, Alt, Shift, then the key, joined by "+".
func chordFromKey(name key.Name, modifiers key.Modifiers) (chordAction, string) {
	switch name {
	case key.NameEscape:
		return chordCancel, ""
	case key.NameCtrl, key.NameShift, key.NameAlt, key.NameSuper, key.NameCommand:
		return chordIgnore, ""
	}
	super := modifiers&(key.ModSuper|key.ModCommand) != 0
	ctrl := modifiers&key.ModCtrl != 0
	alt := modifiers&key.ModAlt != 0
	shift := modifiers&key.ModShift != 0
	held := super || ctrl || alt || shift
	if !held && (name == key.NameDeleteBackward || name == key.NameDeleteForward) {
		return chordClear, ""
	}
	token := chordKeyToken(name, shift)
	if token == "" || (!held && !chordLoneKey(token)) {
		return chordIgnore, ""
	}
	parts := make([]string, 0, 5)
	if super {
		parts = append(parts, "Super")
	}
	if ctrl {
		parts = append(parts, "Ctrl")
	}
	if alt {
		parts = append(parts, "Alt")
	}
	if shift {
		parts = append(parts, "Shift")
	}
	return chordRecord, strings.Join(append(parts, token), "+")
}

var chordNamedKeys = map[key.Name]string{
	key.NameReturn:         "Return",
	key.NameEnter:          "KP_Enter",
	key.NameSpace:          "Space",
	key.NameTab:            "Tab",
	key.NameLeftArrow:      "Left",
	key.NameRightArrow:     "Right",
	key.NameUpArrow:        "Up",
	key.NameDownArrow:      "Down",
	key.NameDeleteBackward: "BackSpace",
	key.NameDeleteForward:  "Delete",
	key.NameHome:           "Home",
	key.NameEnd:            "End",
	key.NamePageUp:         "Prior",
	key.NamePageDown:       "Next",
}

var chordPunctuation = map[string]string{
	"-": "minus", "=": "equal", ",": "comma", ".": "period", "/": "slash",
	"\\": "backslash", ";": "semicolon", "'": "apostrophe", "[": "bracketleft",
	"]": "bracketright", "`": "grave",
}

// chordShifted folds the US shifted symbols back to the key that makes them,
// since Gio names the produced symbol rather than the physical key.
var chordShifted = map[string]string{
	"!": "1", "@": "2", "#": "3", "$": "4", "%": "5", "^": "6", "&": "7",
	"*": "8", "(": "9", ")": "0", "_": "minus", "+": "equal", "<": "comma",
	">": "period", "?": "slash", "|": "backslash", ":": "semicolon",
	"\"": "apostrophe", "{": "bracketleft", "}": "bracketright", "~": "grave",
}

func chordKeyToken(name key.Name, shift bool) string {
	if token, ok := chordNamedKeys[name]; ok {
		return token
	}
	value := string(name)
	if len(value) == 1 {
		character := value[0]
		switch {
		case character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
			return value
		case character >= 'a' && character <= 'z':
			return strings.ToUpper(value)
		}
		if token, ok := chordPunctuation[value]; ok {
			return token
		}
		if !shift {
			// the number pad is the only source of these without Shift
			switch value {
			case "*":
				return "KP_Multiply"
			case "+":
				return "KP_Add"
			}
		}
		if token, ok := chordShifted[value]; ok {
			return token
		}
		return ""
	}
	if function, ok := strings.CutPrefix(value, "F"); ok {
		if number, err := strconv.Atoi(function); err == nil && number >= 1 && number <= 24 {
			return value
		}
	}
	return ""
}

var chordMediaKeys = map[string]string{
	"XF86AudioRaiseVolume":  "Vol +",
	"XF86AudioLowerVolume":  "Vol -",
	"XF86AudioMute":         "Mute",
	"XF86AudioMicMute":      "Mic mute",
	"XF86AudioPlay":         "Play",
	"XF86AudioNext":         "Next",
	"XF86AudioPrev":         "Prev",
	"XF86MonBrightnessUp":   "Brightness +",
	"XF86MonBrightnessDown": "Brightness -",
}

// chordLoneKey reports whether token makes a shortcut with no modifier: a
// function key or a media key.
func chordLoneKey(token string) bool {
	if _, ok := chordMediaKeys[token]; ok {
		return true
	}
	if function, ok := strings.CutPrefix(token, "F"); ok {
		number, err := strconv.Atoi(function)
		return err == nil && number >= 1 && number <= 24
	}
	return false
}

var chordFaces = map[string]string{
	"SUPER":        "Super",
	"CTRL":         "Ctrl",
	"ALT":          "Alt",
	"SHIFT":        "Shift",
	"Return":       "Enter",
	"Escape":       "Esc",
	"space":        "Space",
	"Prior":        "Page Up",
	"Next":         "Page Down",
	"grave":        "`",
	"comma":        ",",
	"bracketleft":  "[",
	"bracketright": "]",
}

// chordKeycaps splits a chord into the faces its keycaps show, matching the
// faces ryoku/wm DisplayKeys gives the keybind sheet.
func chordKeycaps(chord string) []string {
	var faces []string
	for _, raw := range strings.Split(chord, "+") {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		if face, ok := chordFaces[strings.ToUpper(token)]; ok && chordIsModifier(token) {
			faces = append(faces, face)
			continue
		}
		if face, ok := chordFaces[token]; ok {
			faces = append(faces, face)
			continue
		}
		if face, ok := chordMediaKeys[token]; ok {
			faces = append(faces, face)
			continue
		}
		if digit, ok := strings.CutPrefix(token, "KP_"); ok && len(digit) == 1 && digit[0] >= '0' && digit[0] <= '9' {
			faces = append(faces, "Num "+digit)
			continue
		}
		faces = append(faces, token)
	}
	return faces
}

func chordIsModifier(token string) bool {
	switch strings.ToUpper(token) {
	case "SUPER", "CTRL", "ALT", "SHIFT":
		return true
	}
	return false
}
