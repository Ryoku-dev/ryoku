package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/audio"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// These tests drive the voice loop over the daemon's own socket. The
// microphone, the speech engine, the voice, the speaker, the typing tool
// and the model are fakes: no device is opened, nothing is played and
// nothing is typed.

// fakeMic hands each Start's feed to the test and counts the captures
// still open.
type fakeMic struct {
	starts atomic.Int32
	live   atomic.Int32
	opened chan chan []int16
}

func newFakeMic() *fakeMic { return &fakeMic{opened: make(chan chan []int16, 16)} }

func (m *fakeMic) Start(ctx context.Context, node string) (<-chan []int16, func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	feed := make(chan []int16, 256)
	out := make(chan []int16)
	run, cancel := context.WithCancel(ctx)
	m.starts.Add(1)
	m.live.Add(1)
	go func() {
		defer m.live.Add(-1)
		defer close(out)
		for {
			select {
			case <-run.Done():
				return
			case b := <-feed:
				select {
				case out <- b:
				case <-run.Done():
					return
				}
			}
		}
	}()
	m.opened <- feed
	return out, func() error { cancel(); return nil }, nil
}

// next is the feed of the next capture opened.
func (m *fakeMic) next(t *testing.T) chan []int16 {
	t.Helper()
	select {
	case feed := <-m.opened:
		return feed
	case <-time.After(5 * time.Second):
		t.Fatal("the microphone was never opened")
		return nil
	}
}

// awaitLive waits for n captures to be open.
func (m *fakeMic) awaitLive(t *testing.T, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for m.live.Load() != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d captures open, want %d", m.live.Load(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// say feeds a second of speech-loud audio and then the pause that ends it.
func say(feed chan []int16) {
	speak(feed)
	pause(feed)
}

// sayLong feeds four seconds of speech and the pause: room for a long
// dictation's words to be plausible.
func sayLong(feed chan []int16) {
	for range 4 {
		speak(feed)
	}
	pause(feed)
}

func speak(feed chan []int16) {
	for range 10 {
		chunk := make([]int16, 1600)
		for i := range chunk {
			chunk[i] = 4000
			if i/20%2 == 1 {
				chunk[i] = -4000
			}
		}
		feed <- chunk
	}
}

func pause(feed chan []int16) {
	for range 6 {
		feed <- make([]int16, 1600)
	}
}

// fakeEars is the speech engine: partials count up, finals come from a
// queue the test fills.
type fakeEars struct {
	mu       sync.Mutex
	partials []string
	finals   []string
	decodes  int
	// hints are the vocabulary hints of every final decode, in order
	hints  [][]string
	closed bool
}

func (e *fakeEars) Info() stt.Info { return stt.Info{Engine: "fake"} }

func (e *fakeEars) Transcribe(_ context.Context, _ []int16, h stt.Hint) (stt.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.decodes++
	if h.Pass == stt.Final {
		e.hints = append(e.hints, slices.Clone(h.Vocab))
	}
	if h.Pass == stt.Partial {
		if len(e.partials) == 0 {
			return stt.Result{}, errors.New("no partial")
		}
		text := e.partials[0]
		if len(e.partials) > 1 {
			e.partials = e.partials[1:]
		}
		return stt.Result{Text: text, Pass: h.Pass}, nil
	}
	if len(e.finals) == 0 {
		return stt.Result{Pass: h.Pass}, nil
	}
	text := e.finals[0]
	e.finals = e.finals[1:]
	return stt.Result{Text: text, Pass: h.Pass}, nil
}

func (e *fakeEars) Close() error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	return nil
}

func (e *fakeEars) hear(partials []string, finals ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.partials = partials
	e.finals = append(e.finals, finals...)
}

// fakeVoice renders samples per character of silence-free audio.
type fakeVoice struct{ perChar atomic.Int32 }

func (v *fakeVoice) Name() string  { return tts.System }
func (v *fakeVoice) Voice() string { return "fake" }
func (v *fakeVoice) Render(_ context.Context, line string) ([]float32, int, error) {
	pcm := make([]float32, int(v.perChar.Load())*len(line))
	for i := range pcm {
		pcm[i] = 0.25
	}
	return pcm, tts.SpeechRateHz(), nil
}

