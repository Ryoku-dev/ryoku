package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/audio"
	"ryoku-rashin/internal/oni/voice/hygiene"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/oni/voice/turn"
	"ryoku-rashin/internal/oni/voice/vad"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/protocol"
)

// The voice loop runs in the daemon: the microphone is opened per press,
// a dictation, the Settings meter, or while the wake phrase or open mic is
// armed, and never otherwise. Speech is segmented, transcribed by a child
// process, tidied, and routed: Oni's own line heard back is dropped, the
// time and the date are answered locally, a reminder or timer is kept in
// the reminders store, consent is answered
// only from a press, and anything else is asked as a read-only Ask turn.
// Speech that did not follow the wake phrase is never shown, logged or
// kept. With voice off nothing here is built, started or armed.

// oniVoiceLoop is the voice loop's state; everything is built on first use.
type oniVoiceLoop struct {
	mu sync.Mutex
	sp *spec.OniSpec
	// gen ends everything an older voice session started: it moves when
	// voice is turned off
	gen  uint64
	turn *turn.Turn

	character string
	profile   profile.Profile
	router    *route.Router

	stt    stt.Transcriber
	sttKey protocol.OniSTTSettings

	speaker    *tts.Speaker
	speakerKey string
	// line is what Oni is saying now, queue what follows it in this turn
	line  *oniLine
	queue []oniLine

	// press is the talk press or dictation holding the microphone; listen
	// is the wake phrase or open mic between presses
	press  *oniCapture
	listen *oniCapture
	wake   *wake.Wake
	open   *wake.Open
	armKey string
	armAt  presence.Timer

	// work counts transcripts being finished and asks being answered
	work      int
	askCancel context.CancelFunc
	// asking is the turn whose question a model is answering: no rest
	// phase shows before its answer
	asking uint64
}

// oniLine is one line to say for a turn.
type oniLine struct {
	turn   uint64
	source string
	text   string
}

// oniCapture is one microphone run: a press, a dictation or the armed
// listener. Its reader goroutine owns seg and stream; the finals are read
// only once the reader is done.
type oniCapture struct {
	source string // a feature.VoiceSource
	listen bool
	turn   uint64
	gen    uint64
	ctx    context.Context
	cancel context.CancelFunc
	stop   func() error
	seg    *vad.Segmenter
	stream *stt.Stream
	engine stt.Transcriber
	limits stt.Limits
	lang   string
	// vocab is the vocabulary hint every decode of this run gets
	vocab   []string
	read    chan struct{}
	finals  []chan string
	secs    float64
	once    sync.Once
	settled bool
	dict    *oniDictation
	// heard closes when the listener's last burst is done with, so bursts
	// are taken in the order they were said
	heard chan struct{}
}

func routeSource(source string) route.Source {
	switch source {
	case feature.VoiceSourceChord:
		return route.Chord
	case feature.VoiceSourceWake:
		return route.Wake
	case feature.VoiceSourceOpen:
		return route.Open
	}
	return route.Toggle
}

// voiceAllowed is the voice feature's decision under the settings now.
func (d *Daemon) voiceAllowed(id string) (protocol.OniSettings, error) {
	settings := protocol.EffectiveOni(d.Settings())
	decision := feature.Decide(feature.Request{Feature: id}, settings)
	if decision.Allow {
		return settings, nil
	}
	message := i18n.T("Voice is off. Turn it on in Oni's settings.")
	switch {
	case decision.Reason == feature.ReasonOniOff:
		message = i18n.T("Oni is off.")
	case id == feature.VoiceDictation:
		message = i18n.T("Dictation is off. Turn it on in Oni's voice settings.")
	}
	return settings, &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// ensureLocked builds the spec, the turn and the router for the character
// in use. The caller holds v.mu.
func (d *Daemon) voiceEnsureLocked(settings protocol.OniSettings) error {
	v := &d.oni.voice
	if v.sp == nil {
		sp, err := spec.Load()
		if err != nil {
			return protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("Oni's spec does not load: %v.", err))
		}
		v.sp = sp
	}
	if v.turn == nil {
		v.turn = turn.New(v.sp, d.voiceHushLocked)
	}
	if v.router == nil || v.character != settings.Character {
		p := oniActiveProfile(settings.Character)
		router, err := route.New(route.Config{Turn: v.turn, Name: p.Name, Lines: route.Lines{Ack: p.Lines}, Reflex: d.reflexPlan, Hands: d.voiceHands, Consent: voiceConsent{d}, Guide: voiceGuide{d}, Perched: d.voiceGuidePerched,
			HandsStop: func() { d.handsStop(handsByVoice) }, HandsCard: d.oni.hands.card.Load, Memory: d.voiceMemoryOn, Recall: d.voiceRecallOn})
		if err != nil {
			return protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
		}
		v.router, v.profile, v.character = router, p, settings.Character
		v.speaker, v.speakerKey = nil, ""
	}
	return nil
}

