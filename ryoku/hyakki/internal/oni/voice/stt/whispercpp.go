package stt

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// loopback is the only address the resident engine binds and the only
// address its client dials.
const loopback = "127.0.0.1"

// stopGrace is how long a stopping engine gets before it is killed.
const stopGrace = 2 * time.Second

var languagePattern = regexp.MustCompile(`^(auto|[a-z]{2,3})$`)

// WhisperConfig configures the resident whisper.cpp engine.
type WhisperConfig struct {
	// Binary is whisper-server's path or a name on PATH; "" is
	// "whisper-server".
	Binary string
	// Model is the absolute path of a ggml whisper model, pinned by SHA256.
	Model, SHA256, License string
	// VADModel, when set, is a ggml Silero model (pinned by VADSHA256) the
	// engine runs before decoding.
	VADModel, VADSHA256 string
	// Threads is the decode threads; 0 is the CPU count kept within 2..8.
	Threads int
	// Language is a two- or three-letter code or "auto"; "" is "en".
	Language string
	// GPU lets the engine use a GPU; off passes --no-gpu.
	GPU bool
	// Character is the character's name, never sent as vocabulary.
	Character string
	Limits    Limits
	// Clock, Starter, Port and Listening are injected by tests; nil is the
	// real one. Listening reports whether pid listens on 127.0.0.1:port.
	Clock     Clock
	Starter   Starter
	Port      func() (int, error)
	Listening func(pid, port int) bool
}

// Whisper is a whisper-server child owned by Hyakki: started on first use
// or Warm, bound to 127.0.0.1 on a free port, answering only on a random
// 24-hex request path and a random 24-hex inference path, and unloaded
// after Limits.IdleUnload with no decode. The model is verified against its
// pinned SHA-256 before every start.
type Whisper struct {
	cfg       WhisperConfig
	bin       string
	model     *pinned
	vad       *pinned
	clock     Clock
	starter   Starter
	port      func() (int, error)
	listening func(pid, port int) bool
	client    *http.Client

	mu      sync.Mutex
	closed  bool
	srv     *server
	busy    int
	idle    Timer
	idleGen uint64
	unload  time.Duration
}

type server struct {
	proc   Process
	port   int
	infer  string        // full inference URL
	ready  atomic.Bool   // the child was seen listening
	exited chan struct{} // closed once the child is gone
}

