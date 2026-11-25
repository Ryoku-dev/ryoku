package presence

import (
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

func voice(phase, source, text string) feature.VoiceFrame {
	return feature.VoiceFrame{Phase: phase, Source: source, Text: text}
}

// The voice phase derives the character state the spec names, and the voice
// frame always goes out ahead of what it sets off.
func TestVoicePhasesDeriveTheSpecsStates(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	s, _, rec := started(t)
	steps := []struct {
		frame feature.VoiceFrame
		state string
	}{
		{voice(feature.VoiceListening, feature.VoiceSourceChord, ""), "listening"},
		{voice(feature.VoiceTranscribing, feature.VoiceSourceChord, "what time is it"), "thinking"},
		{voice(feature.VoiceSpeaking, feature.VoiceSourceChord, "It's three."), "speaking"},
		{voice(feature.VoiceIdle, "", ""), "idle"},
	}
	for _, step := range steps {
		if want, _, ok := sp.VoiceState(step.frame.Phase); !ok || (want != step.state && want != spec.NoState) {
			t.Fatalf("the spec maps %s to %q, this test expects %q", step.frame.Phase, want, step.state)
		}
		if err := s.Voice(step.frame); err != nil {
			t.Fatal(err)
		}
		want := encode(t, &step.frame, &feature.StateFrame{State: step.state})
		if got := rec.take(); !slices.Equal(got, want) {
			t.Fatalf("%s sent %q, want %q", step.frame.Phase, got, want)
		}
		if snap := s.Snapshot(); snap[0].(*feature.StateFrame).State != step.state || *snap[len(snap)-1].(*feature.VoiceFrame) != step.frame {
			t.Fatalf("after %s the snapshot is %q", step.frame.Phase, encode(t, snap...))
		}
	}
	if err := s.Voice(voice(feature.VoiceIdle, "", "")); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an unchanged phase sent %q", got)
	}
}

func TestArmedShowsTheMicMarkAndNoState(t *testing.T) {
	s, clock, rec := started(t)
	if s.MicMark() {
		t.Fatal("the mic mark shows with the microphone closed")
	}
	for _, source := range []string{feature.VoiceSourceWake, feature.VoiceSourceOpen} {
		armed := voice(feature.VoiceArmed, source, "")
		if err := s.Voice(armed); err != nil {
			t.Fatal(err)
		}
		if got := rec.take(); !slices.Equal(got, encode(t, &armed)) {
			t.Fatalf("arming %s sent %q", source, got)
		}
		if !s.MicMark() {
			t.Fatalf("armed for %s without the mic mark", source)
		}
		if st := s.Snapshot()[0].(*feature.StateFrame); st.State != "idle" {
			t.Fatalf("armed changed the state to %+v", st)
		}
	}
	// a hidden Oni still owes the mic mark: the voice frame reaches the
	// presenter whatever the hidden setting says
	hidden := on()
	hidden.Hidden = true
	s.Settings(hidden)
	rec.take()
	if !s.MicMark() || s.Snapshot()[len(s.Snapshot())-1].(*feature.VoiceFrame).Phase != feature.VoiceArmed {
		t.Fatal("hiding Oni dropped the mic mark")
	}
	s.Settings(on())
	rec.take()
	if err := s.Voice(voice(feature.VoiceIdle, "", "")); err != nil {
		t.Fatal(err)
	}
	if s.MicMark() {
		t.Fatal("the mic mark outlived the microphone")
	}
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed after disarming; only the quiet spell may be", clock.Armed())
	}
}

// A press wakes a sleeping Oni, and the voice frame that closes the
// renderer's sound gate comes before the wake gesture's motif.
func TestAPressWakesOniAfterTheGateCloses(t *testing.T) {
	s, clock, rec := started(t)
	clock.Advance(SleepAfter)
	rec.take()
	listening := voice(feature.VoiceListening, feature.VoiceSourceChord, "")
	if err := s.Voice(listening); err != nil {
		t.Fatal(err)
	}
	want := encode(t, &listening, &feature.GestureFrame{Seq: 2, ID: "wake"}, &feature.StateFrame{State: "listening"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("a press on a sleeping Oni sent %q, want %q", got, want)
	}
	if clock.Armed() != 0 {
		t.Fatal("a quiet spell runs while the user talks")
	}
	clock.Advance(10 * SleepAfter)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni fell asleep while listening: %q", got)
	}
	if err := s.Voice(voice(feature.VoiceIdle, "", "")); err != nil {
		t.Fatal(err)
	}
	if clock.Armed() != 1 {
		t.Fatal("the quiet spell did not start over once the press ended")
	}
}

func TestADictationsWordsNeverReachAFrame(t *testing.T) {
	s, _, rec := started(t)
	const words = "my pin is four four one two"
	if err := s.Voice(voice(feature.VoiceListening, feature.VoiceSourceDictation, "")); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{feature.VoiceTranscribing, feature.VoiceSpeaking, feature.VoiceListening, feature.VoiceArmed} {
		if err := s.Voice(voice(phase, feature.VoiceSourceDictation, words)); err == nil {
			t.Fatalf("a %s frame took a dictation's words", phase)
		}
	}
	if err := s.Voice(voice(feature.VoiceTranscribing, feature.VoiceSourceDictation, "")); err != nil {
		t.Fatal(err)
	}
	for _, f := range append(rec.take(), encode(t, s.Snapshot()...)...) {
		if strings.Contains(f, "four") {
			t.Fatalf("a frame carried the dictation: %s", f)
		}
	}
	if st := s.Snapshot()[0].(*feature.StateFrame); st.State != "thinking" {
		t.Fatalf("a dictation being transcribed shows %+v", st)
	}
}