// fakeSink keeps what was played and when a sound was cut.
type fakeSink struct {
	mu      sync.Mutex
	plays   int
	cancels int
	cut     chan struct{}
}

func (s *fakeSink) Play(pcm []float32, rate int) (func(), error) {
	s.mu.Lock()
	s.plays++
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.cancels++
			s.mu.Unlock()
			select {
			case s.cut <- struct{}{}:
			default:
			}
		})
	}, nil
}

// fakeTyper records each typing call; after cut chunks it closes the window
// typed into.
type fakeTyper struct {
	mu    sync.Mutex
	argv  [][]string
	after int
	then  func()
}

func (f *fakeTyper) run(_ context.Context, name string, args ...string) error {
	f.mu.Lock()
	f.argv = append(f.argv, append([]string{name}, args...))
	n := len(f.argv)
	after, then := f.after, f.then
	f.mu.Unlock()
	if then != nil && n == after {
		then()
	}
	return nil
}

func (f *fakeTyper) calls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.argv)
}

// failingModel plans like fakeModel but every stream fails.
type failingModel struct{ *fakeModel }

func (failingModel) Stream(context.Context, llm.Request) (llm.Stream, error) {
	return nil, errors.New("the model fixture is down")
}

type voiceRig struct {
	*wireRig
	mic     *fakeMic
	ears    *fakeEars
	voice   *fakeVoice
	sink    *fakeSink
	typer   *fakeTyper
	store   *dictate.Store
	mu      sync.Mutex
	asked   []string
	replies []string
}

// serveVoice is a serving daemon with voice on, replies spoken and
// dictation on, every voice part a fake, edit applied to its settings and
// each of with to its deps.
func serveVoice(t *testing.T, edit func(*protocol.Settings), gateway func(*fakeModel) core.Gateway, with ...func(*Deps)) *voiceRig {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled = true
	settings.Oni.Master = true
	settings.Oni.Inspect = true
	settings.Oni.Voice = true
	settings.Oni.VoiceSettings.SpeakReplies = true
	settings.Oni.VoiceSettings.Dictation.Enabled = true
	if edit != nil {
		edit(&settings)
	}
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	w := &wireRig{desk: newDesktop(desktopWindows()), grim: &fakeGrim{}, model: &fakeModel{plan: localEyes}, clock: &askClock{now: time.Unix(1_800_000_000, 0)}, app: &atomic.Bool{}}
	w.shots = &shotLog{}
	w.tess = &fakeOCR{}
	w.screen = newFakeScreen()
	r := &voiceRig{wireRig: w, mic: newFakeMic(), ears: &fakeEars{}, voice: &fakeVoice{}, sink: &fakeSink{cut: make(chan struct{}, 8)}, typer: &fakeTyper{}}
	r.voice.perChar.Store(24)
	r.store = dictate.NewStore(filepath.Join(t.TempDir(), "dictation.json"))
	var gw core.Gateway = w.model
	if gateway != nil {
		gw = gateway(w.model)
	}
	runner := &fakeRunner{prompt: func(_ context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		r.mu.Lock()
		r.asked = append(r.asked, in.Text)
		reply := "Fine."
		if len(r.replies) > 0 {
			reply, r.replies = r.replies[0], r.replies[1:]
		}
		r.mu.Unlock()
		if reply == failedTurn {
			return core.TurnResult{}, errors.New("no model can take this turn")
		}
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: reply})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	deps := Deps{
		Version: "oni-voice-test",
		Gateway: gw,
		Asker:   fakeAsker{runner: runner},
		OniWindows: func() (window.Provider, error) {
			w.detects.Add(1)
			return w.desk, nil
		},
		OniCapture: func(p window.Provider) capture.Service {
			w.shots.mu.Lock()
			w.shots.inner = capture.New(w.grim.run, []capture.Path{capture.PathOutputCrop}, capture.Layout(p.Snapshot))
			w.shots.mu.Unlock()
			return w.shots
		},
		OniGuide: OniGuideDeps{
			OCR:    w.tess,
			Settle: w.screen.take,
			Clock:  w.clock,
		},
		OniVoice: OniVoiceDeps{
			Capture: r.mic,
			Sources: func(context.Context) ([]audio.Source, error) {
				return []audio.Source{{Name: "lab_mic", Desc: "Lab mic"}, {Name: "lab.monitor", Desc: "Monitor", Monitor: true}}, nil
			},
			STT: func(*spec.OniSpec, protocol.OniSTTSettings, string) (stt.Transcriber, error) {
				return r.ears, nil
			},
			Voices: func(protocol.OniTTSSettings, profile.Profile) []tts.Engine { return []tts.Engine{r.voice} },
			Sink:   func() (sound.Sink, error) { return r.sink, nil },
			Type:   r.typer.run,
			Store:  r.store,
			Clock:  w.clock,
		},
	}
	for _, f := range with {
		f(&deps)
	}
	d := New(refuseRealDesktop(t, deps))
	d.appPeer = func(int32) bool { return w.app.Load() }
	d.oni.ask.clock = w.clock
	w.d = d
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), paths.Socket()) {
		if time.Now().After(deadline) {
			t.Fatal("the daemon never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r
}

func (r *voiceRig) answer(replies ...string) {
	r.mu.Lock()
	r.replies = append(r.replies, replies...)
	r.mu.Unlock()
}

// failedTurn, given to answer, makes that Ask turn fail as a turn does when
// no model can take it.
const failedTurn = "\x00fail"

func (r *voiceRig) questions() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.asked)
}

