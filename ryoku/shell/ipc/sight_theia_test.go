package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	wm "ryoku-wm"
)

func TestTheiaOptionDefaultsAndNormalization(t *testing.T) {
	defaults := sightOptionsFromFrame(nil)
	if !defaults.On || !defaults.Enabled || !defaults.Highlight || !defaults.InPlace.Paste || defaults.InPlace.Key != "Control_R" || defaults.InPlace.Action != "auto" || defaults.ReadAloud.Rate != 1 || len(defaults.Features) != 0 || len(defaults.Quiet.Apps) != 0 || defaults.Quiet.Snooze != 0 {
		t.Fatalf("defaults = %#v", defaults)
	}
	for _, test := range []struct {
		input  string
		rate   float64
		snooze int
		action string
		key    string
	}{
		{`{"sight":{"inplace":{"key":"bad","action":"bad","paste":false},"quiet":{"snooze":12},"readaloud":{"rate":0.1}}}`, 0.75, 0, "auto", "Control_R"},
		{`{"sight":{"inplace":{"key":"Shift_R","action":"tone:warm"},"quiet":{"snooze":15},"readaloud":{"rate":9}}}`, 1.5, 15, "tone:warm", "Shift_R"},
		{`{"sight":{"inplace":{"action":"proofread"},"quiet":{"snooze":60},"readaloud":{"rate":1.25}}}`, 1.25, 60, "proofread", "Control_R"},
	} {
		got := sightOptionsFromFrame([]byte(test.input))
		if got.ReadAloud.Rate != test.rate || got.Quiet.Snooze != test.snooze || got.InPlace.Action != test.action || got.InPlace.Key != test.key {
			t.Errorf("options(%s) = %#v", test.input, got)
		}
	}
	got := sightOptionsFromFrame([]byte(`{"sight":{"on":false,"inplace":{"paste":false},"quiet":{"apps":[" org.example.Editor ","","org.example.Editor","browser"]}}}`))
	if got.On || got.InPlace.Paste || !reflect.DeepEqual(got.Quiet.Apps, []string{"org.example.Editor", "browser"}) {
		t.Fatalf("explicit options = %#v", got)
	}
}

func TestTheiaFeatureAndWorkerGates(t *testing.T) {
	o := sightOptionsFromFrame([]byte(`{"sight":{"enabled":false,"highlight":false}}`))
	if o.gestures() || o.feature("inplace") || o.feature("readaloud") {
		t.Fatal("new features are on by default")
	}
	o.Features["inplace"] = true
	if !o.gestures() || !o.feature("inplace") || o.feature("unknown") {
		t.Fatal("in-place-only gestures were not enabled")
	}
	o.On = false
	if o.gestures() || o.feature("inplace") {
		t.Fatal("master off retained a feature")
	}
	base := sightOptionsFromFrame(nil)
	for _, mutate := range []func(*sightOptions){
		func(o *sightOptions) { o.On = false },
		func(o *sightOptions) { o.Enabled = false },
		func(o *sightOptions) { o.Highlight = false },
		func(o *sightOptions) { o.TapKey = "Shift_R" },
		func(o *sightOptions) { o.HighlightKey = "Shift_L" },
		func(o *sightOptions) { o.Features["inplace"] = true },
		func(o *sightOptions) { o.InPlace.Key = "Control_L" },
	} {
		changed := sightOptionsFromFrame(nil)
		mutate(&changed)
		if !sightGestureOptionsChanged(base, changed) {
			t.Fatalf("worker change missed: %#v", changed)
		}
	}
	changed := sightOptionsFromFrame(nil)
	changed.Features["summarize"] = true
	if sightGestureOptionsChanged(base, changed) {
		t.Fatal("an unrelated feature restarts the keyboard reader")
	}
}