// voiceHushLocked is the turn's barge hook: what Oni is saying stops
// mid-word and nothing queued follows. It runs under v.mu, from Barge.
func (d *Daemon) voiceHushLocked() {
	v := &d.oni.voice
	v.line, v.queue = nil, nil
	if v.speaker != nil {
		v.speaker.Stop()
	}
}

// voiceSTTLocked is the speech engine for the settings, built on first use;
// a resident engine starts on its first decode and unloads itself after
// voice.engines.sttIdleUnloadS without one.
func (d *Daemon) voiceSTTLocked(settings protocol.OniSettings) (stt.Transcriber, error) {
	v := &d.oni.voice
	want := settings.VoiceSettings.STT
	if v.stt != nil && v.sttKey == want {
		return v.stt, nil
	}
	if v.stt != nil {
		go v.stt.Close()
		v.stt = nil
	}
	t, err := d.voiceSTT(v.sp, want, v.profile.Name)
	if err != nil {
		return nil, sttRefusal(err)
	}
	v.stt, v.sttKey = t, want
	return t, nil
}

// voiceSpeakerLocked is the speaker for the settings, or nil when replies
// are not spoken or no voice is installed.
func (d *Daemon) voiceSpeakerLocked(settings protocol.OniSettings) *tts.Speaker {
	v := &d.oni.voice
	vs := settings.VoiceSettings
	if !vs.SpeakReplies {
		if v.speaker != nil {
			v.speaker.Stop()
		}
		v.speaker, v.speakerKey = nil, ""
		return nil
	}
	key := vs.TTS.Engine + "|" + v.character
	if v.speaker != nil && v.speakerKey == key {
		return v.speaker
	}
	if v.speaker != nil {
		v.speaker.Stop()
	}
	v.speaker, v.speakerKey = nil, ""
	engines := d.voiceVoices(vs.TTS, v.profile)
	if len(engines) == 0 {
		return nil
	}
	v.speaker = tts.NewSpeaker(tts.Options{Engines: engines, Open: d.voiceSink, SpeakReplies: true, Cache: tts.NewCache("", 0, 0), Report: d.voiceSpoken})
	v.speakerKey = key
	return v.speaker
}

// voiceFrame sends a voice frame through presence while it runs. Text that
// does not fit a frame is cut; a frame the contract refuses is not sent.
func (d *Daemon) voiceFrame(phase, source, text string) {
	svc := d.oni.service.Load()
	if svc == nil {
		return
	}
	if phase == feature.VoiceIdle {
		source = ""
	}
	_ = svc.Voice(feature.VoiceFrame{Phase: phase, Source: source, Text: oniText(text, oniBubbleMax)})
}

// voiceRestPhaseLocked is the phase with nothing pressed, said or worked
// on: listening while a wake session is open, as it takes any speech as a
// request until its ceiling; armed while only the wake phrase or open mic
// listens; else idle.
func (d *Daemon) voiceRestPhaseLocked() (phase, source string) {
	v := &d.oni.voice
	if v.listen != nil {
		if v.listen.source == feature.VoiceSourceWake && v.wake != nil && v.wake.InSession() {
			return feature.VoiceListening, v.listen.source
		}
		return feature.VoiceArmed, v.listen.source
	}
	return feature.VoiceIdle, ""
}

// voiceRestLocked shows the rest phase when turn id is still the current
// one and nothing is pressed or said.
func (d *Daemon) voiceRestLocked(id uint64) {
	v := &d.oni.voice
	if v.turn == nil || !v.turn.Accept(id) || v.press != nil || v.line != nil || v.asking != 0 && v.asking == id {
		return
	}
	phase, source := d.voiceRestPhaseLocked()
	d.voiceFrame(phase, source, "")
}

