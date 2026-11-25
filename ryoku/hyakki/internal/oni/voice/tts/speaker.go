package tts

import (
	"context"
	"errors"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/voice/clips"
)

// Options configure a Speaker.
type Options struct {
	// Engines are tried in order for every sentence (see Order and Pick).
	Engines []Engine
	// Open returns the sink speech plays on, for example sound.PipeSink.
	// It is called on the first sentence that plays, never before.
	Open func() (sound.Sink, error)
	// SpeakReplies is the user's setting; off, Say speaks nothing and
	// Prerender renders nothing.
	SpeakReplies bool
	// Cache keeps rendered short sentences; nil renders every time.
	Cache *Cache
	// Report, when set, hears every sentence and the end of every line. It
	// is called from the speaking goroutine and must not block.
	Report func(Spoken)
}

// Spoken reports one sentence, or with Done the end of a line.
type Spoken struct {
	Turn     uint64
	Sentence string
	Engine   string // the engine that rendered it; "" when none could
	Cached   bool
	Samples  int   // at SpeechRateHz
	Err      error // why no engine could render it, or the sink failed
	Done     bool  // the line ended
	Stopped  bool  // with Done: a stop or a newer line cut it short
}

// Speaker says one line at a time, sentence by sentence: the next sentence
// renders while the current one plays, and a stop or a newer turn's line
// fades the playing sentence within the sink's mute ramp and drops the
// rest. A line from an older turn than the last one said is dropped. A
// Speaker runs nothing until Say.
type Speaker struct {
	o    Options
	rate int

	mu      sync.Mutex
	turn    uint64
	gen     uint64
	abort   context.CancelFunc // ends the current line's renders
	playing func()             // fades the sentence playing
	sink    sound.Sink
}

// NewSpeaker returns a speaker; it opens, starts and creates nothing.
func NewSpeaker(o Options) *Speaker { return &Speaker{o: o, rate: SpeechRateHz()} }

func nothing() {}

// Say speaks line for turn, replacing whatever is being said. It returns
// at once; stop ends this line only, as Stop would. With SpeakReplies off,
// no engine, an empty line or a turn older than the last one, nothing is
// spoken.
func (s *Speaker) Say(turn uint64, line string) (stop func()) {
	if !s.o.SpeakReplies || len(s.o.Engines) == 0 {
		return nothing
	}
	sentences := Sentences(Normalize(line))
	if len(sentences) == 0 {
		return nothing
	}
	s.mu.Lock()
	if turn < s.turn {
		s.mu.Unlock()
		return nothing
	}
	s.stopLocked()
	s.turn = turn
	gen := s.gen
	ctx, abort := context.WithCancel(context.Background())
	s.abort = abort
	s.mu.Unlock()
	go s.run(ctx, gen, turn, sentences)
	return func() {
		s.mu.Lock()
		if s.gen == gen {
			s.stopLocked()
		}
		s.mu.Unlock()
	}
}

// Stop fades the sentence playing to silence within the mute ramp and
// drops the rest of the line. It is safe at any time.
func (s *Speaker) Stop() {
	s.mu.Lock()
	s.stopLocked()
	s.mu.Unlock()
}

func (s *Speaker) stopLocked() {
	s.gen++
	if s.abort != nil {
		s.abort()
		s.abort = nil
	}
	if s.playing != nil {
		s.playing()
		s.playing = nil
	}
}

func (s *Speaker) report(sp Spoken) {
	if s.o.Report != nil {
		s.o.Report(sp)
	}
}

func (s *Speaker) run(ctx context.Context, gen, turn uint64, sentences []string) {
	stopped := false
	var until time.Time
	for _, sent := range sentences {
		pcm, engine, cached, err := s.render(ctx, sent)
		if ctx.Err() != nil {
			stopped = true
			break
		}
		if err != nil {
			s.report(Spoken{Turn: turn, Sentence: sent, Err: err})
			continue
		}
		if !waitUntil(ctx, until) {
			stopped = true
			break
		}
		s.mu.Lock()
		if s.gen != gen {
			s.mu.Unlock()
			stopped = true
			break
		}
		if s.sink == nil {
			if s.o.Open == nil {
				err = errors.New("tts: no sink to speak on")
			} else {
				s.sink, err = s.o.Open()
			}
			if err != nil {
				s.sink = nil
				s.mu.Unlock()
				s.report(Spoken{Turn: turn, Sentence: sent, Engine: engine, Err: err})
				break
			}
		}
		cancel, err := s.sink.Play(pcm, s.rate)
		if err != nil {
			s.mu.Unlock()
			s.report(Spoken{Turn: turn, Sentence: sent, Engine: engine, Err: err})
			break
		}
		s.playing = cancel
		s.mu.Unlock()
		until = time.Now().Add(time.Duration(len(pcm)) * time.Second / time.Duration(s.rate))
		s.report(Spoken{Turn: turn, Sentence: sent, Engine: engine, Cached: cached, Samples: len(pcm)})
	}
	if !stopped && !waitUntil(ctx, until) {
		stopped = true
	}
	s.mu.Lock()
	if s.gen == gen {
		s.playing = nil
		if s.abort != nil {
			s.abort()
			s.abort = nil
		}
	}
	s.mu.Unlock()
	s.report(Spoken{Turn: turn, Done: true, Stopped: stopped})
}

// render tries each engine in order, the cache first for each, and
// returns the PCM with the engine that made it.
func (s *Speaker) render(ctx context.Context, line string) ([]float32, string, bool, error) {
	var errs []error
	for _, e := range s.o.Engines {
		if s.o.Cache != nil {
			if pcm, ok := s.o.Cache.Get(e, line); ok {
				return pcm, e.Name(), true, nil
			}
		}
		pcm, rate, err := e.Render(ctx, line)
		if ctx.Err() != nil {
			return nil, "", false, ctx.Err()
		}
		if err == nil && len(pcm) == 0 {
			err = errors.New("tts: " + e.Name() + " rendered nothing")
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pcm = clips.Resample(pcm, rate, s.rate)
		if s.o.Cache != nil {
			_ = s.o.Cache.Put(e, line, pcm)
		}
		return pcm, e.Name(), false, nil
	}
	return nil, "", false, errors.Join(errs...)
}

// Prerender renders lines (acknowledgements, greetings) into the cache so
// they start without waiting on an engine. Without a cache, or with
// SpeakReplies off, it does nothing.
func (s *Speaker) Prerender(ctx context.Context, lines []string) error {
	if s.o.Cache == nil || !s.o.SpeakReplies {
		return nil
	}
	var errs []error
	for _, line := range lines {
		for _, sent := range Sentences(Normalize(line)) {
			if !Cacheable(sent) {
				continue
			}
			if _, _, _, err := s.render(ctx, sent); err != nil {
				errs = append(errs, err)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return errors.Join(errs...)
}

// waitUntil waits for t; false when ctx ended first.
func waitUntil(ctx context.Context, t time.Time) bool {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
