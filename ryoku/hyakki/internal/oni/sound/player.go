package sound

import (
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Player turns events into sounds: it maps an event to its motif, asks the
// gate, renders the motif once and caches it, and plays it on a sink it
// opens on the first sound that passes the gate. A disabled or closed gate
// never opens the sink, so nothing is spawned while sounds are off or idle.
// A new sound replaces the one playing.
type Player struct {
	gate   *Gate
	events map[string]string
	motifs map[string]spec.Motif
	synth  spec.Synth
	cues   spec.Cues
	open   func() (Sink, error)

	mu   sync.Mutex
	sink Sink
	pcm  map[string][]float32
}

// NewPlayer plays the motifs of s with a character's note overrides
// (profile Animations.Motifs; nil for none) on the sink open returns, for
// example PipeSink. The player starts disabled.
func NewPlayer(s *spec.OniSpec, notes map[string][]int, open func() (Sink, error)) *Player {
	return &Player{
		gate:   NewGate(s.Sound.Synth),
		events: s.Sound.Events,
		motifs: Motifs(&s.Sound, notes),
		synth:  s.Sound.Synth,
		cues:   s.Sound.Cues,
		open:   open,
		pcm:    map[string][]float32{},
	}
}

// Gate is the player's gate: feed it the sounds setting and voice frames.
func (p *Player) Gate() *Gate { return p.gate }

// Play sounds the motif event maps to, if the gate allows it at now. It
// returns the motif played, or "" when the event has no motif or the gate
// held it back.
func (p *Player) Play(now time.Time, event string) (string, error) {
	id, ok := p.events[event]
	if !ok {
		return "", nil
	}
	m, ok := p.motifs[id]
	if !ok || !p.gate.Allow(now, id) {
		return "", nil
	}
	p.mu.Lock()
	if p.sink == nil {
		sink, err := p.open()
		if err != nil {
			p.mu.Unlock()
			return "", err
		}
		p.sink = sink
	}
	pcm, ok := p.pcm[id]
	if !ok {
		pcm = Render(m, p.synth, RateHz)
		p.pcm[id] = pcm
	}
	sink := p.sink
	p.mu.Unlock()
	cancel, err := sink.Play(pcm, RateHz)
	if err != nil {
		return "", err
	}
	p.gate.Track(cancel)
	return id, nil
}
