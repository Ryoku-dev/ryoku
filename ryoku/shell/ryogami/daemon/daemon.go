package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// daemon wires the pieces: the wallpaper surface (rendered by the shell QML),
// the catalog store (served to the picker), the event hub, and the managed
// picker process. One unix socket serves everything.
type daemon struct {
	cfg     config
	cfgMu   sync.Mutex
	surface *wallSurface
	store   *store
	events  *eventHub
	ui      *managedProcess

	currentMu sync.Mutex
	current   string // basename of the last applied wallpaper

	restoreMu sync.Mutex // serializes restoreOutputs: startup, retry, output-added, manual

	random    *randomRotation
	video     *videoPlayer
	optimizer *Optimizer
	grader    *Grader
	upscaler  *Upscaler
	playlists *playlistManager

	// paintSeq orders frame publishes: the video player's delayed yield and
	// death fallback drop their repaint when a newer apply has since painted.
	paintSeq atomic.Int64

	// previous transition preset index (-1 = none); guards the no-repeat pick.
	lastTransition int

	scanMu   sync.Mutex // one rescan at a time; rescans are idempotent
	scanning bool

	sources *sources
	tasks   *taskRegistry

	workshop *workshopLib
	paper    *paperClient
}

func (d *daemon) config() config {
	d.cfgMu.Lock()
	defer d.cfgMu.Unlock()
	return d.cfg
}

func (d *daemon) reloadConfig() {
	fresh := loadConfig()
	d.cfgMu.Lock()
	d.cfg = fresh
	d.cfgMu.Unlock()
}

func (d *daemon) setCurrent(name string) {
	d.currentMu.Lock()
	d.current = name
	d.currentMu.Unlock()
}

func (d *daemon) currentName() string {
	d.currentMu.Lock()
	defer d.currentMu.Unlock()
	return d.current
}

func (d *daemon) broadcast(name string, data interface{}) {
	b, err := json.Marshal(event{Event: name, Data: data})
	if err != nil {
		return
	}
	d.events.publish(string(b))
}

// daemonLive reports whether a daemon is answering on the socket.
func daemonLive(sock string) bool {
	c, err := net.Dial("unix", sock)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func runDaemon() error {
	cfg := loadConfig()
	sock := socketPath()
	_ = os.MkdirAll(filepath.Dir(sock), 0o755)
	// A bare `ryogami` in a terminal must not steal the session daemon's
	// socket: the interloper restores the saved wallpaper over the live one
	// and dies with the terminal, leaving the picker with no daemon at all.
	// When a daemon answers the socket, bow out; a stale socket left by a
	// dead daemon falls through to the rebind.
	if daemonLive(sock) {
		return fmt.Errorf("a daemon is already running on %s", sock)
	}
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "ryogami: listening on %s\n", sock)

	d := &daemon{
		cfg:            cfg,
		surface:        newWallSurface(),
		store:          openStore(cfg.cacheDir()),
		events:         newEventHub(),
		random:         newRandomRotation(),
		lastTransition: -1,
		video:          newVideoPlayer(),
	}
	d.ui = newPickerProcess(d.pickerGpuEnv)
	d.tasks = newTaskRegistry(d)
	d.playlists = newPlaylistManager(cfg.cacheDir(), d)
	d.sources = newSources(d)
	d.paper = newPaperClient(d)
	d.workshop = newWorkshopLib(d)
	d.migrateLegacyConfig()
	cfg = d.config()
	// A finished pipeline replaced sources on disk, so the catalog rescans;
	// every pipeline event also reaches subscribed clients untouched.
	d.optimizer = NewOptimizer(cfg.wallpaperDir(), cfg.videoDir(), func(ev string, data map[string]interface{}) {
		d.broadcast(ev, data)
		if strings.HasSuffix(ev, ".finished") {
			go d.rescan(true)
		}
	})

	// grade.* runs synchronously (no job), so it needs no event sink -- the RPC
	// returns the written path. upscale.* is a cancellable job like optimize.*:
	// its finished event rescans so an enhanced image/clip refreshes the catalog.
	d.grader = NewGrader(cfg.cacheDir())
	d.upscaler = NewUpscaler(stateHome(), func(ev string, data map[string]interface{}) {
		d.broadcast(ev, data)
		if strings.HasSuffix(ev, ".finished") {
			go d.rescan(true)
		}
	})

	// livewall children must not outlive the daemon, and a previous daemon's
	// orphans must not play under a fresh restore.
	d.video.Stop()
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		d.video.Stop()
		d.ui.kill()
		os.Exit(0)
	}()

	// The wallpaper daemon is session-owned, but the picker is demand-owned.
	// Do not preload a half-gigabyte Qt scene at login merely to save a cold
	// open later. Clean up a picker left by an older daemon; wallpaper ui/settings
	// cold-start one when the user actually asks for it.
	d.ui.clearLeftover()

	// Publish the empty snapshot so a subscriber before the first set sees a
	// defined frame, then restore the last wallpaper and rescan the catalog.
	d.surface.publishCurrent()
	go func() {
		d.healAnimatedWebp()
		if d.restoreOnStartup() {
			d.migrateLegacyOutputs()
			switch want, applied := d.restoreOutputs(); {
			case want == 0:
				// Nothing was ever recorded (a fresh install, or a box cut over
				// from awww without setting one through Ryogami). Paint a shipped
				// default so the desktop lands on a wallpaper instead of the empty
				// grey frame; "init" persists it, so the next login restores it.
				d.applyDefaultWallpaper()
			case applied == 0:
				// A login race can leave the file the choice names, or the
				// outputs a live wall spans, not yet present; keep trying rather
				// than leave the desktop on the empty grey frame until a manual set.
				go d.retryRestore()
			}
		}
		d.rescan(false)
		d.playlists.resumeAll()
		d.broadcast("ryogami.wall.scan_done", map[string]interface{}{})
		d.runStartHooks()
	}()
	go d.watchSettingsFile()
	go d.startLibraryWatch()
	go d.watchOutputs()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()
			d.handle(conn)
		}()
	}
}

