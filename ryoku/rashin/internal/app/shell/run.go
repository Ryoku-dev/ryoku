// Package shell owns the Rashin window, navigation, overlays and application lifetime.
package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	gioapp "gioui.org/app"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

const applicationID = "dev.ryoku.rashin"

func init() { gioapp.ID = applicationID }

type Options struct {
	Demo            bool
	PNG             string
	Width, Height   int
	Mode            string
	Route           string
	Session         string
	Project         string
	WorkTab         string
	Worktree        string
	SettingsSection string
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
	ctx      context.Context
	cancel   context.CancelFunc
	store    *state.Store
	client   *conn.Client
	hub      invalidationHub
	themeMu  sync.RWMutex
	theme    *look.Theme
	pending  *look.Theme
	tempDir  string
	serveErr chan error
}

func Run(options Options) error {
	mode, err := look.ParseMode(options.Mode)
	if err != nil {
		return err
	}
	r, err := startRuntime(options, mode)
	if err != nil {
		return err
	}
	defer r.Close()
	preferences := state.LoadAppPreferences()
	if options.ZoomPercent != 0 {
		preferences.ZoomPercent = options.ZoomPercent
	}
	options.Preferences = preferences.Normalize()
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

func startRuntime(options Options, mode palette.Mode) (*runtime, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &runtime{ctx: ctx, cancel: cancel, theme: look.New(mode), serveErr: make(chan error, 1)}
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
		go func() { r.serveErr <- demo.Serve(ctx, socket) }()
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
	go look.Watch(ctx, func() palette.Mode { return mode }, func(theme *look.Theme) {
		r.themeMu.Lock()
		r.pending = theme
		r.themeMu.Unlock()
		r.hub.Invalidate()
	})
	return r, nil
}

func (r *runtime) Theme() *look.Theme {
	r.themeMu.Lock()
	defer r.themeMu.Unlock()
	if r.pending != nil {
		r.theme = r.pending
		r.pending = nil
	}
	return r.theme
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