func TestTheiaQuietMatchingAndExpiry(t *testing.T) {
	s := &sightService{topic: newStateTopic(), options: sightOptionsFromFrame([]byte(`{"sight":{"features":{"quiet":true},"quiet":{"apps":["editor"]}}}`)), daemon: &daemon{wmWindows: []wm.Window{{AppID: "ignored", FocusOrder: 2}, {AppID: "editor", FocusOrder: 0}}}}
	if s.focusedApp() != "editor" || !s.quietBlocked() {
		t.Fatal("focused app exclusion was missed")
	}
	s.daemon.wmWindows[1].AppID = "browser"
	if s.quietBlocked() {
		t.Fatal("quiet apps matched a different id")
	}
	if s.quietNow("") != "browser" || !s.quietBlocked() {
		t.Fatal("Not now did not snooze the focused app")
	}
	s.quietBlockedLocked("terminal", time.Now())
	if s.snoozedApp != "" || s.quietBlockedLocked("browser", time.Now()) {
		t.Fatal("until-app-change snooze survived a focus change")
	}
	s.options.Quiet.Snooze = 15
	s.quietNow("browser")
	until := s.snoozedUntil
	if until.IsZero() || !s.quietBlockedLocked("browser", until.Add(-time.Nanosecond)) || s.quietBlockedLocked("browser", until) || s.snoozedApp != "" {
		t.Fatal("timed snooze did not expire at its deadline")
	}
	s.options.Quiet.Snooze = 60
	s.quietNow("browser")
	s.quietBlockedLocked("terminal", time.Now())
	if !s.quietBlockedLocked("browser", time.Now()) {
		t.Fatal("timed snooze was cleared by a focus change")
	}
	s.options.Features["quiet"] = false
	if s.quietBlockedLocked("editor", time.Now()) || s.quietBlockedLocked("browser", time.Now()) {
		t.Fatal("disabled quiet places still block gestures")
	}
}

func TestTheiaInPlaceTripleTapAndReplacement(t *testing.T) {
	s := &sightService{topic: newStateTopic(), options: sightOptionsFromFrame([]byte(`{"sight":{"features":{"inplace":true},"inplace":{"action":"rewrite"}}}`))}
	taps := newSightTap(sightGestureKey(s.options.InPlace.Key, "Control_R").Code)
	start := time.Unix(100, 0)
	for i := range 3 {
		now := start.Add(time.Duration(i) * 150 * time.Millisecond)
		taps.event(1, taps.key, 1, now)
		action := taps.event(1, taps.key, 0, now.Add(50*time.Millisecond))
		if i < 2 && action == "fire" {
			t.Fatal("in-place fired before the third tap")
		}
		if i == 2 {
			if action != "fire" {
				t.Fatalf("third tap = %q", action)
			}
			s.publishInPlace("The text is here and you can read it.", "editor", "DP-1", s.inplaceEpoch)
		}
	}
	first := s.frame.InPlace
	if first == nil || !strings.HasPrefix(first.ID, "p") || first.Text == "" || first.Language != "en" || first.App != "editor" || first.Action != "rewrite" || first.Output != "DP-1" || first.Created == 0 {
		t.Fatalf("in-place = %#v", first)
	}
	s.publishInPlace("", "editor", "DP-1", s.inplaceEpoch)
	if s.frame.InPlace != first {
		t.Fatal("empty PRIMARY replaced in-place")
	}
	s.publishInPlace("This is the next text and it is ready.", "editor", "DP-2", s.inplaceEpoch)
	if s.frame.InPlace.ID == first.ID || s.frame.InPlace.Output != "DP-2" {
		t.Fatal("a new fire did not replace in-place")
	}
	s.inplaceDone(first.ID)
	if s.frame.InPlace == nil {
		t.Fatal("a stale dismissal cleared a newer result")
	}
	s.inplaceDone(s.frame.InPlace.ID)
	if s.frame.InPlace != nil {
		t.Fatal("inplaceDone did not clear the result")
	}
	s.publishInPlace("The text is here.", "editor", "DP-1", s.inplaceEpoch-1)
	if s.frame.InPlace != nil {
		t.Fatal("a stale PRIMARY read republished in-place")
	}
	s.options.On = false
	s.publishInPlace("The text is here.", "editor", "DP-1", s.inplaceEpoch)
	if s.frame.InPlace != nil {
		t.Fatal("master off published in-place")
	}
	s.options.On = true
	s.options.Features["quiet"] = true
	s.options.Quiet.Apps = []string{"editor"}
	s.daemon = &daemon{wmWindows: []wm.Window{{AppID: "editor", FocusOrder: 0}}}
	s.publishInPlace("The text is here.", "editor", "DP-1", s.inplaceEpoch)
	if s.frame.InPlace != nil {
		t.Fatal("a quiet app published in-place")
	}
}

