package stt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func answer(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"text": " " + text + " "})
	}
}

func newTestWhisper(t *testing.T, eng *fakeEngine, clock Clock, l Limits) *Whisper {
	t.Helper()
	bin, model, sum := fixture(t, "whisper-server")
	w, err := NewWhisper(WhisperConfig{Binary: bin, Model: model, SHA256: sum, License: "MIT", Character: "Oni", Limits: l, Clock: clock, Starter: eng})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

var allowedWhisperFlags = map[string]bool{
	"--model": true, "--threads": true, "--language": true, "--no-fallback": false, "--best-of": true,
	"--beam-size": true, "--no-timestamps": false, "--host": true, "--port": true, "--request-path": true,
	"--inference-path": true, "--no-gpu": false, "--vad": false, "--vad-model": true,
}

func TestWhisperArgvAllowlistLoopbackAndRandomPaths(t *testing.T) {
	// a proxy in the environment must never see speech
	var proxied atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxied.Add(1) }))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("http_proxy", proxy.URL)
	t.Setenv("ALL_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")

	eng := &fakeEngine{t: t, handler: answer("open the notes"), load: 50 * time.Millisecond}
	w := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	res, err := w.Transcribe(context.Background(), speech(time.Second), Hint{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "open the notes" || res.Pass != Final {
		t.Fatalf("result = %+v", res)
	}
	if proxied.Load() != 0 {
		t.Fatal("speech went through the environment's proxy")
	}
	// unload and start again: new random paths
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	w2 := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	if _, err := w2.Transcribe(context.Background(), speech(time.Second), Hint{}); err != nil {
		t.Fatal(err)
	}

	hex24 := regexp.MustCompile(`^/[pi][0-9a-f]{24}$`)
	paths := map[string]bool{}
	for _, argv := range eng.argvs {
		if !strings.HasSuffix(argv[0], "/whisper-server") {
			t.Fatalf("argv[0] = %q", argv[0])
		}
		for i := 1; i < len(argv); i++ {
			takesValue, ok := allowedWhisperFlags[argv[i]]
			if !ok {
				t.Fatalf("flag %q is not on the allowlist in %q", argv[i], argv)
			}
			if takesValue {
				i++
			}
		}
		if got := flagValue(argv, "--host"); got != "127.0.0.1" {
			t.Fatalf("--host = %q, want loopback only", got)
		}
		for _, f := range []string{"--request-path", "--inference-path"} {
			p := flagValue(argv, f)
			if !hex24.MatchString(p) || paths[p] {
				t.Fatalf("%s = %q: not a fresh random 24-hex path", f, p)
			}
			paths[p] = true
		}
		if !strings.Contains(strings.Join(argv, " "), "--no-gpu") {
			t.Fatal("a CPU engine was started without --no-gpu")
		}
	}
	for _, hit := range eng.hits {
		if !strings.HasPrefix(hit, "POST /p") || !strings.Contains(hit, "/i") {
			t.Fatalf("the engine saw %q outside its random paths", hit)
		}
	}
}

func TestWhisperDialsLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"192.0.2.1:80", "[::1]:80", "localhost:80", "0.0.0.0:80"} {
		if _, err := dialLoopback(context.Background(), "tcp", addr); err == nil {
			t.Fatalf("dialed %s", addr)
		}
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c, err := dialLoopback(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if !procListening(os.Getpid(), port) {
		t.Fatal("this process's own listening socket was not recognised")
	}
	// the same port held by another process is a stranger, not the engine
	if procListening(os.Getppid(), port) {
		t.Fatal("another process was taken for the one listening")
	}
}

func TestWhisperVocabularyHintIsCappedAndNeverTheCharacter(t *testing.T) {
	var mu sync.Mutex
	got := map[string]string{}
	eng := &fakeEngine{t: t, handler: func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		mu.Lock()
		for k, v := range r.MultipartForm.Value {
			got[k] = v[0]
		}
		mu.Unlock()
		answer("ok")(w, r)
	}}
	w := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	names := []string{"Oni", "hey oni", "Mara", "mara", "Tobias"}
	for i := range 20 {
		names = append(names, "Name"+string(rune('A'+i)))
	}
	if _, err := w.Transcribe(context.Background(), speech(2*time.Second), Hint{Vocab: names, Language: "en"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	prompt := strings.Split(got["prompt"], ", ")
	if len(prompt) != MaxVocab {
		t.Fatalf("prompt has %d names, want %d: %q", len(prompt), MaxVocab, got["prompt"])
	}
	for _, n := range prompt {
		if strings.Contains(strings.ToLower(n), "oni") {
			t.Fatalf("the character's name reached the prompt: %q", got["prompt"])
		}
	}
	if prompt[0] != "Mara" || prompt[1] != "Tobias" {
		t.Fatalf("prompt order/dedupe wrong: %q", prompt)
	}
	if got["audio_ctx"] != "256" || got["language"] != "en" || got["response_format"] != "json" {
		t.Fatalf("decode fields = %v", got)
	}
}

// slowEngine blocks every decode until the client goes away and records
// that the engine saw the cancellation (whisper-server aborts a decode
// whose connection closed).
func slowEngine(t *testing.T) (*fakeEngine, *atomic.Int32) {
	var aborted atomic.Int32
	return &fakeEngine{t: t, handler: func(w http.ResponseWriter, r *http.Request) {
		// like whisper-server, read the whole request before decoding
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			aborted.Add(1)
		case <-time.After(10 * time.Second):
			answer("too late")(w, r)
		}
	}}, &aborted
}

func TestWhisperBudgetsAbandonSlowDecodes(t *testing.T) {
	l := specLimits(t)
	if l.Partial != 1200*time.Millisecond || l.Final != 3*time.Second {
		t.Fatalf("spec budgets = %v/%v", l.Partial, l.Final)
	}
	// the same rule at test speed
	l.Partial, l.Final = 60*time.Millisecond, 150*time.Millisecond
	eng, aborted := slowEngine(t)
	w := newTestWhisper(t, eng, &fakeClock{}, l)
	if err := w.Warm(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pass   Pass
		budget time.Duration
	}{{Partial, l.Partial}, {Final, l.Final}} {
		began := time.Now()
		_, err := w.Transcribe(context.Background(), speech(time.Second), Hint{Pass: c.pass})
		took := time.Since(began)
		if !errors.Is(err, ErrBudget) {
			t.Fatalf("%s: err = %v, want ErrBudget", c.pass, err)
		}
		if took < c.budget || took > c.budget+time.Second {
			t.Fatalf("%s: gave up after %v, budget %v", c.pass, took, c.budget)
		}
	}
	waitFor(t, "the engine to see both decodes abandoned", func() bool { return aborted.Load() == 2 })
}

func TestWhisperCallerCancelIsNotABudgetMiss(t *testing.T) {
	eng, aborted := slowEngine(t)
	w := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := w.Transcribe(ctx, speech(time.Second), Hint{Pass: Final})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrBudget) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	waitFor(t, "the engine to abandon the decode", func() bool { return aborted.Load() == 1 })
}

