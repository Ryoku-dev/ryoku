package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

type sightGestureTestEvent struct {
	ms    int
	code  uint16
	value int32
	want  string
}

func TestSightTripleTapTransitions(t *testing.T) {
	for _, test := range []struct {
		name   string
		events []sightGestureTestEvent
	}{
		{"triple tap fires", []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, "prewarm"}, {200, 100, 1, ""}, {250, 100, 0, "fire"}}},
		{"inclusive duration and gap", []sightGestureTestEvent{{0, 100, 1, ""}, {250, 100, 0, ""}, {600, 100, 1, ""}, {850, 100, 0, "prewarm"}, {1200, 100, 1, ""}, {1450, 100, 0, "fire"}}},
		{"slow taps", []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {401, 100, 1, ""}, {450, 100, 0, ""}, {801, 100, 1, ""}, {850, 100, 0, ""}}},
		{"chord in between", []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, "prewarm"}, {200, 30, 1, "cancel"}, {220, 30, 0, ""}, {250, 100, 1, ""}, {300, 100, 0, ""}}},
		{"another key already held", []sightGestureTestEvent{{0, 30, 1, ""}, {10, 100, 1, ""}, {20, 100, 0, ""}, {30, 30, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, ""}}},
		{"another key during tap", []sightGestureTestEvent{{0, 100, 1, ""}, {10, 30, 1, ""}, {20, 30, 0, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, ""}}},
		{"long press", []sightGestureTestEvent{{0, 100, 1, ""}, {251, 100, 0, ""}, {300, 100, 1, ""}, {350, 100, 0, ""}, {400, 100, 1, ""}, {450, 100, 0, "prewarm"}}},
		{"autorepeat invalidates tap", []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, "prewarm"}, {200, 100, 1, ""}, {220, 100, 2, "cancel"}, {250, 100, 0, ""}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := newSightTap(100)
			start := time.Unix(100, 0)
			for _, event := range test.events {
				if got := machine.event(1, event.code, event.value, start.Add(time.Duration(event.ms)*time.Millisecond)); got != event.want {
					t.Fatalf("event %#v = %q, want %q", event, got, event.want)
				}
			}
		})
	}
}

func TestSightPrewarmTimeoutAndLock(t *testing.T) {
	for _, action := range []string{"timeout", "lock", "unplug"} {
		t.Run(action, func(t *testing.T) {
			machine := newSightTap(100)
			start := time.Unix(100, 0)
			for _, event := range []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, "prewarm"}} {
				machine.event(1, event.code, event.value, start.Add(time.Duration(event.ms)*time.Millisecond))
			}
			var got string
			switch action {
			case "timeout":
				if got := machine.tick(start.Add(500 * time.Millisecond)); got != "" {
					t.Fatalf("cancelled at inclusive boundary: %q", got)
				}
				got = machine.tick(start.Add(501 * time.Millisecond))
			case "lock":
				got = machine.lock(true)
			case "unplug":
				got = machine.removeDevice(1)
			}
			if got != "cancel" {
				t.Fatalf("%s = %q", action, got)
			}
			if got := machine.tick(start.Add(time.Second)); got != "" {
				t.Fatalf("cancel fired twice: %q", got)
			}
		})
	}
}

func TestSightThirdTapCancelsActiveSelection(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: preferred, Width: 100, Height: 80, Scale: 1}}, &preferred)
	ctx := context.Background()
	id := s.beginCapture(ctx)
	if !s.fireCapture(ctx, id) {
		t.Fatal("selection did not open")
	}
	var pending string
	var workers sync.WaitGroup
	taps := newSightTap(100)
	start := time.Unix(100, 0)
	for _, event := range []sightGestureTestEvent{{0, 100, 1, ""}, {50, 100, 0, ""}, {100, 100, 1, ""}, {150, 100, 0, "prewarm"}, {200, 100, 1, ""}, {250, 100, 0, "fire"}} {
		s.gestureTap(ctx, taps.event(1, event.code, event.value, start.Add(time.Duration(event.ms)*time.Millisecond)), &pending, &workers)
		if event.want == "prewarm" && (s.frame.Select == nil || s.frame.Select.ID != id) {
			t.Fatal("prewarm replaced existing selection")
		}
	}
	workers.Wait()
	if s.frame.Select != nil || s.capture != nil {
		t.Fatal("third tap retained active selection")
	}
}

