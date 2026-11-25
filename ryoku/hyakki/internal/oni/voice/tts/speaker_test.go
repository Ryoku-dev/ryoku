package tts

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/clips"
)

// fakeEngine renders every sentence as a constant level lasting secs, and
// fails the sentences in fail.
type fakeEngine struct {
	name  string
	level float32
	secs  float64
	fail  map[string]error

	mu    sync.Mutex
	lines []string
}

func (f *fakeEngine) Name() string  { return f.name }
func (f *fakeEngine) Voice() string { return "fake|" + f.name }

func (f *fakeEngine) Render(ctx context.Context, line string) ([]float32, int, error) {
	f.mu.Lock()
	f.lines = append(f.lines, line)
	f.mu.Unlock()
	if err := f.fail[line]; err != nil {
		return nil, 0, err
	}
	secs := f.secs
	if secs == 0 {
		secs = 0.05
	}
	pcm := make([]float32, int(secs*float64(SpeechRateHz())))
	for i := range pcm {
		pcm[i] = f.level
	}
	return pcm, SpeechRateHz(), ctx.Err()
}

func (f *fakeEngine) rendered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.lines)
}

// reports collects a speaker's reports.
type reports struct {
	ch chan Spoken
}

func newReports() *reports { return &reports{ch: make(chan Spoken, 64)} }

func (r *reports) fn(s Spoken) { r.ch <- s }

func (r *reports) next(t *testing.T) Spoken {
	t.Helper()
	select {
	case s := <-r.ch:
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("no report")
	}
	return Spoken{}
}

// until returns reports up to and including the line's end.
func (r *reports) until(t *testing.T) []Spoken {
	t.Helper()
	var out []Spoken
	for {
		s := r.next(t)
		out = append(out, s)
		if s.Done {
			return out
		}
	}
}

func refuseSink(t *testing.T) func() (sound.Sink, error) {
	return func() (sound.Sink, error) {
		t.Error("a sink was opened")
		return nil, errors.New("no sink")
	}
}

// records reads a FileSink capture, waiting for want lines.
func records(t *testing.T, path string, want int) []sound.FileRecord {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var out []sound.FileRecord
		if f, err := os.Open(path); err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(nil, 64<<20)
			for sc.Scan() {
				var r sound.FileRecord
				if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				out = append(out, r)
			}
			f.Close()
		}
		if len(out) >= want || time.Now().After(deadline) {
			return out
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func samples(r sound.FileRecord) []float32 {
	out := make([]float32, len(r.PCM)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.NativeEndian.Uint32(r.PCM[i*4:]))
	}
	return out
}

func TestSpeakRepliesOffSpeaksNothing(t *testing.T) {
	e := &fakeEngine{name: System, level: 0.5}
	cache := NewCache(t.TempDir(), 0, 0)
	before := runtime.NumGoroutine()
	s := NewSpeaker(Options{Engines: []Engine{e}, Open: refuseSink(t), Cache: cache})
	stop := s.Say(1, "Hello there. Anyone home?")
	stop()
	s.Stop()
	if err := s.Prerender(context.Background(), []string{"On it."}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := e.rendered(); len(got) > 0 {
		t.Fatalf("rendered %q with speakReplies off", got)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("%d goroutines started with speakReplies off", n-before)
	}
	if entries, _ := os.ReadDir(cache.dir); len(entries) > 0 {
		t.Fatalf("the cache holds %d files", len(entries))
	}
}

// Stop fades the sentence playing to silence within the mute ramp plus the
// stream's pacing lead, measured on the samples the sink wrote, and the
// sentences after it never play.
func TestStopFadesToSilenceWithinTheRamp(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	rate := SpeechRateHz()
	g := sp.Sound.Gating
	ramp := int(math.Round(min(g.QuietMs, g.MuteRampMs) / 1000 * float64(rate)))
	const leadMs = 10 // sound's stream pacing lead
	lead := leadMs * rate / 1000

	capture := filepath.Join(t.TempDir(), "speech.jsonl")
	e := &fakeEngine{name: System, level: 0.5, secs: 2}
	r := newReports()
	s := NewSpeaker(Options{
		Engines: []Engine{e}, SpeakReplies: true, Report: r.fn,
		Open: func() (sound.Sink, error) { return sound.FileSink(capture), nil },
	})
	s.Say(1, "This sentence plays first. This one never plays. Nor this.")
	first := r.next(t)
	if first.Err != nil || first.Engine != System || first.Samples != 2*rate {
		t.Fatalf("first sentence: %+v", first)
	}
	time.Sleep(300 * time.Millisecond)
	s.Stop()
	stopped := time.Now()
	end := r.until(t)
	if last := end[len(end)-1]; !last.Stopped || len(end) != 1 {
		t.Fatalf("after Stop: %+v", end)
	}
	recs := records(t, capture, 1)
	time.Sleep(200 * time.Millisecond)
	if recs = records(t, capture, 1); len(recs) != 1 {
		t.Fatalf("%d sounds played, want only the first sentence", len(recs))
	}
	rec := recs[0]
	pcm := samples(rec)
	if rec.RateHz != rate || rec.CutAt < 0 || len(pcm) != rec.Written {
		t.Fatalf("record: rate %d cut %d written %d pcm %d", rec.RateHz, rec.CutAt, rec.Written, len(pcm))
	}
	if heard := time.Duration(rec.CutAt) * time.Second / time.Duration(rate); heard < 250*time.Millisecond ||
		heard > stopped.Sub(rec.Start)+20*time.Millisecond {
		t.Fatalf("cut at %v of play, Stop came %v after the start", heard, stopped.Sub(rec.Start))
	}
	if after := rec.Written - rec.CutAt; after > ramp+lead {
		t.Fatalf("%d samples written after the cut, want at most ramp %d + lead %d", after, ramp, lead)
	}
	if rec.Written >= 2*rate {
		t.Fatal("the whole sentence was written")
	}
	full := 0
	for full < len(pcm) && pcm[full] == 0.5 {
		full++
	}
	if full < rec.CutAt-lead {
		t.Fatalf("the level dropped at sample %d, before the cut at %d", full, rec.CutAt)
	}
	for i := full + 1; i < len(pcm); i++ {
		if pcm[i] > pcm[i-1] || pcm[i] < 0 {
			t.Fatalf("the fade rises at sample %d: %v after %v", i, pcm[i], pcm[i-1])
		}
	}
	if tail := pcm[len(pcm)-1]; tail > 0.01 {
		t.Fatalf("the last sample is %v, not silence", tail)
	}
	if got := e.rendered(); len(got) > 2 {
		t.Fatalf("rendered %q after the stop", got)
	}
}

func TestNewerTurnReplacesAndOlderTurnIsDropped(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "speech.jsonl")
	e := &fakeEngine{name: System, level: 0.25, secs: 1}
	r := newReports()
	s := NewSpeaker(Options{
		Engines: []Engine{e}, SpeakReplies: true, Report: r.fn,
		Open: func() (sound.Sink, error) { return sound.FileSink(capture), nil },
	})
	s.Say(2, "Turn two speaks.")
	if got := r.next(t); got.Turn != 2 || got.Err != nil {
		t.Fatalf("turn two: %+v", got)
	}
	stale := s.Say(1, "Turn one is stale.")
	stale()
	s.Say(3, "Turn three takes over.")
	// The two lines' goroutines report concurrently: sort by turn.
	byTurn := map[uint64][]Spoken{}
	for done := 0; done < 2; {
		got := r.next(t)
		byTurn[got.Turn] = append(byTurn[got.Turn], got)
		if got.Done {
			done++
		}
	}
	if two := byTurn[2]; len(two) != 1 || !two[0].Done || !two[0].Stopped {
		t.Fatalf("turn two was not cut: %+v", two)
	}
	if three := byTurn[3]; len(three) != 2 || three[0].Sentence != "Turn three takes over." || three[0].Err != nil ||
		!three[1].Done || three[1].Stopped {
		t.Fatalf("turn three: %+v", three)
	}
	if len(byTurn) != 2 {
		t.Fatalf("reports for turns %v", byTurn)
	}
	if lines := e.rendered(); slices.Contains(lines, "Turn one is stale.") {
		t.Fatalf("a stale turn rendered: %q", lines)
	}
	recs := records(t, capture, 2)
	if len(recs) != 2 || recs[0].CutAt < 0 || recs[1].CutAt >= 0 || recs[1].Written != recs[1].Samples {
		t.Fatalf("records: %d, first cut %d", len(recs), recs[0].CutAt)
	}
}

