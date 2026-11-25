package daemon

import (
	"os"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

// These tests drive guides by voice over the daemon's own socket, with the
// voice fakes, the guide's text reader and settle pictures faked too.

// stepFileLong is a step longer than the voice discipline allows aloud.
const stepFileLong = "Open the File menu at the top left of the window.\n[POINT:0.1,0.05,0.1,0.05:File][CONF:0.9]"

// frameLog reads a peer's oni frames, keeping every voice frame it passes.
type frameLog struct {
	p      *socketPeer
	voices []feature.VoiceFrame
}

func (l *frameLog) until(what string, match func(feature.Payload) bool) feature.Payload {
	l.p.t.Helper()
	return l.p.await(what, func(f feature.Payload) bool {
		if v, ok := f.(*feature.VoiceFrame); ok {
			l.voices = append(l.voices, *v)
		}
		return match(f)
	})
}

// quiet reads frames for a moment and fails on one that matches.
func (l *frameLog) quiet(what string, match func(feature.Payload) bool) {
	l.p.t.Helper()
	timeout := time.After(150 * time.Millisecond)
	for {
		select {
		case e := <-l.p.conn.Events():
			if e.Event != protocol.EvtOni {
				continue
			}
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				l.p.t.Fatalf("the daemon sent a frame the contract refuses: %v: %s", err, e.Data)
			}
			if match(f) {
				l.p.t.Fatalf("%s: %s", what, e.Data)
			}
		case <-timeout:
			return
		}
	}
}

func spoken(text string) func(feature.Payload) bool {
	return func(f feature.Payload) bool {
		v, ok := f.(*feature.VoiceFrame)
		return ok && v.Phase == feature.VoiceSpeaking && v.Text == text
	}
}

func anySpoken(f feature.Payload) bool {
	v, ok := f.(*feature.VoiceFrame)
	return ok && v.Phase == feature.VoiceSpeaking
}

func consentFor(id string) func(feature.Payload) bool {
	return func(f feature.Payload) bool {
		c, ok := f.(*feature.ConsentFrame)
		return ok && c.Feature == id
	}
}

// chord says text with the talk chord held. With open mic armed, the
// listener's feed comes back once the press ends; it is returned.
func (r *voiceRig) chord(t *testing.T, surface *socketPeer, text string, open bool) chan []int16 {
	t.Helper()
	r.ears.hear(nil, text)
	talk(t, surface, protocol.OniVoiceStart)
	say(r.mic.next(t))
	time.Sleep(50 * time.Millisecond)
	talk(t, surface, protocol.OniVoiceStop)
	if open {
		return r.mic.next(t)
	}
	return nil
}

// overheard is open-mic speech.
func (r *voiceRig) overheard(listen chan []int16, text string) {
	r.ears.hear(nil, text)
	say(listen)
}

// voiceGuideRig serves voice with open mic armed and guides on, Oni perched
// on the editor.
func voiceGuideRig(t *testing.T, keepOn, speak bool) (r *voiceRig, surface *socketPeer, log *frameLog, listen chan []int16) {
	t.Helper()
	r = serveVoice(t, func(s *protocol.Settings) {
		s.Oni.VoiceSettings.OpenMic = true
		s.Oni.VoiceSettings.SpeakReplies = speak
	}, nil)
	r.guideOn(t, keepOn)
	r.tess.set(fileWord, saveWord)
	surface, _ = r.presenting(t)
	listen = r.mic.next(t)
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	r.desk.await(t, 1, "0x1")
	return r, surface, &frameLog{p: surface}, listen
}

