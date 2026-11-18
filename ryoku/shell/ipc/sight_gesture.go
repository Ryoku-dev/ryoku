package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	wm "ryoku-wm"
)

const (
	sightTapDuration  = 250 * time.Millisecond
	sightTapGap       = 350 * time.Millisecond
	sightPrimaryQuiet = 150 * time.Millisecond
	sightPrimaryWait  = 1500 * time.Millisecond
	// Apps such as Chromium set PRIMARY on mouse-up, which can land just after
	// the modifier comes up when both are released together.
	sightReleaseGrace = 400 * time.Millisecond
)

func sightGestureKey(name, fallback string) wm.GestureKey {
	for _, key := range wm.GestureKeys {
		if key.Name == name {
			return key
		}
	}
	for _, key := range wm.GestureKeys {
		if key.Name == fallback {
			return key
		}
	}
	panic("missing default Theia gesture key")
}

// Both machines observe keys without grabbing them. Device identity keeps one
// keyboard's release from releasing the same key held on another keyboard.
type sightTap struct {
	key         uint16
	down        map[heldKeyID]bool
	pressed     heldKeyID
	started     time.Time
	lastRelease time.Time
	count       int
	valid       bool
	locked      bool
	deadline    time.Time
}

func newSightTap(key uint16) *sightTap {
	return &sightTap{key: key, down: make(map[heldKeyID]bool)}
}
func (t *sightTap) reset() string {
	action := ""
	if t.count == 2 {
		action = "cancel"
	}
	t.count, t.valid = 0, false
	t.deadline = time.Time{}
	return action
}
func (t *sightTap) lock(locked bool) string {
	t.locked = locked
	if locked {
		return t.reset()
	}
	return ""
}
func (t *sightTap) tick(now time.Time) string {
	if !t.deadline.IsZero() && now.After(t.deadline) {
		return t.reset()
	}
	return ""
}
func (t *sightTap) event(source uint64, code uint16, value int32, now time.Time) string {
	action := t.tick(now)
	id := heldKeyID{source: source, code: code}
	switch value {
	case 2:
		if id == t.pressed && t.valid {
			return t.reset()
		}
	case 1:
		if t.down[id] {
			return t.reset()
		}
		free := len(t.down) == 0
		t.down[id] = true
		if code != t.key || !free || t.locked {
			if cancel := t.reset(); cancel != "" {
				action = cancel
			}
			return action
		}
		if t.count != 0 && now.Sub(t.lastRelease) > sightTapGap {
			if cancel := t.reset(); cancel != "" {
				action = cancel
			}
		}
		t.pressed, t.started, t.valid = id, now, true
		t.deadline = now.Add(sightTapDuration)
	case 0:
		delete(t.down, id)
		if id != t.pressed || !t.valid {
			return action
		}
		t.valid = false
		if t.locked || now.Sub(t.started) > sightTapDuration {
			return t.reset()
		}
		t.count++
		t.lastRelease = now
		t.deadline = now.Add(sightTapGap)
		switch t.count {
		case 2:
			return "prewarm"
		case 3:
			t.count = 0
			t.deadline = time.Time{}
			return "fire"
		}
	}
	return action
}
func (t *sightTap) removeDevice(source uint64) string {
	for id := range t.down {
		if id.source == source {
			delete(t.down, id)
		}
	}
	return t.reset()
}

type sightHighlightGesture struct {
	key         uint16
	down        map[heldKeyID]bool
	pressed     heldKeyID
	started     time.Time
	holding     bool
	highlighted bool
	locked      bool
	pending     bool
	lastPrimary time.Time
	limit       time.Time
	grace       time.Time
}

func newSightHighlightGesture(key uint16) *sightHighlightGesture {
	return &sightHighlightGesture{key: key, down: make(map[heldKeyID]bool)}
}
func (h *sightHighlightGesture) reset() {
	h.holding, h.highlighted, h.pending, h.grace = false, false, false, time.Time{}
}
func (h *sightHighlightGesture) lock(locked bool) {
	h.locked = locked
	if locked {
		h.reset()
	}
}

