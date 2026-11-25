package daemon

import (
	"fmt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guide"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

// Speech and guides. With Oni perched and guides on, a question about
// doing something on screen starts a guide toward what was said. Only a
// press of the talk key moves a guide on or answers its pictures' consent
// question or the offer to keep it; wake phrase and open-mic speech never
// do, though any of them may stop it or have its strand thrown again. A
// guide a spoken question started has its card lines said aloud when
// replies are spoken, each kept to the voice discipline while the card
// shows the line in full.

// voiceGuide lets the router move the running guide. Each call runs on the
// guide's queue; the router calls them with no voice lock held.
type voiceGuide struct{ d *Daemon }

func (g voiceGuide) Active() bool {
	d := g.d
	ch := make(chan bool, 1)
	d.guideDo(func() { ch <- d.guideLive(d.oni.guide.run) || d.oni.guide.walk != nil })
	return <-ch
}

func (g voiceGuide) Next(p route.Press) error {
	if !p.Valid() {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only a press of the talk key moves a guide on."))
	}
	d := g.d
	type took struct{ run, walk bool }
	ch := make(chan took, 1)
	d.guideDo(func() {
		run := d.oni.guide.run
		if !d.guideLive(run) {
			ch <- took{walk: d.oni.guide.walk != nil}
			return
		}
		d.guideHandle(run, guide.Advance{By: guide.ByVoice})
		ch <- took{run: true}
	})
	switch t := <-ch; {
	case t.run:
		return nil
	case t.walk:
		go func() {
			if _, ok, err := d.guideNextWalk(nil, p.Source().String()); ok && err != nil {
				d.warn("oni voice guide: %v", err)
			}
		}()
		return nil
	}
	return voiceNoGuide()
}

func (g voiceGuide) Circle() error {
	d := g.d
	ch := make(chan bool, 1)
	d.guideDo(func() {
		run := d.oni.guide.run
		if !d.guideLive(run) {
			ch <- false
			return
		}
		d.guideHandle(run, guide.Circle{})
		ch <- true
	})
	if !<-ch {
		return voiceNoGuide()
	}
	return nil
}

func (g voiceGuide) Stop() { g.d.guideUnwind("voice") }

func voiceNoGuide() error {
	return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("No guide is running. Start one first."))
}

// voiceGuidePerched reports whether a screen question should start a
// guide: Oni perched on a window, with guides allowed.
func (d *Daemon) voiceGuidePerched() bool {
	if guideRefusal(protocol.EffectiveOni(d.Settings()), feature.Guide) != nil {
		return false
	}
	svc := d.oni.service.Load()
	if svc == nil {
		return false
	}
	_, ok := svc.Perched()
	return ok
}

// voiceGuideStart starts the guide a spoken question asked for. Its card
// carries what happens next; a guide that cannot start says so.
func (d *Daemon) voiceGuideStart(goal, source string, id, gen uint64) {
	v := &d.oni.voice
	defer func() {
		v.mu.Lock()
		v.work--
		v.mu.Unlock()
		d.stateChanged()
	}()
	_, err := d.guideBegin(nil, goal, source)
	if err == nil {
		return
	}
	d.warn("oni voice guide: %v", err)
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen != v.gen || !v.turn.Accept(id) {
		return
	}
	d.voiceSayLocked(protocol.EffectiveOni(d.Settings()), id, source, i18n.T("I could not start it."))
	d.voiceRestLocked(id)
}

// voiceGuideFrame says f's line aloud for a guide a spoken question
// started, when replies are spoken and the line is new: a step's opening,
// another line's first sentence when it keeps the voice discipline, else a
// short cue; the card shows each in full. On the guide's queue: the voice
// loop's lock it takes is never held while waiting on the queue.
func (d *Daemon) voiceGuideFrame(run *guideRun, f feature.GuideFrame) {
	if run.voice == "" || f.Text == "" {
		return
	}
	key := fmt.Sprintf("%d\x00%s\x00%s", f.Step, f.Kicker, f.Text)
	if key == run.said {
		return
	}
	run.said = key
	line := voiceGuideLine(f)
	if line == "" {
		return
	}
	settings, err := d.voiceAllowed(feature.Voice)
	if err != nil || !settings.VoiceSettings.SpeakReplies {
		return
	}
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.turn == nil || v.router == nil || d.voiceSpeakerLocked(settings) == nil {
		return
	}
	d.voiceSayLocked(settings, v.turn.ID(), run.voice, line)
}

// voiceGuideLine is what is said of a guide card: the step's opening
// (route.Lead), or the line's first sentence when it fits the voice
// discipline, else a cue to read the card.
func voiceGuideLine(f feature.GuideFrame) string {
	var cue string
	switch f.Phase {
	case feature.GuidePointing, feature.GuideWaiting:
		if l := route.Lead(f.Text); l != "" {
			return l
		}
		return i18n.T("Look where I point.")
	case feature.GuideExplaining:
		cue = i18n.T("See the card.")
	case feature.GuideDone:
		cue = i18n.T("The guide is over.")
	default:
		return ""
	}
	if s := route.Sentences(f.Text); len(s) > 0 && route.Fits(s[0]) {
		return s[0]
	}
	return cue
}
