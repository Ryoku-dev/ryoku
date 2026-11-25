package apprules

import (
	"errors"
	"fmt"
	"strings"
)

// Chord is one key press with its modifiers: Mods in the fixed order ctrl,
// shift, alt, altgr, logo, each at most once, and Key an XKB keysym name.
type Chord struct {
	Mods []string
	Key  string
}

// ErrChord is returned for a chord these rules cannot read. Its text never
// repeats the chord.
var ErrChord = errors.New("not a key chord")

// The modifiers, in their fixed order, and the spellings people use.
var (
	modOrder = []string{"ctrl", "shift", "alt", "altgr", "logo"}
	modNames = map[string]string{
		"ctrl": "ctrl", "control": "ctrl", "shift": "shift", "alt": "alt", "option": "alt", "altgr": "altgr",
		"super": "logo", "meta": "logo", "win": "logo", "cmd": "logo", "logo": "logo",
	}
)

// keyNames maps the spellings people use onto keysyms. A single letter or
// digit is its own keysym; nothing else is read.
var keyNames = map[string]string{
	"enter": "Return", "return": "Return", "kp_enter": "KP_Enter", "tab": "Tab", "esc": "Escape", "escape": "Escape",
	"backspace": "BackSpace", "delete": "Delete", "del": "Delete", "space": "space",
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	"arrowup": "Up", "arrowdown": "Down", "arrowleft": "Left", "arrowright": "Right",
	"home": "Home", "end": "End", "pageup": "Page_Up", "pagedown": "Page_Down",
	"page_up": "Page_Up", "page_down": "Page_Down", "insert": "Insert", "menu": "Menu", "print": "Print",
	"f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4", "f5": "F5", "f6": "F6",
	"f7": "F7", "f8": "F8", "f9": "F9", "f10": "F10", "f11": "F11", "f12": "F12",
}

// ParseChord reads a chord such as "shift+Tab", "Ctrl+Z" or "PageDown".
// Modifiers come first, each once; the key is last. Spellings are matched
// without case; a letter key is kept lower-case so no shift is implied.
func ParseChord(s string) (Chord, error) {
	parts := strings.Split(strings.TrimSpace(s), "+")
	if len(parts) == 0 || len(parts) > len(modOrder)+1 {
		return Chord{}, ErrChord
	}
	seen := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		m, ok := modNames[strings.ToLower(strings.TrimSpace(p))]
		if !ok || seen[m] {
			return Chord{}, fmt.Errorf("%w: a modifier is unknown or given twice", ErrChord)
		}
		seen[m] = true
	}
	key := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if name, ok := keyNames[key]; ok {
		key = name
	} else if len(key) != 1 || !(key[0] >= 'a' && key[0] <= 'z' || key[0] >= '0' && key[0] <= '9') {
		return Chord{}, fmt.Errorf("%w: the key is unknown", ErrChord)
	}
	c := Chord{Key: key}
	for _, m := range modOrder {
		if seen[m] {
			c.Mods = append(c.Mods, m)
		}
	}
	return c, nil
}

// String is the chord in its one canonical spelling, such as "ctrl+z".
func (c Chord) String() string {
	return strings.Join(append(append([]string(nil), c.Mods...), c.Key), "+")
}

func (c Chord) has(mod string) bool {
	for _, m := range c.Mods {
		if m == mod {
			return true
		}
	}
	return false
}

// safeKeys are the only chords Hands presses itself: moving between fields
// and through text, deleting the character just typed, and undo.
var safeKeys = map[string]bool{
	"Tab": true, "shift+Tab": true, "Left": true, "Right": true, "Up": true, "Down": true,
	"Home": true, "End": true, "Page_Up": true, "Page_Down": true, "BackSpace": true, "ctrl+z": true,
}

// pasteKeys put the clipboard into the window. Hands has no clipboard path.
var pasteKeys = map[string]bool{
	"ctrl+v": true, "ctrl+shift+v": true, "shift+Insert": true, "ctrl+shift+Insert": true,
}

// Press judges one chord. The safe set is pressed by Hands; Enter in any
// form is the person's submit; a paste, any logo chord and the console
// switches are refused; every other key (Delete, Escape, closing and
// quitting chords, function keys, letters with a modifier) is the person's
// press.
func Press(c Chord) Verdict {
	s := c.String()
	switch {
	case c.Key == "" || len(c.Mods) > len(modOrder):
		return Verdict{KindRefused, WhySystemKey}
	case c.has("logo"):
		return Verdict{KindRefused, WhySystemKey}
	case c.has("ctrl") && c.has("alt") && (strings.HasPrefix(c.Key, "F") && len(c.Key) > 1 || c.Key == "BackSpace" || c.Key == "Delete"):
		return Verdict{KindRefused, WhySystemKey}
	case pasteKeys[s]:
		return Verdict{KindRefused, WhyPaste}
	case c.Key == "Return" || c.Key == "KP_Enter":
		return Verdict{KindHandoff, WhySubmit}
	case safeKeys[s]:
		return Verdict{KindSafe, WhySafe}
	}
	return Verdict{KindHandoff, WhyRiskyKey}
}

// PressChord reads and judges chord in one step: a chord that cannot be read
// is refused.
func PressChord(chord string) (Chord, Verdict) {
	c, err := ParseChord(chord)
	if err != nil {
		return Chord{}, Verdict{KindRefused, WhySystemKey}
	}
	return c, Press(c)
}

// Safe says whether Hands may press chord itself.
func Safe(chord string) bool {
	_, v := PressChord(chord)
	return v.Kind == KindSafe
}