func (r *voiceRig) patch(t *testing.T, patch string) {
	t.Helper()
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(patch)); err != nil {
		t.Fatal(err)
	}
}

// oniLog is every oni frame a peer received, raw and decoded.
type oniLog struct {
	raw    []string
	voices []feature.VoiceFrame
}

// voiceUntil reads p's oni frames until one voice frame matches want.
func voiceUntil(t *testing.T, p *socketPeer, log *oniLog, want func(feature.VoiceFrame) bool) feature.VoiceFrame {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-p.conn.Events():
			if !ok {
				t.Fatal("the connection closed")
			}
			if e.Event != protocol.EvtOni {
				continue
			}
			log.raw = append(log.raw, string(e.Data))
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				t.Fatalf("the daemon sent a frame the contract refuses: %v: %s", err, e.Data)
			}
			v, ok := f.(*feature.VoiceFrame)
			if !ok {
				continue
			}
			log.voices = append(log.voices, *v)
			if want(*v) {
				return *v
			}
		case <-timeout:
			t.Fatalf("no voice frame matched; got %+v", log.voices)
		}
	}
}

func phaseIs(phase string) func(feature.VoiceFrame) bool {
	return func(v feature.VoiceFrame) bool { return v.Phase == phase }
}

func voiceIs(phase, text string) func(feature.VoiceFrame) bool {
	return func(v feature.VoiceFrame) bool { return v.Phase == phase && v.Text == text }
}

func talk(t *testing.T, p *socketPeer, action string) protocol.OniVoiceResult {
	t.Helper()
	var r protocol.OniVoiceResult
	if err := p.call(protocol.MOniTalk, protocol.OniTalkParams{Action: action}, &r); err != nil {
		t.Fatalf("oni.talk %s: %v", action, err)
	}
	return r
}