func TestLevelsGoOutAtMostAtTheSpecRateWhileListeningOrSpeaking(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Duration(float64(time.Second) / sp.Voice.Levels.MaxHz); LevelEvery != want || sp.Voice.Levels.MaxHz > 15 {
		t.Fatalf("LevelEvery %v, spec %v Hz", LevelEvery, sp.Voice.Levels.MaxHz)
	}
	s, clock, rec := started(t)
	level := feature.LevelFrame{Mic: 0.5}
	push := func() int {
		sent := 0
		for range 100 { // a second of 10 ms frames
			ok, err := s.Level(level)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				sent++
			}
			clock.Advance(10 * time.Millisecond)
		}
		return sent
	}
	for _, f := range []feature.VoiceFrame{
		voice(feature.VoiceIdle, "", ""),
		voice(feature.VoiceArmed, feature.VoiceSourceWake, ""),
		voice(feature.VoiceTranscribing, feature.VoiceSourceChord, "hello"),
	} {
		if err := s.Voice(f); err != nil {
			t.Fatal(err)
		}
		rec.take()
		armed := clock.Armed()
		if n := push(); n != 0 {
			t.Fatalf("%d levels went out in the %s phase", n, f.Phase)
		}
		if clock.Armed() != armed {
			t.Fatal("a dropped level armed a timer")
		}
	}
	for _, f := range []feature.VoiceFrame{
		voice(feature.VoiceListening, feature.VoiceSourceChord, ""),
		voice(feature.VoiceSpeaking, feature.VoiceSourceChord, "It's three."),
	} {
		if err := s.Voice(f); err != nil {
			t.Fatal(err)
		}
		rec.take()
		start := clock.Now()
		n := push()
		if hz := float64(n) / clock.Now().Sub(start).Seconds(); n == 0 || hz > sp.Voice.Levels.MaxHz {
			t.Fatalf("%d levels in a second of %s, %.1f Hz", n, f.Phase, hz)
		}
		frames := decodeAll(t, rec.take())
		if len(frames) != n || slices.ContainsFunc(frames, func(p feature.Payload) bool { return p.Kind() != feature.KindLevel }) {
			t.Fatalf("%s sent %q for %d levels", f.Phase, kinds(frames), n)
		}
		if clock.Armed() != 0 {
			t.Fatal("levels armed a timer")
		}
		if slices.Contains(kinds(s.Snapshot()), feature.KindLevel) {
			t.Fatal("a level is in the snapshot")
		}
	}
	if _, err := s.Level(feature.LevelFrame{Mic: 2}); err == nil {
		t.Fatal("a level above 1 was taken")
	}
	hidden := on()
	hidden.Hidden = true
	s.Settings(hidden)
	rec.take()
	clock.Advance(time.Second)
	if ok, _ := s.Level(level); ok {
		t.Fatal("a level went to a hidden Oni")
	}
	off := on()
	off.Master = false
	s.Settings(off)
	rec.take()
	if ok, _ := s.Level(level); ok || len(rec.take()) != 0 {
		t.Fatal("a level went out with Oni off")
	}
}

// With the voice loop never reporting, presence is as before: the same
// snapshot, no level and no timer beyond the quiet spell.
func TestVoiceOffChangesNothing(t *testing.T) {
	s, clock, rec := started(t)
	snap := s.Snapshot()
	if got, want := kinds(snap), []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}; !slices.Equal(got, want) {
		t.Fatalf("snapshot kinds = %q", got)
	}
	if out := encode(t, snap[4]); out[0] != `{"kind":"voice","data":{"phase":"idle"}}` {
		t.Fatalf("voice = %s", out[0])
	}
	if clock.Armed() != 1 || s.MicMark() {
		t.Fatalf("%d timers, mic mark %v with voice off", clock.Armed(), s.MicMark())
	}
	s.Activity(Activity{Session: "ask-1", State: protocol.StateWorking})
	s.Activity(Activity{Session: "ask-1", State: protocol.StateIdle})
	for _, p := range decodeAll(t, rec.take()) {
		if p.Kind() == feature.KindVoice || p.Kind() == feature.KindLevel {
			t.Fatalf("a %s frame went out with voice off", p.Kind())
		}
	}
}

// The motion frame names the talk chord only while voice is on, so a
// presenter's key reader starts and stops with the voice switch.
func TestTheTalkChordRidesTheMotionFrameOnlyWhileVoiceIsOn(t *testing.T) {
	s, _, rec := started(t)
	if m := s.Snapshot()[2].(*feature.MotionFrame); m.Chord != "" {
		t.Fatalf("voice off, motion = %+v", m)
	}
	next := on()
	next.Voice, next.Chord = true, "alt+super+t"
	s.Settings(next)
	want := encode(t, &feature.MotionFrame{Reduce: protocol.OniReduceFollow, Courtesy: true, Chord: "alt+super+t"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("turning voice on sent %q, want %q", got, want)
	}
	next.Voice = false
	s.Settings(next)
	want = encode(t, &feature.MotionFrame{Reduce: protocol.OniReduceFollow, Courtesy: true})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("turning voice off sent %q, want %q", got, want)
	}
}
