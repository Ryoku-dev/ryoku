package stt

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"
)

// Stream turns one growing speech segment into partials and a final.
// Each partial re-decodes the trailing Limits.Window of the buffer so far
// (the segmenter's Partial events carry it); a partial asked for while one
// is still decoding is skipped rather than queued, so partials never pile
// up behind a slow engine. Final abandons a partial in flight and reuses
// the last partial's text when that partial decoded exactly the same audio.
type Stream struct {
	t      Transcriber
	hint   Hint
	window int

	mu     sync.Mutex
	gen    uint64
	cancel context.CancelFunc
	last   decoded
}

type decoded struct {
	ok   bool
	sum  [sha256.Size]byte
	text string
}

// NewStream starts a segment. vocab is the vocabulary hint (Vocabulary
// is applied by the adapter); language "" is the adapter's.
func NewStream(t Transcriber, l Limits, language string, vocab []string) *Stream {
	return &Stream{t: t, hint: Hint{Language: language, Vocab: vocab}, window: int(l.Window.Seconds() * SampleRate)}
}

func digest(pcm []int16) [sha256.Size]byte {
	b := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	return sha256.Sum256(b)
}

// Partial decodes the trailing window of pcm under the partial budget.
// ok is false when the partial was skipped (another is decoding) or
// overtaken by Final; its error is then nil.
func (s *Stream) Partial(ctx context.Context, pcm []int16) (res Result, ok bool, err error) {
	if s.window > 0 && len(pcm) > s.window {
		pcm = pcm[len(pcm)-s.window:]
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return Result{}, false, nil
	}
	pctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	gen := s.gen
	s.mu.Unlock()
	defer cancel()

	h := s.hint
	h.Pass = Partial
	res, err = s.t.Transcribe(pctx, pcm, h)

	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen {
		return Result{}, false, nil
	}
	s.cancel = nil
	if err != nil {
		return Result{}, false, err
	}
	s.last = decoded{ok: true, sum: digest(pcm), text: res.Text}
	return res, true, nil
}

// Final decodes the whole segment under the final budget, or reuses the
// partial that decoded exactly this audio. The stream is then ready for
// the next segment.
func (s *Stream) Final(ctx context.Context, pcm []int16) (Result, error) {
	s.mu.Lock()
	s.gen++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	last := s.last
	s.last = decoded{}
	s.mu.Unlock()

	if last.ok && len(pcm) > 0 && last.sum == digest(pcm) {
		return Result{Text: last.text, Pass: Final, Reused: true}, nil
	}
	h := s.hint
	h.Pass = Final
	return s.t.Transcribe(ctx, pcm, h)
}
