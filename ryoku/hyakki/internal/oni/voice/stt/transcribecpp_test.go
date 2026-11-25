package stt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu    sync.Mutex
	argvs [][]string
	// run sees the argv while the WAV still exists.
	run func(ctx context.Context, argv []string) ([]byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, argv []string) ([]byte, error) {
	r.mu.Lock()
	r.argvs = append(r.argvs, append([]string(nil), argv...))
	r.mu.Unlock()
	return r.run(ctx, argv)
}

func newTestDictate(t *testing.T, r Runner, l Limits) (*Dictate, string) {
	t.Helper()
	bin, model, sum := fixture(t, "ryoku-dictate")
	scratch := t.TempDir()
	d, err := NewDictate(DictateConfig{Binary: bin, Model: model, SHA256: sum, License: "CC-BY-4.0", Language: "en", Limits: l, ScratchDir: scratch, Runner: r})
	if err != nil {
		t.Fatal(err)
	}
	return d, scratch
}

func TestDictateUsesOnlyTheFileInterface(t *testing.T) {
	var wavMode os.FileMode
	var wavSize int64
	r := &fakeRunner{run: func(ctx context.Context, argv []string) ([]byte, error) {
		info, err := os.Stat(argv[2])
		if err != nil {
			return nil, err
		}
		wavMode, wavSize = info.Mode().Perm(), info.Size()
		return []byte("make that thursday\n"), nil
	}}
	d, scratch := newTestDictate(t, r, specLimits(t))
	pcm := speech(time.Second)
	res, err := d.Transcribe(context.Background(), pcm, Hint{Vocab: []string{"Mara"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "make that thursday" {
		t.Fatalf("text = %q", res.Text)
	}
	argv := r.argvs[0]
	want := []string{d.bin, "--selftest", argv[2], "--model", d.cfg.Model, "--device", "cpu", "--language", "en"}
	if !slices.Equal(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	if filepath.Dir(filepath.Dir(argv[2])) != scratch || wavMode != 0o600 || wavSize != 44+2*int64(len(pcm)) {
		t.Fatalf("wav %s mode %v size %d", argv[2], wavMode, wavSize)
	}
	if left, _ := os.ReadDir(scratch); len(left) != 0 {
		t.Fatalf("speech left on disk: %v", left)
	}
}

func TestDictateBudgetAndCancelRemoveTheFile(t *testing.T) {
	l := specLimits(t)
	l.Final = 80 * time.Millisecond
	r := &fakeRunner{run: func(ctx context.Context, argv []string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	d, scratch := newTestDictate(t, r, l)
	began := time.Now()
	if _, err := d.Transcribe(context.Background(), speech(time.Second), Hint{}); !errors.Is(err, ErrBudget) {
		t.Fatalf("err = %v, want ErrBudget", err)
	}
	if took := time.Since(began); took < l.Final || took > l.Final+time.Second {
		t.Fatalf("gave up after %v, budget %v", took, l.Final)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	if _, err := d.Transcribe(ctx, speech(time.Second), Hint{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if left, _ := os.ReadDir(scratch); len(left) != 0 {
		t.Fatalf("speech left on disk: %v", left)
	}
}

func TestDictateMissingEngineAndClose(t *testing.T) {
	_, model, sum := fixture(t, "x")
	t.Setenv("PATH", t.TempDir())
	if _, err := NewDictate(DictateConfig{Model: model, SHA256: sum, Limits: specLimits(t)}); !errors.Is(err, ErrMissingEngine) {
		t.Fatalf("err = %v, want ErrMissingEngine", err)
	}
	r := &fakeRunner{run: func(context.Context, []string) ([]byte, error) { return []byte("hi\n"), nil }}
	d, _ := newTestDictate(t, r, specLimits(t))
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Transcribe(context.Background(), speech(time.Second), Hint{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("after Close: err = %v", err)
	}
	if len(r.argvs) != 0 {
		t.Fatal("a closed adapter ran the engine")
	}
}

// Warm, which the daemon calls as the talk key goes down, checks the pinned
// model then, so the first partial does not wait for the hash of a model of
// hundreds of megabytes; it runs no engine. A model that is not the pinned
// file is refused there, and a closed adapter warms nothing.
func TestDictateWarmChecksTheModelAndRunsNothing(t *testing.T) {
	r := &fakeRunner{run: func(context.Context, []string) ([]byte, error) { return []byte("hi\n"), nil }}
	d, _ := newTestDictate(t, r, specLimits(t))
	var w interface{ Warm() error } = d
	if err := w.Warm(); err != nil {
		t.Fatal(err)
	}
	if len(r.argvs) != 0 {
		t.Fatalf("Warm ran %q", r.argvs)
	}
	if err := os.WriteFile(d.cfg.Model, []byte("not the pinned model"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); !errors.Is(err, ErrModel) {
		t.Fatalf("Warm with a swapped model: err = %v, want ErrModel", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Warm after Close: err = %v, want ErrClosed", err)
	}
}

func TestDictateRejectsUnknownDeviceAndUnpinnedModel(t *testing.T) {
	bin, model, sum := fixture(t, "ryoku-dictate")
	for _, c := range []DictateConfig{
		{Device: "--serve"},
		{Language: "-x"},
		{SHA256: "abc"},
	} {
		c.Binary, c.Model, c.Limits = bin, model, specLimits(t)
		if c.SHA256 == "" {
			c.SHA256 = sum
		}
		if _, err := NewDictate(c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
