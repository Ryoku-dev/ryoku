package surface

import (
	"context"
	"errors"
	"os"

	"ryoku-rashin/internal/oni/keys"
	"ryoku-rashin/internal/protocol"
)

// keySource is a running talk-chord reader.
type keySource interface {
	Stop()
	Kind() string
}

// keyStarter starts a reader for chord that calls edge from its own
// goroutine.
type keyStarter func(chord string, edge func(keys.Edge)) (keySource, error)

// startKeys reads the keyboards, or under HYAKKI_ONI_LAB=1 with
// HYAKKI_ONI_KEYS_FIFO set, the lab's pipe of input_event bytes.
func (l Lab) startKeys() keyStarter {
	return func(chord string, edge func(keys.Edge)) (keySource, error) {
		src, err := keys.Start(keys.Options{Chord: chord, FIFO: l.KeysFIFO, Getenv: os.Getenv}, edge)
		if err != nil {
			return nil, err
		}
		return src, nil
	}
}

// keysHost is the presenter's hold-to-talk: the reader runs only while the
// motion frame names a chord, which the daemon sends only while voice is
// on, and each edge becomes one oni.talk call, in order.
type keysHost struct {
	startKeys keyStarter
	keySrc    keySource
	keyChord  string
	// keyGen counts readers, so an edge posted by one since stopped is
	// dropped
	keyGen int
	// talkWant is a start owed to a link still opening: the chord went
	// down with the daemon gone and is still held
	talkWant bool
	// talkTail closes when the last talk call sent has returned
	talkTail chan struct{}
}

// setChord starts, restarts or stops the reader for the chord the motion
// frame names, "" while voice is off.
func (h *host) setChord(chord string) {
	if chord == h.keyChord {
		return
	}
	h.stopKeys()
	h.keyChord = chord
	if chord == "" || h.startKeys == nil {
		return
	}
	gen := h.keyGen
	src, err := h.startKeys(chord, func(e keys.Edge) {
		h.be.post(func() {
			if gen == h.keyGen {
				h.chordEdge(e)
			}
		})
	})
	if err != nil {
		reason := "error"
		var un *keys.Unavailable
		switch {
		case errors.As(err, &un):
			reason = string(un.Reason)
		case errors.Is(err, keys.ErrFIFORefused):
			reason = "fifo-refused"
		}
		h.led.Add(Entry{Ev: EvKeys, Reason: reason})
		return
	}
	h.keySrc = src
	h.led.Add(Entry{Ev: EvKeys, Reason: src.Kind()})
}

// stopKeys stops the reader; a start owed to an opening link is dropped
// with it.
func (h *host) stopKeys() {
	h.keyGen++
	h.talkWant = false
	if h.keySrc != nil {
		h.keySrc.Stop()
		h.keySrc = nil
		h.led.Add(Entry{Ev: EvKeys, Reason: "stopped"})
	}
}

// chordEdge turns one edge into oni.talk. Down starts the daemon when it is
// gone (the press is the user asking); Up and Cancel only reach a link that
// is open or opening, since a daemon that is gone has no press to end.
func (h *host) chordEdge(e keys.Edge) {
	h.led.Add(Entry{Ev: EvChord, Event: e.String()})
	action := protocol.OniVoiceStart
	switch e {
	case keys.Up:
		action = protocol.OniVoiceStop
	case keys.Cancel:
		action = protocol.OniVoiceCancel
	}
	if h.link != nil {
		h.talk(h.link, action)
		return
	}
	if e != keys.Down {
		// the press ended before the daemon answered: nothing was opened
		h.talkWant = false
		return
	}
	h.talkWant = true
	if !h.linking {
		h.retried = false
		h.led.Add(Entry{Ev: EvRedial, Reason: "chord"})
		h.connect(true)
	}
}

// talkOpened sends the start owed to a link that just opened.
func (h *host) talkOpened(l Link) {
	if h.talkWant {
		h.talkWant = false
		h.talk(l, protocol.OniVoiceStart)
	}
}

// talk sends one oni.talk call once the previous one returned, so a stop
// never overtakes its start.
func (h *host) talk(l Link, action string) {
	prev := h.talkTail
	done := make(chan struct{})
	h.talkTail = done
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		l.Talk(ctx, action)
	}()
}
