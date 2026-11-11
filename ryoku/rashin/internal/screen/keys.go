package screen

import (
	"fmt"
	"strings"
)

// keyArgs turns a chord such as ctrl+shift+t, alt+F4 or Return into wtype's
// arguments: each modifier pressed, the key typed, the modifiers let go in
// reverse. Names are XKB keysyms, with the spellings people use mapped onto
// them.
func keyArgs(chord string) ([]string, error) {
	parts := strings.Split(strings.TrimSpace(chord), "+")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return nil, fmt.Errorf("no key in %q", chord)
	}
	var modifiers []string
	for _, part := range parts[:len(parts)-1] {
		modifier, ok := keyModifiers[strings.ToLower(strings.TrimSpace(part))]
		if !ok {
			return nil, fmt.Errorf("%q is not a modifier", part)
		}
		modifiers = append(modifiers, modifier)
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if name, ok := keyNames[strings.ToLower(key)]; ok {
		key = name
	}
	// Ctrl+L is how people write ctrl+l; the capital keysym would add a shift
	if len(modifiers) > 0 && len(key) == 1 {
		key = strings.ToLower(key)
	}
	var args []string
	for _, modifier := range modifiers {
		args = append(args, "-M", modifier)
	}
	args = append(args, "-k", key)
	for i := len(modifiers) - 1; i >= 0; i-- {
		args = append(args, "-m", modifiers[i])
	}
	return args, nil
}

var keyModifiers = map[string]string{
	"ctrl": "ctrl", "control": "ctrl", "shift": "shift", "alt": "alt", "option": "alt",
	"super": "logo", "meta": "logo", "win": "logo", "cmd": "logo", "logo": "logo", "altgr": "altgr",
}

var keyNames = map[string]string{
	"enter": "Return", "return": "Return", "tab": "Tab", "esc": "Escape", "escape": "Escape",
	"backspace": "BackSpace", "delete": "Delete", "del": "Delete", "space": "space",
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	"arrowup": "Up", "arrowdown": "Down", "arrowleft": "Left", "arrowright": "Right",
	"home": "Home", "end": "End", "pageup": "Page_Up", "pagedown": "Page_Down",
	"page_up": "Page_Up", "page_down": "Page_Down", "insert": "Insert", "menu": "Menu",
	"f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4", "f5": "F5", "f6": "F6",
	"f7": "F7", "f8": "F8", "f9": "F9", "f10": "F10", "f11": "F11", "f12": "F12",
}
