// Package keys is Oni's hold-to-talk key source: it reads raw evdev key
// events from the keyboards, or in the lab from a FIFO carrying the same
// bytes, and reports only that the talk chord went down, came up, or was
// spoiled by another key. StartStop reads the same keyboards for Hands'
// stop and reports only that Esc went down; StartAnyKey reads them for the
// live app pause and reports only that some key went down. No key identity
// ever leaves the package.
package keys

import (
	"fmt"
	"strconv"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// evdev key codes (linux/input-event-codes.h) the chord is made of. A chord
// key names a physical key in the US layout, as evdev does.
const (
	keyLeftCtrl   = 29
	keyRightCtrl  = 97
	keyLeftAlt    = 56
	keyRightAlt   = 100 // AltGr: never counts as Alt
	keyLeftShift  = 42
	keyRightShift = 54
	keyLeftMeta   = 125
	keyRightMeta  = 126
	keySpace      = 57

	// keyCount is KEY_CNT: codes at or past it are not keys.
	keyCount = 0x300
)

var letterCodes = map[byte]uint16{
	'q': 16, 'w': 17, 'e': 18, 'r': 19, 't': 20, 'y': 21, 'u': 22, 'i': 23, 'o': 24, 'p': 25,
	'a': 30, 's': 31, 'd': 32, 'f': 33, 'g': 34, 'h': 35, 'j': 36, 'k': 37, 'l': 38,
	'z': 44, 'x': 45, 'c': 46, 'v': 47, 'b': 48, 'n': 49, 'm': 50,
	'1': 2, '2': 3, '3': 4, '4': 5, '5': 6, '6': 7, '7': 8, '8': 9, '9': 10, '0': 11,
}

// fCode is the code of function key n, 1 to 24.
func fCode(n int) uint16 {
	switch {
	case n <= 10:
		return uint16(58 + n) // F1 59 … F10 68
	case n <= 12:
		return uint16(76 + n) // F11 87, F12 88
	}
	return uint16(170 + n) // F13 183 … F24 194
}

// Chord is a parsed talk chord: the modifiers it needs and at most one key.
type Chord struct {
	ctrl, alt, shift, super bool
	key                     uint16 // 0 for a modifier-only chord
	name                    string
}

// String is the chord in protocol.NormalizeOniChord's form.
func (c Chord) String() string { return c.name }

// ParseChord validates a talk chord as Settings stores it: it must pass
// protocol.NormalizeOniChord (Ctrl, Alt or Super, at most one other key)
// and name a key this reader can see.
func ParseChord(s string) (Chord, error) {
	canon, ok := protocol.NormalizeOniChord(s)
	if !ok {
		return Chord{}, fmt.Errorf("talk chord %q needs Ctrl, Alt or Super with at most one other key", s)
	}
	c := Chord{name: canon}
	for part := range strings.SplitSeq(canon, "+") {
		switch part {
		case "ctrl":
			c.ctrl = true
		case "alt":
			c.alt = true
		case "shift":
			c.shift = true
		case "super":
			c.super = true
		case "space":
			c.key = keySpace
		default:
			if len(part) == 1 {
				c.key = letterCodes[part[0]]
			} else if n, err := strconv.Atoi(part[1:]); err == nil && n >= 1 && n <= 24 {
				c.key = fCode(n)
			}
			if c.key == 0 {
				return Chord{}, fmt.Errorf("talk chord %q names a key the reader cannot see", s)
			}
		}
	}
	return c, nil
}

// counts reports whether code is one of the keys the chord is made of. Right
// Alt is AltGr on most layouts and never counts.
func (c Chord) counts(code uint16) bool {
	switch code {
	case keyLeftCtrl, keyRightCtrl:
		return c.ctrl
	case keyLeftAlt:
		return c.alt
	case keyLeftShift, keyRightShift:
		return c.shift
	case keyLeftMeta, keyRightMeta:
		return c.super
	}
	return c.key != 0 && code == c.key
}