// voiceSayLocked says text for turn id after what is being said, or, with
// replies not spoken, shows it in the bubble. It reports whether it will
// be spoken.
func (d *Daemon) voiceSayLocked(settings protocol.OniSettings, id uint64, source, text string) bool {
	v := &d.oni.voice
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if d.voiceSpeakerLocked(settings) == nil || len(tts.Sentences(tts.Normalize(text))) == 0 {
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Bubble(0, "answer", oniText(text, oniBubbleMax))
		}
		return false
	}
	l := oniLine{turn: id, source: source, text: text}
	if v.line != nil {
		v.queue = append(v.queue, l)
		return true
	}
	d.voiceStartLineLocked(l)
	return true
}

func (d *Daemon) voiceStartLineLocked(l oniLine) {
	v := &d.oni.voice
	v.line = &l
	d.voiceFrame(feature.VoiceSpeaking, l.source, l.text)
	v.speaker.Say(l.turn, l.text)
}

// voiceSpoken hears the speaker: each sentence opens the echo window for
// what it said, and the end of a line starts the next one or rests.
func (d *Daemon) voiceSpoken(s tts.Spoken) {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.line == nil || s.Turn != v.line.turn {
		return
	}
	if s.Sentence != "" && s.Err == nil {
		v.turn.Said(s.Sentence, d.voiceClock().Now())
	}
	if !s.Done {
		return
	}
	v.line = nil
	for !s.Stopped && len(v.queue) > 0 && v.speaker != nil {
		next := v.queue[0]
		v.queue = v.queue[1:]
		if v.turn.Accept(next.turn) {
			d.voiceStartLineLocked(next)
			return
		}
	}
	v.queue = nil
	d.voiceRestLocked(s.Turn)
}

// voiceTalk serves oni.talk. The surface drawing Oni passes the held chord;
// every other caller, the app's button or a compositor bind running the
// CLI, is a toggle.
func (d *Daemon) voiceTalk(c *connection, p protocol.OniTalkParams) (protocol.OniVoiceResult, error) {
	switch p.Action {
	case protocol.OniVoiceStart:
		settings, err := d.voiceAllowed(feature.Voice)
		if err != nil {
			return protocol.OniVoiceResult{}, err
		}
		source := feature.VoiceSourceToggle
		d.oni.clientsMu.Lock()
		if d.oni.presenter == c {
			source = feature.VoiceSourceChord
		}
		d.oni.clientsMu.Unlock()
		if err := d.voicePress(settings, source, nil); err != nil {
			return protocol.OniVoiceResult{}, err
		}
		return protocol.OniVoiceResult{Phase: feature.VoiceListening}, nil
	case protocol.OniVoiceStop, protocol.OniVoiceCancel:
		v := &d.oni.voice
		v.mu.Lock()
		press := v.press
		v.mu.Unlock()
		if press == nil || press.dict != nil {
			v.mu.Lock()
			phase, _ := d.voiceRestPhaseLocked()
			v.mu.Unlock()
			return protocol.OniVoiceResult{Phase: phase}, nil
		}
		return protocol.OniVoiceResult{Phase: d.voiceFinish(press, p.Action == protocol.OniVoiceCancel)}, nil
	}
	return protocol.OniVoiceResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Unknown action %s.", p.Action))
}

// voicePress opens the microphone for one press: what Oni is saying stops,
// a new turn begins and an armed listener pauses until the press ends.
func (d *Daemon) voicePress(settings protocol.OniSettings, source string, dict *oniDictation) error {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.press != nil {
		if v.press.dict == nil && dict == nil {
			return nil
		}
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The microphone is already open for another press or a dictation."))
	}
	if err := d.voiceEnsureLocked(settings); err != nil {
		return err
	}
	engine, err := d.voiceSTTLocked(settings)
	if err != nil {
		return err
	}
	if w, ok := engine.(interface{ Warm() error }); ok {
		go func() { _ = w.Warm() }()
	}
	id := v.turn.Barge()
	if v.askCancel != nil {
		v.askCancel()
		v.askCancel = nil
	}
	d.voicePauseListenLocked()
	c, err := d.voiceCaptureLocked(settings, engine, vad.Hold, source, id, dict)
	if err != nil {
		d.voiceResumeListenLocked(settings)
		return err
	}
	v.press = c
	d.voiceFrame(feature.VoiceListening, source, "")
	return nil
}

