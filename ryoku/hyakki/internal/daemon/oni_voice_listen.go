package daemon

import (
	"fmt"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/voice/hygiene"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/vad"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/protocol"
)

// The wake phrase and open mic keep the microphone open between presses,
// and so keep the daemon up, only while armed. Their ceilings are kept by
// one timer at the next deadline, armed only while they are; reaching one
// disarms and turns the switch off, so arming again is the user's choice.
// Speech that did not follow the wake phrase is dropped where it is heard:
// no frame, no log line, nothing kept.

// voiceArmLocked arms or disarms the wake phrase and open mic to match
// the settings. The caller holds v.mu.
func (d *Daemon) voiceArmLocked(eff protocol.OniSettings) {
	v := &d.oni.voice
	vs := eff.VoiceSettings
	if !vs.Wake.Enabled && !vs.OpenMic {
		if v.wake != nil || v.open != nil || v.listen != nil {
			d.voiceDisarmLocked()
			if v.press == nil && v.line == nil {
				d.voiceFrame(feature.VoiceIdle, "", "")
			}
		}
		return
	}
	key := fmt.Sprintf("%t|%s|%t|%s|%+v", vs.Wake.Enabled, vs.Wake.Phrase, vs.OpenMic, vs.Mic, vs.STT)
	if key == v.armKey && (v.wake != nil || v.open != nil) {
		return
	}
	d.voiceDisarmLocked()
	clock := d.voiceClock()
	if vs.Wake.Enabled {
		cfg, err := wake.FromSpec(v.sp, vs.Wake.Phrase)
		if err == nil {
			var w *wake.Wake
			if w, err = wake.New(cfg, clock.Now, d.warn); err == nil {
				w.Arm()
				v.wake = w
			}
		}
		if err != nil {
			d.warn("oni wake: %v", err)
		}
	}
	if vs.OpenMic {
		o, err := wake.NewOpen(wake.OpenLimit(v.sp), clock.Now, d.warn)
		if err != nil {
			d.warn("oni open mic: %v", err)
		} else {
			o.Start()
			v.open = o
			v.turn.Barge()
		}
	}
	if v.wake == nil && v.open == nil {
		return
	}
	v.armKey = key
	d.voiceResumeListenLocked(eff)
	if v.listen == nil {
		// the microphone or the speech engine is missing: nothing listens,
		// so nothing is armed
		d.voiceDisarmLocked()
		return
	}
	d.voiceArmTimerLocked()
	if v.press == nil && v.line == nil {
		d.voiceFrame(feature.VoiceArmed, v.listen.source, "")
	}
}

// voiceDisarmLocked closes the listener's microphone and forgets the wake
// phrase and open mic. The caller holds v.mu.
func (d *Daemon) voiceDisarmLocked() {
	v := &d.oni.voice
	d.voicePauseListenLocked()
	if v.wake != nil {
		v.wake.Disarm()
		v.wake = nil
	}
	if v.open != nil {
		v.open.Stop()
		v.open = nil
	}
	if v.armAt != nil {
		v.armAt.Stop()
		v.armAt = nil
	}
	v.armKey = ""
}

// voicePauseListenLocked closes the listener's microphone, keeping the wake
// phrase and open mic armed, while a press or a dictation holds it.
func (d *Daemon) voicePauseListenLocked() {
	v := &d.oni.voice
	l := v.listen
	if l == nil {
		return
	}
	v.listen = nil
	l.settled = true
	go d.voiceEnd(l, true)
}

// voiceListenSourceLocked is what the listener hears for: open mic while
// it is on, else the wake phrase; "" when neither is armed. It reads the
// deadlines, which apply no ceiling, so a ceiling is always met by the
// timer or a burst and turns its switch off.
func (d *Daemon) voiceListenSourceLocked() string {
	switch {
	case !d.voiceOpenDeadlineLocked().IsZero():
		return feature.VoiceSourceOpen
	case !d.voiceWakeDeadlineLocked().IsZero():
		return feature.VoiceSourceWake
	}
	return ""
}

// voiceResumeListenLocked opens the listener's microphone again when the
// wake phrase or open mic is armed and no press holds the microphone.
func (d *Daemon) voiceResumeListenLocked(eff protocol.OniSettings) {
	v := &d.oni.voice
	if v.press != nil || v.listen != nil || v.sp == nil {
		return
	}
	source := d.voiceListenSourceLocked()
	if source == "" {
		return
	}
	engine, err := d.voiceSTTLocked(eff)
	if err != nil {
		d.warn("oni voice: the microphone stays closed: %v", err)
		return
	}
	c, err := d.voiceCaptureLocked(eff, engine, vad.Wake, source, v.turn.ID(), nil)
	if err != nil {
		d.warn("oni voice: the microphone stays closed: %v", err)
		return
	}
	v.listen = c
}

