package sound

import (
	"math"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

type mic uint8

const (
	micUnknown mic = iota // no voice frame yet, or the link dropped
	micQuiet              // the last voice frame said idle
	micLive               // listening, transcribing, speaking or anything unknown
)

// Gate decides whether a motif may sound. It is closed until sounds are
// enabled and a voice frame has said the microphone is idle; any other voice
// phase, a lost link or turning sounds off closes it again and cancels the
// sound playing, whose sink ramps it out within the spec's mute ramp. Motifs
// closer together than the debounce are dropped. The zero value is a
// disabled gate using the built-in spec's debounce. Safe for concurrent use.
type Gate struct {
	mu       sync.Mutex
	debounce time.Duration
	tuned    bool
	on       bool
	mic      mic
	last     time.Time
	sounded  bool
	cancel   func()
}

// NewGate returns a disabled gate using the debounce of s.
func NewGate(s spec.Synth) *Gate {
	return &Gate{debounce: seconds(s.DebounceS), tuned: true}
}

// Enable turns sounds on or off; off cancels the sound playing.
func (g *Gate) Enable(on bool) {
	g.mu.Lock()
	g.on = on
	stop := g.closing()
	g.mu.Unlock()
	run(stop)
}

// Voice records a voice pipeline phase (feature.VoiceFrame.Phase). Only
// "idle" means the microphone is quiet; every other phase is mic-live and
// cancels the sound playing at once.
func (g *Gate) Voice(phase string) {
	g.mu.Lock()
	g.mic = micLive
	if phase == "idle" {
		g.mic = micQuiet
	}
	stop := g.closing()
	g.mu.Unlock()
	run(stop)
}

// LinkLost forgets the microphone state: the gate stays closed until an
// idle voice frame is seen again, and the sound playing is cancelled.
func (g *Gate) LinkLost() {
	g.mu.Lock()
	g.mic = micUnknown
	stop := g.closing()
	g.mu.Unlock()
	run(stop)
}

// Allow reports whether motif may start at now: sounds are on, the
// microphone is known to be quiet and the last allowed motif is at least
// the debounce old. A true result claims the slot.
func (g *Gate) Allow(now time.Time, motif string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if motif == "" || !g.open() {
		return false
	}
	if g.sounded && now.Sub(g.last) < g.window() {
		return false
	}
	g.last, g.sounded = now, true
	return true
}

// Track hands the gate the cancel of the sound now playing. It cancels the
// sound tracked before, and cancels this one at once if the gate closed
// since Allow.
func (g *Gate) Track(cancel func()) {
	if cancel == nil {
		return
	}
	g.mu.Lock()
	prev := g.cancel
	g.cancel = cancel
	stop := g.closing()
	g.mu.Unlock()
	run(prev)
	run(stop)
}

func (g *Gate) open() bool { return g.on && g.mic == micQuiet }

// closing takes the tracked cancel when the gate is closed.
func (g *Gate) closing() func() {
	if g.open() {
		return nil
	}
	stop := g.cancel
	g.cancel = nil
	return stop
}

func (g *Gate) window() time.Duration {
	if g.tuned {
		return g.debounce
	}
	return seconds(builtin().Sound.Synth.DebounceS)
}

func run(f func()) {
	if f != nil {
		f()
	}
}

func seconds(s float64) time.Duration { return time.Duration(math.Round(s * float64(time.Second))) }

func millis(ms float64) time.Duration {
	return time.Duration(math.Round(ms * float64(time.Millisecond)))
}
