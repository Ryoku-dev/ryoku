package keys

import "errors"

// AnyKeyFIFOEnv names the lab pipe StartAnyKey reads instead of the
// keyboards. It is its own variable, never the talk chord's
// (HYAKKI_ONI_KEYS_FIFO) or the Esc stop's (HYAKKI_ONI_STOP_FIFO): readers
// sharing one pipe would each take bytes the other never sees.
const AnyKeyFIFOEnv = "HYAKKI_ONI_ANYKEY_FIFO"

// AnyKeyOptions builds StartAnyKey's options from getenv: the system
// keyboards, or under HYAKKI_ONI_LAB=1 with AnyKeyFIFOEnv set, the lab pipe
// it names. Outside the lab the variable is ignored, and StartAnyKey would
// refuse a FIFO there anyway.
func AnyKeyOptions(getenv func(string) string) Options {
	opts := Options{Getenv: getenv}
	if getenv != nil && getenv("HYAKKI_ONI_LAB") == "1" {
		opts.FIFO = getenv(AnyKeyFIFOEnv)
	}
	return opts
}

// StartAnyKey reads the same keyboards as the talk chord, or in the lab its
// own FIFO, and calls onKey each time any key goes down on any of them. It
// never says which key: onKey takes nothing, so no key identity leaves the
// package. A key held down repeating is not another press. After the kernel
// dropped events the presses in them are unknown, so a drop counts as one
// press: a key nobody could read is never assumed unpressed.
//
// Keys are read, never grabbed, so every key still reaches the focused
// window. Typing through a virtual keyboard on the Wayland wire never
// reaches these devices, so Hands' own typing is not reported. onKey runs on
// the source's own goroutines, one call at a time, and may call Stop. It
// returns *Unavailable when no keyboard can be read, and ErrFIFORefused for
// a FIFO outside the lab; opts.Chord is not used.
//
// The source reads only between StartAnyKey and Stop. Its readers sleep in
// the poller: no timer, no poll.
func StartAnyKey(opts Options, onKey func()) (*Source, error) {
	if onKey == nil {
		return nil, errors.New("keys: StartAnyKey needs onKey")
	}
	return start(opts, &Source{onPress: onKey, presses: anyDecoder{}})
}

// anyDecoder counts key down-edges. It holds nothing, so no key is ever
// remembered.
type anyDecoder struct{}

func (anyDecoder) feed(part *[]byte, b []byte) (downs int) {
	for {
		ev, rest, ok := nextEvent(part, b)
		if !ok {
			return downs
		}
		b = rest
		typ, code, value := parseEvent(ev)
		switch {
		case typ == evSyn && code == synDropped:
			downs++
		case typ == evKey && code < keyCount && value == valueDown:
			downs++
		}
	}
}