func TestWhisperUnloadsAfterIdle(t *testing.T) {
	release := make(chan struct{})
	var block atomic.Bool
	eng := &fakeEngine{t: t, handler: func(w http.ResponseWriter, r *http.Request) {
		if block.Load() {
			<-release
		}
		answer("ok")(w, r)
	}}
	clock := &fakeClock{}
	l := specLimits(t)
	w := newTestWhisper(t, eng, clock, l)

	if _, err := w.Transcribe(context.Background(), speech(time.Second), Hint{}); err != nil {
		t.Fatal(err)
	}
	armed := clock.armed()
	if len(armed) != 1 || armed[0].d != 10*time.Minute || armed[0].d != l.IdleUnload {
		t.Fatalf("idle timers after a decode = %d (%v), want one of %v", len(armed), armed, l.IdleUnload)
	}
	first := armed[0]

	// a decode in progress stops the timer, and a stale firing does nothing
	block.Store(true)
	done := make(chan error, 1)
	go func() {
		_, err := w.Transcribe(context.Background(), speech(time.Second), Hint{})
		done <- err
	}()
	waitFor(t, "the busy decode to stop the idle timer", func() bool { return len(clock.armed()) == 0 })
	first.f()
	if eng.proc(0).exited() {
		t.Fatal("a stale idle timer unloaded an engine that was decoding")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	armed = clock.armed()
	if len(armed) != 1 {
		t.Fatalf("idle timers after the second decode = %d", len(armed))
	}
	armed[0].f()
	p := eng.proc(0)
	if !p.exited() {
		t.Fatal("the engine is still loaded after the idle time")
	}
	if sigs := p.gotSignals(); len(sigs) == 0 || sigs[0] != syscall.SIGTERM {
		t.Fatalf("unload signals = %v, want SIGTERM first", sigs)
	}
	if w.LastUnload() <= 0 {
		t.Fatal("the unload time was not recorded")
	}

	// the next decode loads it again
	block.Store(false)
	if _, err := w.Transcribe(context.Background(), speech(time.Second), Hint{}); err != nil {
		t.Fatal(err)
	}
	if eng.started() != 2 {
		t.Fatalf("starts = %d, want a reload after unload", eng.started())
	}
}

func TestWhisperWarmUnloadsWhenNeverUsed(t *testing.T) {
	eng := &fakeEngine{t: t, handler: answer("ok")}
	clock := &fakeClock{}
	w := newTestWhisper(t, eng, clock, specLimits(t))
	if err := w.Warm(); err != nil {
		t.Fatal(err)
	}
	if eng.started() != 1 {
		t.Fatal("Warm did not start the engine")
	}
	armed := clock.armed()
	if len(armed) != 1 {
		t.Fatalf("Warm armed %d idle timers", len(armed))
	}
	armed[0].f()
	if !eng.proc(0).exited() {
		t.Fatal("a warmed engine that was never used stayed loaded")
	}
}

func TestWhisperMissingEngine(t *testing.T) {
	_, model, sum := fixture(t, "x")
	_, err := NewWhisper(WhisperConfig{Binary: "/nonexistent/whisper-server", Model: model, SHA256: sum, Limits: specLimits(t)})
	if !errors.Is(err, ErrMissingEngine) {
		t.Fatalf("err = %v, want ErrMissingEngine", err)
	}
	t.Setenv("PATH", t.TempDir())
	_, err = NewWhisper(WhisperConfig{Model: model, SHA256: sum, Limits: specLimits(t)})
	if !errors.Is(err, ErrMissingEngine) {
		t.Fatalf("err = %v, want ErrMissingEngine with no whisper-server on PATH", err)
	}
}

func TestWhisperRefusesAnUnpinnedModel(t *testing.T) {
	eng := &fakeEngine{t: t, handler: answer("ok")}
	bin, model, _ := fixture(t, "whisper-server")
	if _, err := NewWhisper(WhisperConfig{Binary: bin, Model: model, SHA256: "", Limits: specLimits(t), Starter: eng}); !errors.Is(err, ErrModel) {
		t.Fatalf("unpinned model: err = %v", err)
	}
	w, err := NewWhisper(WhisperConfig{Binary: bin, Model: model, SHA256: strings.Repeat("0", 64), Limits: specLimits(t), Starter: eng, Clock: &fakeClock{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Transcribe(context.Background(), speech(time.Second), Hint{}); !errors.Is(err, ErrModel) {
		t.Fatalf("mismatched model: err = %v", err)
	}
	if eng.started() != 0 {
		t.Fatal("the engine started on a model that is not the pinned file")
	}
}

func TestWhisperChildThatNeverListens(t *testing.T) {
	eng := &fakeEngine{t: t, deaf: true}
	w := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	_, err := w.Transcribe(context.Background(), speech(time.Second), Hint{})
	if err == nil || !strings.Contains(err.Error(), "couldn't bind") {
		t.Fatalf("err = %v, want the child's own reason", err)
	}
	waitFor(t, "the dead child to be forgotten", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.srv == nil
	})
	_, _ = w.Transcribe(context.Background(), speech(time.Second), Hint{})
	if eng.started() != 2 {
		t.Fatalf("starts = %d, want a fresh start after the child died", eng.started())
	}
}

func TestWhisperCloseUnloadsAndRefuses(t *testing.T) {
	eng := &fakeEngine{t: t, handler: answer("ok")}
	w := newTestWhisper(t, eng, &fakeClock{}, specLimits(t))
	if _, err := w.Transcribe(context.Background(), speech(time.Second), Hint{}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !eng.proc(0).exited() {
		t.Fatal("Close left the engine running")
	}
	if _, err := w.Transcribe(context.Background(), speech(time.Second), Hint{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("after Close: err = %v", err)
	}
	if _, err := w.Transcribe(context.Background(), nil, Hint{}); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("no audio: err = %v", err)
	}
}

func TestWhisperRejectsFlagLikeValues(t *testing.T) {
	bin, model, sum := fixture(t, "whisper-server")
	for _, c := range []WhisperConfig{
		{Language: "--public"},
		{Language: "en; rm"},
		{Model: "relative/model.bin"},
		{VADModel: "-vm"},
	} {
		c.Binary, c.SHA256, c.Limits = bin, sum, specLimits(t)
		if c.Model == "" {
			c.Model = model
		}
		if _, err := NewWhisper(c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
