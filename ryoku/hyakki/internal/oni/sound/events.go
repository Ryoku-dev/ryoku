package sound

import (
	"sync"

	"ryoku-rashin/internal/oni/spec"
)

var builtin = sync.OnceValue(func() *spec.OniSpec {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s
})

// Event returns the motif a strand or gesture event plays (spec
// sound.events).
func Event(id string) (motif string, ok bool) {
	motif, ok = builtin().Sound.Events[id]
	return motif, ok
}

// Motifs returns the spec's motifs with a character's note overrides
// (profile Animations.Motifs) applied. An override replaces only the notes;
// delay, spacing and length stay the spec's. An override for an unknown
// motif, or with no notes or a note outside MIDI 0-127, is ignored.
func Motifs(s *spec.Sound, notes map[string][]int) map[string]spec.Motif {
	out := make(map[string]spec.Motif, len(s.Motifs))
	for id, m := range s.Motifs {
		if n, ok := notes[id]; ok && validNotes(n) {
			m.Notes = append([]int(nil), n...)
		}
		out[id] = m
	}
	return out
}

func validNotes(n []int) bool {
	if len(n) == 0 {
		return false
	}
	for _, v := range n {
		if v < 0 || v > 127 {
			return false
		}
	}
	return true
}
