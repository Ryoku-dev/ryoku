package main

import (
	"context"
	"encoding/json"
	"time"

	wm "ryoku-wm"
)

// The daemon reaches the compositor only through d.wmc and the state its watcher
// keeps warm. activeMonitor resolves on the keybind hot path, so it is a mutex
// read with no fork.

func (d *daemon) startWM() {
	d.wmTopic = d.registerTopic("wm")
	d.wmFocusTopic = d.registerTopic("wm.focus")
	d.publishWM()
	d.publishWMFocus()
	d.registerCall("wm.act", func(raw json.RawMessage) (any, error) {
		var a struct {
			Action string   `json:"action"`
			Args   []string `json:"args"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		// ActOutput so value-reporting actions (the launcher's
		// focus-follows-mouse freeze hands back the previous mode) reach the
		// caller; actions that print nothing answer with an empty result.
		return d.wmc.ActOutput(wm.Action(a.Action), a.Args...)
	})
	go d.watchWindowManager()
}

func (d *daemon) activeMonitor() string {
	d.wmMu.Lock()
	defer d.wmMu.Unlock()
	return d.activeMon
}

// onWMFrame folds one watch frame into the cache. Focus has its own compact
// topic: cursor crossing is a hot path, and parsing the output, workspace and
// window lists again on every crossing used to stall the GUI thread on larger
// setups. Providers suppress unchanged list frames before this process boundary.
func (d *daemon) onWMFrame(f wm.Frame) {
	d.wmMu.Lock()
	changed := true
	switch f.Kind {
	case wm.FrameFocus:
		changed = d.activeMon != f.FocusedOutput
		d.activeMon = f.FocusedOutput
	case wm.FrameOutputs:
		d.wmOutputs = f.Outputs
	case wm.FrameWorkspaces:
		d.wmWorkspaces = f.Workspaces
	case wm.FrameWindows:
		d.wmWindows = f.Windows
	case wm.FrameKeyboard:
		d.wmKbdLayout = f.KeyboardLayout
		d.wmKbdList = f.KeyboardLayouts
	case wm.FrameOverview:
		d.wmOverview = f.OverviewOpen
	case wm.FrameReady:
		// A reconnect refreshes provider-static capabilities even though ready
		// was already true.
		d.wmReady = true
	default:
		d.wmMu.Unlock()
		return
	}
	if !changed {
		d.wmMu.Unlock()
		return
	}
	if d.wmVersions == nil {
		d.wmVersions = map[string]int{}
	}
	d.wmVersions[string(f.Kind)]++
	d.wmMu.Unlock()

	switch f.Kind {
	case wm.FrameOutputs, wm.FrameWorkspaces:
		select {
		case d.widgetSig <- struct{}{}:
		default:
		}
	}

	// A hotplug (dock/undock, lid, VT resume) moves the output set. The night
	// light self-heals from it: the compositor can revoke a running backend's
	// gamma ownership on a connector change without telling it, which leaves
	// the screen cold while the bar still reads on. See rearmOnOutputs.
	if f.Kind == wm.FrameOutputs && d.nightlight != nil {
		d.nightlight.rearmOnOutputs(f.Outputs)
	}
	if f.Kind == wm.FrameFocus {
		d.publishWMFocus()
		return
	}
	d.publishWM()
}

// wmTopicFrame carries caps and state in one coalesced frame so a QML consumer
// needs a single subscription. Every capability is present with an explicit
// boolean and the lists are never null, so a consumer never tells absent from
// false. Versions tags each section with the count of frames that changed it
// since the daemon started. The frame is always a full snapshot, so a consumer
// rebinds only the sections whose version moved and catches up on any it missed
// while the coalescing topic dropped intermediate frames. The provider-static
// fields (caps, model, config) ride the "ready" version, which moves when the
// provider stream (re)connects, so a compositor switch refreshes them.
type wmTopicFrame struct {
	Provider        string                 `json:"provider"`
	WorkspaceModel  string                 `json:"workspaceModel"`
	Ready           bool                   `json:"ready"`
	Caps            map[wm.Capability]bool `json:"caps"`
	FocusedOutput   string                 `json:"focusedOutput"`
	Outputs         []wm.Output            `json:"outputs"`
	Workspaces      []wm.Workspace         `json:"workspaces"`
	Windows         []wm.Window            `json:"windows"`
	ConfigFiles     []string               `json:"configFiles"`
	OverviewOpen    bool                   `json:"overviewOpen"`
	KeyboardLayout  string                 `json:"keyboardLayout"`
	KeyboardLayouts []string               `json:"keyboardLayouts"`
	Versions        map[string]int         `json:"versions"`
}

type wmFocusFrame struct {
	FocusedOutput string `json:"focusedOutput"`
	Version       int    `json:"version"`
}

func (d *daemon) publishWMFocus() {
	if d.wmFocusTopic == nil {
		return
	}
	d.wmMu.Lock()
	frame := wmFocusFrame{
		FocusedOutput: d.activeMon,
		Version:       d.wmVersions[string(wm.FrameFocus)],
	}
	d.wmMu.Unlock()
	if b, err := json.Marshal(frame); err == nil {
		d.wmFocusTopic.publish(b)
	}
}

func (d *daemon) publishWM() {
	if d.wmTopic == nil {
		return
	}
	caps, _ := d.wmc.Caps()
	all := wm.All()
	frame := wmTopicFrame{
		Provider:        caps.Name,
		WorkspaceModel:  string(caps.WorkspaceModel),
		Caps:            make(map[wm.Capability]bool, len(all)),
		Outputs:         []wm.Output{},
		Workspaces:      []wm.Workspace{},
		Windows:         []wm.Window{},
		ConfigFiles:     []string{},
		KeyboardLayouts: []string{},
	}
	for _, c := range all {
		frame.Caps[c] = caps.Has(c)
	}
	if caps.ConfigFiles != nil {
		frame.ConfigFiles = caps.ConfigFiles
	}
	d.wmMu.Lock()
	frame.Ready = d.wmReady
	frame.FocusedOutput = d.activeMon
	frame.OverviewOpen = d.wmOverview
	frame.KeyboardLayout = d.wmKbdLayout
	if d.wmKbdList != nil {
		frame.KeyboardLayouts = d.wmKbdList
	}
	if d.wmOutputs != nil {
		frame.Outputs = d.wmOutputs
	}
	if d.wmWorkspaces != nil {
		frame.Workspaces = d.wmWorkspaces
	}
	if d.wmWindows != nil {
		frame.Windows = d.wmWindows
	}
	frame.Versions = make(map[string]int, len(d.wmVersions)+1)
	for k, v := range d.wmVersions {
		frame.Versions[k] = v
	}
	d.wmMu.Unlock()
	if b, err := json.Marshal(frame); err == nil {
		d.wmTopic.publish(b)
	}
}

// watchWindowManager supervises the provider's watch stream for the daemon's
// life, reconnecting with backoff so a compositor restart never leaves the cache
// permanently cold. Backoff resets once a stream has delivered a frame.
func (d *daemon) watchWindowManager() {
	const minBackoff = 150 * time.Millisecond
	const maxBackoff = 5 * time.Second
	backoff := minBackoff
	for {
		select {
		case <-d.quit:
			return
		default:
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-d.quit:
				cancel()
			case <-ctx.Done():
			}
		}()
		gotFrame := false
		_ = d.wmc.Watch(ctx, func(f wm.Frame) {
			gotFrame = true
			d.onWMFrame(f)
		})
		cancel()

		select {
		case <-d.quit:
			return
		default:
		}
		if gotFrame {
			backoff = minBackoff
		}
		select {
		case <-d.quit:
			return
		case <-time.After(backoff):
		}
		backoff = capDur(backoff*2, maxBackoff)
	}
}
