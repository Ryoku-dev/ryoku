package reflex

import (
	"testing"
	"time"
)

func launches(p Plan) int {
	n := 0
	for _, a := range p.Actions {
		if starts[a.Intent] {
			n++
		}
	}
	return n
}

func TestSessionLaunchCooldown(t *testing.T) {
	lim := limits()
	var s Session
	p, ok := s.Plan("open firefox", testNow, testCat)
	if !ok || launches(p) != 1 {
		t.Fatalf("first: %+v %v", p, ok)
	}
	p, ok = s.Plan("open firefox", testNow.Add(lim.LaunchCooldown()-time.Millisecond), testCat)
	if !ok || len(p.Actions) != 0 || p.Line != lineAlready {
		t.Fatalf("inside the cooldown: %+v %v", p, ok)
	}
	// Another target is not held by the first one's cooldown.
	if p, _ = s.Plan("open files", testNow.Add(time.Second), testCat); launches(p) != 1 {
		t.Fatalf("other app: %+v", p)
	}
	if p, _ = s.Plan("open firefox", testNow.Add(lim.LaunchCooldown()), testCat); launches(p) != 1 {
		t.Fatalf("after the cooldown: %+v", p)
	}
}

// A decoder loop heard as 43 utterances in a row starts one app once.
func TestSessionRepeatedUtteranceLaunchesOnce(t *testing.T) {
	var s Session
	total := 0
	for i := range 43 {
		p, _ := s.Plan("open firefox", testNow.Add(time.Duration(i)*50*time.Millisecond), testCat)
		total += launches(p)
	}
	if total != 1 {
		t.Fatalf("%d launches", total)
	}
}

func TestSessionPerMinuteCap(t *testing.T) {
	lim := limits()
	var s Session
	say := []string{"turn it up", "turn it down"}
	for i := range lim.MaxActionsPerMinute {
		p, _ := s.Plan(say[i%2], testNow.Add(time.Duration(i)*time.Second), testCat)
		if len(p.Actions) != 1 {
			t.Fatalf("action %d held: %+v", i, p)
		}
	}
	p, ok := s.Plan("mute", testNow.Add(30*time.Second), testCat)
	if !ok || len(p.Actions) != 0 || p.Line != lineEnough {
		t.Fatalf("over the cap: %+v %v", p, ok)
	}
	if p, _ = s.Plan("mute", testNow.Add(time.Minute+time.Second), testCat); len(p.Actions) != 1 {
		t.Fatalf("a minute later: %+v", p)
	}
}

func TestSessionDanglingVerb(t *testing.T) {
	lim := limits()
	var s Session
	p, ok := s.Plan("open", testNow, testCat)
	if !ok || len(p.Actions) != 0 || p.Line != "" {
		t.Fatalf("verb alone: %+v %v", p, ok)
	}
	p, ok = s.Plan("firefox", testNow.Add(lim.DanglingVerb()-time.Second), testCat)
	if !ok || len(p.Actions) != 1 || p.Actions[0].Arg != "Firefox" {
		t.Fatalf("object in time: %+v %v", p, ok)
	}
	// Used once: the next bare word plans nothing.
	if _, ok = s.Plan("files", testNow.Add(lim.DanglingVerb()), testCat); ok {
		t.Fatal("the verb carried twice")
	}

	s = Session{}
	s.Plan("open", testNow, testCat)
	if _, ok = s.Plan("firefox", testNow.Add(lim.DanglingVerb()+time.Second), testCat); ok {
		t.Fatal("the verb outlived danglingVerbS")
	}

	// A full command after the verb stands on its own.
	s = Session{}
	s.Plan("search for", testNow, testCat)
	p, _ = s.Plan("turn it up", testNow.Add(time.Second), testCat)
	if len(p.Actions) != 1 || p.Actions[0].Intent != VolumeUp {
		t.Fatalf("own command: %+v", p)
	}
	// A clock that steps back never carries a verb.
	s = Session{}
	s.Plan("open", testNow, testCat)
	if _, ok = s.Plan("firefox", testNow.Add(-time.Second), testCat); ok {
		t.Fatal("verb carried back in time")
	}
}

func TestSessionEarlyFire(t *testing.T) {
	lim := limits()
	var s Session
	a, ok := s.Early("open firefox", testNow, testCat)
	if !ok || a.Intent != OpenApp || a.Confidence < lim.EarlyFireMin {
		t.Fatalf("exact app: %+v %v", a, ok)
	}
	// The full transcript that follows does not open it again.
	if p, _ := s.Plan("open firefox", testNow.Add(time.Second), testCat); launches(p) != 0 {
		t.Fatalf("opened twice: %+v", p)
	}
	if a, ok = s.Early("open youtube", testNow, testCat); !ok || a.Intent != OpenSite {
		t.Fatalf("site: %+v %v", a, ok)
	}
	// Only opening fires early.
	for _, say := range []string{"turn it up", "take a screenshot", "search for cats", "open"} {
		if _, ok := s.Early(say, testNow, testCat); ok {
			t.Errorf("%q fired early", say)
		}
	}
	// An app that only sounds close waits for the end of the sentence.
	weak := "open firfx"
	p, _ := Parse(weak, testNow, nil, testCat)
	if len(p.Actions) != 1 || p.Actions[0].Confidence >= lim.EarlyFireMin {
		t.Fatalf("fixture: %+v", p)
	}
	if _, ok := (&Session{}).Early(weak, testNow, testCat); ok {
		t.Error("a weak match fired early")
	}
}