// primary returns whether a displayed bubble must be cleared. Changes during
// settling still extend the quiet period, but never the release-time cap.
func (h *sightHighlightGesture) primary(now time.Time) bool {
	h.lastPrimary = now
	if h.locked {
		return true
	}
	if h.holding && !now.Before(h.started) {
		h.highlighted = true
		return false
	}
	if !h.grace.IsZero() && !now.After(h.grace) {
		h.grace = time.Time{}
		h.highlighted, h.pending, h.limit = true, true, now.Add(sightPrimaryWait)
		return false
	}
	return true
}
func (h *sightHighlightGesture) event(source uint64, code uint16, value int32, now time.Time) bool {
	id := heldKeyID{source: source, code: code}
	switch value {
	case 1:
		free := len(h.down) == 0
		h.down[id] = true
		h.reset()
		if code == h.key && free && !h.locked {
			h.holding, h.pressed, h.started = true, id, now
			// The keyboard and PRIMARY notifications arrive on separate queues.
			// A coalesced PRIMARY event can already be known when its earlier
			// physical modifier press is dequeued.
			h.highlighted = !h.lastPrimary.IsZero() && !h.lastPrimary.Before(now)
		}
		return true
	case 0:
		delete(h.down, id)
		if id == h.pressed && h.holding {
			h.holding = false
			if h.locked {
				break
			}
			if h.highlighted {
				h.pending = true
				h.limit = now.Add(sightPrimaryWait)
			} else {
				h.grace = now.Add(sightReleaseGrace)
			}
		}
	}
	return false
}
func (h *sightHighlightGesture) deadline() time.Time {
	if !h.pending {
		return time.Time{}
	}
	quiet := h.lastPrimary.Add(sightPrimaryQuiet)
	if quiet.Before(h.limit) {
		return quiet
	}
	return h.limit
}
func (h *sightHighlightGesture) tick(now time.Time) bool {
	if !h.pending || h.locked || now.Before(h.deadline()) {
		return false
	}
	h.pending = false
	return true
}
func (h *sightHighlightGesture) removeDevice(source uint64) {
	for id := range h.down {
		if id.source == source {
			delete(h.down, id)
		}
	}
	h.reset()
}

// Prewarming must not replace a visible selection: the third tap toggles that
// selection off instead. Calls share the same cancellation path as sight.cancel.
func (s *sightService) gestureTap(ctx context.Context, action string, captureID *string, workers *sync.WaitGroup) {
	if (action == "prewarm" || action == "fire") && s.quietBlocked() {
		if *captureID != "" {
			s.cancelCapture(*captureID)
			*captureID = ""
		}
		return
	}
	switch action {
	case "prewarm":
		s.mu.Lock()
		selecting := s.frame.Select != nil
		s.mu.Unlock()
		if !selecting {
			*captureID = s.beginCapture(ctx)
		}
	case "cancel":
		if *captureID != "" {
			s.cancelCapture(*captureID)
			*captureID = ""
		}
	case "fire":
		s.mu.Lock()
		selection := s.frame.Select
		s.mu.Unlock()
		if selection != nil {
			s.cancelCapture(selection.ID)
		} else if id := *captureID; id != "" {
			workers.Add(1)
			go func() { defer workers.Done(); s.fireCapture(ctx, id) }()
		}
		*captureID = ""
	}
}

