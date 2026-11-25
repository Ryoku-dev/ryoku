// Package vad cuts 16 kHz signed 16-bit mono capture into speech segments
// with the energy rules in the spec's voice.vad and voice.press numbers. It
// is pure: no device, no file, no goroutine; the caller pushes PCM as it
// arrives and acts on the returned events.
package vad

import (
	"errors"
	"fmt"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Mode is what opened the mic; it sets how long a session may last.
type Mode uint8

const (
	// Hold is the talk chord held down: a press shorter than the tap drop
	// yields nothing, and the hold is cut at voice.press.maxHoldS.
	Hold Mode = iota + 1
	// OpenMic is cut at voice.press.openMicS.
	OpenMic
	// Wake is a session the wake phrase opened, cut at voice.wake.sessionS.
	Wake
)

// Config holds the segmenter's numbers; FromSpec fills it from the spec.
// Every duration is a whole number of Frames.
type Config struct {
	SampleRateHz int
	Frame        time.Duration
	SpeechRMS    float64 // a frame whose RMS reaches this is speech
	Pause        time.Duration
	MinSpeech    time.Duration
	FirstPartial time.Duration
	PartialEvery time.Duration
	MaxSegment   time.Duration
	PreRoll      time.Duration
	TapDrop      time.Duration // 0: no tap rule
	MaxSession   time.Duration
}

func ms(v float64) time.Duration { return time.Duration(v * float64(time.Millisecond)) }

// FromSpec returns the segmenter numbers for a mode.
func FromSpec(sp *spec.OniSpec, m Mode) Config {
	v := sp.Voice
	c := Config{
		SampleRateHz: v.Vad.SampleRateHz,
		Frame:        ms(v.Vad.FrameMs),
		SpeechRMS:    v.Vad.SpeechRms,
		Pause:        ms(v.Vad.PauseMs),
		MinSpeech:    ms(v.Vad.MinSpeechMs),
		FirstPartial: ms(v.Vad.FirstPartialMs),
		PartialEvery: ms(v.Vad.PartialEveryMs),
		MaxSegment:   ms(v.Vad.MaxSegmentS * 1000),
		PreRoll:      ms(v.Vad.PreRollMs),
	}
	switch m {
	case Hold:
		c.TapDrop = ms(v.Press.TapDropMs)
		c.MaxSession = ms(v.Press.MaxHoldS * 1000)
	case OpenMic:
		c.MaxSession = ms(v.Press.OpenMicS * 1000)
	case Wake:
		c.MaxSession = ms(v.Wake.SessionS * 1000)
	}
	return c
}

// Kind names an event.
type Kind uint8

const (
	// SpeechStart: a segment holds MinSpeech of speech frames.
	SpeechStart Kind = iota + 1
	// Partial: decode the segment so far for a partial transcript.
	Partial
	// Final: decode the whole segment; Reason says what ended it.
	Final
	// PauseEnd: the speaker paused for Pause after the segment.
	PauseEnd
	// Cut: the session reached MaxSession; the segmenter takes no more PCM.
	Cut
)

func (k Kind) String() string {
	switch k {
	case SpeechStart:
		return "speech-start"
	case Partial:
		return "partial"
	case Final:
		return "final"
	case PauseEnd:
		return "pause-end"
	case Cut:
		return "cut"
	}
	return fmt.Sprintf("kind(%d)", uint8(k))
}

// Reason is what ended a segment (Final only).
type Reason uint8

const (
	ReasonPause Reason = iota + 1
	ReasonMaxSegment
	ReasonEnd
	ReasonCut
)

// Event is one segmenter decision.
type Event struct {
	Kind Kind
	// At is the audio time the event was decided, counted from the first
	// pushed sample.
	At time.Duration
	// Start is the audio time of the segment's first speech frame.
	Start time.Duration
	// PCM (Partial and Final) is the pre-roll plus the segment so far; a
	// Final ends at the last speech frame. It is shared and read-only.
	PCM    []int16
	Reason Reason
}

// Segmenter turns pushed PCM into events. It is not safe for concurrent use.
type Segmenter struct {
	c     Config
	now   func() time.Time
	began time.Time
	frame int // samples per frame

	pauseF, minSpeechF, firstPartialF, partialEveryF, maxSegmentF int64

	carry  []int16 // an incomplete frame
	frames int64   // frames analysed

	ring       []int16 // the last PreRoll of audio
	ringPos    int
	ringFilled int

	open        bool // a segment has a speech frame
	started     bool // ... and MinSpeech of them
	start       int64
	speech      int64
	lastSpeech  int64
	buf         []int16
	nextPartial int64
	partialAt   int64 // lastSpeech when the previous partial went out

	pending []Event // held back while a press is still a tap
	out     []Event
	closed  bool
}

// New returns a segmenter for one press or session. now is the wall clock
// a press is timed by (nil times it by audio alone); the session length is
// the longer of the audio pushed and the time since New.
func New(c Config, now func() time.Time) (*Segmenter, error) {
	if c.SampleRateHz <= 0 || c.Frame <= 0 {
		return nil, errors.New("vad: sample rate and frame must be positive")
	}
	samples := int64(c.SampleRateHz) * int64(c.Frame)
	if samples%int64(time.Second) != 0 {
		return nil, errors.New("vad: frame is not a whole number of samples")
	}
	if !(c.SpeechRMS > 0) {
		return nil, errors.New("vad: speech threshold must be positive")
	}
	for _, d := range []struct {
		name     string
		v        time.Duration
		positive bool
	}{
		{"pause", c.Pause, true}, {"min speech", c.MinSpeech, true}, {"first partial", c.FirstPartial, true},
		{"partial cadence", c.PartialEvery, true}, {"max segment", c.MaxSegment, true},
		{"max session", c.MaxSession, true}, {"pre-roll", c.PreRoll, false}, {"tap drop", c.TapDrop, false},
	} {
		if d.v < 0 || (d.positive && d.v == 0) || d.v%c.Frame != 0 {
			return nil, fmt.Errorf("vad: %s %v is not a positive whole number of %v frames", d.name, d.v, c.Frame)
		}
	}
	s := &Segmenter{
		c:             c,
		now:           now,
		frame:         int(samples / int64(time.Second)),
		pauseF:        int64(c.Pause / c.Frame),
		minSpeechF:    int64(c.MinSpeech / c.Frame),
		firstPartialF: int64(c.FirstPartial / c.Frame),
		partialEveryF: int64(c.PartialEvery / c.Frame),
		maxSegmentF:   int64(c.MaxSegment / c.Frame),
	}
	s.ring = make([]int16, int64(c.PreRoll/c.Frame)*int64(s.frame))
	s.carry = make([]int16, 0, s.frame)
	if now != nil {
		s.began = now()
	}
	return s, nil
}

func (s *Segmenter) audio() time.Duration { return time.Duration(s.frames) * s.c.Frame }

func (s *Segmenter) held() time.Duration {
	h := s.audio()
	if s.now != nil {
		h = max(h, s.now().Sub(s.began))
	}
	return h
}

// tapping reports whether the press is still short enough to be dropped.
func (s *Segmenter) tapping() bool { return s.c.TapDrop > 0 && s.held() < s.c.TapDrop }

func (s *Segmenter) emit(e Event) {
	if s.tapping() {
		s.pending = append(s.pending, e)
		return
	}
	s.flush()
	s.out = append(s.out, e)
}

func (s *Segmenter) flush() {
	if len(s.pending) > 0 {
		s.out = append(s.out, s.pending...)
		s.pending = nil
	}
}

func (s *Segmenter) take() []Event {
	if !s.tapping() {
		s.flush()
	}
	out := s.out
	s.out = nil
	return out
}

// Push analyses pcm (16 kHz s16 mono, any length) and returns the events it
// decided, oldest first. After a Cut or End it returns nil.
func (s *Segmenter) Push(pcm []int16) []Event {
	if s.closed {
		return nil
	}
	s.cutIfDue()
	for len(pcm) > 0 && !s.closed {
		if len(s.carry) > 0 || len(pcm) < s.frame {
			n := min(s.frame-len(s.carry), len(pcm))
			s.carry = append(s.carry, pcm[:n]...)
			pcm = pcm[n:]
			if len(s.carry) == s.frame {
				s.step(s.carry)
				s.carry = s.carry[:0]
			}
			continue
		}
		s.step(pcm[:s.frame])
		pcm = pcm[s.frame:]
	}
	return s.take()
}

// End closes the press or session: a tap yields nothing, otherwise a
// started segment ends in a Final. Later calls return nil.
func (s *Segmenter) End() []Event {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.tapping() {
		s.pending, s.out = nil, nil
		return nil
	}
	if s.started {
		s.final(ReasonEnd, true)
	}
	s.reset()
	return s.take()
}

func (s *Segmenter) at(frames int64) time.Duration { return time.Duration(frames) * s.c.Frame }

func voiced(f []int16, rms float64) bool {
	var sum int64
	for _, v := range f {
		sum += int64(v) * int64(v)
	}
	return float64(sum) >= rms*rms*float64(len(f))
}

func (s *Segmenter) step(f []int16) {
	i := s.frames
	s.frames++
	end := s.frames
	speech := voiced(f, s.c.SpeechRMS)
	if !s.open && speech {
		s.open, s.start, s.speech, s.partialAt = true, i, 0, -1
		s.buf = make([]int16, 0, s.ringFilled+s.frame*int(s.firstPartialF+s.partialEveryF))
		s.buf = append(s.buf, s.ring[s.ringPos:s.ringFilled]...)
		s.buf = append(s.buf, s.ring[:min(s.ringPos, s.ringFilled)]...)
	}
	s.remember(f)
	if s.open {
		s.buf = append(s.buf, f...)
		if speech {
			s.speech++
			s.lastSpeech = i
		}
		silence := i - s.lastSpeech
		switch {
		case !s.started && s.speech >= s.minSpeechF:
			s.started = true
			s.nextPartial = s.start + s.firstPartialF
			s.emit(Event{Kind: SpeechStart, At: s.at(end), Start: s.at(s.start)})
		case !s.started && silence >= s.pauseF:
			s.reset()
		}
		if s.started {
			switch {
			case silence >= s.pauseF:
				s.final(ReasonPause, true)
				s.emit(Event{Kind: PauseEnd, At: s.at(end), Start: s.at(s.start)})
				s.reset()
			case end-s.start >= s.maxSegmentF:
				s.final(ReasonMaxSegment, false)
				s.reset()
			case end >= s.nextPartial && s.lastSpeech > s.partialAt:
				s.partialAt = s.lastSpeech
				for s.nextPartial <= end {
					s.nextPartial += s.partialEveryF
				}
				s.emit(Event{Kind: Partial, At: s.at(end), Start: s.at(s.start), PCM: s.buf[:len(s.buf):len(s.buf)]})
			}
		}
	}
	s.cutIfDue()
}

// remember keeps the last PreRoll of audio in the ring.
func (s *Segmenter) remember(f []int16) {
	if len(s.ring) == 0 {
		return
	}
	n := copy(s.ring[s.ringPos:], f)
	if n < len(f) {
		copy(s.ring, f[n:])
	}
	s.ringPos = (s.ringPos + len(f)) % len(s.ring)
	s.ringFilled = min(s.ringFilled+len(f), len(s.ring))
}

// final emits the segment; trim drops the silence after its last speech
// frame.
func (s *Segmenter) final(r Reason, trim bool) {
	pcm := s.buf
	if trim {
		pcm = pcm[:len(pcm)-int(s.frames-1-s.lastSpeech)*s.frame]
	}
	s.emit(Event{Kind: Final, At: s.at(s.frames), Start: s.at(s.start), PCM: pcm[:len(pcm):len(pcm)], Reason: r})
}

func (s *Segmenter) reset() {
	s.open, s.started, s.buf = false, false, nil
}

func (s *Segmenter) cutIfDue() {
	if s.closed || s.held() < s.c.MaxSession {
		return
	}
	if s.started {
		s.final(ReasonCut, false)
	}
	s.reset()
	s.emit(Event{Kind: Cut, At: s.audio()})
	s.closed = true
	s.carry = nil
}