// voiceCaptureLocked opens the microphone the settings name.
func (d *Daemon) voiceCaptureLocked(settings protocol.OniSettings, engine stt.Transcriber, mode vad.Mode, source string, id uint64, dict *oniDictation) (*oniCapture, error) {
	v := &d.oni.voice
	cfg := vad.FromSpec(v.sp, mode)
	if mode == vad.Wake {
		// the armed listener runs as long as wake may stay armed; its
		// ceilings are kept by the wake and open-mic timers
		cfg.MaxSession = time.Duration(v.sp.Voice.Wake.ArmedMaxS * float64(time.Second))
		if open := wake.OpenLimit(v.sp); cfg.MaxSession < open {
			cfg.MaxSession = open
		}
	}
	clock := d.voiceClock()
	seg, err := vad.New(cfg, clock.Now)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	pcm, stop, err := d.voiceCapture().Start(ctx, settings.VoiceSettings.Mic)
	if err != nil {
		cancel()
		return nil, micRefusal(err)
	}
	c := &oniCapture{
		source: source, listen: mode == vad.Wake, turn: id, gen: v.gen, ctx: ctx, cancel: cancel, stop: stop, seg: seg, dict: dict,
		engine: engine, limits: stt.LimitsFromSpec(v.sp), lang: settings.VoiceSettings.STT.Language, vocab: d.voiceVocab(v.profile.Name),
		read: make(chan struct{}),
	}
	go func() {
		defer close(c.read)
		d.voicePump(c, pcm)
	}()
	return c, nil
}

// voicePump feeds the capture to the segmenter and the meter until the
// recorder stops. Mic levels go out only while a press listens.
func (d *Daemon) voicePump(c *oniCapture, pcm <-chan []int16) {
	meter := audio.NewMeter(d.oni.voice.sp)
	for chunk := range pcm {
		events := c.seg.Push(chunk)
		if !c.listen && c.dict == nil {
			if svc := d.oni.service.Load(); svc != nil {
				for _, level := range meter.Push(chunk) {
					_, _ = svc.Level(feature.LevelFrame{Mic: level})
				}
			}
		}
		d.voiceEvents(c, events)
	}
}

// voiceEvents acts on segmenter events: a partial is decoded in the
// background and shown while the press is still open, a final is decoded
// in the background and kept in order, a cut ends the press.
func (d *Daemon) voiceEvents(c *oniCapture, events []vad.Event) {
	for _, ev := range events {
		switch ev.Kind {
		case vad.SpeechStart:
			c.stream = stt.NewStream(c.engine, c.limits, c.lang, c.vocab)
			if c.dict != nil {
				d.oni.voice.mu.Lock()
				if d.oni.voice.press == c {
					d.voiceFrame(feature.VoiceTranscribing, feature.VoiceSourceDictation, "")
				}
				d.oni.voice.mu.Unlock()
			}
		case vad.Partial:
			if c.listen || c.dict != nil || c.stream == nil {
				continue
			}
			go d.voicePartial(c, c.stream, ev.PCM)
		case vad.Final:
			stream := c.stream
			if stream == nil {
				stream = stt.NewStream(c.engine, c.limits, c.lang, c.vocab)
			}
			c.stream = nil
			secs := float64(len(ev.PCM)) / float64(stt.SampleRate)
			if c.listen {
				// the burst began secs before the segmenter closed it, which
				// is when Oni's own lines it may hold were being heard
				began := d.voiceClock().Now().Add(-time.Duration(secs * float64(time.Second)))
				prev, done := c.heard, make(chan struct{})
				c.heard = done
				go func(pcm []int16) {
					defer close(done)
					if prev != nil {
						<-prev
					}
					d.voiceHeard(c, stream, pcm, secs, began)
				}(ev.PCM)
				continue
			}
			ch := make(chan string, 1)
			c.finals = append(c.finals, ch)
			c.secs += secs
			go func(pcm []int16) {
				r, err := stream.Final(c.ctx, pcm)
				if err != nil {
					r.Text = ""
				}
				ch <- r.Text
			}(ev.PCM)
		case vad.Cut:
			if !c.listen {
				go d.voiceCut(c)
			}
		}
	}
}

// voiceCut ends a press the segmenter cut at its ceiling, as a stop would.
func (d *Daemon) voiceCut(c *oniCapture) {
	if c.dict != nil {
		// a dictation's words are typed only when the user stops it
		return
	}
	d.voiceFinish(c, false)
}

