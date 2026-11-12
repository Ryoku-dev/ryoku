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
	"ryoku-rashin/internal/app/env"
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
	// stops the palette watcher, set only while the colours follow the wallpaper
	unwatch  context.CancelFunc
	themeMu  sync.RWMutex
	mode     palette.Mode
	look     palette.Look
	theme    *look.Theme
	pending  *look.Theme
	tempDir  string
	serveErr chan error
}

// the colours words and the palette modes they paint
var coloursModes = map[string]palette.Mode{
	state.ColoursRyoku:      palette.Dark,
	state.ColoursRyokuLight: palette.Light,
	state.ColoursWallpaper:  palette.Follow,
}

// an unknown word is the ryoku.dev stock, as the preferences read it
func coloursMode(word string) palette.Mode {
	if mode, ok := coloursModes[word]; ok {
		return mode
	}
	return palette.Dark
}

func modeColours(mode palette.Mode) string {
	for word, painted := range coloursModes {
		if painted == mode {
			return word
		}
	}
	return state.ColoursRyoku
}

func Run(options Options) error {
	preferences := state.LoadAppPreferences()
	if options.ZoomPercent != 0 {
		preferences.ZoomPercent = options.ZoomPercent
	}
	options.Preferences = preferences.Normalize()
	mode := coloursMode(options.Preferences.Colours)
	// --mode paints this run only, what is stored stays
	if options.Mode != "" {
		var err error
		if mode, err = palette.ParseMode(options.Mode); err != nil {
			return err
		}
	}
	r, err := startRuntime(options, mode)
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

func startRuntime(options Options, mode palette.Mode) (*runtime, error) {
	ctx, cancel := context.WithCancel(context.Background())
	preferences := options.Preferences.Normalize()
	current := palette.ParseLook(preferences.Contrast, preferences.Tint)
	r := &runtime{ctx: ctx, cancel: cancel, mode: mode, look: current, theme: look.New(mode, current), serveErr: make(chan error, 1)}
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
	r.watch(mode)
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

func (r *runtime) currentLook() palette.Look {
	r.themeMu.RLock()
	defer r.themeMu.RUnlock()
	return r.look
}

// the colours word of what the window paints, which --mode may have set for this run
func (r *runtime) colours() string {
	r.themeMu.RLock()
	defer r.themeMu.RUnlock()
	return modeColours(r.mode)
}

func (r *runtime) themeChanged(theme *look.Theme) {
	r.themeMu.Lock()
	// a palette that resolved as the colours stopped following is not painted over the stock
	if r.mode != palette.Follow {
		r.themeMu.Unlock()
		return
	}
	if theme.Look != r.look {
		// the look moved while this palette resolved, so the palette is taken again with the new look
		theme = look.New(palette.Follow, r.look)
	}
	r.pending = theme
	r.themeMu.Unlock()
	r.hub.Invalidate()
}

func (r *runtime) setLook(contrast, tint string) {
	r.themeMu.RLock()
	mode := r.mode
	r.themeMu.RUnlock()
	r.restyle(mode, palette.ParseLook(contrast, tint))
}

func (r *runtime) setColours(word string) {
	r.restyle(coloursMode(word), r.currentLook())
}

func (r *runtime) restyle(mode palette.Mode, current palette.Look) {
	theme := look.New(mode, current)
	r.themeMu.Lock()
	r.mode, r.look, r.pending = mode, current, theme
	r.themeMu.Unlock()
	r.watch(mode)
	r.hub.Invalidate()
}

// the palette files are watched only while the colours follow the wallpaper, so the stock costs nothing at rest
func (r *runtime) watch(mode palette.Mode) {
	following := mode == palette.Follow
	if following == (r.unwatch != nil) {
		return
	}
	if !following {
		r.unwatch()
		r.unwatch = nil
		return
	}
	ctx, cancel := context.WithCancel(r.ctx)
	r.unwatch = cancel
	go look.Watch(ctx, r.currentLook, r.themeChanged)
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
