package tts

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"ryoku-rashin/internal/oni/voice/clips"
)

// espeak-ng's speaking rate in words per minute at speed 1, and the range
// it accepts.
const (
	espeakWPM    = 175
	espeakMinWPM = 80
	espeakMaxWPM = 450
)

// SystemOptions configure the system voice.
type SystemOptions struct {
	Path  string  // espeak-ng; "" finds it on PATH
	Voice string  // an espeak-ng voice name; "" is its default
	Speed float64 // profile voice.speed; 0 is 1
	Run   Runner  // nil is Exec
}

// SystemEngine is the system voice: one espeak-ng child per sentence,
// reading the text on stdin and writing a WAV to stdout, never to an audio
// device or a file.
type SystemEngine struct {
	path, voice string
	wpm         int
	run         Runner
}

// NewSystem finds espeak-ng; it fails with ErrUnavailable when it is not
// installed. Nothing is started.
func NewSystem(o SystemOptions) (*SystemEngine, error) {
	p := o.Path
	if p == "" {
		p = "espeak-ng"
	}
	p, err := exec.LookPath(p)
	if err != nil {
		return nil, fmt.Errorf("%w: espeak-ng", ErrUnavailable)
	}
	speed := o.Speed
	if speed == 0 {
		speed = 1
	}
	run := o.Run
	if run == nil {
		run = Exec
	}
	wpm := int(math.Round(espeakWPM * speed))
	return &SystemEngine{path: p, voice: o.Voice, wpm: max(espeakMinWPM, min(espeakMaxWPM, wpm)), run: run}, nil
}

func (e *SystemEngine) Name() string { return System }

func (e *SystemEngine) Voice() string {
	return "espeak-ng|" + e.path + "|" + e.voice + "|" + strconv.Itoa(e.wpm)
}

// Render speaks line through espeak-ng and converts it to SpeechRateHz.
func (e *SystemEngine) Render(ctx context.Context, line string) ([]float32, int, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, 0, fmt.Errorf("tts: nothing to say")
	}
	argv := []string{e.path, "--stdout", "--stdin", "-b", "1", "-s", strconv.Itoa(e.wpm)}
	if e.voice != "" {
		argv = append(argv, "-v", e.voice)
	}
	ctx, cancel := context.WithTimeout(ctx, renderLimit)
	defer cancel()
	out, err := e.run(ctx, argv, []byte(line))
	if err != nil {
		return nil, 0, err
	}
	pcm, rate, err := clips.DecodeWAV(out)
	if err != nil {
		return nil, 0, fmt.Errorf("tts: espeak-ng: %w", err)
	}
	return clips.Resample(pcm, rate, SpeechRateHz()), SpeechRateHz(), nil
}