func (d *Daemon) voicePartial(c *oniCapture, stream *stt.Stream, pcm []int16) {
	r, ok, err := stream.Partial(c.ctx, pcm)
	if !ok || err != nil {
		return
	}
	text := hygiene.Collapse(r.Text)
	if strings.TrimSpace(text) == "" {
		return
	}
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.press == c && !c.settled {
		d.voiceFrame(feature.VoiceTranscribing, c.source, text)
	}
}

// voiceEnd closes c's microphone and waits for its reader, then takes the
// segment still open. It returns whether any segment was heard.
func (d *Daemon) voiceEnd(c *oniCapture, discard bool) bool {
	heard := false
	c.once.Do(func() {
		if discard {
			c.cancel()
		}
		_ = c.stop()
		<-c.read
		d.voiceEvents(c, c.seg.End())
		heard = len(c.finals) > 0
	})
	return heard
}

// voiceFinish ends a talk press: cancel drops it, stop transcribes what
// was heard and answers it in the background. It returns the phase the
// press left.
func (d *Daemon) voiceFinish(c *oniCapture, discard bool) string {
	v := &d.oni.voice
	v.mu.Lock()
	if v.press != c || c.settled {
		phase, _ := d.voiceRestPhaseLocked()
		v.mu.Unlock()
		return phase
	}
	c.settled = true
	v.press = nil
	v.mu.Unlock()
	heard := d.voiceEnd(c, discard)
	v.mu.Lock()
	defer v.mu.Unlock()
	settings := protocol.EffectiveOni(d.Settings())
	d.voiceResumeListenLocked(settings)
	if discard || !heard || c.gen != v.gen {
		c.cancel()
		d.voiceRestLocked(c.turn)
		phase, _ := d.voiceRestPhaseLocked()
		return phase
	}
	v.work++
	go d.voiceComplete(c)
	return feature.VoiceTranscribing
}

// voiceTranscript waits for c's finals in order and tidies them; "" when
// nothing plausible was said or the press was dropped.
func voiceTranscript(c *oniCapture) string {
	var parts []string
	for _, ch := range c.finals {
		select {
		case text := <-ch:
			if text = strings.TrimSpace(text); text != "" {
				parts = append(parts, text)
			}
		case <-c.ctx.Done():
			return ""
		}
	}
	text, ok := hygiene.Tidy(strings.Join(parts, " "), c.secs)
	if !ok {
		return ""
	}
	return text
}

// voiceComplete turns a press's speech into its transcript and routes it.
func (d *Daemon) voiceComplete(c *oniCapture) {
	v := &d.oni.voice
	defer func() {
		c.cancel()
		v.mu.Lock()
		v.work--
		v.mu.Unlock()
		d.stateChanged()
	}()
	text := voiceTranscript(c)
	v.mu.Lock()
	if c.gen != v.gen || !v.turn.Accept(c.turn) {
		v.mu.Unlock()
		return
	}
	if text == "" {
		d.voiceRestLocked(c.turn)
		v.mu.Unlock()
		return
	}
	d.voiceFrame(feature.VoiceTranscribing, c.source, text)
	v.mu.Unlock()
	d.voiceRoute(route.Utterance{Text: text, Source: routeSource(c.source), Turn: c.turn, At: d.voiceClock().Now()}, c.source)
}