func TestSightHighlightTransitions(t *testing.T) {
	for _, test := range []struct {
		name                      string
		otherKey, primary, locked bool
		want                      bool
	}{
		{"hold primary release", false, true, false, true},
		{"hold with another key", true, true, false, false},
		{"release without primary change", false, false, false, false},
		{"locked", false, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newSightHighlightGesture(56)
			start := time.Unix(100, 0)
			h.lock(test.locked)
			h.event(1, 56, 1, start)
			if test.otherKey {
				h.event(1, 15, 1, start.Add(20*time.Millisecond))
			}
			if test.primary {
				h.primary(start.Add(50 * time.Millisecond))
			}
			h.event(1, 56, 0, start.Add(100*time.Millisecond))
			if got := h.tick(start.Add(199 * time.Millisecond)); got {
				t.Fatal("read before PRIMARY settled")
			}
			if got := h.tick(start.Add(200 * time.Millisecond)); got != test.want {
				t.Fatalf("read = %v, want %v", got, test.want)
			}
			if h.tick(start.Add(time.Second)) {
				t.Fatal("read twice")
			}
		})
	}
}

func TestSightHighlightQuietPeriodCapAndClearing(t *testing.T) {
	h := newSightHighlightGesture(56)
	start := time.Unix(100, 0)
	h.event(1, 56, 1, start)
	if h.primary(start.Add(50 * time.Millisecond)) {
		t.Fatal("PRIMARY during hold clears")
	}
	h.event(1, 56, 0, start.Add(100*time.Millisecond))
	for ms := 150; ms <= 1550; ms += 100 {
		if !h.primary(start.Add(time.Duration(ms) * time.Millisecond)) {
			t.Fatal("PRIMARY outside hold does not clear")
		}
		if h.tick(start.Add(time.Duration(ms) * time.Millisecond)) {
			t.Fatal("read before cap or quiet period")
		}
	}
	if !h.tick(start.Add(1600 * time.Millisecond)) {
		t.Fatal("PRIMARY wait exceeded 1.5 second cap")
	}

	s := &sightService{topic: newStateTopic(), frame: sightFrame{Highlight: &sightHighlight{ID: "h1"}}}
	if h.event(1, 30, 1, start.Add(2*time.Second)) {
		s.clearHighlight()
	}
	if s.frame.Highlight != nil {
		t.Fatal("key press retained highlight")
	}
	if h.tick(start.Add(3 * time.Second)) {
		t.Fatal("key press retained pending highlight")
	}
}

func TestSightHighlightOpenAndDismiss(t *testing.T) {
	s := &sightService{topic: newStateTopic(), frame: sightFrame{Cards: []sightCard{}}}
	if _, err := s.openHighlight("stale", "translate", 10, 20, "DP-1"); err == nil || err.Error() != "stale-highlight" {
		t.Fatalf("stale id = %v", err)
	}
	s.frame.Highlight = &sightHighlight{ID: "h1", Text: "stored text", Language: "en", Output: "DP-2", Created: 123}
	s.dismissHighlight("stale")
	if s.frame.Highlight == nil {
		t.Fatal("stale dismiss cleared another highlight")
	}
	if _, err := s.openHighlight("h1", "invalid", 10, 20, "DP-1"); err == nil {
		t.Fatal("accepted invalid mode")
	}
	result, err := s.openHighlight("h1", "ask", 10, 20, "DP-1")
	if err != nil {
		t.Fatal(err)
	}
	card := s.frame.Cards[0]
	if s.frame.Highlight != nil || card.Kind != "text" || card.Text != "stored text" || card.Language != "en" || card.Mode != "ask" || card.X != 10 || card.Y != 20 || card.Output != "DP-1" || card.Plate || card.Image != "" || result.(map[string]any)["id"] != card.ID {
		t.Fatalf("opened card = %#v, result %#v", card, result)
	}
	if _, err := s.openHighlight("h1", "ask", 10, 20, "DP-1"); err == nil || err.Error() != "stale-highlight" {
		t.Fatalf("consumed id = %v", err)
	}
	s.frame.Highlight = &sightHighlight{ID: "h2"}
	s.dismissHighlight("h2")
	if s.frame.Highlight != nil {
		t.Fatal("dismiss retained highlight")
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(s.topic.last, &frame); err != nil {
		t.Fatal(err)
	}
	if string(frame["highlight"]) != "null" {
		t.Fatalf("highlight JSON = %s", frame["highlight"])
	}
}

func TestSightGestureKeySettings(t *testing.T) {
	options := sightOptionsFromFrame([]byte(`{"sight":{"tapKey":"Control_R","highlightKey":"Shift_L"}}`))
	if options.TapKey != "Control_R" || options.HighlightKey != "Shift_L" {
		t.Fatalf("options = %#v", options)
	}
	key := sightGestureKey(options.TapKey, "Alt_R")
	if key.Code != 97 {
		t.Fatalf("gesture key = %#v", key)
	}
	want := sightOptionsFromFrame([]byte(`{}`))
	got := sightOptionsFromFrame([]byte(`{"sight":{"tapKey":"nope","highlightKey":"Super_L"}}`))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unknown keys did not fall back: %#v", got)
	}
}

