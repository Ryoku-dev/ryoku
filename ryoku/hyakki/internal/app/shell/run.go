// Package shell owns the Hyakki window, navigation, overlays and application lifetime.
package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gioapp "gioui.org/app"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

const applicationID = "dev.ryoku.Hyakki"

func init() { gioapp.ID = applicationID }

type Options struct {
	Demo          bool
	PNG           string
	Width, Height int
	// Mode forces dark, light or follow for this run, as LITHIC_MODE does
	Mode            string
	Route           string
	Session         string
	Project         string
	WorkTab         string
	Worktree        string
	SettingsSection string
	Memory          env.MemoryPlace
	Teams           env.TeamsPlace
	ZoomPercent     int
	Script          string
	Shots           []int
	Preferences     state.AppPreferences
	// ActivationToken is consumed by initial native window creation only.
	ActivationToken string
}

type invalidationHub struct {
	mu      sync.Mutex
	next    uint64
	targets map[uint64]func()
}

func (h *invalidationHub) Add(target func()) func() {
	if target == nil {
		return func() {}
	}
	h.mu.Lock()
	if h.targets == nil {
		h.targets = make(map[uint64]func())
	}
	h.next++
	id := h.next
	h.targets[id] = target
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		delete(h.targets, id)
		h.mu.Unlock()
	}
}

func (h *invalidationHub) Invalidate() {
	h.mu.Lock()
	targets := make([]func(), 0, len(h.targets))
	for _, target := range h.targets {
		targets = append(targets, target)
	}
	h.mu.Unlock()
	for _, target := range targets {
		target()
	}
}

type runtime struct {
	ctx    context.Context
	cancel context.CancelFunc
	store  *state.Store
	client *conn.Client
	hub    invalidationHub
	// one theme for the runtime's life, worn anew in place when the desktop changes
	theme   *look.Theme
	themeMu sync.Mutex
	// a desktop reading the watcher left for the window's goroutine
	pending  *palette.Desktop
	tempDir  string
	serveErr chan error
	// the demo's computer for the voice settings, nil off the demo
	demoVoice *demoVoice
	// the demo's keeps file beside its socket, "" off the demo, and whether
	// the demo machine has no text reader
	demoKeeps string
	demoNoOCR bool
}

func Run(options Options) error {
	preferences := state.LoadAppPreferences()
	if options.ZoomPercent != 0 {
		preferences.ZoomPercent = options.ZoomPercent
	}
	options.Preferences = preferences.Normalize()
	// --mode is LITHIC_MODE for this run, so the first reading and every later one keep it and nothing is stored
	if options.Mode != "" {
		if _, err := palette.ParseMode(options.Mode); err != nil {
			return err
		}
		if err := os.Setenv("LITHIC_MODE", options.Mode); err != nil {
			return err
		}
	}
	r, err := startRuntime(options)
	if err != nil {
		return err
	}
	defer r.Close()
	// This selects a fresh demo machine, not a conversation to attach.
	if options.Demo && options.Session == demo.FeatureOnboarding {
		options.Session = ""
	}
	width, height := options.Width, options.Height
	if width <= 0 {
		width = preferences.Width
	}
	if height <= 0 {
		height = preferences.Height
	}
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 820
	}
	options.Width, options.Height = width, height
	if options.PNG != "" {
		return r.renderPNG(options)
	}
	return r.runWindow(options)
}

