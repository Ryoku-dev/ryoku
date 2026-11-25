package audio

import (
	"context"
	"errors"
	"math"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// MeterFloorDB is the quietest level the meter shows: a window at or below
// this many dB under full scale reads 0, full scale reads 1, and the scale
// between is linear in dB.
const MeterFloorDB = -60.0

// Meter turns capture into mic levels for the Settings test meter: one level
// in [0, 1] per Window samples. It keeps the part of a window not yet full
// between pushes and nothing else.
type Meter struct {
	Window int
	sum    float64
	n      int
}

// NewMeter sizes the window from the spec: the fewest whole voice.vad
// frames that keep the level rate at or under voice.levels.maxHz.
func NewMeter(sp *spec.OniSpec) *Meter {
	v := sp.Voice
	frames := math.Ceil(1000 / v.Levels.MaxHz / v.Vad.FrameMs)
	perFrame := float64(v.Vad.SampleRateHz) * v.Vad.FrameMs / 1000
	return &Meter{Window: int(frames * perFrame)}
}

// Push adds capture and returns the level of every window it completes.
func (m *Meter) Push(pcm []int16) []float64 {
	if m.Window <= 0 {
		return nil
	}
	var levels []float64
	for _, s := range pcm {
		f := float64(s)
		m.sum += f * f
		m.n++
		if m.n == m.Window {
			levels = append(levels, Level(math.Sqrt(m.sum/float64(m.n))))
			m.sum, m.n = 0, 0
		}
	}
	return levels
}

// Level maps an RMS sample value to the meter's [0, 1] scale.
func Level(rms float64) float64 {
	if !(rms > 0) {
		return 0
	}
	db := 20 * math.Log10(rms/32768)
	return min(max((db-MeterFloorDB)/-MeterFloorDB, 0), 1)
}

// ErrNoAudio is returned by Test when the recorder ended before one level
// was measured.
var ErrNoAudio = errors.New("audio: the microphone gave no sound")

// Test records from node for d and returns the levels measured, for the
// Settings mic meter. It keeps no audio: samples are folded into the meter
// and dropped, nothing is written anywhere, and the recorder is killed and
// reaped before Test returns. It ends early, with the levels so far, when
// ctx ends.
func Test(ctx context.Context, c Capture, node string, d time.Duration, m *Meter) ([]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	pcm, stop, err := c.Start(ctx, node)
	if err != nil {
		return nil, err
	}
	var levels []float64
	for chunk := range pcm {
		levels = append(levels, m.Push(chunk)...)
	}
	serr := stop()
	if len(levels) == 0 {
		return nil, errors.Join(ErrNoAudio, serr)
	}
	return levels, serr
}
