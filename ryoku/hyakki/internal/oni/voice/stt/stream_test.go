package stt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// scripted is a Transcriber whose decodes the test releases.
type scripted struct {
	mu      sync.Mutex
	calls   []Hint
	lens    []int
	gate    chan struct{}
	started chan struct{}
}

func (s *scripted) Info() Info { return Info{Engine: "scripted"} }
func (s *scripted) Close() error {
	return nil
}

func (s *scripted) Transcribe(ctx context.Context, pcm []int16, h Hint) (Result, error) {
	s.mu.Lock()
	s.calls = append(s.calls, h)
	s.lens = append(s.lens, len(pcm))
	gate, started := s.gate, s.started
	s.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	return Result{Text: "words", Pass: h.Pass}, nil
}

func TestStreamPartialsSkipWhileBusyAndFinalOvertakes(t *testing.T) {
	tr := &scripted{gate: make(chan struct{}), started: make(chan struct{}, 4)}
	l := specLimits(t)
	s := NewStream(tr, l, "en", []string{"Mara"})
	pcm := speech(2 * time.Second)

	type out struct {
		ok  bool
		err error
	}
	first := make(chan out, 1)
	go func() {
		_, ok, err := s.Partial(context.Background(), pcm[:SampleRate])
		first <- out{ok, err}
	}()
	<-tr.started
	if _, ok, err := s.Partial(context.Background(), pcm[:SampleRate+4800]); ok || err != nil {
		t.Fatalf("a second partial ran while one was decoding: ok=%v err=%v", ok, err)
	}
	// the final abandons the partial in flight
	tr.mu.Lock()
	tr.gate = nil
	tr.mu.Unlock()
	res, err := s.Final(context.Background(), pcm)
	if err != nil || res.Reused || res.Pass != Final {
		t.Fatalf("final = %+v, %v", res, err)
	}
	if got := <-first; got.ok || got.err != nil {
		t.Fatalf("the overtaken partial reported ok=%v err=%v", got.ok, got.err)
	}
	if tr.calls[0].Pass != Partial || tr.calls[1].Pass != Final || len(tr.calls) != 2 {
		t.Fatalf("calls = %+v", tr.calls)
	}
	if !slices.Equal(tr.calls[1].Vocab, []string{"Mara"}) || tr.calls[1].Language != "en" {
		t.Fatalf("hint = %+v", tr.calls[1])
	}
}

func TestStreamFinalReusesTheSamePartial(t *testing.T) {
	tr := &scripted{}
	s := NewStream(tr, specLimits(t), "", nil)
	pcm := speech(time.Second)
	if _, ok, err := s.Partial(context.Background(), pcm); !ok || err != nil {
		t.Fatalf("partial ok=%v err=%v", ok, err)
	}
	res, err := s.Final(context.Background(), pcm)
	if err != nil || !res.Reused || res.Text != "words" {
		t.Fatalf("final = %+v, %v", res, err)
	}
	if len(tr.calls) != 1 {
		t.Fatalf("decodes = %d, want the final to reuse the partial", len(tr.calls))
	}
	// different audio is decoded again; the stream is reset after a final
	if _, ok, _ := s.Partial(context.Background(), pcm); !ok {
		t.Fatal("partial skipped")
	}
	longer := speech(1500 * time.Millisecond)
	if res, _ := s.Final(context.Background(), longer); res.Reused {
		t.Fatal("a final reused a partial of other audio")
	}
}

func TestStreamPartialReDecodesTheTrailingWindow(t *testing.T) {
	tr := &scripted{}
	l := specLimits(t)
	s := NewStream(tr, l, "", nil)
	long := speech(12 * time.Second)
	if _, ok, err := s.Partial(context.Background(), long); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if want := int(l.Window.Seconds() * SampleRate); tr.lens[0] != want {
		t.Fatalf("partial decoded %d samples, want the %v window (%d)", tr.lens[0], l.Window, want)
	}
}

func TestLimitsVocabularyAndAudioCtx(t *testing.T) {
	l := specLimits(t)
	if l != (Limits{Partial: 1200 * time.Millisecond, Final: 3 * time.Second, IdleUnload: 600 * time.Second, Window: 9 * time.Second}) {
		t.Fatalf("limits = %+v", l)
	}
	v := Vocabulary([]string{" Mara\x07 ", "MARA", "", "Oni", "Ask Oni now", "Onigiri", strings.Repeat("x", 60), "Lee"}, "oni")
	if !slices.Equal(v, []string{"Mara", "Onigiri", "Lee"}) {
		t.Fatalf("vocabulary = %q", v)
	}
	for _, c := range []struct {
		d    time.Duration
		want int
	}{{0, 256}, {time.Second, 256}, {9 * time.Second, 512}, {30 * time.Second, 1500}} {
		if got := AudioCtx(c.d); got != c.want {
			t.Fatalf("AudioCtx(%v) = %d, want %d", c.d, got, c.want)
		}
	}
}

func TestDownloadNeedsConsent(t *testing.T) {
	dir := t.TempDir()
	m, ok := FindModel("whisper-base.en-q5_1")
	if !ok {
		t.Fatal("base.en is not in the catalogue")
	}
	if _, err := Download(context.Background(), m, dir, false, nil); !errors.Is(err, ErrNoConsent) {
		t.Fatalf("err = %v, want ErrNoConsent", err)
	}
	if _, err := os.Stat(filepath.Join(dir, m.File)); !os.IsNotExist(err) {
		t.Fatal("a file appeared without consent")
	}
	bad := m
	bad.File = "../escape.bin"
	if _, err := Download(context.Background(), bad, dir, true, nil); !errors.Is(err, ErrModel) {
		t.Fatalf("escaping file name: err = %v", err)
	}
	for _, m := range Catalogue {
		if m.Size <= 0 || !sha256Hex.MatchString(m.SHA256) || m.License == "" {
			t.Fatalf("catalogue entry %s is not pinned", m.ID)
		}
	}
}

func TestDefaultsAndInstalledModels(t *testing.T) {
	for id, engine := range map[string]string{DefaultModel: EngineDictate, FallbackModel: EngineWhisper, FallbackVAD: EngineVAD} {
		m, ok := FindModel(id)
		if !ok || m.Engine != engine {
			t.Fatalf("%s: found=%v engine=%q, want %q", id, ok, m.Engine, engine)
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m, _ := FindModel(FallbackModel)
	if Installed(ModelDir(), m) {
		t.Fatal("an empty model directory reported the model installed")
	}
	if err := os.MkdirAll(ModelDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ModelDir(), m.File)
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Installed(ModelDir(), m) {
		t.Fatal("a file of the wrong size counted as installed")
	}
	if err := os.Truncate(path, m.Size); err != nil {
		t.Fatal(err)
	}
	if got, ok := Locate(m); !ok || got != path {
		t.Fatalf("Locate = %q, %v", got, ok)
	}
}