// A press goes listening, its partials, its final transcript, an
// acknowledgement, the spoken answer and back to idle, in that order; the
// model hears the voice block with the question.
func TestOniVoicePressAnswersInOrder(t *testing.T) {
	r := serveVoice(t, nil, nil)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	var log oniLog
	r.ears.hear([]string{"tell me"}, "tell me a joke")
	r.answer("Why did the oni nap? It was tired of haunting.")

	if got := talk(t, surface, protocol.OniVoiceStart); got.Phase != feature.VoiceListening {
		t.Fatalf("start answered %q", got.Phase)
	}
	feed := r.mic.next(t)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceListening, ""))
	speak(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceTranscribing, "tell me"))
	pause(feed)
	time.Sleep(50 * time.Millisecond)
	if got := talk(t, surface, protocol.OniVoiceStop); got.Phase != feature.VoiceTranscribing {
		t.Fatalf("stop answered %q", got.Phase)
	}
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Why did the oni nap?"))
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceIdle))
	r.mic.awaitLive(t, 0)

	var phases []string
	for _, v := range log.voices {
		phases = append(phases, v.Phase+":"+v.Text)
		if v.Source != "" && v.Source != feature.VoiceSourceChord {
			t.Errorf("a press from the presenter is the chord, not %q", v.Source)
		}
	}
	ack := log.voices[len(log.voices)-3]
	want := []string{"listening:", "transcribing:tell me", "transcribing:tell me a joke", "speaking:" + ack.Text, "speaking:Why did the oni nap?", "idle:"}
	if !slices.Equal(phases, want) || ack.Text == "" {
		t.Fatalf("frames %q, want %q", phases, want)
	}
	asked := r.questions()
	if len(asked) != 1 || !strings.HasSuffix(asked[0], "\n\ntell me a joke") || !strings.Contains(asked[0], route.VoiceBlock(false)) {
		t.Fatalf("the model heard %q", asked)
	}
	// the CLI is a toggle, not the chord
	talk(t, cli, protocol.OniVoiceStart)
	r.mic.next(t)
	if v := voiceUntil(t, surface, &log, phaseIs(feature.VoiceListening)); v.Source != feature.VoiceSourceToggle {
		t.Fatalf("a press from the CLI is %q", v.Source)
	}
	if got := talk(t, cli, protocol.OniVoiceCancel); got.Phase != feature.VoiceIdle {
		t.Fatalf("cancel answered %q", got.Phase)
	}
	r.mic.awaitLive(t, 0)
}

// An Ask turn that ends in an error (no model can take it, the provider
// failed) is said as such after the acknowledgement, never dropped in
// silence.
func TestOniVoiceAskThatFailsSaysSo(t *testing.T) {
	r := serveVoice(t, nil, nil)
	surface, _ := r.presenting(t)
	var log oniLog
	r.ears.hear(nil, "tell me a joke")
	r.answer(failedTurn)

	talk(t, surface, protocol.OniVoiceStart)
	feed := r.mic.next(t)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceListening, ""))
	speak(feed)
	pause(feed)
	time.Sleep(50 * time.Millisecond)
	talk(t, surface, protocol.OniVoiceStop)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "I could not get an answer."))
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceIdle))
	if asked := r.questions(); len(asked) != 1 {
		t.Fatalf("the model was asked %d times, want once", len(asked))
	}
}

// Pressing while Oni speaks cuts the sound mid-word and listens.
func TestOniVoiceBargeInCutsPlayback(t *testing.T) {
	r := serveVoice(t, nil, nil)
	surface, _ := r.presenting(t)
	var log oniLog
	r.voice.perChar.Store(24000) // a second a character: the line plays for long
	r.ears.hear(nil, "what time is it")
	talk(t, surface, protocol.OniVoiceStart)
	say(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	talk(t, surface, protocol.OniVoiceStop)
	spoken := voiceUntil(t, surface, &log, phaseIs(feature.VoiceSpeaking))
	if !strings.HasPrefix(spoken.Text, "It's ") {
		t.Fatalf("the time was answered %q", spoken.Text)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.sink.mu.Lock()
		plays := r.sink.plays
		r.sink.mu.Unlock()
		if plays > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing played")
		}
		time.Sleep(5 * time.Millisecond)
	}
	talk(t, surface, protocol.OniVoiceStart)
	select {
	case <-r.sink.cut:
	case <-time.After(5 * time.Second):
		t.Fatal("the press did not cut the playing line")
	}
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceListening))
	if r.questions() != nil {
		t.Fatal("the time went to a model")
	}
	talk(t, surface, protocol.OniVoiceCancel)
}

