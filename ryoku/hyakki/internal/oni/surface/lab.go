// Package surface is oni-surface: the native presenter that draws Oni on a
// layer-shell surface, sized to the sprite, driven by the daemon's oni frames
// and the shared motion engine.
package surface

import (
	"fmt"
	"strconv"
	"strings"
)

// Power tiers HYAKKI_ONI_POWER can force.
const (
	PowerSaver = "saver"
	PowerFull  = "full"
)

// Lab is the proof hooks. Every field is zero unless HYAKKI_ONI_LAB=1, so a
// shipped surface ignores the other variables entirely.
type Lab struct {
	On        bool
	Ledger    string
	TimeScale float64
	Power     string
	Script    string
	// Profile is a directory for a CPU profile and an allocation profile
	// of the whole run, written at exit.
	Profile string
	// SoundFile replaces the audio player with a file of the PCM it would
	// have played (sound.FileSink).
	SoundFile string
	// Arrivals is a file where the scripted daemon records every
	// oni.strandArrived call.
	Arrivals string
	// KeysFIFO is a named pipe of struct input_event bytes the talk-chord
	// reader reads instead of the keyboards.
	KeysFIFO string
}

// LabFromEnv reads the lab hooks through getenv.
func LabFromEnv(getenv func(string) string) (Lab, error) {
	if getenv("HYAKKI_ONI_LAB") != "1" {
		return Lab{TimeScale: 1}, nil
	}
	l := Lab{On: true, TimeScale: 1,
		Ledger:    getenv("HYAKKI_ONI_LEDGER"),
		Script:    getenv("HYAKKI_ONI_SCRIPT"),
		Profile:   getenv("HYAKKI_ONI_PROFILE"),
		SoundFile: getenv("HYAKKI_ONI_SOUND_FILE"),
		Arrivals:  getenv("HYAKKI_ONI_ARRIVALS"),
		KeysFIFO:  getenv("HYAKKI_ONI_KEYS_FIFO"),
	}
	if v := strings.TrimSpace(getenv("HYAKKI_ONI_TIMESCALE")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0.1 || f > 1 {
			return Lab{}, fmt.Errorf("HYAKKI_ONI_TIMESCALE %q is not between 0.1 and 1", v)
		}
		l.TimeScale = f
	}
	switch v := getenv("HYAKKI_ONI_POWER"); v {
	case "", PowerSaver, PowerFull:
		l.Power = v
	default:
		return Lab{}, fmt.Errorf("HYAKKI_ONI_POWER %q is not saver or full", v)
	}
	return l, nil
}

// batterySaver is the tier the engine runs at: the lab override, else the
// daemon's motion frame.
func (l Lab) batterySaver(daemon bool) bool {
	switch l.Power {
	case PowerSaver:
		return true
	case PowerFull:
		return false
	}
	return daemon
}
