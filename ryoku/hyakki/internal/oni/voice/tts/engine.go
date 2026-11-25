// Package tts speaks Oni's lines: an engine renders a sentence to PCM in a
// child process or from a clip pack, and a Speaker plays the sentences one
// at a time on a sound.Sink, so a stop fades within the sink's mute ramp.
// Nothing runs, opens or is created until a line is spoken.
package tts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
)

// Engine names, as the profile's voice.engine and settings spell them.
const (
	System = profile.VoiceSystem
	Clips  = profile.VoiceClips
	Kokoro = profile.VoiceKokoro
)

// Engine renders one sentence.
type Engine interface {
	// Name is the engine, as reported for every sentence it speaks.
	Name() string
	// Voice names everything that changes how a sentence sounds (program,
	// voice, speed, pack); the line cache keys on it.
	Voice() string
	// Render returns the sentence as mono PCM at SpeechRateHz.
	Render(ctx context.Context, line string) (pcm []float32, rateHz int, err error)
}

// ErrUnavailable is an engine that is not installed.
var ErrUnavailable = errors.New("tts: engine not installed")

// SpeechRateHz is the rate engines render at and the sink plays speech at:
// the spec's sound.gating.speechSampleRateHz.
func SpeechRateHz() int { return int(builtin().Sound.Gating.SpeechSampleRateHz) }

var builtin = sync.OnceValue(func() *spec.OniSpec {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s
})

// Order is the engines to try, each once: the settings choice, then the
// profile's voice.engine, then system as the fallback. Empty or unknown
// names are skipped.
func Order(setting, profileEngine string) []string {
	var out []string
	for _, n := range []string{setting, profileEngine, System} {
		if slices.Contains([]string{System, Clips, Kokoro}, n) && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Pick returns the engines of have in order; a name with no engine (not
// installed, no pack) is skipped.
func Pick(order []string, have map[string]Engine) []Engine {
	var out []Engine
	for _, n := range order {
		if e := have[n]; e != nil {
			out = append(out, e)
		}
	}
	return out
}

// Runner runs argv with stdin and returns its stdout. Tests pass a fake;
// Exec is the real one.
type Runner func(ctx context.Context, argv []string, stdin []byte) ([]byte, error)

// Caps on one engine child.
const (
	maxOutput   = 64 << 20 // about 20 minutes of 16-bit speech at 24 kHz
	maxStderr   = 4 << 10
	waitDelay   = 2 * time.Second
	renderLimit = 30 * time.Second
)

// Exec runs argv as a child process with stdin as its input, killing it
// when ctx ends, and returns its stdout. The child gets no terminal and is
// never given an audio device: every engine here writes audio to stdout.
func Exec(ctx context.Context, argv []string, stdin []byte) ([]byte, error) {
	if len(argv) == 0 {
		return nil, errors.New("tts: empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	out := &capped{max: maxOutput}
	errs := &capped{max: maxStderr, quiet: true}
	cmd.Stdout, cmd.Stderr = out, errs
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("tts: %s: %w: %s", argv[0], err, bytes.TrimSpace(errs.buf.Bytes()))
	}
	if out.over {
		return nil, fmt.Errorf("tts: %s wrote more than %d bytes", argv[0], maxOutput)
	}
	return out.buf.Bytes(), nil
}

// capped keeps at most max bytes; quiet drops the rest, otherwise it
// marks the overflow.
type capped struct {
	buf   bytes.Buffer
	max   int
	over  bool
	quiet bool
}

func (c *capped) Write(p []byte) (int, error) {
	room := c.max - c.buf.Len()
	if len(p) > room {
		c.over = !c.quiet
		c.buf.Write(p[:max(room, 0)])
		return len(p), nil
	}
	return c.buf.Write(p)
}