// voiceRoute does what the router decided for one utterance.
func (d *Daemon) voiceRoute(u route.Utterance, source string) {
	v := &d.oni.voice
	v.mu.Lock()
	router, gen := v.router, v.gen
	v.mu.Unlock()
	if router == nil {
		return
	}
	if d.voiceGuardRoute(u, source, gen) {
		return
	}
	if d.voiceRewriteRoute(u, source, gen) {
		return
	}
	// the router may answer the consent card or move the guide on, which
	// take their own locks, so it runs outside the loop's
	dec := router.Route(u)
	settings := protocol.EffectiveOni(d.Settings())
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen != v.gen || !v.turn.Accept(u.Turn) {
		return
	}
	switch dec.Kind {
	case route.Answered:
		d.voiceSayLocked(settings, u.Turn, source, d.remindAnswer(dec))
	case route.NeedsPress:
		line := i18n.T(route.NeedsPressLine)
		switch dec.Line {
		case route.NextNeedsPressLine:
			line = i18n.T(route.NextNeedsPressLine)
		case route.ReflexNeedsPressLine:
			line = i18n.T(route.ReflexNeedsPressLine)
		case route.FirmNeedsButtonLine:
			line = i18n.T(route.FirmNeedsButtonLine)
		}
		d.voiceSayLocked(settings, u.Turn, source, line)
	case route.Dismissed:
		d.voiceHushLocked()
		if v.wake != nil {
			v.wake.EndSession()
		}
	case route.Reflexed:
		d.reflexDecidedLocked(u, source, gen, dec)
	case route.HandsTook:
		if dec.Ack != "" {
			d.voiceSayLocked(settings, u.Turn, source, dec.Ack)
		}
	case route.Asked:
		if dec.Ask == nil {
			break
		}
		if dec.Ack != "" {
			d.voiceSayLocked(settings, u.Turn, source, dec.Ack)
		}
		if v.askCancel != nil {
			v.askCancel()
		}
		ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
		v.askCancel = cancel
		v.asking = u.Turn
		v.work++
		go d.voiceAsk(ctx, cancel, *dec.Ask, source, gen, v.profile.Persona)
	case route.Guided:
		if dec.Ack != "" {
			d.voiceSayLocked(settings, u.Turn, source, dec.Ack)
		}
		v.work++
		go d.voiceGuideStart(dec.Goal, source, u.Turn, gen)
	case route.Noted:
		if dec.Note == nil {
			break
		}
		v.work++
		go d.voiceMemory(*dec.Note, dec.Press, u.Turn, source, gen)
	case route.Recalled:
		if dec.Recall == nil {
			break
		}
		if dec.Ack != "" {
			d.voiceSayLocked(settings, u.Turn, source, dec.Ack)
		}
		v.work++
		go d.voiceRecall(*dec.Recall, u.Text, u.Turn, source, gen)
	}
	d.voiceRestLocked(u.Turn)
}

// voiceAsk answers a voice question as an Ask turn: a temporary read-only
// Ask session of origin voice, run by the same machinery and on the same
// route as a typed Ask, with the voice block before the question. The
// first sentence of the reply is said (all of it when the user asked for
// more), and the whole of it shown in the bubble when replies are not
// spoken.
func (d *Daemon) voiceAsk(ctx context.Context, cancel context.CancelFunc, ask route.Ask, source string, gen uint64, persona string) {
	v := &d.oni.voice
	defer func() {
		cancel()
		v.mu.Lock()
		v.work--
		v.mu.Unlock()
		d.stateChanged()
	}()
	reply, err := d.voiceAskSession(ctx, ask, persona)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.asking == ask.Turn {
		v.asking = 0
	}
	if ctx.Err() != nil || gen != v.gen || !v.turn.Accept(ask.Turn) {
		return
	}
	settings := protocol.EffectiveOni(d.Settings())
	if err != nil {
		d.warn("oni voice ask: %v", err)
		d.voiceSayLocked(settings, ask.Turn, source, i18n.T("I could not get an answer."))
		d.voiceRestLocked(ask.Turn)
		return
	}
	if !d.voiceSayLocked(settings, ask.Turn, source, route.Spoken(reply, ask.Long)) {
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Bubble(0, "answer", oniText(reply, oniBubbleMax))
		}
	}
	d.voiceRestLocked(ask.Turn)
}

// voiceAskTarget hears an Ask session's events until its turn ends, and
// keeps why the turn failed when it did.
type voiceAskTarget struct {
	once sync.Once
	done chan struct{}
	mu   sync.Mutex
	// failed is set when the turn ended in an error; message is the last
	// error the session reported
	failed  bool
	message string
}

func (t *voiceAskTarget) sendEvent(name string, data any) bool {
	if name != protocol.EvtSessionEvent {
		return true
	}
	ev, ok := data.(protocol.SessionEventData)
	if !ok {
		return true
	}
	switch ev.Kind {
	case protocol.EvError:
		var e protocol.ErrorData
		if json.Unmarshal(ev.Data, &e) == nil && e.Message != "" {
			t.mu.Lock()
			t.message = e.Message
			t.mu.Unlock()
		}
	case protocol.EvTurnEnd:
		var end protocol.TurnEndData
		if json.Unmarshal(ev.Data, &end) == nil && end.Stop == protocol.StopError {
			t.mu.Lock()
			t.failed = true
			t.mu.Unlock()
		}
		t.once.Do(func() { close(t.done) })
	}
	return true
}