func (s *sightService) watchGestures(ctx context.Context, options sightOptions, presence <-chan []byte) {
	messages := make(chan keyDeviceMessage, 80)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runKeyboardReader(ctx, "/sys/class/input", "/dev/input", func(m keyDeviceMessage, n int) {
			if !m.hasEvent {
				s.setWatching(n > 0)
			}
			sendKeyDeviceMessage(ctx, messages, m)
		})
	}()
	taps := newSightTap(sightGestureKey(options.TapKey, "Alt_R").Code)
	highlights := newSightHighlightGesture(sightGestureKey(options.HighlightKey, "Alt_L").Code)
	inplace := newSightTap(sightGestureKey(options.InPlace.Key, "Control_R").Code)
	var primary *primarySelectionState
	var changes <-chan struct{}
	var lastPrimary time.Time
	if s.daemon.clip != nil {
		primary = &s.daemon.clip.primary
		changes = primary.notifications()
		lastPrimary = primary.lastChange()
	}
	var workers sync.WaitGroup
	var captureID string
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var ready <-chan time.Time
	stopTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		ready = nil
	}
	arm := func() {
		stopTimer()
		deadline := taps.deadline
		if next := highlights.deadline(); !next.IsZero() && (deadline.IsZero() || next.Before(deadline)) {
			deadline = next
		}
		if next := inplace.deadline; !next.IsZero() && (deadline.IsZero() || next.Before(deadline)) {
			deadline = next
		}
		if !deadline.IsZero() {
			timer.Reset(time.Until(deadline.Add(time.Nanosecond)))
			ready = timer.C
		}
	}
	act := func(action string) { s.gestureTap(ctx, action, &captureID, &workers) }
	lock := func(locked bool) {
		act(taps.lock(locked))
		highlights.lock(locked)
		inplace.lock(locked)
		if locked {
			s.clearHighlight()
			s.inplaceDone("")
			s.stopOutputs()
		}
	}
	// Read the coalesced timestamp at every keyboard boundary too: a release
	// must see a PRIMARY event even if both channels became ready together.
	syncPrimary := func() {
		if primary == nil {
			return
		}
		changed := primary.lastChange()
		if changed.After(lastPrimary) {
			lastPrimary = changed
			if highlights.primary(changed) {
				s.clearHighlight()
			}
		}
	}
	type highlightResult struct {
		text, language, output, app string
		inplace                     bool
		epoch                       uint64
		primary                     time.Time
	}
	results := make(chan highlightResult, 1)
	readPrimary := func(inPlace bool) {
		if primary == nil || s.quietBlocked() {
			return
		}
		s.mu.Lock()
		epoch := s.highlightEpoch
		if inPlace {
			s.inplaceEpoch++
			epoch = s.inplaceEpoch
		}
		s.mu.Unlock()
		s.daemon.wmMu.Lock()
		output := s.daemon.activeMon
		s.daemon.wmMu.Unlock()
		app := s.focusedApp()
		stamp := lastPrimary
		workers.Add(1)
		go func() {
			defer workers.Done()
			text, err := primary.read()
			if err != nil || strings.TrimSpace(text) == "" {
				return
			}
			result := highlightResult{text: text, language: detectSightLanguage(text), output: output, app: app, inplace: inPlace, epoch: epoch, primary: stamp}
			select {
			case results <- result:
			case <-ctx.Done():
			}
		}()
	}
	inplaceAct := func(action string) {
		if action == "fire" {
			readPrimary(true)
		}
	}
	defer func() {
		stopTimer()
		s.clearHighlight()
		s.inplaceDone("")
		s.cancelCapture("")
		workers.Wait()
		<-done
	}()
	lock(s.sessionLocked())
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-presence:
			var frame presenceFrame
			if json.Unmarshal(data, &frame) == nil {
				lock(frame.Locked)
			}
		case <-changes:
			syncPrimary()
		case <-ready:
			lock(s.sessionLocked())
			syncPrimary()
			now := time.Now()
			act(taps.tick(now))
			inplaceAct(inplace.tick(now))
			if options.Highlight && highlights.tick(now) {
				readPrimary(false)
			}
		case result := <-results:
			lock(s.sessionLocked())
			syncPrimary()
			if result.inplace {
				if !inplace.locked && ctx.Err() == nil {
					s.publishInPlace(result.text, result.app, result.output, result.epoch)
				}
			} else {
				s.mu.Lock()
				if s.options.On && !highlights.locked && s.highlightEpoch == result.epoch && lastPrimary.Equal(result.primary) && ctx.Err() == nil && !s.quietBlockedLocked(s.focusedApp(), time.Now()) {
					s.next++
					s.frame.Highlight = &sightHighlight{ID: fmt.Sprintf("h%x-%x", time.Now().UnixNano(), s.next), Text: result.text, Language: result.language, Output: result.output, App: result.app, Created: time.Now().UnixMilli()}
					s.publishLocked()
				}
				s.mu.Unlock()
			}
		case message := <-messages:
			lock(s.sessionLocked())
			syncPrimary()
			if message.hasEvent {
				now := time.Now()
				if message.timestampUS > 0 {
					if clock := monotonicTimestampUS(); clock >= message.timestampUS {
						now = now.Add(-time.Duration(clock-message.timestampUS) * time.Microsecond)
					}
				}
				if highlights.event(message.readerID, message.event.code, message.event.value, now) {
					s.clearHighlight()
				}
				if options.Enabled {
					act(taps.event(message.readerID, message.event.code, message.event.value, now))
				}
				if options.feature("inplace") {
					inplaceAct(inplace.event(message.readerID, message.event.code, message.event.value, now))
				}
			} else if !message.opened {
				act(taps.removeDevice(message.readerID))
				highlights.removeDevice(message.readerID)
				inplace.removeDevice(message.readerID)
				s.clearHighlight()
			}
		}
		arm()
	}
}
