// Package audio carries out a reflex's sound actions: the default output's
// volume and mute through pactl's set verbs, and play or pause for a media
// player over the session bus. Nothing else on the sound server is touched:
// every pactl argv names @DEFAULT_SINK@ and is one of a fixed few.
package audio

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"ryoku-rashin/internal/reflex/proc"
)

// Step is how far one "volume up" or "volume down" moves, in percent.
const Step = 5

// sink is the only sink any argv names.
const sink = "@DEFAULT_SINK@"

// The exact pactl argv this package may run.
var (
	argvGetVolume = []string{"get-sink-volume", sink}
	argvUp        = []string{"set-sink-volume", sink, "+" + strconv.Itoa(Step) + "%"}
	argvDown      = []string{"set-sink-volume", sink, "-" + strconv.Itoa(Step) + "%"}
	argvFull      = []string{"set-sink-volume", sink, "100%"}
	argvMuteOn    = []string{"set-sink-mute", sink, "1"}
	argvMuteOff   = []string{"set-sink-mute", sink, "0"}
	argvMuteFlip  = []string{"set-sink-mute", sink, "toggle"}
	pactlArgv     = [][]string{argvGetVolume, argvUp, argvDown, argvFull, argvMuteOn, argvMuteOff, argvMuteFlip}
)

// Allowed reports whether name and args are one of the pactl commands
// above, exactly.
func Allowed(name string, args ...string) bool {
	return name == "pactl" && slices.ContainsFunc(pactlArgv, func(a []string) bool { return slices.Equal(a, args) })
}

// ErrAtFull means the volume is already at 100 %; nothing was changed.
var ErrAtFull = errors.New("audio: volume already full")

// Volume changes the default output.
type Volume struct{ run proc.Runner }

// NewVolume returns a Volume that runs pactl through run (proc.Exec for
// the real one). Every call is checked against Allowed.
func NewVolume(run proc.Runner) *Volume { return &Volume{run: proc.Guard(Allowed, run)} }

// Up raises the volume one step, never past 100 %.
func (v *Volume) Up(ctx context.Context) error {
	out, err := v.run(ctx, "pactl", argvGetVolume...)
	if err != nil {
		return err
	}
	level, ok := level(out)
	if !ok {
		return fmt.Errorf("audio: unreadable volume %q", out)
	}
	switch {
	case level >= 100:
		return ErrAtFull
	case level+Step > 100:
		_, err = v.run(ctx, "pactl", argvFull...)
	default:
		_, err = v.run(ctx, "pactl", argvUp...)
	}
	return err
}

// Down lowers the volume one step; the sound server stops at zero.
func (v *Volume) Down(ctx context.Context) error {
	_, err := v.run(ctx, "pactl", argvDown...)
	return err
}

// MuteMode says what Mute does.
type MuteMode int

const (
	MuteToggle MuteMode = iota
	MuteOn
	MuteOff
)

// Mute mutes, unmutes or toggles the default output.
func (v *Volume) Mute(ctx context.Context, mode MuteMode) error {
	argv := argvMuteFlip
	switch mode {
	case MuteOn:
		argv = argvMuteOn
	case MuteOff:
		argv = argvMuteOff
	}
	_, err := v.run(ctx, "pactl", argv...)
	return err
}

var percent = regexp.MustCompile(`(\d{1,4})%`)

// level is the loudest channel in pactl's get-sink-volume answer.
func level(out []byte) (int, bool) {
	best, ok := 0, false
	for _, m := range percent.FindAllSubmatch(out, -1) {
		n, err := strconv.Atoi(string(m[1]))
		if err == nil && (!ok || n > best) {
			best, ok = n, true
		}
	}
	return best, ok
}