func TestTheiaModesAndCardSources(t *testing.T) {
	for _, mode := range []string{"summarize", "deepread", "solve", "xy", "abcdefghijklmnop"} {
		if sightMode(mode, "explain") != mode {
			t.Errorf("rejected mode %q", mode)
		}
	}
	for _, mode := range []string{"", "x", "UPPER", "tone:warm", "abcdefghijklmnopq"} {
		if sightMode(mode, "rewrite") != "rewrite" {
			t.Errorf("bad fallback for %q", mode)
		}
	}
	s := &sightService{topic: newStateTopic(), options: sightOptionsFromFrame(nil), daemon: &daemon{wmWindows: []wm.Window{{AppID: "editor", FocusOrder: 0}}}}
	if _, err := s.open(sightCard{Kind: "text", Text: "text", Mode: "summarize", Source: "forged", App: "forged"}); err != nil {
		t.Fatal(err)
	}
	if card := s.frame.Cards[0]; card.Source != "open" || card.App != "editor" || card.Mode != "summarize" {
		t.Fatalf("opened card = %#v", card)
	}
	s.frame.Highlight = &sightHighlight{ID: "h1", Text: "text", App: "editor"}
	if _, err := s.openHighlight("h1", "reply", 1, 2, "DP-1"); err != nil {
		t.Fatal(err)
	}
	if card := s.frame.Cards[1]; card.Source != "highlight" || card.App != "editor" || card.Mode != "reply" {
		t.Fatalf("highlight card = %#v", card)
	}
}

func TestTheiaFrameWireShape(t *testing.T) {
	frame := sightFrame{Cards: []sightCard{{App: "editor", Source: "look"}}, Highlight: &sightHighlight{App: "browser"}, InPlace: &sightInPlace{ID: "p1", Text: "text", Language: "en", App: "editor", Action: "translate", Output: "DP-1", Created: 123}, Speaking: &sightSpeaking{ID: "speech1", Card: "s1", Started: 456}}
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["cards"].([]any)[0].(map[string]any)["source"] != "look" || fields["cards"].([]any)[0].(map[string]any)["app"] != "editor" || fields["highlight"].(map[string]any)["app"] != "browser" {
		t.Fatalf("card provenance = %s", wire)
	}
	wantPlace := map[string]any{"id": "p1", "text": "text", "language": "en", "app": "editor", "action": "translate", "output": "DP-1", "created": float64(123)}
	wantSpeaking := map[string]any{"id": "speech1", "card": "s1", "started": float64(456)}
	if !reflect.DeepEqual(fields["inplace"], wantPlace) || !reflect.DeepEqual(fields["speaking"], wantSpeaking) {
		t.Fatalf("frame = %s", wire)
	}
	wire, _ = json.Marshal(sightFrame{})
	if !strings.Contains(string(wire), `"inplace":null`) || !strings.Contains(string(wire), `"speaking":null`) {
		t.Fatalf("missing null states: %s", wire)
	}
}

func TestTheiaMasterOffClearsStateAndBlocksCapture(t *testing.T) {
	s := &sightService{topic: newStateTopic(), options: sightOptionsFromFrame(nil), enabled: true, rashin: true, frame: sightFrame{Watching: true, Hidden: true, Cards: []sightCard{{ID: "s1"}}, Select: &sightSelection{ID: "capture"}, Highlight: &sightHighlight{ID: "h1"}, InPlace: &sightInPlace{ID: "p1"}}}
	s.options.On = false
	s.mu.Lock()
	s.clearTheiaLocked()
	s.mu.Unlock()
	s.setWatching(true)
	if s.frame.Watching || s.frame.Reason != "theia-off" || s.frame.Hidden || len(s.frame.Cards) != 0 || s.frame.Select != nil || s.frame.Highlight != nil || s.frame.InPlace != nil {
		t.Fatalf("off state = %#v", s.frame)
	}
	if s.beginCapture(context.Background()) != "" {
		t.Fatal("master off began a capture")
	}
	if _, err := s.open(sightCard{Kind: "text", Text: "text"}); err == nil || err.Error() != "theia-off" {
		t.Fatalf("open while off = %v", err)
	}
	preferred := "DP-1"
	capture := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 100, Scale: 1}}, &preferred)
	id := capture.beginCapture(context.Background())
	if id == "" {
		t.Fatal("capture did not start")
	}
	capture.mu.Lock()
	capture.options.On = false
	capture.mu.Unlock()
	if capture.fireCapture(context.Background(), id) || capture.frame.Select != nil {
		t.Fatal("in-flight capture published after master off")
	}
}