// NewWhisper checks the configuration and that the engine is installed.
// It starts nothing.
func NewWhisper(c WhisperConfig) (*Whisper, error) {
	if err := c.Limits.check(); err != nil {
		return nil, err
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if !languagePattern.MatchString(c.Language) {
		return nil, fmt.Errorf("stt: language %q is not a language code", c.Language)
	}
	if !filepath.IsAbs(c.Model) || c.VADModel != "" && !filepath.IsAbs(c.VADModel) {
		return nil, fmt.Errorf("%w: model paths must be absolute", ErrModel)
	}
	if c.Threads <= 0 {
		c.Threads = max(2, min(8, runtime.NumCPU()))
	}
	name := c.Binary
	if name == "" {
		name = "whisper-server"
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
	w := &Whisper{cfg: c, bin: bin, model: model, clock: c.Clock, starter: c.Starter, port: c.Port}
	if c.VADModel != "" {
		if w.vad, err = newPinned(c.VADModel, c.VADSHA256); err != nil {
			return nil, err
		}
	}
	if w.clock == nil {
		w.clock = wallClock{}
	}
	if w.starter == nil {
		w.starter = ExecStarter{}
	}
	if w.port == nil {
		w.port = freePort
	}
	w.listening = c.Listening
	if w.listening == nil {
		w.listening = procListening
	}
	w.client = &http.Client{Transport: &http.Transport{
		Proxy:             nil, // speech never leaves for a proxy
		DialContext:       dialLoopback,
		DisableKeepAlives: true,
	}}
	return w, nil
}

// dialLoopback refuses every address but 127.0.0.1.
func dialLoopback(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host != loopback || network != "tcp" && network != "tcp4" {
		return nil, fmt.Errorf("stt: refusing to dial %s", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, "tcp4", addr)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp4", net.JoinHostPort(loopback, "0"))
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomPath(prefix string) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "/" + prefix + hex.EncodeToString(b[:]), nil
}

// Info names the engine program, the pinned model and the VAD model.
func (w *Whisper) Info() Info {
	info := Info{Engine: "whisper.cpp whisper-server " + w.bin, Model: filepath.Base(w.cfg.Model), License: w.cfg.License, SHA256: w.cfg.SHA256}
	if w.vad != nil {
		info.VAD = filepath.Base(w.cfg.VADModel)
	}
	return info
}

// argv is the engine command line: every flag is one whisper-server
// 1.9.1 documents in --help.
func (w *Whisper) argv(port int, request, inference string) []string {
	a := []string{w.bin,
		"--model", w.cfg.Model,
		"--threads", strconv.Itoa(w.cfg.Threads),
		"--language", w.cfg.Language,
		"--no-fallback",
		"--best-of", "1",
		"--beam-size", "1",
		"--no-timestamps",
		"--host", loopback,
		"--port", strconv.Itoa(port),
		"--request-path", request,
		"--inference-path", inference,
	}
	if !w.cfg.GPU {
		a = append(a, "--no-gpu")
	}
	if w.vad != nil {
		a = append(a, "--vad", "--vad-model", w.cfg.VADModel)
	}
	return a
}

// Warm starts the engine loading, as on talk-key-down, without waiting
// for it. An engine warmed and never used still unloads after the idle
// time.
func (w *Whisper) Warm() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrClosed
	}
	if _, err := w.ensure(); err != nil {
		return err
	}
	if w.busy == 0 {
		w.armIdle()
	}
	return nil
}

// ensure returns the running engine, starting it when there is none.
// Called with mu held.
func (w *Whisper) ensure() (*server, error) {
	if w.srv != nil {
		return w.srv, nil
	}
	if err := w.model.verify(); err != nil {
		return nil, err
	}
	if w.vad != nil {
		if err := w.vad.verify(); err != nil {
			return nil, err
		}
	}
	port, err := w.port()
	if err != nil {
		return nil, err
	}
	request, err := randomPath("p")
	if err != nil {
		return nil, err
	}
	inference, err := randomPath("i")
	if err != nil {
		return nil, err
	}
	proc, err := w.starter.Start(w.argv(port, request, inference))
	if err != nil {
		return nil, fmt.Errorf("stt: starting %s: %w", w.bin, err)
	}
	srv := &server{
		proc:   proc,
		port:   port,
		infer:  "http://" + net.JoinHostPort(loopback, strconv.Itoa(port)) + request + inference,
		exited: make(chan struct{}),
	}
	w.srv = srv
	go func() {
		_ = proc.Wait()
		close(srv.exited)
		w.mu.Lock()
		if w.srv == srv {
			w.srv = nil
		}
		w.mu.Unlock()
	}()
	return srv, nil
}

// readyPoll is how often a loading engine is checked for its socket.
const readyPoll = 10 * time.Millisecond

// waitReady waits until the child itself listens on its port. The
// listening socket must be one of the child's own descriptors, so a
// stranger that took the port first is never sent speech. (whisper-server
// prints its listening line into a block-buffered pipe, so the line cannot
// be the signal.)
func (w *Whisper) waitReady(ctx context.Context, srv *server) error {
	for !srv.ready.Load() {
		if w.listening(srv.proc.Pid(), srv.port) {
			srv.ready.Store(true)
			break
		}
		select {
		case <-srv.exited:
			return fmt.Errorf("stt: %s exited before it listened: %s", filepath.Base(w.bin), srv.proc.StderrTail())
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPoll):
		}
	}
	return nil
}

// procListening reports whether pid holds a TCP socket listening on
// 127.0.0.1:port, from /proc/net/tcp and the process's descriptors.
func procListening(pid, port int) bool {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return false
	}
	addrs := map[string]bool{fmt.Sprintf("0100007F:%04X", port): true, fmt.Sprintf("7F000001:%04X", port): true}
	inodes := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) >= 10 && addrs[f[1]] && f[3] == "0A" {
			inodes["socket:["+f[9]+"]"] = true
		}
	}
	if len(inodes) == 0 {
		return false
	}
	dir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if link, err := os.Readlink(filepath.Join(dir, e.Name())); err == nil && inodes[link] {
			return true
		}
	}
	return false
}

