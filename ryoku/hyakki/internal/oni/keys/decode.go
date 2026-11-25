package keys

import (
	"encoding/binary"
	"math/bits"
)

// Edge is all the key source reports: the chord went down, came up, or was
// spoiled while down by a key that is not part of it.
type Edge int

const (
	// Down: the last key of the chord went down with nothing else held.
	Down Edge = iota + 1
	// Up: a key of the chord came up after Down.
	Up
	// Cancel: another key went down while the chord was down, so the
	// press was a shortcut, not a talk.
	Cancel
)

func (e Edge) String() string {
	switch e {
	case Down:
		return "down"
	case Up:
		return "up"
	case Cancel:
		return "cancel"
	}
	return ""
}

// EventSize is a struct input_event on a 64-bit kernel: a struct timeval
// (16 bytes), __u16 type, __u16 code and __s32 value.
const EventSize = 24

// evdev event types and codes the decoder reads.
const (
	evSyn      = 0x00
	evKey      = 0x01
	synDropped = 3

	valueUp     = 0
	valueDown   = 1
	valueRepeat = 2
)

// Decoder turns key events into chord edges. One Decoder holds the keys held
// across every keyboard, so a chord split over two keyboards still counts.
// It is not safe for concurrent use.
type Decoder struct {
	chord Chord
	held  [keyCount / 64]uint64
	down  bool
	// spent: the chord went down or was spoiled; it goes down again only
	// once none of its keys is held
	spent bool
	// part is the tail of an event split across Feed calls
	part []byte
}

// NewDecoder decodes for chord.
func NewDecoder(c Chord) *Decoder { return &Decoder{chord: c} }

// Feed decodes raw struct input_event bytes, in the machine's byte order as
// the kernel writes them, and returns the edges they make, in order. An
// event split across calls is joined.
func (d *Decoder) Feed(b []byte) []Edge {
	var edges []Edge
	d.feed(&d.part, b, func(e Edge) { edges = append(edges, e) })
	return edges
}

// feed decodes one stream's bytes; part holds that stream's split event, so
// several keyboards share the keys held without mixing their bytes.
func (d *Decoder) feed(part *[]byte, b []byte, emit func(Edge)) {
	for {
		ev, rest, ok := nextEvent(part, b)
		if !ok {
			return
		}
		b = rest
		d.event(ev, emit)
	}
}

// nextEvent returns a stream's next whole event, joining part, the event
// split by its last read, with b, and the rest of b. When b holds no whole
// event its bytes are kept in part and ok is false. The event is valid until
// the next call.
func nextEvent(part *[]byte, b []byte) (ev, rest []byte, ok bool) {
	if len(*part) > 0 {
		need := EventSize - len(*part)
		if len(b) < need {
			*part = append(*part, b...)
			return nil, nil, false
		}
		*part = append(*part, b[:need]...)
		ev = *part
		*part = (*part)[:0]
		return ev, b[need:], true
	}
	if len(b) >= EventSize {
		return b[:EventSize], b[EventSize:], true
	}
	*part = append(*part, b...)
	return nil, nil, false
}

// parseEvent reads a struct input_event's type, code and value.
func parseEvent(ev []byte) (typ, code uint16, value int32) {
	return binary.NativeEndian.Uint16(ev[16:18]), binary.NativeEndian.Uint16(ev[18:20]),
		int32(binary.NativeEndian.Uint32(ev[20:24]))
}

func (d *Decoder) event(ev []byte, emit func(Edge)) {
	typ, code, value := parseEvent(ev)
	switch {
	case typ == evSyn && code == synDropped:
		d.dropped(emit)
	case typ == evKey:
		d.key(code, value, emit)
	}
}

// dropped: the kernel's buffer overran and the keys held are unknown, so a
// press in progress is cancelled and the decoder starts over.
func (d *Decoder) dropped(emit func(Edge)) {
	if d.down {
		emit(Cancel)
	}
	d.held, d.down, d.spent = [keyCount / 64]uint64{}, false, false
}

func (d *Decoder) isHeld(code uint16) bool { return d.held[code/64]&(1<<(code%64)) != 0 }

func (d *Decoder) key(code uint16, value int32, emit func(Edge)) {
	if code >= keyCount {
		return
	}
	switch value {
	case valueDown:
		if d.isHeld(code) {
			return
		}
		d.held[code/64] |= 1 << (code % 64)
		switch {
		case d.down && !d.chord.counts(code):
			d.down, d.spent = false, true
			emit(Cancel)
		case !d.down && !d.spent && d.complete():
			d.down, d.spent = true, true
			emit(Down)
		case !d.down && !d.chord.counts(code) && d.anyChordKey():
			// a shortcut that shares the chord's modifiers: the chord
			// does not count until its keys are let go
			d.spent = true
		}
	case valueUp:
		if !d.isHeld(code) {
			return
		}
		d.held[code/64] &^= 1 << (code % 64)
		if d.down && !d.complete() {
			d.down = false
			emit(Up)
		}
		if d.spent && !d.anyChordKey() {
			d.spent = false
		}
	}
	// valueRepeat and anything else: a held key repeating changes nothing
}

// complete reports whether exactly the chord is held: every key it needs,
// no other key, and never Right Alt (AltGr, which LCtrl+RAlt also sends).
func (d *Decoder) complete() bool {
	c := d.chord
	either := func(a, b uint16) bool { return d.isHeld(a) || d.isHeld(b) }
	if c.ctrl != either(keyLeftCtrl, keyRightCtrl) || c.alt != d.isHeld(keyLeftAlt) ||
		c.shift != either(keyLeftShift, keyRightShift) || c.super != either(keyLeftMeta, keyRightMeta) {
		return false
	}
	if c.key != 0 && !d.isHeld(c.key) {
		return false
	}
	for w, word := range d.held {
		for word != 0 {
			low := word & -word
			word &^= low
			code := uint16(w*64 + bits.TrailingZeros64(low))
			if !c.counts(code) {
				return false
			}
		}
	}
	return true
}

// anyChordKey reports whether any key the chord is made of is held.
func (d *Decoder) anyChordKey() bool {
	for _, code := range []uint16{keyLeftCtrl, keyRightCtrl, keyLeftAlt, keyLeftShift, keyRightShift, keyLeftMeta, keyRightMeta, d.chord.key} {
		if code != 0 && d.chord.counts(code) && d.isHeld(code) {
			return true
		}
	}
	return false
}