// A spoken question about the perched window starts a guide toward it, not
// an Ask. Its picture's consent question, a next and the keep offer are
// answered only by a press: open-mic yes and next are told to press and
// change nothing. Open mic may have the strand thrown again. Each step is
// said within the voice discipline while the card shows it whole.
func TestOniVoiceGuideMovesOnlyFromAPress(t *testing.T) {
	r, surface, log, listen := voiceGuideRig(t, true, true)
	r.model.reply(stepFileLong)
	listen = r.chord(t, surface, "How do I save this file?", true)
	if c := log.until("the step's consent question", consentFor(feature.Guide)).(*feature.ConsentFrame); c.Class != feature.ConsentDisclosure {
		t.Fatalf("consent = %+v", c)
	}
	if len(r.model.sent()) != 0 || len(r.questions()) != 0 {
		t.Fatalf("before any yes: %d pictures sent, asked %q", len(r.model.sent()), r.questions())
	}

	// open mic cannot say yes
	r.overheard(listen, "yes")
	log.until("the press line", spoken(route.NeedsPressLine))
	if !(voiceConsent{r.d}).Pending() || len(r.model.sent()) != 0 {
		t.Fatal("open-mic speech answered the step's consent question")
	}
	// the chord can, and the step goes out
	listen = r.chord(t, surface, "yes", true)
	card := log.until("step 1's card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame)
	if card.Text != "Open the File menu at the top left of the window." {
		t.Fatalf("the card shows %q", card.Text)
	}
	log.until("step 1 said", spoken("Open the File menu."))
	sent := r.model.sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Messages[1].Content, "How do I save this file?") {
		t.Fatalf("requests = %+v", sent)
	}
	if err := surface.arrived(card.Target, ""); err != nil {
		t.Fatal(err)
	}
	log.until("step 1 waiting", guidePhase(feature.GuideWaiting, 1))

	// open mic cannot move it on
	r.overheard(listen, "next")
	log.until("the next press line", spoken(route.NextNeedsPressLine))
	time.Sleep(50 * time.Millisecond)
	if r.screen.frames.Load() != 0 || len(r.model.sent()) != 1 || !r.guideRunning() {
		t.Fatalf("open-mic next moved the guide: %d settle frames, %d reads", r.screen.frames.Load(), len(r.model.sent()))
	}
	// but may have the strand thrown again
	r.overheard(listen, "circle it")
	again := log.until("the strand again", func(f feature.Payload) bool {
		s, ok := f.(*feature.StrandTargetFrame)
		return ok && s.Seq != card.Target && s.Mode == feature.StrandModeLasso
	}).(*feature.StrandTargetFrame)
	if err := surface.arrived(again.Seq, ""); err != nil {
		t.Fatal(err)
	}
	log.until("step 1 waiting again", guidePhase(feature.GuideWaiting, 1))
	if len(r.model.sent()) != 1 {
		t.Fatal("throwing the strand again read the window")
	}

	// the chord's next goes on: the window changed, step 2 reads
	r.screen.level.Store(120)
	r.model.reply(stepSave)
	listen = r.chord(t, surface, "next", true)
	r.pump(t, func() bool { return len(r.model.sent()) == 2 })
	card = log.until("step 2's card", guidePhase(feature.GuidePointing, 2)).(*feature.GuideFrame)
	log.until("step 2 said", spoken("Press Save."))
	if err := surface.arrived(card.Target, ""); err != nil {
		t.Fatal(err)
	}
	log.until("step 2 waiting", guidePhase(feature.GuideWaiting, 2))

	// "done" from the chord finishes it, and the keep offer waits
	r.screen.level.Store(200)
	listen = r.chord(t, surface, "done", true)
	r.pump(t, func() bool { return r.d.guideOfferWaiting() != "" })
	offer := log.until("the keep offer", consentFor(feature.GuideKeep)).(*feature.ConsentFrame)
	r.overheard(listen, "yes")
	log.until("the press line for the offer", spoken(route.NeedsPressLine))
	if _, err := os.Stat(keep.DefaultStorePath()); !os.IsNotExist(err) || r.d.guideOfferWaiting() != offer.ID {
		t.Fatalf("open-mic speech answered the keep offer: %v", err)
	}
	r.chord(t, surface, "yes", true)
	if a := log.until("the offer's answer", func(f feature.Payload) bool {
		a, ok := f.(*feature.ConsentAnswerFrame)
		return ok && a.ID == offer.ID
	}).(*feature.ConsentAnswerFrame); a.Answer != "yes" {
		t.Fatalf("the offer was answered %q", a.Answer)
	}
	if list := keptEntries(t); len(list) != 1 || len(list[0].Steps) != 2 {
		t.Fatalf("kept %+v", list)
	}
}

// Open-mic "stop" dismisses a guide whose consent question waits, and one
// pointing; a spoken no from the chord declines the picture. With replies
// not spoken, a step is shown and never said.
func TestOniVoiceGuideStopsFromAnySourceAndSaysNothingWhenQuiet(t *testing.T) {
	r, surface, log, listen := voiceGuideRig(t, false, false)

	// stop while the consent question waits: nothing is sent
	r.model.reply(stepFile)
	listen = r.chord(t, surface, "where is the save button", true)
	log.until("the consent question", consentFor(feature.Guide))
	r.overheard(listen, "stop")
	r.awaitNoGuide(t)
	if (voiceConsent{r.d}).Pending() || len(r.model.sent()) != 0 || !r.shots.cleared() {
		t.Fatal("a dismissed guide left its picture or sent it")
	}

	// a spoken no from the chord declines it
	listen = r.chord(t, surface, "how do I save this file", true)
	log.until("the consent question again", consentFor(feature.Guide))
	listen = r.chord(t, surface, "no", true)
	r.awaitNoGuide(t)
	if len(r.model.sent()) != 0 || !r.shots.cleared() {
		t.Fatal("a declined picture was sent or kept")
	}

	// with the route said yes to, the step is shown and, replies quiet,
	// not said; open-mic stop ends it and takes the strand back
	r.allowRoute()
	listen = r.chord(t, surface, "how do I save this file", true)
	card := log.until("step 1's card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame)
	if card.Text != "Open the File menu." {
		t.Fatalf("the card shows %q", card.Text)
	}
	log.quiet("a step was said with replies quiet", anySpoken)
	r.overheard(listen, "stop")
	log.until("the guide winding back", guidePhase(feature.GuideWinding, 0))
	r.awaitNoGuide(t)
	for _, v := range log.voices {
		if v.Phase == feature.VoiceSpeaking {
			t.Fatalf("Oni spoke with replies quiet: %+v", log.voices)
		}
	}
	if len(r.questions()) != 0 {
		t.Fatalf("a guide question was asked as an Ask: %q", r.questions())
	}
}

// Without a perch, or with guides off, the same question is an Ask.
func TestOniVoiceScreenQuestionIsAnAskWithoutAGuide(t *testing.T) {
	r := serveVoice(t, nil, nil)
	r.guideOn(t, false)
	surface, _ := r.presenting(t)
	var log oniLog
	r.answer("Use the File menu.")
	r.chord(t, surface, "how do I save this file", false)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Use the File menu."))

	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	r.desk.await(t, 1, "0x1")
	r.patch(t, `{"oni":{"guide":{"enabled":false}}}`)
	r.answer("Press Save.")
	r.chord(t, surface, "how do I save this file", false)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Press Save."))
	if asked := r.questions(); len(asked) != 2 || len(r.model.sent()) != 0 || r.guideRunning() {
		t.Fatalf("asked %q, %d pictures sent", asked, len(r.model.sent()))
	}
}