func TestTheiaCaptureProvenanceAndMode(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 100, Scale: 1}}, &preferred)
	s.daemon.wmWindows = []wm.Window{{AppID: "editor", FocusOrder: 0}}
	id := s.beginCapture(context.Background())
	if id == "" || !s.fireCapture(context.Background(), id) {
		t.Fatal("capture did not become selectable")
	}
	if _, err := s.commit(id, sightBox{Output: "DP-1", Width: 20, Height: 20}, "deepread"); err != nil {
		t.Fatal(err)
	}
	card := s.frame.Cards[0]
	if card.App != "editor" || card.Source != "look" || card.Mode != "deepread" {
		t.Fatalf("capture card = %#v", card)
	}
}

func TestTheiaEmptyOutputCallResults(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	d := &daemon{quit: make(chan struct{})}
	settings := d.registerTopic("settings")
	d.registerTopic("presence")
	stopped := make(chan struct{})
	settings.onDemand(func(active bool) {
		if !active {
			close(stopped)
		}
	})
	close(d.quit)
	d.startSight()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("configuration watcher did not stop")
	}
	for _, name := range []string{"stopSpeaking", "inplaceDone"} {
		result, err := d.callHandler("sight." + name)(json.RawMessage(`{}`))
		wire, marshalErr := json.Marshal(result)
		if err != nil || marshalErr != nil || string(wire) != "{}" {
			t.Errorf("sight.%s = %s, %v, %v", name, wire, err, marshalErr)
		}
	}
}

func TestTheiaStartupWaitsForSavedMasterState(t *testing.T) {
	for _, order := range []string{"settings-first", "changes-first"} {
		t.Run(order, func(t *testing.T) {
			configDir := t.TempDir()
			configPath := filepath.Join(configDir, "rashin.json")
			if err := os.WriteFile(configPath, []byte(`{"enabled":true}`), 0600); err != nil {
				t.Fatal(err)
			}
			savedPath := filepath.Join(configDir, "shell.json")
			if err := os.WriteFile(savedPath, []byte(`{"sight":{"on":false}}`), 0600); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(savedPath)
			if err != nil {
				t.Fatal(err)
			}
			d := &daemon{quit: make(chan struct{})}
			settings := d.registerTopic("settings")
			d.registerTopic("presence")
			s := &sightService{daemon: d, topic: newStateTopic(), options: sightOptionsFromFrame(nil)}
			frames := s.topic.subscribe()
			defer s.topic.unsubscribe(frames)
			if order == "settings-first" {
				settings.publish(saved)
			}
			changes := make(chan struct{})
			var starts atomic.Int32
			stopped := make(chan struct{})
			go func() {
				defer close(stopped)
				s.watchConfigurationLoop(context.Background(), configPath, changes, func(ctx context.Context, _ sightOptions, _ <-chan []byte) {
					starts.Add(1)
					<-ctx.Done()
				})
			}()
			defer func() {
				close(d.quit)
				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Error("configuration loop did not stop")
				}
				if got := starts.Load(); got != 0 {
					t.Errorf("saved master-off started %d gesture readers", got)
				}
			}()
			sendChange := func() {
				t.Helper()
				select {
				case changes <- struct{}{}:
				case <-time.After(time.Second):
					t.Fatal("configuration change was not consumed")
				}
			}
			sendChange()
			sendChange()
			if order == "changes-first" {
				settings.publish(saved)
			}
			deadline := time.NewTimer(time.Second)
			defer deadline.Stop()
			for {
				select {
				case raw := <-frames.frames:
					var frame sightFrame
					if err := json.Unmarshal(raw, &frame); err != nil {
						t.Fatal(err)
					}
					if frame.Reason == "theia-off" {
						if frame.Watching {
							t.Fatal("master-off frame is watching")
						}
						sendChange()
						sendChange()
						return
					}
				case <-deadline.C:
					t.Fatal("saved master state was not applied")
				}
			}
		})
	}
}
