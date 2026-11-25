package sound

import (
	"encoding/binary"
	"io"
	"math"
	"time"
)

const (
	// leadMs bounds how far writes run ahead of the play position, so a
	// cancel can still shape what is heard.
	leadMs = 10
	// tickMs is how often a stream tops its writer up.
	tickMs = 5
)

// stream paces a buffer into a writer in real time, at most lead samples
// ahead of the play position. A cancel fades the samples not yet written so
// the sound reaches zero ramp samples after the cancel's play position, and
// nothing is written past that. Positions are sample counts since the
// stream started, so the pacing is testable without a clock.
type stream struct {
	pcm     []float32
	lead    int
	ramp    int
	written int
	end     int
	cut     int // play position at cancel, -1 while uncancelled
	from    int // write position at cancel
	stop    int // cut + ramp: where the fade reaches zero
}

func newStream(pcm []float32, rateHz int, ramp time.Duration) *stream {
	return &stream{
		pcm:  pcm,
		lead: samplesOf(millis(leadMs), rateHz),
		ramp: samplesOf(ramp, rateHz),
		end:  len(pcm),
		cut:  -1,
	}
}

func samplesOf(d time.Duration, rateHz int) int {
	return int(math.Round(d.Seconds() * float64(rateHz)))
}

// pump returns the samples that may be written once played samples have
// been heard, faded if the stream was cancelled.
func (s *stream) pump(played int) []float32 {
	want := min(s.end, max(played, 0)+s.lead)
	if want <= s.written {
		return nil
	}
	out := append([]float32(nil), s.pcm[s.written:want]...)
	if s.cut >= 0 {
		span := float64(s.stop - s.from)
		for i := range out {
			out[i] *= float32(float64(s.stop-(s.written+i)) / span)
		}
	}
	s.written = want
	return out
}

// cancel fades the stream out from the play position played. A second
// cancel changes nothing.
func (s *stream) cancel(played int) {
	if s.cut >= 0 {
		return
	}
	s.cut, s.from = max(played, 0), s.written
	s.stop = max(s.cut+s.ramp, s.from)
	s.end = min(s.end, s.stop)
}

func (s *stream) done() bool { return s.written >= s.end }

// run writes the stream to w in real time until it is done or stop closes
// and the fade is written. It returns the cancel's play position, or -1.
func (s *stream) run(w io.Writer, rateHz int, stop <-chan struct{}) (cut int, err error) {
	start := time.Now()
	played := func() int { return samplesOf(time.Since(start), rateHz) }
	tick := time.NewTicker(millis(tickMs))
	defer tick.Stop()
	var buf []byte
	for {
		if chunk := s.pump(played()); len(chunk) > 0 {
			buf = encode(buf[:0], chunk)
			if _, err := w.Write(buf); err != nil {
				return s.cut, err
			}
		}
		if s.done() {
			return s.cut, nil
		}
		select {
		case <-stop:
			s.cancel(played())
			stop = nil
		case <-tick.C:
		}
	}
}

// encode appends samples as native-endian float32, the format both
// players are told to expect.
func encode(dst []byte, pcm []float32) []byte {
	for _, v := range pcm {
		dst = binary.NativeEndian.AppendUint32(dst, math.Float32bits(v))
	}
	return dst
}
