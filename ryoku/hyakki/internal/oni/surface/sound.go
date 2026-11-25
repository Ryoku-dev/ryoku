package surface

import (
	"time"

	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/strand"
)

// Sound outcomes in the ledger.
const (
	soundPlayed = "played"
	soundGated  = "gated"
	soundError  = "error"
)

// soundSink is where motifs play: a PCM file in the lab when
// HYAKKI_ONI_SOUND_FILE is set, else a pw-cat or paplay child.
func (l Lab) soundSink() func() (sound.Sink, error) {
	if l.On && l.SoundFile != "" {
		path := l.SoundFile
		return func() (sound.Sink, error) { return sound.FileSink(path), nil }
	}
	return sound.PipeSink
}

// newPlayer builds the player for a character's motif notes and hands it
// the gate state the host already knows, so a character change neither
// opens a closed gate nor forgets the microphone.
func (h *host) newPlayer(p profile.Profile) {
	if h.player != nil {
		h.player.Gate().Enable(false)
	}
	h.player = sound.NewPlayer(h.sp, p.Animations.Motifs, h.openSink)
	g := h.player.Gate()
	if h.voice != "" {
		g.Voice(h.voice)
	}
	g.Enable(h.motionF.Sounds)
}

// sound plays the motif for event through the gate; events without a motif
// are not recorded.
func (h *host) sound(event string) {
	if h.player == nil {
		return
	}
	if _, ok := h.sp.Sound.Events[event]; !ok {
		return
	}
	motif, err := h.player.Play(time.Now(), event)
	reason := soundPlayed
	switch {
	case err != nil:
		reason = soundError
	case motif == "":
		reason = soundGated
	}
	h.led.Add(Entry{Ev: EvSound, Event: event, Motif: motif, Reason: reason})
}

// strandSounds plays the motifs of the phase the strand just entered. A
// quiet target plays none.
func (h *host) strandSounds(prev strand.Phase, f strand.Frame) {
	if h.target == nil || h.target.Quiet {
		return
	}
	cur := f.Phase
	if cur != prev {
		switch cur {
		case strand.PhaseReach, strand.PhaseSwing:
			h.sound("strand-reach")
		case strand.PhaseLasso, strand.PhaseRegion:
			if prev != strand.PhaseLasso && prev != strand.PhaseRegion {
				h.sound("strand-lasso")
			}
		case strand.PhaseCurl:
			h.sound("strand-perch")
		case strand.PhaseRewind:
			h.sound("strand-rewind")
		}
	}
	if f.Arrived && f.Skipped == "" {
		h.sound("strand-arrive")
	}
}