// handle serves one connection: a `subscribe <topic>` line streams that topic;
// anything else enters the request loop, which answers verb lines and JSON
// requests in order and, after a JSON `subscribe`, pushes broadcast events on
// the same connection. One-shot clients just close after their reply.
func (d *daemon) handle(conn net.Conn) {
	r := bufio.NewReaderSize(conn, 64*1024)
	first, err := r.ReadString('\n')
	if err != nil && first == "" {
		return
	}
	cmd := strings.TrimSpace(first)
	if cmd == "" {
		return
	}
	if name, okSub := strings.CutPrefix(cmd, "subscribe "); okSub && !strings.HasPrefix(cmd, "{") {
		d.serveTopic(conn, r, strings.TrimSpace(name))
		return
	}
	d.serveRequests(conn, r, cmd)
}

func (d *daemon) serveTopic(conn net.Conn, r *bufio.Reader, name string) {
	if name != "wallpaper" {
		fmt.Fprintf(conn, "err unknown topic: %s\n", name)
		return
	}
	last, has, ch := d.surface.topic.subscribe()
	defer d.surface.topic.unsubscribe(ch)
	if has {
		if _, err := fmt.Fprintf(conn, "%s\n", last); err != nil {
			return
		}
	}
	done := make(chan struct{})
	go func() {
		// Further client input or EOF ends the stream.
		buf := make([]byte, 256)
		for {
			if _, err := r.Read(buf); err != nil {
				close(done)
				return
			}
		}
	}()
	for {
		select {
		case frame := <-ch:
			if _, err := fmt.Fprintf(conn, "%s\n", frame); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// Requests run in order on their own goroutine while this one delivers events, so
// a slow or stuck request never holds back a toggle meant for the same picker.
func (d *daemon) serveRequests(conn net.Conn, r *bufio.Reader, first string) {
	quit := make(chan struct{})
	defer close(quit)

	var writeMu sync.Mutex
	write := func(line string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := fmt.Fprintf(conn, "%s\n", line)
		return err
	}

	lines := make(chan string, 4)
	go func() {
		defer close(lines)
		for {
			l, err := r.ReadString('\n')
			if l != "" {
				select {
				case lines <- l:
				case <-quit:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	subscribed := make(chan chan string, 1)
	// Carries whether this connection painted a hover preview, which it restores on leaving.
	finished := make(chan bool, 1)
	go func() {
		var events chan string
		hovered := false
		defer func() { finished <- hovered }()
		serve := func(line string) bool {
			cmd := strings.TrimSpace(line)
			if cmd == "" {
				return true
			}
			if !strings.HasPrefix(cmd, "{") {
				return write(d.dispatchVerb(cmd)) == nil
			}
			hovered = hovered || strings.Contains(cmd, `"palette.hover"`)
			was := events
			reply := d.dispatchJSON(cmd, &events)
			if err := write(reply); err != nil {
				if was == nil && events != nil {
					d.events.unsubscribe(events)
				}
				return false
			}
			if was == nil && events != nil {
				subscribed <- events
			}
			return true
		}
		if !serve(first) {
			return
		}
		for {
			select {
			case l, open := <-lines:
				if !open || !serve(l) {
					return
				}
			case <-quit:
				return
			}
		}
	}()

	// Nil until the connection subscribes; a nil channel never fires.
	var events chan string
	defer func() {
		if events != nil {
			d.events.unsubscribe(events)
		}
	}()
	for {
		select {
		case ch := <-subscribed:
			events = ch
		case ev := <-events:
			if write(ev) != nil {
				d.events.unsubscribe(events)
				events = nil
				// The reader and a waiting request see the close and wind the worker down.
				conn.Close()
			}
		case hovered := <-finished:
			select {
			case ch := <-subscribed:
				events = ch
			default:
			}
			if hovered {
				d.restoreHoverPreview()
			}
			return
		}
	}
}

// dispatchJSON answers one JSON-RPC line with the full serialized Response. A
// `subscribe` request attaches this connection to the event hub.
func (d *daemon) dispatchJSON(cmd string, events *chan string) string {
	var req request
	if err := json.Unmarshal([]byte(cmd), &req); err != nil {
		return fmt.Sprintf("err parse: %v", err)
	}
	var resp response
	if req.Method == "subscribe" {
		if *events == nil {
			*events = d.events.subscribe()
		}
		resp = ok(req.ID, map[string]interface{}{"subscribed": true})
	} else {
		resp = d.dispatchRequest(&req)
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf("err serialize: %v", err)
	}
	return string(b)
}

// resetCache drops every derived artifact (thumbnails, colour data, animated
// previews, transcoded clips and stills) and the catalogue itself, then scans
// the folders from nothing. The picker's Refresh button: for a library the
// mtime-gated rescan cannot mend (a thumb that failed, a clip transcoded with
// flags livewall no longer expects, a file the watcher missed).
func (d *daemon) resetCache() {
	cfg := d.config()
	cacheDir := cfg.cacheDir()
	for _, sub := range []string{"wallpaper/thumbs", "wallpaper/thumbs-sm", "wallpaper/video-thumbs", "wallpaper/anim"} {
		if err := os.RemoveAll(filepath.Join(cacheDir, sub)); err != nil {
			fmt.Fprintf(os.Stderr, "ryogami: cache reset: %v\n", err)
		}
	}
	// A playing clip reads its transcode from this cache: stop it first, then
	// restore the stored wallpaper after the wipe so the clip is transcoded
	// afresh and plays again.
	playing := d.video.Playing()
	if playing {
		d.video.Stop()
	}
	pruneLivewallCache(0)
	d.store.replaceAll(map[string]Entry{})
	d.broadcast("ryogami.wall.cache", map[string]interface{}{"status": "started", "progress": 0, "total": 0})
	if playing {
		d.restoreOutputs()
	}
	d.rescan(true)
}

// rescan rebuilds the catalog off the connection path; force regenerates
// nothing extra today (thumbs are mtime-gated), it only bypasses the
// one-at-a-time gate's early return so an explicit rebuild always runs.
func (d *daemon) rescan(force bool) {
	d.scanMu.Lock()
	if d.scanning && !force {
		d.scanMu.Unlock()
		return
	}
	d.scanning = true
	d.scanMu.Unlock()
	defer func() {
		d.scanMu.Lock()
		d.scanning = false
		d.scanMu.Unlock()
	}()

	cfg := d.config()
	prior := d.store.snapshotEntries()
	// The chip appears only once a thumbnail is actually built, so warm rescans never flash one.
	built := 0
	fresh, err := ScanDirs(cfg.wallpaperDir(), cfg.videoDir(), cfg.cacheDir(), prior, func(e Entry) {
		if built == 0 {
			d.tasks.start("scan", "scan", "Scan", 0, nil)
		}
		built++
		d.tasks.progress("scan", built, 0, e.Name)
		d.broadcast("ryogami.wall.cached", e)
	})
	if err != nil {
		d.tasks.finish("scan", taskFailed, built, err.Error())
		fmt.Fprintf(os.Stderr, "ryogami: scan: %v\n", err)
		return
	}
	d.tasks.finish("scan", taskCompleted, built, "")
	d.store.replaceAll(fresh)
	d.runAfterScan(prior, fresh)
	d.broadcast("ryogami.wall.cache", map[string]interface{}{"status": "ready", "count": len(fresh)})
}
