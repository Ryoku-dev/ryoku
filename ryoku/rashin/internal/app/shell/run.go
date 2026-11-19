// Package shell owns the Rashin window, navigation, overlays and application lifetime.
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

const applicationID = "dev.ryoku.rashin"

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
		directory, err := os.MkdirTemp("/dev/shm", "rashin-app-demo-")
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create demo directory: %w", err)
		}
		r.tempDir = directory
		socket := filepath.Join(directory, "rashin.sock")
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
