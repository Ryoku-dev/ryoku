package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/screen"
)

// screenIdle is how long a screen nobody acts on, watches or holds stays up;
// screenWatch is how long a client's watch lasts unless it asks again
const (
	screenIdle  = 10 * time.Minute
	screenWatch = 20 * time.Second
)

// agentWait bounds an agent's call that waits on the user: harnesses give up
// on a tool call after half a minute or so, so the call returns before that
// and the agent calls again, while what it waits on stays up
var agentWait = 20 * time.Second

// screenManager keeps one screen per Agent session that asked for one. A
// screen exists only while it is in use: the agent's first action asks the
// user for it, and it ends with its session, when idle, or on Stop.
type screenManager struct {
	mu        sync.Mutex
	items     map[string]*agentScreen
	broadcast func(string, any)
	ask       func(context.Context, string, protocol.PermissionData) (string, error)
	// available says this desktop can host a screen at all
	available func() bool
	// granted says a session may have its screen without being asked, because
	// its team was given one; nil asks every session
	granted func(id string) bool
	// sandbox is what the apps on a session's screen run in: set for a team
	// session, nil for others; a nil func runs every screen's apps as the user
	sandbox func(id string) *sandbox.Spec
}

type agentScreen struct {
	// acting keeps actions on the screen one at a time, so typing lands in order
	acting sync.Mutex
	mu     sync.Mutex
	state  protocol.ScreenState
	screen *screen.Screen
	ready  chan struct{}
	err    error
	held   chan struct{}
	asked  chan struct{}
	// end stops asking for the screen, or starting it, when it goes away first
	end context.CancelFunc
	// watched is when the last client watching asked to go on; frames are
	// read while that is recent, so a client that went away stops them
	watched  time.Time
	last     time.Time
	frame    []byte
	stopLoop context.CancelFunc
}

func newScreenManager(broadcast func(string, any), ask func(context.Context, string, protocol.PermissionData) (string, error), available func() bool) *screenManager {
	return &screenManager{items: map[string]*agentScreen{}, broadcast: broadcast, ask: ask, available: available}
}

func (m *screenManager) list() protocol.ScreensResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := protocol.ScreensResult{Screens: []protocol.ScreenState{}}
	for _, item := range m.items {
		item.mu.Lock()
		result.Screens = append(result.Screens, item.state)
		item.mu.Unlock()
	}
	return result
}

func (m *screenManager) publish(item *agentScreen) {
	item.mu.Lock()
	state := item.state
	item.mu.Unlock()
	m.broadcast(protocol.EvtScreen, state)
}

// ensure finds a session's screen or starts bringing one up. Asking the user
// and starting belong to the screen, not to a call, so a call that stops
// waiting leaves them going for the agent's next one.
func (m *screenManager) ensure(id string) *agentScreen {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item := m.items[id]; item != nil {
		return item
	}
	life, end := context.WithCancel(context.Background())
	item := &agentScreen{ready: make(chan struct{}), end: end, last: time.Now(), state: protocol.ScreenState{Session: id, State: protocol.ScreenStarting, Holder: protocol.HolderAgent}}
	m.items[id] = item
	go m.bring(life, id, item)
	return item
}

// bring asks the user for the screen, unless the session's team was given
// one, and starts it once they agree
func (m *screenManager) bring(ctx context.Context, id string, item *agentScreen) {
	fail := func(err error) {
		item.mu.Lock()
		item.err = err
		item.state.State, item.state.Error = protocol.ScreenStopped, err.Error()
		item.mu.Unlock()
		close(item.ready)
		m.publish(item)
	}
	if !m.available() {
		fail(errors.New(i18n.T("This desktop cannot host a screen for the agent.")))
		return
	}
	if m.granted == nil || !m.granted(id) {
		option, err := m.ask(ctx, id, protocol.PermissionData{
			Title: i18n.T("Give the agent a screen of its own"),
			Kind:  "screen",
			Input: i18n.T("It opens apps on a hidden screen you can watch and take over. Nothing appears on your desktop."),
			Options: []protocol.PermissionOption{
				{ID: "allow", Label: i18n.T("Give it a screen"), Kind: protocol.OptAllowOnce},
				{ID: "deny", Label: i18n.T("Not now"), Kind: protocol.OptRejectOnce},
			},
		})
		if err != nil {
			fail(err)
			return
		}
		if option != "allow" {
			fail(errors.New(i18n.T("The user did not give you a screen. Do the work without one, or ask them in your reply.")))
			return
		}
	}
	m.publish(item)
	var contained *sandbox.Spec
	if m.sandbox != nil {
		contained = m.sandbox(id)
	}
	started, err := screen.Start(ctx, screen.Options{Dir: filepath.Join(paths.ScreensDir(), id), Home: paths.ScreenHome(), Sandbox: contained})
	if err != nil {
		fail(fmt.Errorf("%s %v", i18n.T("The screen did not start:"), err))
		return
	}
	width, height := started.Size()
	item.mu.Lock()
	item.screen = started
	item.state.State, item.state.Width, item.state.Height = protocol.ScreenRunning, width, height
	m.startFrames(item)
	item.mu.Unlock()
	close(item.ready)
	m.publish(item)
	go func() {
		<-started.Done()
		m.remove(id, item)
	}()
}

