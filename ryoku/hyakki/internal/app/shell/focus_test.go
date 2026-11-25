package shell

import (
	"encoding/json"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	gioapp "gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func deliverFocus(t *testing.T, store *state.Store, params protocol.UIOpenParams) {
	t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: data}); err != nil {
		t.Fatal(err)
	}
}

func TestRepeatedFocusRaisesAndNavigatesEvenAfterLeavingTheTarget(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	var actions []system.Action
	var tokens []string
	stage.shell.windowToken = func(token string) { tokens = append(tokens, token) }
	stage.shell.windowAction = func(action system.Action) {
		if len(tokens) != len(actions)+1 {
			t.Fatal("native raise did not receive its token option first")
		}
		actions = append(actions, action)
	}
	request := protocol.UIOpenParams{Tab: protocol.TabSettings, View: "features", ActivationToken: "one-shot"}
	for range 2 {
		stage.open("ask/new")
		deliverFocus(t, stage.runtime.store, request)
		stage.frame()
		if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
			t.Fatalf("repeated focus did not return to settings/features: %#v", place)
		}
	}
	if len(actions) != 2 {
		t.Fatalf("native actions = %d, want two independent raises", len(actions))
	}
	for index, action := range actions {
		if action != system.ActionRaise || tokens[index] != request.ActivationToken {
			t.Fatal("focus did not request native activation with its own token")
		}
	}
	before := stage.shell.router.Current()
	deliverFocus(t, stage.runtime.store, protocol.UIOpenParams{})
	stage.frame()
	if stage.shell.router.Current() != before {
		t.Fatal("a bare open reset the current navigation")
	}
	if len(actions) != 3 || tokens[2] != "" {
		t.Fatal("a bare open did not raise independently of the previous token")
	}
	stage.frame()
	stage.runtime.store.ApplySnapshot(protocol.Snapshot{Settings: stage.settings()})
	stage.shell.applyFocus(stage.runtime.store.Snapshot())
	if len(actions) != 3 {
		t.Fatal("a redraw or daemon snapshot replayed an old activation")
	}
}

func TestFocusUsesOnlyDeliveriesVisibleToItsSnapshot(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.shell = newShell(stage.runtime, Options{Route: protocol.TabSettings, SettingsSection: "features"})
	raises := 0
	stage.shell.windowAction = func(action system.Action) {
		if action != system.ActionRaise {
			t.Fatal("focus requested an action other than native raise")
		}
		raises++
	}
	stale := stage.runtime.store.Snapshot()
	stage.shell.applyFocus(stale)
	if raises != 0 {
		t.Fatal("initial navigation spuriously raised the native window")
	}
	if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
		t.Fatal("initial focus handling lost the startup route")
	}
	for range 2 {
		deliverFocus(t, stage.runtime.store, protocol.UIOpenParams{Tab: protocol.TabSettings, View: "features"})
	}
	stage.shell.applyFocus(stale)
	if raises != 0 {
		t.Fatal("a stale rendering snapshot consumed newer activation requests")
	}
	fresh := stage.runtime.store.Snapshot()
	stage.shell.applyFocus(fresh)
	if raises != 2 {
		t.Fatalf("batched identical requests raised %d times, want 2", raises)
	}
	stage.shell.applyFocus(fresh)
	stage.shell.applyFocus(stale)
	if raises != 2 {
		t.Fatal("reused or stale snapshots replayed a consumed activation")
	}
}

func TestBareFocusKeepsTheOpenConversation(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/ask-20261110-094200-a17c")
	before := stage.shell.router.Current()
	if before.Session == "" {
		t.Fatal("the conversation did not open before activation")
	}
	raises := 0
	stage.shell.windowAction = func(action system.Action) {
		if action == system.ActionRaise {
			raises++
		}
	}
	activations := activationQueue{}
	stage.runtime.store.SetFocusActivation(activations.enqueue)
	defer stage.runtime.store.SetFocusActivation(nil)
	for _, request := range []protocol.UIOpenParams{{ActivationToken: "one-shot"}, {}} {
		deliverFocus(t, stage.runtime.store, request)
		activations.afterEvent(activationWakeup{}, stage.shell.activateWindow)
		stage.frame()
		if stage.shell.router.Current() != before {
			t.Fatal("activation alone replaced the open conversation")
		}
	}
	if raises != 2 {
		t.Fatalf("bare focus raised %d times, want 2", raises)
	}
}

