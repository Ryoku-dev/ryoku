package stt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DictateConfig configures the ryoku-dictate adapter.
type DictateConfig struct {
	// Binary is ryoku-dictate's path or a name on PATH; "" is
	// "ryoku-dictate".
	Binary string
	// Model is the absolute path of a transcribe.cpp GGUF model (Parakeet
	// or Whisper), pinned by SHA256.
	Model, SHA256, License string
	// Device is "cpu" (the default), "gpu" or "auto".
	Device string
	// Language is a two- or three-letter code; "" lets the model decide.
	Language string
	Limits   Limits
	// ScratchDir holds the per-decode WAV; "" is $XDG_RUNTIME_DIR, or the
	// temporary directory when that is unset.
	ScratchDir string
	// Runner is injected by tests; nil runs the real program.
	Runner Runner
}

// Dictate decodes through ryoku-dictate, the Ryoku dictation engine, when
// it is installed; Hyakki never requires it. ryoku-dictate takes speech
// two ways: its own PipeWire capture (the serve socket), which Hyakki never
// uses, and a WAV file (--selftest). This adapter uses only the file: each
// decode writes the PCM to a private file, runs one child that loads the
// model and decodes it, and deletes the file. Children live only during a
// decode; Close cancels and joins them before returning.
type Dictate struct {
	cfg    DictateConfig
	bin    string
	model  *pinned
	runner Runner

	mu     sync.Mutex
	closed bool
	next   uint64
	active map[uint64]context.CancelFunc
	work   sync.WaitGroup
}

// NewDictate checks the configuration and that ryoku-dictate is
// installed.
func NewDictate(c DictateConfig) (*Dictate, error) {
	if err := c.Limits.check(); err != nil {
		return nil, err
	}
	switch c.Device {
	case "":
		c.Device = "cpu"
	case "cpu", "gpu", "auto":
	default:
		return nil, fmt.Errorf("stt: device %q is not cpu, gpu or auto", c.Device)
	}
	if c.Language != "" && (c.Language == "auto" || !languagePattern.MatchString(c.Language)) {
		return nil, fmt.Errorf("stt: language %q is not a language code", c.Language)
	}
	if !filepath.IsAbs(c.Model) {
		return nil, fmt.Errorf("%w: model paths must be absolute", ErrModel)
	}
	name := c.Binary
	if name == "" {
		name = "ryoku-dictate"
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingEngine, name)
	}
	if bin, err = filepath.Abs(bin); err != nil {
		return nil, err
	}
	model, err := newPinned(c.Model, c.SHA256)
	if err != nil {
		return nil, err
	}
	d := &Dictate{cfg: c, bin: bin, model: model, runner: c.Runner}
	if d.runner == nil {
		d.runner = ExecRunner{}
	}
	return d, nil
}

// Info names the engine program and the pinned model.
func (d *Dictate) Info() Info {
	return Info{Engine: "transcribe.cpp ryoku-dictate " + d.bin, Model: filepath.Base(d.cfg.Model), License: d.cfg.License, SHA256: d.cfg.SHA256}
}

// Warm checks the pinned model now, as on talk-key-down, so the first
// decode does not wait for its hash; nothing runs, as nothing is resident.
func (d *Dictate) Warm() error {
	d.mu.Lock()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return ErrClosed
	}
	return d.model.verify()
}

func (d *Dictate) argv(wav string) []string {
	a := []string{d.bin, "--selftest", wav, "--model", d.cfg.Model, "--device", d.cfg.Device}
	if d.cfg.Language != "" {
		a = append(a, "--language", d.cfg.Language)
	}
	return a
}

func (d *Dictate) scratch() string {
	if d.cfg.ScratchDir != "" {
		return d.cfg.ScratchDir
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return os.TempDir()
}

// Transcribe writes pcm to a private file, decodes it under the pass
// budget and removes the file whatever happens. Hint.Vocab is not used:
// ryoku-dictate has no prompt.
func (d *Dictate) Transcribe(ctx context.Context, pcm []int16, h Hint) (Result, error) {
	if len(pcm) == 0 {
		return Result{}, ErrNoAudio
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return Result{}, ErrClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	if d.active == nil {
		d.active = make(map[uint64]context.CancelFunc)
	}
	d.next++
	id := d.next
	d.active[id] = cancel
	d.work.Add(1)
	d.mu.Unlock()
	defer func() {
		cancel()
		d.mu.Lock()
		delete(d.active, id)
		d.mu.Unlock()
		d.work.Done()
	}()
	if h.Language != "" && h.Language != d.cfg.Language {
		return Result{}, fmt.Errorf("stt: ryoku-dictate was set up for %q, not %q", d.cfg.Language, h.Language)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	began := time.Now()
	if err := d.model.verify(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	dir, err := os.MkdirTemp(d.scratch(), "hyakki-stt-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "speech.wav")
	f, err := os.OpenFile(wav, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Result{}, err
	}
	if err := writeWAV(f, pcm); err != nil {
		f.Close()
		return Result{}, err
	}
	if err := f.Close(); err != nil {
		return Result{}, err
	}
	text, err := bounded(ctx, d.cfg.Limits, h.Pass, func(ctx context.Context) (string, error) {
		out, err := d.runner.Run(ctx, d.argv(wav))
		if err != nil {
			return "", err
		}
		return lastLine(string(out)), nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Text: text, Pass: h.Pass, Took: time.Since(began)}, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Close cancels active children and waits for their private speech-file
// cleanup. The caller may exit immediately after it returns.
func (d *Dictate) Close() error {
	d.mu.Lock()
	d.closed = true
	for _, cancel := range d.active {
		cancel()
	}
	d.mu.Unlock()
	d.work.Wait()
	return nil
}

var _ Transcriber = (*Dictate)(nil)