// With open mic on, Oni's own line heard back is no command: no frame
// carries it and no model is asked, while the user's next question is.
// Open-mic speech never answers a consent card; a press does.
func TestOniVoiceOpenMicDropsItsOwnEchoAndNeverAnswersConsent(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.VoiceSettings.OpenMic = true }, nil)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	var log oniLog
	feed := r.mic.next(t)
	if !r.d.voiceResident() {
		t.Fatal("open mic does not keep the daemon up")
	}
	r.ears.hear(nil, "what time is it")
	say(feed)
	spoken := voiceUntil(t, surface, &log, phaseIs(feature.VoiceSpeaking))
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceArmed, ""))
	// the line comes back through the microphone
	r.ears.hear(nil, spoken.Text)
	say(feed)
	r.answer("Here.")
	r.ears.hear(nil, "where is my phone")
	say(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Here."))
	for _, v := range log.voices[slices.IndexFunc(log.voices, func(v feature.VoiceFrame) bool { return v.Phase == feature.VoiceArmed }):] {
		if v.Text == spoken.Text {
			t.Fatalf("Oni's own line was taken as a command: %+v", log.voices)
		}
	}
	if asked := r.questions(); len(asked) != 1 || !strings.HasSuffix(asked[0], "where is my phone") {
		t.Fatalf("the model heard %q", asked)
	}

	// a consent card waits; open mic says yes to it
	perchOn(t, cli, "0x1")
	inspectFor(t, cli)
	if !(voiceConsent{r.d}).Pending() {
		t.Fatal("no consent card waits")
	}
	r.ears.hear(nil, "yes")
	say(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, route.NeedsPressLine))
	if !(voiceConsent{r.d}).Pending() || len(r.model.sent()) != 0 {
		t.Fatal("open-mic speech answered the consent card")
	}
	// the chord does answer it
	r.ears.hear(nil, "yes")
	talk(t, surface, protocol.OniVoiceStart)
	say(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	talk(t, surface, protocol.OniVoiceStop)
	deadline := time.Now().Add(5 * time.Second)
	for (voiceConsent{r.d}).Pending() || len(r.model.sent()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the chord's yes did not answer the card")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The microphone hears Oni's acknowledgement and the reply after it as one
// burst, and its transcript arrives once the burst has ended and been
// decoded, when the acknowledgement's own echo window may already have
// closed. The burst began inside both windows, so it is Oni's own speech:
// no second question.
func TestOniVoiceOpenMicDropsAckAndReplyHeardBackTogether(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.VoiceSettings.OpenMic = true }, nil)
	surface, _ := r.presenting(t)
	var log oniLog
	feed := r.mic.next(t)
	r.answer("Here it is. It was under the lamp.")
	r.ears.hear(nil, "where is my phone")
	say(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Here it is."))
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceArmed, ""))
	var said []string
	for _, v := range log.voices {
		if v.Phase == feature.VoiceSpeaking {
			said = append(said, v.Text)
		}
	}
	if len(said) != 2 {
		t.Fatalf("Oni said %q, want an acknowledgement and the reply", said)
	}
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	// the acknowledgement's window has just closed; the reply's has not
	r.clock.Advance(time.Duration(sp.EchoMs(len([]rune(said[0])))*float64(time.Millisecond)) + 10*time.Millisecond)
	r.ears.hear(nil, strings.TrimSuffix(said[0], ".")+", "+said[1])
	say(feed)
	time.Sleep(300 * time.Millisecond)
	r.ears.hear(nil, "thank you")
	say(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceTranscribing, "thank you"))
	for _, v := range log.voices {
		if v.Phase == feature.VoiceTranscribing && strings.Contains(v.Text, said[1]) {
			t.Fatalf("Oni's own lines heard back were taken as a command: %+v", log.voices)
		}
	}
	if asked := r.questions(); len(asked) > 2 || !strings.HasSuffix(asked[0], "where is my phone") {
		t.Fatalf("the model heard %q", asked)
	}
}