// armIdle (re)arms the idle unload. Called with mu held and nothing busy.
func (w *Whisper) armIdle() {
	if w.idle != nil {
		w.idle.Stop()
	}
	w.idleGen++
	gen := w.idleGen
	w.idle = w.clock.AfterFunc(w.cfg.Limits.IdleUnload, func() { w.idleFire(gen) })
}

func (w *Whisper) idleFire(gen uint64) {
	w.mu.Lock()
	if gen != w.idleGen || w.busy > 0 || w.srv == nil {
		w.mu.Unlock()
		return
	}
	srv := w.srv
	w.srv, w.idle = nil, nil
	w.mu.Unlock()
	w.stop(srv)
}

// stop ends the child: SIGTERM, then SIGKILL after stopGrace. It records
// how long the unload took.
func (w *Whisper) stop(srv *server) {
	began := time.Now()
	_ = srv.proc.Signal(syscall.SIGTERM)
	select {
	case <-srv.exited:
	case <-time.After(stopGrace):
		_ = srv.proc.Signal(syscall.SIGKILL)
		<-srv.exited
	}
	w.mu.Lock()
	w.unload = time.Since(began)
	w.mu.Unlock()
}

// LastUnload is how long the most recent unload took to end the child.
func (w *Whisper) LastUnload() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.unload
}

// Transcribe decodes pcm under the pass budget, starting the engine if it
// is not running. The vocabulary hint is capped and never holds the
// character's name.
func (w *Whisper) Transcribe(ctx context.Context, pcm []int16, h Hint) (Result, error) {
	if len(pcm) == 0 {
		return Result{}, ErrNoAudio
	}
	began := time.Now()
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return Result{}, ErrClosed
	}
	srv, err := w.ensure()
	if err != nil {
		w.mu.Unlock()
		return Result{}, err
	}
	w.busy++
	if w.idle != nil {
		w.idle.Stop()
		w.idle = nil
	}
	w.idleGen++
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.busy--
		if w.busy == 0 && w.srv != nil && !w.closed {
			w.armIdle()
		}
		w.mu.Unlock()
	}()

	body, ctype, err := w.request(pcm, h)
	if err != nil {
		return Result{}, err
	}
	text, err := bounded(ctx, w.cfg.Limits, h.Pass, func(ctx context.Context) (string, error) {
		if err := w.waitReady(ctx, srv); err != nil {
			return "", err
		}
		return w.post(ctx, srv, body, ctype)
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Text: text, Pass: h.Pass, Took: time.Since(began)}, nil
}

func (w *Whisper) request(pcm []int16, h Hint) ([]byte, string, error) {
	var wav bytes.Buffer
	if err := writeWAV(&wav, pcm); err != nil {
		return nil, "", err
	}
	language := w.cfg.Language
	if h.Language != "" {
		if !languagePattern.MatchString(h.Language) {
			return nil, "", fmt.Errorf("stt: language %q is not a language code", h.Language)
		}
		language = h.Language
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "speech.wav")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(wav.Bytes()); err != nil {
		return nil, "", err
	}
	fields := [][2]string{
		{"response_format", "json"},
		{"temperature", "0"},
		{"language", language},
		{"no_timestamps", "true"},
		{"audio_ctx", strconv.Itoa(AudioCtx(duration(len(pcm))))},
	}
	if vocab := Vocabulary(h.Vocab, w.cfg.Character); len(vocab) > 0 {
		fields = append(fields, [2]string{"prompt", strings.Join(vocab, ", ")})
	}
	for _, f := range fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), mw.FormDataContentType(), nil
}

func (w *Whisper) post(ctx context.Context, srv *server, body []byte, ctype string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.infer, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", ctype)
	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if resp.StatusCode != http.StatusOK {
		_ = json.Unmarshal(data, &out)
		return "", fmt.Errorf("stt: whisper-server answered %d %s", resp.StatusCode, out.Error)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("stt: whisper-server answer: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// Close unloads the engine; the Whisper is unusable after.
func (w *Whisper) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	if w.idle != nil {
		w.idle.Stop()
		w.idle = nil
	}
	w.idleGen++
	srv := w.srv
	w.srv = nil
	w.mu.Unlock()
	if srv != nil {
		w.stop(srv)
	}
	return nil
}

var _ Transcriber = (*Whisper)(nil)