func TestHiddenWindowActivationDoesNotWaitForAFrame(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	raises := 0
	token := ""
	stage.shell.windowToken = func(value string) { token = value }
	stage.shell.windowAction = func(action system.Action) {
		if action != system.ActionRaise || token != "hidden-window" {
			t.Fatal("hidden-window delivery lost its native activation grant")
		}
		raises++
	}
	wakeups := make(chan struct{}, 2)
	activations := activationQueue{invalidate: func() { wakeups <- struct{}{} }}
	stage.runtime.store.SetFocusActivation(activations.enqueue)
	defer stage.runtime.store.SetFocusActivation(nil)
	before := stage.shell.router.Current()
	request := protocol.UIOpenParams{Tab: protocol.TabSettings, View: "features", ActivationToken: "hidden-window"}
	delivered := make(chan struct{})
	go func() {
		defer close(delivered)
		for range 2 {
			deliverFocus(t, stage.runtime.store, request)
		}
	}()
	<-delivered
	if raises != 0 {
		t.Fatal("the Store delivery goroutine performed native activation")
	}
	for range 2 {
		<-wakeups
		// Gio's inactive-window wakeup has no specified concrete event type.
		activations.afterEvent(activationWakeup{}, stage.shell.activateWindow)
	}
	if raises != 2 {
		t.Fatalf("without a frame, native raises = %d, want 2", raises)
	}
	if stage.shell.router.Current() != before {
		t.Fatal("the delivery goroutine changed frame-owned navigation")
	}
	stage.frame()
	if raises != 2 {
		t.Fatal("the first visible frame repeated an already consumed native activation")
	}
	if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
		t.Fatal("the first visible frame did not apply the requested navigation")
	}
}

type activationWakeup struct{}

func (activationWakeup) ImplementsEvent() {}

func TestActivationQueueDrainsAfterEveryKindOfNativeEvent(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  event.Event
	}{
		{name: "inactive wakeup", raw: activationWakeup{}},
		{name: "configure", raw: gioapp.ConfigEvent{}},
		{name: "frame", raw: gioapp.FrameEvent{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			wakeups := 0
			queue := activationQueue{invalidate: func() { wakeups++ }}
			queue.enqueue("repeated")
			queue.enqueue("repeated")
			queue.enqueue("")
			if wakeups != 3 {
				t.Fatal("each delivery must wake the native event owner")
			}
			retained := queue.pending
			var tokens []string
			activate := func(token string) { tokens = append(tokens, token) }
			queue.afterEvent(test.raw, activate)
			if len(tokens) != 3 || tokens[0] != "repeated" || tokens[1] != "repeated" || tokens[2] != "" {
				t.Fatal("the event did not drain each repeated and bare activation in order")
			}
			for _, token := range retained {
				if token != "" {
					t.Fatal("a consumed activation grant remained in queue storage")
				}
			}
			queue.afterEvent(test.raw, activate)
			if len(tokens) != 3 {
				t.Fatal("another event replayed consumed activation grants")
			}
		})
	}
}

func TestActivationQueueAllowsDeliveryDuringNativeDrain(t *testing.T) {
	queue := activationQueue{}
	queue.enqueue("first")
	calls := 0
	queue.afterEvent(activationWakeup{}, func(token string) {
		calls++
		if token != "first" {
			t.Fatal("the first native drain consumed the wrong grant")
		}
		// Reentering enqueue proves no queue mutex is held during native work.
		queue.enqueue("next")
	})
	queue.afterEvent(activationWakeup{}, func(token string) {
		calls++
		if token != "next" {
			t.Fatal("delivery during native work was lost")
		}
	})
	if calls != 2 {
		t.Fatal("delivery during native work was not independently drained")
	}
}

func TestActivationQueueDestroyClearsGrantsAndRejectsCapturedCallbacks(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	wakeups := 0
	queue := activationQueue{invalidate: func() { wakeups++ }}
	captured := queue.enqueue
	stage.runtime.store.SetFocusActivation(captured)
	deliverFocus(t, stage.runtime.store, protocol.UIOpenParams{ActivationToken: "pending"})
	retained := queue.pending
	activate := func(string) { t.Fatal("native activation touched a destroyed window") }
	queue.afterEvent(gioapp.DestroyEvent{}, activate)
	stage.runtime.store.SetFocusActivation(nil)
	captured("late")
	queue.afterEvent(activationWakeup{}, activate)
	queue.close()
	if wakeups != 1 || !queue.closed || queue.pending != nil || queue.invalidate != nil {
		t.Fatal("teardown retained grants or woke the window through a captured callback")
	}
	for _, token := range retained {
		if token != "" {
			t.Fatal("teardown left an activation grant in queue storage")
		}
	}
}

func TestActivationQueueConcurrentStoreDeliveryAndTeardown(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	var destroyed atomic.Bool
	var lateWakeups atomic.Int32
	queue := activationQueue{invalidate: func() {
		if destroyed.Load() {
			lateWakeups.Add(1)
		}
	}}
	captured := queue.enqueue
	stage.runtime.store.SetFocusActivation(captured)
	start := make(chan struct{})
	failures := make(chan error, 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			failures <- stage.runtime.store.Apply(protocol.Event{
				Event: protocol.EvtUIFocus,
				Data:  json.RawMessage(`{"activationToken":"racing"}`),
			})
			// Store may have copied the handler before it was unregistered.
			captured("captured")
		}()
	}
	close(start)
	activate := func(string) { t.Fatal("teardown performed native activation") }
	queue.afterEvent(gioapp.DestroyEvent{}, activate)
	destroyed.Store(true)
	stage.runtime.store.SetFocusActivation(nil)
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	queue.afterEvent(activationWakeup{}, activate)
	if lateWakeups.Load() != 0 || queue.pending != nil {
		t.Fatal("an in-flight Store callback touched the window after teardown")
	}
}