// With the wake phrase armed, speech that does not start with it leaves no
// trace: no frame, no question, no file or log line holds it. The phrase
// opens a session that is answered; wake speech never answers consent.
// Wake disarms at its ceiling and turns its switch off.
func TestOniVoiceWakeDiscardsOtherSpeechAndDisarmsAtItsCeiling(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.VoiceSettings.Wake.Enabled = true }, nil)
	surface, _ := r.presenting(t)
	var log oniLog
	feed := r.mic.next(t)
	r.ears.hear(nil, "the secret word is zebra")
	say(feed)
	r.ears.hear(nil, "hey oni what time is it")
	say(feed)
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceSpeaking))
	for _, raw := range log.raw {
		if strings.Contains(raw, "zebra") {
			t.Fatalf("a frame holds non-wake speech: %s", raw)
		}
	}
	if r.questions() != nil {
		t.Fatalf("the model heard %q", r.questions())
	}
	for _, root := range []string{os.Getenv("HOME"), os.Getenv("XDG_RUNTIME_DIR")} {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if data, readErr := os.ReadFile(path); readErr == nil && strings.Contains(string(data), "zebra") {
				t.Errorf("%s holds non-wake speech", path)
			}
			return nil
		})
	}
	// inside the wake session, a yes to a consent card asks for the key
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	inspectFor(t, cli)
	r.ears.hear(nil, "yes")
	say(feed)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, route.NeedsPressLine))
	if !(voiceConsent{r.d}).Pending() || len(r.model.sent()) != 0 {
		t.Fatal("wake-phrase speech answered the consent card")
	}
	// the ceiling: an hour armed, then off, the switch with it
	if r.clock.Armed() == 0 {
		t.Fatal("no ceiling timer is armed while wake listens")
	}
	r.clock.Advance(time.Hour)
	r.mic.awaitLive(t, 0)
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceIdle))
	deadline := time.Now().Add(5 * time.Second)
	for r.d.Settings().Oni.VoiceSettings.Wake.Enabled || r.d.voiceResident() {
		if time.Now().After(deadline) {
			t.Fatal("wake stayed armed past its ceiling")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after wake disarmed", n)
	}
}

// After a wake session's answer, while the session is still open and would
// take any speech as a request, the rest phase is listening: armed says only
// the phrase is heard. The session's ceiling brings armed back.
func TestOniVoiceWakeSessionRestsListeningUntilItsCeiling(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.VoiceSettings.Wake.Enabled = true }, nil)
	surface, _ := r.presenting(t)
	var log oniLog
	feed := r.mic.next(t)
	r.ears.hear(nil, "hey oni what time is it")
	say(feed)
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceSpeaking))
	rest := voiceUntil(t, surface, &log, func(v feature.VoiceFrame) bool { return v.Phase != feature.VoiceSpeaking })
	if rest.Phase != feature.VoiceListening || rest.Source != feature.VoiceSourceWake {
		t.Fatalf("inside the open session the answer rested as %+v, want listening from wake", rest)
	}
	r.clock.Advance(15 * time.Second)
	ended := voiceUntil(t, surface, &log, func(v feature.VoiceFrame) bool { return v.Phase != feature.VoiceListening })
	if ended.Phase != feature.VoiceArmed || ended.Source != feature.VoiceSourceWake {
		t.Fatalf("the session's ceiling showed %+v, want armed from wake", ended)
	}
}