// up waits for the session's screen, asking the user for it first, until the
// deadline; ready is false when the user has not answered yet. A screen that
// failed is forgotten once an agent's call has heard why, so its next call
// asks again.
func (m *screenManager) up(ctx context.Context, id string, deadline <-chan time.Time) (item *agentScreen, ready bool, err error) {
	item = m.ensure(id)
	select {
	case <-item.ready:
	case <-deadline:
		return item, false, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	if err := item.failure(); err != nil {
		m.mu.Lock()
		if m.items[id] == item {
			delete(m.items, id)
		}
		m.mu.Unlock()
		return nil, false, err
	}
	return item, true, nil
}

// free waits while the user holds the screen or has the agent's ask, until
// the deadline
func (item *agentScreen) free(ctx context.Context, deadline <-chan time.Time) (bool, error) {
	for {
		item.mu.Lock()
		wait := item.held
		if item.asked != nil {
			wait = item.asked
		}
		item.mu.Unlock()
		if wait == nil {
			return true, nil
		}
		select {
		case <-wait:
		case <-deadline:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func (item *agentScreen) snapshot() protocol.ScreenState {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.state
}

func (item *agentScreen) failure() error {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.err
}

// find is a session's screen that is up, for the user's calls, which never
// wait on it
func (m *screenManager) find(id string) (*agentScreen, error) {
	m.mu.Lock()
	item := m.items[id]
	m.mu.Unlock()
	if item == nil {
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This session has no screen."))
	}
	select {
	case <-item.ready:
	default:
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The screen is not up yet."))
	}
	if err := item.failure(); err != nil {
		return nil, err
	}
	return item, nil
}

// act does one thing on a session's screen. An agent's action waits for the
// screen and while the user has it, up to agentWait, then says what it still
// waits on without doing anything; a person's is refused until they take it.
func (m *screenManager) act(ctx context.Context, p protocol.ScreenActParams) (protocol.ScreenActResult, error) {
	var item *agentScreen
	var err error
	switch p.By {
	case protocol.HolderAgent:
		timer := time.NewTimer(agentWait)
		defer timer.Stop()
		var ready, free bool
		item, ready, err = m.up(ctx, p.ID, timer.C)
		if err != nil {
			return protocol.ScreenActResult{}, err
		}
		if !ready {
			return protocol.ScreenActResult{Screen: item.snapshot(), Waiting: protocol.ScreenWaitConsent}, nil
		}
		free, err = item.free(ctx, timer.C)
		if err == nil && !free {
			return protocol.ScreenActResult{Screen: item.snapshot(), Waiting: protocol.ScreenWaitUser}, nil
		}
	case protocol.HolderUser:
		item, err = m.find(p.ID)
		if err == nil {
			item.mu.Lock()
			held := item.held != nil
			item.mu.Unlock()
			if !held {
				err = protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Take control of the screen first."))
			}
		}
	default:
		err = protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("An action is by the agent or the user."))
	}
	if err != nil {
		return protocol.ScreenActResult{}, err
	}
	item.acting.Lock()
	words, err := doScreen(ctx, item.screen, p.Action)
	item.acting.Unlock()
	if err != nil {
		return protocol.ScreenActResult{}, err
	}
	item.mu.Lock()
	item.last = time.Now()
	if p.By == protocol.HolderAgent {
		item.state.Action = words
	}
	watched := time.Since(item.watched) < screenWatch
	item.mu.Unlock()
	result := protocol.ScreenActResult{}
	if p.Look || watched {
		frame, captureErr := item.screen.Settled(ctx)
		if captureErr != nil {
			return protocol.ScreenActResult{}, captureErr
		}
		m.showFrame(item, frame)
		if p.Look {
			result.Image = base64.StdEncoding.EncodeToString(frame)
		}
	} else {
		m.publish(item)
	}
	item.mu.Lock()
	result.Screen = item.state
	item.mu.Unlock()
	return result, nil
}

// doScreen performs one action and says what was done, for the screen's
// status line
func doScreen(ctx context.Context, s *screen.Screen, action protocol.ScreenAction) (string, error) {
	switch action.Kind {
	case "click", "double", "right", "middle":
		button, count := "left", 1
		switch action.Kind {
		case "double":
			count = 2
		case "right", "middle":
			button = action.Kind
		}
		if err := s.Click(action.X, action.Y, button, count); err != nil {
			return "", err
		}
		return i18n.Tf("Clicked at %d, %d", action.X, action.Y), nil
	case "move":
		return i18n.Tf("Pointed at %d, %d", action.X, action.Y), s.Move(action.X, action.Y)
	case "press", "release":
		if err := s.Move(action.X, action.Y); err != nil {
			return "", err
		}
		return i18n.Tf("Pointed at %d, %d", action.X, action.Y), s.Press(action.Kind == "press")
	case "drag":
		return i18n.Tf("Dragged from %d, %d to %d, %d", action.X, action.Y, action.ToX, action.ToY), s.Drag(action.X, action.Y, action.ToX, action.ToY)
	case "scroll":
		return i18n.Tf("Scrolled at %d, %d", action.X, action.Y), s.Scroll(action.X, action.Y, action.DX, action.DY)
	case "type":
		return i18n.Tf("Typed %d characters", len([]rune(action.Text))), s.Type(ctx, action.Text)
	case "key":
		return i18n.Tf("Pressed %s", action.Keys), s.Key(ctx, action.Keys)
	case "open":
		command := strings.TrimSpace(action.Command)
		if command == "" {
			return "", errors.New(i18n.T("Name a command to open."))
		}
		return i18n.Tf("Opened %s", strings.Fields(command)[0]), s.Open(command)
	case "wait":
		seconds := min(max(action.Seconds, 0.2), 30)
		select {
		case <-time.After(time.Duration(seconds * float64(time.Second))):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return i18n.T("Waited"), nil
	case "look":
		return i18n.T("Looked at the screen"), nil
	}
	return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a screen action.", action.Kind))
}

// showFrame writes a frame where clients read it and tells them, unless it
// is the frame they already have
func (m *screenManager) showFrame(item *agentScreen, frame []byte) {
	item.mu.Lock()
	if bytes.Equal(frame, item.frame) {
		item.mu.Unlock()
		m.publish(item)
		return
	}
	item.frame = frame
	path := filepath.Join(paths.ScreensDir(), item.state.Session+".png")
	item.mu.Unlock()
	temporary := path + ".part"
	if err := os.WriteFile(temporary, frame, 0o600); err != nil || os.Rename(temporary, path) != nil {
		return
	}
	item.mu.Lock()
	item.state.Frame = path
	item.state.FrameSeq++
	item.mu.Unlock()
	m.publish(item)
}

// control hands a screen between the user and the agent, lets a client watch
// it, lets the agent ask the user for help on it, or ends it
func (m *screenManager) control(ctx context.Context, p protocol.ScreenControlParams) (protocol.ScreenState, error) {
	switch p.Op {
	case protocol.ScreenStop:
		m.stop(p.ID)
		return protocol.ScreenState{Session: p.ID, State: protocol.ScreenStopped}, nil
	case protocol.ScreenAsk:
		return m.askUser(ctx, p)
	case protocol.ScreenWatch, protocol.ScreenUnwatch:
		return m.watch(p.ID, p.Op == protocol.ScreenWatch)
	}
	item, err := m.find(p.ID)
	if err != nil {
		return protocol.ScreenState{}, err
	}
	item.mu.Lock()
	switch p.Op {
	case protocol.ScreenTake:
		if item.held == nil {
			item.held = make(chan struct{})
		}
		item.state.Holder = protocol.HolderUser
	case protocol.ScreenRelease:
		if item.held != nil {
			close(item.held)
			item.held = nil
		}
		if item.asked != nil {
			close(item.asked)
			item.asked = nil
		}
		item.state.Holder, item.state.Ask = protocol.HolderAgent, ""
	default:
		item.mu.Unlock()
		return protocol.ScreenState{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Unknown screen operation %s.", p.Op))
	}
	item.last = time.Now()
	state := item.state
	item.mu.Unlock()
	m.publish(item)
	return state, nil
}

// askUser has the agent ask the user to do something on its screen and waits
// for them to hand it back, up to agentWait. The ask stays up past that, and
// the agent's next action goes on waiting for them; a call the harness
// cancels takes it back.
func (m *screenManager) askUser(ctx context.Context, p protocol.ScreenControlParams) (protocol.ScreenState, error) {
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return protocol.ScreenState{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say what you need the user to do on the screen."))
	}
	timer := time.NewTimer(agentWait)
	defer timer.Stop()
	item, ready, err := m.up(ctx, p.ID, timer.C)
	if err != nil {
		return protocol.ScreenState{}, err
	}
	if !ready {
		return item.snapshot(), nil
	}
	item.mu.Lock()
	first := item.asked == nil
	if first {
		item.asked = make(chan struct{})
	}
	asked := item.asked
	item.state.Ask, item.last = reason, time.Now()
	item.mu.Unlock()
	m.publish(item)
	if first {
		m.broadcast(protocol.EvtNotice, protocol.Notice{Level: protocol.NoticeWarn, Text: i18n.Tf("The agent needs you on its screen: %s", reason), Session: p.ID})
	}
	select {
	case <-asked:
	case <-timer.C:
	case <-ctx.Done():
		item.mu.Lock()
		if item.asked == asked {
			close(asked)
			item.asked, item.state.Ask = nil, ""
		}
		item.mu.Unlock()
		m.publish(item)
		return protocol.ScreenState{}, ctx.Err()
	}
	return item.snapshot(), nil
}

// watch has a client's watch begin or go on, or end. A screen still being
// asked for or started takes it too, and its frames begin once it is up.
func (m *screenManager) watch(id string, on bool) (protocol.ScreenState, error) {
	m.mu.Lock()
	item := m.items[id]
	m.mu.Unlock()
	if item == nil {
		return protocol.ScreenState{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This session has no screen."))
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	item.watched = time.Time{}
	if on {
		item.watched = time.Now()
		m.startFrames(item)
	}
	return item.state, nil
}

// startFrames begins reading frames for a watched screen that is up; it is
// called with item.mu held
func (m *screenManager) startFrames(item *agentScreen) {
	if item.stopLoop != nil || item.screen == nil || time.Since(item.watched) >= screenWatch {
		return
	}
	var loop context.Context
	loop, item.stopLoop = context.WithCancel(context.Background())
	go m.frames(loop, item)
}

// frames reads a frame every second while a client watches and sends it on
// when it changed
func (m *screenManager) frames(ctx context.Context, item *agentScreen) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		item.mu.Lock()
		watched := time.Since(item.watched) < screenWatch
		if !watched {
			item.stopLoop = nil
		}
		item.mu.Unlock()
		if !watched {
			return
		}
		frame, err := item.screen.Capture(ctx)
		if err == nil {
			m.showFrame(item, frame)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *screenManager) stop(id string) {
	m.mu.Lock()
	item := m.items[id]
	m.mu.Unlock()
	if item == nil {
		return
	}
	item.end()
	<-item.ready
	m.remove(id, item)
}

// remove ends a screen and tells clients it is gone
func (m *screenManager) remove(id string, item *agentScreen) {
	m.mu.Lock()
	if m.items[id] != item {
		m.mu.Unlock()
		return
	}
	delete(m.items, id)
	m.mu.Unlock()
	item.end()
	item.mu.Lock()
	if item.stopLoop != nil {
		item.stopLoop()
		item.stopLoop = nil
	}
	if item.held != nil {
		close(item.held)
		item.held = nil
	}
	if item.asked != nil {
		close(item.asked)
		item.asked = nil
	}
	item.state = protocol.ScreenState{Session: id, State: protocol.ScreenStopped, Holder: protocol.HolderAgent}
	running := item.screen
	item.mu.Unlock()
	if running != nil {
		running.Stop()
	}
	_ = os.Remove(filepath.Join(paths.ScreensDir(), id+".png"))
	m.publish(item)
}

// idleStop ends the screens nobody used for screenIdle: one the user was asked
// for and never answered, one that failed and the agent never asked about
// again, or one up and left alone
func (m *screenManager) idleStop(now time.Time) {
	m.mu.Lock()
	var idle []string
	for id, item := range m.items {
		item.mu.Lock()
		alone := item.err != nil || now.Sub(item.watched) >= screenWatch && item.held == nil && item.asked == nil
		if alone && now.Sub(item.last) >= screenIdle {
			idle = append(idle, id)
		}
		item.mu.Unlock()
	}
	m.mu.Unlock()
	for _, id := range idle {
		m.stop(id)
	}
}

func (m *screenManager) busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items) > 0
}

func (m *screenManager) stopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.items))
	for id := range m.items {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.stop(id)
	}
}
