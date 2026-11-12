package kit

import (
	"reflect"
	"testing"

	"gioui.org/io/key"
)

func TestChordFromKey(t *testing.T) {
	cases := []struct {
		name      key.Name
		modifiers key.Modifiers
		action    chordAction
		chord     string
	}{
		{key.NameReturn, key.ModShift | key.ModSuper, chordRecord, "Super+Shift+Return"},
		{"Q", key.ModSuper, chordRecord, "Super+Q"},
		{"T", key.ModAlt | key.ModCtrl | key.ModSuper, chordRecord, "Super+Ctrl+Alt+T"},
		{"!", key.ModShift | key.ModSuper, chordRecord, "Super+Shift+1"},
		{"_", key.ModShift | key.ModCtrl, chordRecord, "Ctrl+Shift+minus"},
		{"-", key.ModCtrl, chordRecord, "Ctrl+minus"},
		{"`", key.ModSuper, chordRecord, "Super+grave"},
		{"[", key.ModSuper, chordRecord, "Super+bracketleft"},
		{"*", key.ModSuper, chordRecord, "Super+KP_Multiply"},
		{"*", key.ModSuper | key.ModShift, chordRecord, "Super+Shift+8"},
		{key.NameEnter, key.ModSuper, chordRecord, "Super+KP_Enter"},
		{key.NamePageUp, key.ModSuper, chordRecord, "Super+Prior"},
		{key.NamePageDown, key.ModCommand, chordRecord, "Super+Next"},
		{key.NameTab, key.ModAlt, chordRecord, "Alt+Tab"},
		{key.NameSpace, key.ModCtrl, chordRecord, "Ctrl+Space"},
		{key.NameDeleteBackward, key.ModSuper, chordRecord, "Super+BackSpace"},
		{key.NameF5, 0, chordRecord, "F5"},
		{key.NameF12, key.ModShift, chordRecord, "Shift+F12"},
		{"A", key.ModShift, chordRecord, "Shift+A"},
		{"A", 0, chordIgnore, ""},
		{key.NameReturn, 0, chordIgnore, ""},
		{key.NameSuper, key.ModSuper, chordIgnore, ""},
		{key.NameShift, key.ModShift, chordIgnore, ""},
		{"é", key.ModSuper, chordIgnore, ""},
		{key.NameEscape, key.ModSuper, chordCancel, ""},
		{key.NameEscape, 0, chordCancel, ""},
		{key.NameDeleteBackward, 0, chordClear, ""},
		{key.NameDeleteForward, 0, chordClear, ""},
	}
	for _, test := range cases {
		action, chord := chordFromKey(test.name, test.modifiers)
		if action != test.action || chord != test.chord {
			t.Errorf("chordFromKey(%q, %v) = %v %q, want %v %q", test.name, test.modifiers, action, chord, test.action, test.chord)
		}
	}
}

func TestChordLoneKey(t *testing.T) {
	for _, token := range []string{"F1", "F12", "F24", "XF86AudioMute", "XF86MonBrightnessUp"} {
		if !chordLoneKey(token) {
			t.Errorf("chordLoneKey(%q) = false", token)
		}
	}
	for _, token := range []string{"F0", "F25", "Fx", "Q", "Return", "Print"} {
		if chordLoneKey(token) {
			t.Errorf("chordLoneKey(%q) = true", token)
		}
	}
}

func TestChordKeycaps(t *testing.T) {
	cases := map[string][]string{
		"Super+Shift+Return":    {"Super", "Shift", "Enter"},
		"SUPER + CTRL + Prior":  {"Super", "Ctrl", "Page Up"},
		"Super+KP_3":            {"Super", "Num 3"},
		"Super+grave":           {"Super", "`"},
		"Super+comma":           {"Super", ","},
		"Super+bracketright":    {"Super", "]"},
		"XF86AudioRaiseVolume":  {"Vol +"},
		"Ctrl+Next":             {"Ctrl", "Page Down"},
		"Alt+Escape":            {"Alt", "Esc"},
		"Super+minus":           {"Super", "minus"},
		"Super+Shift+BackSpace": {"Super", "Shift", "BackSpace"},
		"":                      nil,
	}
	for chord, want := range cases {
		if got := chordKeycaps(chord); !reflect.DeepEqual(got, want) {
			t.Errorf("chordKeycaps(%q) = %q, want %q", chord, got, want)
		}
	}
}