// voiceArmTimerLocked arms the one timer at the next wake or open-mic
// deadline; none while neither is armed.
func (d *Daemon) voiceArmTimerLocked() {
	v := &d.oni.voice
	if v.armAt != nil {
		v.armAt.Stop()
		v.armAt = nil
	}
	var next time.Time
	for _, t := range []time.Time{d.voiceWakeDeadlineLocked(), d.voiceOpenDeadlineLocked()} {
		if !t.IsZero() && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	if next.IsZero() {
		return
	}
	clock := d.voiceClock()
	gen := v.gen
	v.armAt = clock.AfterFunc(max(0, next.Sub(clock.Now())), func() { d.voiceTick(gen) })
}

func (d *Daemon) voiceWakeDeadlineLocked() time.Time {
	if w := d.oni.voice.wake; w != nil {
		return w.Deadline()
	}
	return time.Time{}
}

func (d *Daemon) voiceOpenDeadlineLocked() time.Time {
	if o := d.oni.voice.open; o != nil {
		return o.Deadline()
	}
	return time.Time{}
}

// voiceTick applies the wake and open-mic ceilings at their deadline.
func (d *Daemon) voiceTick(gen uint64) {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen != v.gen {
		return
	}
	v.armAt = nil
	var notices []wake.Notice
	if v.wake != nil {
		notices = append(notices, v.wake.Tick())
	}
	if v.open != nil {
		notices = append(notices, v.open.Tick())
	}
	d.voiceNoticesLocked(notices...)
}

// voiceNoticesLocked acts on ceilings reached: wake disarmed or open mic
// ended turns its switch off; with neither left the microphone closes.
// Otherwise the listener goes on and the timer moves to the next deadline.
func (d *Daemon) voiceNoticesLocked(notices ...wake.Notice) {
	v := &d.oni.voice
	patch := map[string]any{}
	for _, n := range notices {
		switch n {
		case wake.NoticeDisarmed:
			v.wake = nil
			patch["wake"] = map[string]any{"enabled": false}
		case wake.NoticeOpenEnded:
			v.open = nil
			patch["openMic"] = false
		}
	}
	if len(patch) > 0 {
		go d.voiceSettingsPatch(patch)
	}
	source := d.voiceListenSourceLocked()
	if source == "" {
		d.voiceDisarmLocked()
		if v.press == nil && v.line == nil {
			d.voiceFrame(feature.VoiceIdle, "", "")
		}
		go d.stateChanged()
		return
	}
	if v.listen != nil {
		v.listen.source = source
	}
	d.voiceArmTimerLocked()
	if v.press == nil && v.line == nil && v.listen != nil {
		d.voiceFrame(feature.VoiceArmed, source, "")
	}
}

// voiceHeard takes one burst the listener heard, which began at began. Open
// mic lets it through; the wake phrase lets it through only after the
// phrase, inside the session the phrase opened. Anything else is dropped
// here and leaves no trace. Oni's own speech heard back is dropped too: a
// line counts while its echo window was open when the burst began, as the
// transcript arrives only after the burst has ended and been decoded.
func (d *Daemon) voiceHeard(c *oniCapture, stream *stt.Stream, pcm []int16, secs float64, began time.Time) {
	r, err := stream.Final(c.ctx, pcm)
	if err != nil {
		return
	}
	text, ok := hygiene.Tidy(r.Text, secs)
	if !ok {
		return
	}
	v := &d.oni.voice
	v.mu.Lock()
	if v.listen != c || c.gen != v.gen {
		v.mu.Unlock()
		return
	}
	// the ceilings first, so one reached turns its switch off
	var notices []wake.Notice
	if v.wake != nil {
		notices = append(notices, v.wake.Tick())
	}
	if v.open != nil {
		notices = append(notices, v.open.Tick())
	}
	for _, n := range notices {
		if n != wake.NoticeNone {
			d.voiceNoticesLocked(notices...)
			break
		}
	}
	if v.listen != c {
		v.mu.Unlock()
		return
	}
	now := d.voiceClock().Now()
	var heard wake.Heard
	source := d.voiceListenSourceLocked()
	switch source {
	case feature.VoiceSourceOpen:
		heard = v.open.Hear(text)
	case feature.VoiceSourceWake:
		was := v.wake.InSession()
		heard = v.wake.Hear(text)
		if heard.Woke && !was {
			v.turn.Barge()
		}
	}
	d.voiceArmTimerLocked()
	if heard.Text == "" {
		if heard.Woke {
			d.voiceFrame(feature.VoiceListening, source, "")
		}
		v.mu.Unlock()
		return
	}
	if v.turn.Heard(heard.Text, began) {
		v.mu.Unlock()
		return
	}
	id := v.turn.ID()
	d.voiceFrame(feature.VoiceTranscribing, source, heard.Text)
	v.work++
	v.mu.Unlock()
	d.voiceRoute(route.Utterance{Text: heard.Text, Source: routeSource(source), Turn: id, At: now}, source)
	v.mu.Lock()
	v.work--
	v.mu.Unlock()
}
