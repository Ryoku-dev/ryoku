package keys

import "errors"

// keyEsc is KEY_ESC.
const keyEsc = 1

// StartStop reads the same keyboards as the talk chord, or in the lab the
// same kind of FIFO, and calls onStop each time Esc goes down on any of
// them, whatever else is held. Nothing else is reported: no key, no other
// edge. Esc is read, never grabbed, so it still reaches the focused window.
// onStop runs on the source's own goroutines, one call at a time, and may
// call Stop. It returns *Unavailable when no keyboard can be read, and
// ErrFIFORefused for a FIFO outside the lab; opts.Chord is not used.
//
// The source reads only between StartStop and Stop. Its readers sleep in
// the poller: no timer, no poll.
func StartStop(opts Options, onStop func()) (*Source, error) {
	if onStop == nil {
		return nil, errors.New("keys: StartStop needs onStop")
	}
	return start(opts, &Source{onPress: onStop, presses: new(escDecoder)})
}

// pressDecoder counts presses in one stream's bytes, as Decoder.feed reads
// them: it says how many, never which key.
type pressDecoder interface {
	feed(part *[]byte, b []byte) (downs int)
}

// escDecoder finds Esc going down. It holds only whether Esc is down, shared
// by every keyboard, so a held Esc repeating, or pressed on a second
// keyboard while held on the first, is not another stop.
type escDecoder struct{ held bool }

// feed decodes one stream's bytes, as Decoder.feed does, and returns how
// many times Esc went down.
func (d *escDecoder) feed(part *[]byte, b []byte) (downs int) {
	for {
		ev, rest, ok := nextEvent(part, b)
		if !ok {
			return downs
		}
		b = rest
		typ, code, value := parseEvent(ev)
		switch {
		case typ == evSyn && code == synDropped:
			// the keys held are unknown: the next Esc press counts
			d.held = false
		case typ == evKey && code == keyEsc && value == valueDown:
			if !d.held {
				d.held = true
				downs++
			}
		case typ == evKey && code == keyEsc && value == valueUp:
			d.held = false
		}
	}
}

// feedPress decodes b for presses and calls onPress outside mu, so onPress
// may call Stop; report keeps the calls one at a time across keyboards.
func (s *Source) feedPress(part *[]byte, b []byte) bool {
	s.report.Lock()
	defer s.report.Unlock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return false
	}
	downs := s.presses.feed(part, b)
	s.mu.Unlock()
	for range downs {
		s.mu.Lock()
		stopped := s.stopped
		s.mu.Unlock()
		if stopped {
			return false
		}
		s.onPress()
	}
	return true
}