func TestPrimaryChangeHookCoalescesWithoutReading(t *testing.T) {
	p := &primarySelectionState{}
	changes := p.notifications()
	w := &clipWatcher{offers: make(map[uint32]*clipOffer)}
	p.set(w)
	for _, id := range []uint32{0, 0, 0} {
		if err := w.handleDeviceEvent(wlMessage{opcode: dataDevicePrimaryEvent, payload: wlUint(id)}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-changes:
	default:
		t.Fatal("PRIMARY change did not notify")
	}
	if p.lastChange().IsZero() {
		t.Fatal("PRIMARY timestamp not retained")
	}
	select {
	case <-changes:
		t.Fatal("notification was not coalesced")
	default:
	}
}

func TestSightHighlightQueuedPrimaryAndChord(t *testing.T) {
	h := newSightHighlightGesture(56)
	start := time.Unix(100, 0)
	// The PRIMARY queue can be drained before the earlier keyboard press.
	h.primary(start.Add(50 * time.Millisecond))
	h.event(1, 56, 1, start)
	h.event(1, 56, 0, start.Add(100*time.Millisecond))
	if !h.tick(start.Add(200 * time.Millisecond)) {
		t.Fatal("queued PRIMARY change was lost")
	}
	h.event(1, 29, 1, start.Add(time.Second))
	h.event(2, 56, 1, start.Add(1100*time.Millisecond))
	h.primary(start.Add(1150 * time.Millisecond))
	h.event(2, 56, 0, start.Add(1200*time.Millisecond))
	if h.tick(start.Add(1400 * time.Millisecond)) {
		t.Fatal("hold began with another device's key already held")
	}
}

// Chromium sets PRIMARY on mouse-up; releasing the modifier a moment before the
// button must still count, while a later or interrupted change must not.
func TestSightHighlightLatePrimaryAfterRelease(t *testing.T) {
	start := time.Unix(100, 0)
	release := func() *sightHighlightGesture {
		h := newSightHighlightGesture(56)
		h.event(1, 56, 1, start)
		h.event(1, 56, 0, start.Add(100*time.Millisecond))
		return h
	}
	h := release()
	if h.primary(start.Add(300 * time.Millisecond)) {
		t.Fatal("a PRIMARY change inside the grace window cleared instead of counting")
	}
	if h.tick(start.Add(449 * time.Millisecond)) {
		t.Fatal("read before PRIMARY settled")
	}
	if !h.tick(start.Add(450 * time.Millisecond)) {
		t.Fatal("late PRIMARY change was lost")
	}
	h = release()
	if !h.primary(start.Add(501*time.Millisecond)) || h.tick(start.Add(time.Second)) {
		t.Fatal("a PRIMARY change after the grace window counted")
	}
	h = release()
	h.event(1, 30, 1, start.Add(200*time.Millisecond))
	if !h.primary(start.Add(300*time.Millisecond)) || h.tick(start.Add(time.Second)) {
		t.Fatal("a key press did not end the grace window")
	}
}

func TestSightHighlightWireShape(t *testing.T) {
	wire, err := json.Marshal(sightHighlight{ID: "h1", Text: "Some text", Language: "en", Output: "DP-1", Created: 123})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"id": "h1", "text": "Some text", "language": "en", "output": "DP-1", "created": float64(123)}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("highlight = %s", wire)
	}
}

func TestSightPrimaryReaderTextAndEmpty(t *testing.T) {
	for _, selected := range []string{"Цей текст українською мовою пояснює, як працює система.", ""} {
		t.Run(selected, func(t *testing.T) {
			client, server := clipSocketPair(t)
			defer client.Close()
			defer server.Close()
			watcher := &clipWatcher{conn: client, primaryID: 10, offers: map[uint32]*clipOffer{10: {types: []string{"text/plain"}}}}
			primary := &primarySelectionState{}
			primary.set(watcher)
			received := make(chan struct{})
			go func() {
				defer close(received)
				request, fd := fakeReadFDRequest(t, server)
				file := os.NewFile(uintptr(fd), "sight-primary")
				defer file.Close()
				if request.object != 10 || request.opcode != dataOfferReceiveRequest {
					t.Error("Sight did not request the primary selection")
					return
				}
				if _, err := file.WriteString(selected); err != nil {
					t.Error(err)
				}
			}()
			text, err := primary.read()
			<-received
			if selected == "" {
				if err == nil || err.Error() != "no-selection" {
					t.Fatalf("empty PRIMARY = %q, %v", text, err)
				}
			} else if err != nil || text != selected {
				t.Fatalf("PRIMARY = %q, %v", text, err)
			}
		})
	}
}