func startRuntime(options Options) (*runtime, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &runtime{ctx: ctx, cancel: cancel, theme: look.NewFor(palette.ReadDesktop(), nil), serveErr: make(chan error, 1)}
	r.store = state.New(r.hub.Invalidate)
	clientOptions := client.Options{Client: protocol.ClientApp, Start: true}
	if options.Demo {
		directory, err := os.MkdirTemp("/dev/shm", "hyakki-app-demo-")
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create demo directory: %w", err)
		}
		r.tempDir = directory
		r.demoVoice = newDemoVoice(directory, options.Session == demo.FeatureOniNoMic)
		socket := filepath.Join(directory, "oni.sock")
		r.demoKeeps, r.demoNoOCR = demo.KeepsPath(socket), options.Session == demoNoOCR
		clientOptions.Socket, clientOptions.Start = socket, false
		go func() { r.serveErr <- demo.Serve(ctx, socket, demoFeatures(options)...) }()
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(socket); err == nil {
				break
			}
			select {
			case err := <-r.serveErr:
				cancel()
				_ = os.RemoveAll(directory)
				return nil, fmt.Errorf("start demo daemon: %w", err)
			default:
			}
			if time.Now().After(deadline) {
				cancel()
				_ = os.RemoveAll(directory)
				return nil, errors.New("demo daemon did not open its socket")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	r.client = conn.NewOptions(r.store, clientOptions)
	// a PNG is one look at one moment; a window follows the desktop for its whole life
	if options.PNG == "" {
		go func() { _ = palette.WatchDesktop(ctx, r.offerDesktop) }()
	}
	return r, nil
}

func demoFeatures(options Options) []string {
	if options.Session == demo.FeatureOnboarding {
		return []string{demo.FeatureOnboarding}
	}
	switch options.Route {
	case protocol.TabAsk:
		switch options.Session {
		case "ask-compare":
			return []string{"compare"}
		case "ask-artifacts":
			return []string{"artifacts", "mindmap"}
		case "ask-searchsteps-live", "ask-searchsteps-done":
			return []string{"searchsteps"}
		}
	case protocol.TabIndex:
		if strings.HasPrefix(options.Session, "collections/") {
			return []string{"collections"}
		}
	case protocol.TabSettings:
		switch options.SettingsSection {
		case "ask":
			return []string{"prompts", "history"}
		case "agents":
			return []string{"mcp"}
		case "teams":
			return []string{"diskcare"}
		case "memory":
			return []string{"profile"}
		case "oni", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "oni-privacy", "parts":
			// --session oni-local answers the demo's screen asks on this computer,
			// oni-voice turns talking and dictation on, oni-no-mic too on a
			// machine with no microphone and a keyboard Oni may not read,
			// oni-guide turns guides and Keep on with walkthroughs kept,
			// oni-guide-no-ocr too on a machine without a text reader,
			// oni-reflex turns quick commands on with reminders, routines and
			// a ledger to list, oni-reflex-no-wake too on a machine whose
			// wake unit cannot be armed, and oni-small-computer is Oni on a
			// computer under the local decision hardware gate
			switch {
			case options.Session == demo.FeatureOniPhase16, options.Session == demo.FeatureOniPhase17:
				return []string{options.Session}
			case options.Session == demo.FeatureOniLocal:
				return []string{demo.FeatureOni, demo.FeatureOniLocal}
			case options.Session == demo.FeatureOniVoice:
				return []string{demo.FeatureOniVoice}
			case options.Session == demo.FeatureOniNoMic:
				return []string{demo.FeatureOniVoice, demo.FeatureOniNoMic}
			case guideSession(options.Session):
				return []string{demo.FeatureOniGuide}
			case options.Session == demo.FeatureOniReflex, options.Session == demo.FeatureOniReflexNoWake:
				return []string{options.Session}
			case options.Session == demo.FeatureOniSmallComputer:
				return []string{demo.FeatureOni, demo.FeatureOniSmallComputer}
			}
			return []string{demo.FeatureOni}
		case "hands":
			// --session oni-hands-no-browser is a machine with no
			// Chromium-family browser, oni-hands-paused a task paused on the
			// first fixture tab, oni-hands-app Hands in one app window,
			// oni-hands-app-paused a task in a picked window paused by a
			// key, oni-hands-desktop Hands on a separate desktop,
			// oni-hands-no-nest the same without weston and niri, oni Oni
			// with Hands still off; otherwise Hands in a browser is on
			switch options.Session {
			case demo.FeatureOniHandsNoBrowser, demo.FeatureOniHandsPaused, demo.FeatureOni,
				demo.FeatureOniHandsApp, demo.FeatureOniHandsAppPaused, demo.FeatureOniHandsDesktop, demo.FeatureOniHandsNoNest:
				return []string{options.Session}
			}
			return []string{demo.FeatureOniHands}
		}
	case protocol.TabLearning:
		return []string{protocol.FeatureAutolearn, protocol.FeatureDecisions, protocol.FeatureRules}
	case protocol.TabDreams:
		if options.Teams.Team == "dream-research" {
			return []string{"research"}
		}
	case protocol.TabRoutines:
		if options.Teams.Routine == "routine-personal-ask" {
			return []string{"schedules"}
		}
	}
	return nil
}

// Theme is the window's theme, the same one for the runtime's life. Read it
// on the window's goroutine.
func (r *runtime) Theme() *look.Theme { return r.theme }

// offerDesktop leaves a new reading for the window's goroutine and asks for
// the frame that wears it; the hub's redrawer asks again when the request
// lands mid-frame. It runs on the watcher's goroutine.
func (r *runtime) offerDesktop(desktop palette.Desktop) {
	r.themeMu.Lock()
	r.pending = &desktop
	r.themeMu.Unlock()
	r.hub.Invalidate()
}

// wearDesktop puts on the reading the watcher left, if any. Call it on the
// window's goroutine as a frame starts, so every part of a frame sees one look.
func (r *runtime) wearDesktop() {
	r.themeMu.Lock()
	pending := r.pending
	r.pending = nil
	r.themeMu.Unlock()
	if pending != nil {
		r.theme.SetDesktop(*pending)
	}
}

func (r *runtime) Close() {
	r.cancel()
	if r.client != nil {
		r.client.Close()
	}
	if r.tempDir != "" {
		_ = os.RemoveAll(r.tempDir)
	}
}