// failure is why the ended turn gave no answer, or nil.
func (t *voiceAskTarget) failure() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.failed {
		return nil
	}
	if t.message == "" {
		return errors.New("the ask turn failed")
	}
	return errors.New(t.message)
}

func (d *Daemon) voiceAskSession(ctx context.Context, ask route.Ask, persona string) (string, error) {
	params := ask.Session()
	params.Temporary = true
	info, err := d.manager.newSession(ctx, params)
	if err != nil {
		return "", err
	}
	defer func() { _ = d.manager.delete(info.ID) }()
	target := &voiceAskTarget{done: make(chan struct{})}
	if _, err := d.manager.attach(info.ID, info.LastSeq, target); err != nil {
		return "", err
	}
	defer d.manager.detach(info.ID, target)
	// the model hears the voice block with the question; the conversation
	// shows the question as it was said
	say := route.Prompt("", persona, ask.Long) + "\n\n" + ask.Question
	if _, _, err := d.manager.promptOptions(ctx, protocol.PromptParams{ID: info.ID, Text: ask.Question}, promptTurn{say: say}); err != nil {
		return "", err
	}
	select {
	case <-target.done:
	case <-ctx.Done():
		_ = d.manager.cancel(info.ID)
		return "", ctx.Err()
	}
	if err := target.failure(); err != nil {
		return "", err
	}
	last, err := d.manager.lastExchange(info.ID)
	if err == nil && strings.TrimSpace(last.answer) == "" {
		err = errors.New("the ask turn gave no answer")
	}
	return strings.TrimSpace(last.answer), err
}

// voiceSpeak serves oni.speak: stop is its only action.
func (d *Daemon) voiceSpeak(p protocol.OniSpeakParams) (protocol.OniVoiceResult, error) {
	if p.Action != protocol.OniVoiceStop {
		return protocol.OniVoiceResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Unknown action %s.", p.Action))
	}
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.line != nil {
		id := v.line.turn
		d.voiceHushLocked()
		d.voiceRestLocked(id)
	}
	if v.press != nil {
		return protocol.OniVoiceResult{Phase: feature.VoiceListening}, nil
	}
	phase, _ := d.voiceRestPhaseLocked()
	return protocol.OniVoiceResult{Phase: phase}, nil
}

// voiceConsent lets the router answer the card on screen: a picture's
// consent question, a guide step's among them, the reflex decision card,
// the focus guard's close card, the rewrite card, the offer to keep a
// finished guide or the offer to keep a run of quick commands as a
// routine. Only a valid Press answers any: wake phrase and open-mic speech
// never do.
type voiceConsent struct{ d *Daemon }

func (c voiceConsent) Pending() bool {
	a := &c.d.oni.ask
	a.mu.Lock()
	asking := a.pending != nil
	a.mu.Unlock()
	return asking || c.d.reflexCardWaiting() != "" || c.d.guardCardWaiting() != "" || c.d.rewriteCardWaiting() != "" || c.d.memoryCardWaiting() != "" || c.d.recallCardWaiting() != "" || c.d.guideOfferWaiting() != "" || c.d.routineOfferWaiting() != ""
}

func (c voiceConsent) Answer(p route.Press, allow bool) error {
	if !p.Valid() {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only a press of the talk key answers a question."))
	}
	// Rewrite and guard closes require their app card, never spoken approval.
	if c.d.rewriteCardWaiting() != "" || c.d.guardCardWaiting() != "" {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T(route.FirmNeedsButtonLine))
	}
	// a Hands step's picture is the button's alone: no spoken yes sends it
	q, err := c.d.oniConsentTake("", allow, func(q *oniQuestion) error {
		if q.hands != nil {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T(route.FirmNeedsButtonLine))
		}
		return nil
	})
	if err == nil {
		go func() {
			if _, err := c.d.oniConsentGive(q, allow); err != nil {
				c.d.warn("oni voice consent: %v", err)
			}
		}()
		return nil
	}
	if id := c.d.reflexCardWaiting(); id != "" {
		_, err := c.d.reflexCardTake(id, allow)
		return err
	}
	// a forget or a correction, and passages for a hosted route, go only
	// on a yes from the talk key
	if id := c.d.memoryCardWaiting(); id != "" {
		go func() {
			if _, err := c.d.memoryCardTake(id, allow); err != nil {
				c.d.warn("oni voice memory: %v", err)
			}
		}()
		return nil
	}
	if id := c.d.recallCardWaiting(); id != "" {
		go func() {
			if _, err := c.d.recallCardTake(id, allow, nil); err != nil {
				c.d.warn("oni voice recall: %v", err)
			}
		}()
		return nil
	}
	id := c.d.guideOfferWaiting()
	if id == "" {
		id = c.d.routineOfferWaiting()
	}
	if id == "" {
		return err
	}
	if allow {
		_, err = c.d.offerKeep(id)
		return err
	}
	return c.d.offerSkip(id)
}