// writePack writes a clip pack that says only "on it".
func writePack(t *testing.T) *clips.Pack {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"manifest.json": []byte(`{"version":1,"clips":{"on-it":"on-it.wav"}}`),
		"on-it.wav":     toneWAV(16000, 0.3),
		"LICENSE":       []byte("CC0-1.0\n"),
		"PROVENANCE.md": []byte("Tones generated by this test.\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := clips.Open(os.DirFS(dir), ".")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The profile's engine speaks what it can; a sentence it cannot say falls
// to the next engine, and every report names the engine used.
func TestEnginesFallBackInOrder(t *testing.T) {
	sys := &fakeEngine{name: System, level: 0.1}
	kokoro := &fakeEngine{name: Kokoro, fail: map[string]error{"Open the browser.": errors.New("model failed")}}
	have := map[string]Engine{Clips: NewClips(writePack(t), "test"), Kokoro: kokoro, System: sys}
	r := newReports()
	var sinks []float32
	var mu sync.Mutex
	s := NewSpeaker(Options{
		Engines: Pick(Order(Kokoro, Clips), have), SpeakReplies: true, Report: r.fn,
		Open: func() (sound.Sink, error) {
			return sinkFunc(func(pcm []float32, _ int) (func(), error) {
				mu.Lock()
				sinks = append(sinks, pcm[0])
				mu.Unlock()
				return func() {}, nil
			}), nil
		},
	})
	kokoro.fail["On it."] = errors.New("model failed")
	s.Say(1, "On it. Open the browser.")
	got := r.until(t)
	if len(got) != 3 || got[0].Engine != Clips || got[1].Engine != System || got[2].Stopped {
		t.Fatalf("reports: %+v", got)
	}
	if got[0].Sentence != "On it." || got[0].Samples != int(0.3*float64(SpeechRateHz())) {
		t.Fatalf("clip sentence: %+v", got[0])
	}
	if !slices.Equal(kokoro.rendered(), []string{"On it.", "Open the browser."}) {
		t.Fatalf("kokoro was not tried first: %q", kokoro.rendered())
	}
	if !slices.Equal(sys.rendered(), []string{"Open the browser."}) {
		t.Fatalf("system rendered %q", sys.rendered())
	}

	none := &fakeEngine{name: System, fail: map[string]error{"Nothing works.": errors.New("broken")}}
	r = newReports()
	s = NewSpeaker(Options{Engines: []Engine{none}, SpeakReplies: true, Report: r.fn, Open: refuseSink(t)})
	s.Say(1, "Nothing works.")
	got = r.until(t)
	if len(got) != 2 || got[0].Err == nil || got[0].Engine != "" || got[1].Stopped {
		t.Fatalf("no engine: %+v", got)
	}
}

type sinkFunc func(pcm []float32, rateHz int) (func(), error)

func (f sinkFunc) Play(pcm []float32, rateHz int) (func(), error) { return f(pcm, rateHz) }