// A dictation into a window that closes midway stops at a chunk, types
// nothing anywhere else and returns the rest; its words ride on no frame.
func TestOniDictationStopsAtAChunkWhenItsWindowCloses(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.VoiceSettings.Dictation.Mode = protocol.OniDictateExact }, nil)
	surface, _ := r.presenting(t)
	app := r.dialApp(t)
	var log oniLog
	text := "the quarterly report goes to the board on friday and the summary needs two more charts before then"
	r.ears.hear(nil, text)
	r.typer.after = 2
	r.typer.then = func() {
		snap := desktopWindows()
		snap.Windows = snap.Windows[1:]
		r.desk.set(snap)
	}
	var started protocol.OniDictateResult
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart}, &started); err != nil {
		t.Fatal(err)
	}
	if started.Phase != feature.VoiceListening || started.Mode != protocol.OniDictateExact {
		t.Fatalf("start answered %+v", started)
	}
	sayLong(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	var done protocol.OniDictateResult
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}, &done); err != nil {
		t.Fatal(err)
	}
	calls := r.typer.calls()
	var typed strings.Builder
	for _, argv := range calls {
		if !insert.Allowed(argv[0], argv[1:]...) {
			t.Fatalf("typed with %q", argv)
		}
		typed.WriteString(argv[len(argv)-1])
	}
	if len(calls) != 2 || done.Refused != "closed" || done.Landed != len([]rune(typed.String())) || typed.String()+done.Rest != text {
		t.Fatalf("dictation %+v after typing %q", done, typed.String())
	}
	voiceUntil(t, surface, &log, phaseIs(feature.VoiceIdle))
	for _, raw := range log.raw {
		for _, word := range []string{"quarterly", "charts"} {
			if strings.Contains(raw, word) {
				t.Fatalf("a frame holds the dictation: %s", raw)
			}
		}
	}
	for _, v := range log.voices {
		if v.Source != "" && v.Source != feature.VoiceSourceDictation {
			t.Fatalf("a dictation frame from %q", v.Source)
		}
	}
}

// A polish that fails falls back to clean and says so.
func TestOniDictationPolishFailureFallsBackToClean(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) {
		s.Oni.VoiceSettings.Dictation.Mode = protocol.OniDictatePolished
	}, func(m *fakeModel) core.Gateway { return failingModel{m} })
	app := r.dialApp(t)
	r.ears.hear(nil, "um send it to the team")
	var started protocol.OniDictateResult
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart}, &started); err != nil {
		t.Fatal(err)
	}
	sayLong(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	var done protocol.OniDictateResult
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}, &done); err != nil {
		t.Fatal(err)
	}
	if done.Mode != protocol.OniDictateClean || done.Fallback != string(dictate.FallbackFailed) || done.Rest != "" || done.Landed == 0 {
		t.Fatalf("polish failure answered %+v", done)
	}
	var typed strings.Builder
	for _, argv := range r.typer.calls() {
		typed.WriteString(argv[len(argv)-1])
	}
	if got := strings.ToLower(typed.String()); strings.Contains(got, "um ") || !strings.Contains(got, "send it to the team") {
		t.Fatalf("typed %q", typed.String())
	}
}

// The mic meter returns levels and the microphones (no monitors) and runs
// with voice off.
func TestOniVoiceTestMeasuresWithVoiceOff(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.Voice = false }, nil)
	app := r.dialApp(t)
	done := make(chan protocol.OniVoiceTestResult, 1)
	go func() {
		var res protocol.OniVoiceTestResult
		if err := app.call(protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Ms: 300}, &res); err != nil {
			t.Error(err)
		}
		done <- res
	}()
	speak(r.mic.next(t))
	res := <-done
	if len(res.Levels) == 0 || res.Levels[0] <= 0 || len(res.Sources) != 1 || res.Sources[0].Name != "lab_mic" {
		t.Fatalf("the meter answered %+v", res)
	}
	r.mic.awaitLive(t, 0)
}