// voicePresent makes svc the running presence with the voice phase as it
// is now, so a renderer that opens while the microphone is armed, open or
// Oni is speaking starts from that, the mic mark included.
func (d *Daemon) voicePresent(svc *presence.Service) {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	d.oni.service.Store(svc)
	switch {
	case v.press != nil:
		d.voiceFrame(feature.VoiceListening, v.press.source, "")
	case v.line != nil:
		d.voiceFrame(feature.VoiceSpeaking, v.line.source, v.line.text)
	case v.listen != nil:
		d.voiceFrame(feature.VoiceArmed, v.listen.source, "")
	}
}

// voiceResident reports whether the voice loop keeps the daemon up at
// rest: only the wake phrase or open mic armed does.
func (d *Daemon) voiceResident() bool {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.listen != nil || v.wake != nil || v.open != nil
}

// voiceBusy reports a press, a dictation, a line being said, or a
// transcript or an answer on its way.
func (d *Daemon) voiceBusy() bool {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.press != nil || v.line != nil || v.work > 0
}

// voiceSettings follows the settings: voice off ends everything the loop
// started; the wake phrase and open mic arm and disarm with their
// switches; a changed engine or voice is rebuilt on next use.
func (d *Daemon) voiceSettings(settings protocol.Settings) {
	eff := protocol.EffectiveOni(settings)
	if !feature.Decide(feature.Request{Feature: feature.Voice}, eff).Allow {
		d.voiceOff()
		return
	}
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.sp == nil && !eff.VoiceSettings.Wake.Enabled && !eff.VoiceSettings.OpenMic {
		// nothing was ever built: build nothing now either
		return
	}
	if err := d.voiceEnsureLocked(eff); err != nil {
		d.warn("oni voice: %v", err)
		return
	}
	if v.stt != nil && v.sttKey != eff.VoiceSettings.STT {
		go v.stt.Close()
		v.stt = nil
	}
	if v.speaker != nil {
		d.voiceSpeakerLocked(eff)
	}
	d.voiceArmLocked(eff)
}

// voiceOff stops everything the voice loop started and forgets its
// engines: the microphone closes, speech stops, its timers are disarmed
// and a resident speech engine is unloaded. Kept reminders are the
// reminders' own and ring as a bubble while voice is off.
func (d *Daemon) voiceOff() {
	v := &d.oni.voice
	v.mu.Lock()
	if v.sp == nil {
		v.mu.Unlock()
		return
	}
	v.gen++
	press := v.press
	v.press = nil
	if press != nil {
		press.settled = true
	}
	d.voiceDisarmLocked()
	d.voiceHushLocked()
	if v.turn != nil {
		v.turn.Barge()
	}
	if v.askCancel != nil {
		v.askCancel()
		v.askCancel = nil
	}
	engine := v.stt
	v.stt, v.sttKey = nil, protocol.OniSTTSettings{}
	v.speaker, v.speakerKey = nil, ""
	v.router, v.character = nil, ""
	d.voiceFrame(feature.VoiceIdle, "", "")
	v.mu.Unlock()
	if press != nil {
		d.voiceEnd(press, true)
	}
	if engine != nil {
		_ = engine.Close()
	}
}

// voiceSettingsPatch turns a voice switch off in the settings, as when wake
// or open mic reached its ceiling: arming again is the user's choice.
func (d *Daemon) voiceSettingsPatch(patch map[string]any) {
	raw, err := json.Marshal(map[string]any{"oni": map[string]any{"voiceSettings": patch}})
	if err != nil {
		return
	}
	if _, err := d.applySettings(context.Background(), raw); err != nil {
		d.warn("oni voice settings: %v", err)
	}
}