// With voice off nothing starts and no timer is armed; voice on but
// disarmed keeps nothing open and leaves the daemon free to exit; arming
// keeps it up and opens the microphone, disarming closes it.
func TestOniVoiceOffStartsNothingAndOnlyArmedStaysResident(t *testing.T) {
	r := serveVoice(t, func(s *protocol.Settings) { s.Oni.Voice = false }, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	var refused *protocol.Error
	if err := cli.call(protocol.MOniTalk, protocol.OniTalkParams{Action: protocol.OniVoiceStart}, nil); !errors.As(err, &refused) || refused.Code != protocol.CodeDisabled {
		t.Fatalf("oni.talk with voice off answered %v", err)
	}
	if r.mic.starts.Load() != 0 || r.clock.Armed() != 0 || r.d.voiceResident() || r.d.voiceBusy() {
		t.Fatal("voice off started something")
	}
	r.patch(t, `{"oni":{"voice":true}}`)
	if r.mic.starts.Load() != 0 || r.clock.Armed() != 0 || r.d.voiceResident() || r.d.voiceBusy() {
		t.Fatal("voice on but disarmed started something")
	}
	r.patch(t, `{"oni":{"voiceSettings":{"wake":{"enabled":true}}}}`)
	r.mic.awaitLive(t, 1)
	if !r.d.voiceResident() || r.clock.Armed() != 1 {
		t.Fatal("armed wake does not keep the daemon up with its one ceiling timer")
	}
	r.patch(t, `{"oni":{"voice":false}}`)
	r.mic.awaitLive(t, 0)
	if r.d.voiceResident() || r.d.voiceBusy() || r.clock.Armed() != 0 {
		t.Fatal("voice off left the loop running")
	}
	if r.ears.mu.Lock(); !r.ears.closed {
		r.ears.mu.Unlock()
		t.Fatal("voice off left the speech engine loaded")
	}
	r.ears.mu.Unlock()
}

// awaitHint waits for the nth final decode's vocabulary hint.
func (e *fakeEars) awaitHint(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.mu.Lock()
		if len(e.hints) >= n {
			hint := e.hints[n-1]
			e.mu.Unlock()
			return hint
		}
		e.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("no final decode %d", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Talk and dictation give the speech engine the dictionary's written forms
// and then the writing words as its vocabulary hint: deduplicated, at most
// 14, never the character's name, and none at all when there are none.
func TestOniVoiceHintsTheUsersVocabulary(t *testing.T) {
	r := serveVoice(t, nil, nil)
	surface, _ := r.presenting(t)
	app := r.dialApp(t)
	var log oniLog
	press := func() {
		t.Helper()
		r.ears.hear(nil, "hello there")
		talk(t, surface, protocol.OniVoiceStart)
		say(r.mic.next(t))
		time.Sleep(50 * time.Millisecond)
		talk(t, surface, protocol.OniVoiceStop)
		voiceUntil(t, surface, &log, phaseIs(feature.VoiceIdle))
	}
	press()
	if hint := r.ears.awaitHint(t, 1); hint != nil {
		t.Fatalf("with no words the hint is %q", hint)
	}

	name := profile.Default().Profile.Name
	words := []string{"Alder", "Birch", "Cedar", "Dogwood", "Elm", "Fir", "Ginkgo", "Hazel", "Hyakki", "Ivy", "Juniper", "Kauri", "Larch", "Maple", "Nutmeg", "Olive", "Pine", name}
	if err := os.MkdirAll(paths.DataDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.DataDir(), "words.txt"), []byte(strings.Join(words, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.SetWord("hi yaki", "hyakki"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.SetWord("my friend", name+" Labs"); err != nil {
		t.Fatal(err)
	}
	want := []string{"hyakki", "Alder", "Birch", "Cedar", "Dogwood", "Elm", "Fir", "Ginkgo", "Hazel", "Ivy", "Juniper", "Kauri", "Larch", "Maple"}
	check := func(hint []string, what string) {
		t.Helper()
		if !slices.Equal(hint, want) {
			t.Fatalf("%s hint is %q, want %q", what, hint, want)
		}
		for _, h := range hint {
			if strings.Contains(strings.ToLower(h), strings.ToLower(name)) {
				t.Fatalf("%s hint names the character: %q", what, hint)
			}
		}
	}
	press()
	check(r.ears.awaitHint(t, 2), "a press's")

	r.ears.hear(nil, "hello there")
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart}, nil); err != nil {
		t.Fatal(err)
	}
	say(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	if err := app.call(protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}, nil); err != nil {
		t.Fatal(err)
	}
	check(r.ears.awaitHint(t, 3), "a dictation's")
}
