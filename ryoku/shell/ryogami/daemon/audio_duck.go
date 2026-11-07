package main

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Ducking mutes the wallpaper's own streams: the daemon does not own the mixer.

const duckSettingKey = "playback.muteOnOtherAudio"

const (
	duckRestartMin = time.Second
	duckRestartMax = time.Minute
	duckHealthyRun = time.Minute
	duckDebounce   = 150 * time.Millisecond
	duckListLimit  = 5 * time.Second
)

var wallpaperAudioBins = []string{
	"ryogami-live", "skwd-paper-v2", "skwd-wall-vk", "paper-vk",
}

var duckState struct {
	mu     sync.Mutex
	ducked bool
}

// pactlRunner is every way the ducking code reaches pactl, so tests can count spawns.
type pactlRunner struct {
	lookPath  func() string
	list      func(ctx context.Context, bin string) ([]byte, error)
	subscribe func(ctx context.Context, bin string) (io.Reader, func() error, error)
}

var duckPactl = pactlRunner{
	lookPath: func() string {
		p, err := exec.LookPath("pactl")
		if err != nil {
			return ""
		}
		return p
	},
	list: func(ctx context.Context, bin string) ([]byte, error) {
		return exec.CommandContext(ctx, bin, "list", "sink-inputs").Output()
	},
	subscribe: func(ctx context.Context, bin string) (io.Reader, func() error, error) {
		cmd := exec.CommandContext(ctx, bin, "subscribe")
		// The unit stops only the daemon, so the watcher must end with it rather than outlive it.
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		return stdout, cmd.Wait, nil
	},
}

func parseOtherAudio(out string, wallpaperBins []string) bool {
	inSet := func(bin string) bool {
		for _, w := range wallpaperBins {
			if bin == w {
				return true
			}
		}
		return false
	}
	var corked, wallpaper bool
	fresh := true
	other := false
	flush := func() {
		if fresh {
			return
		}
		if !wallpaper && !corked {
			other = true
		}
	}
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "Sink Input #"):
			flush()
			corked, wallpaper, fresh = false, false, false
		case strings.HasPrefix(t, "Corked:"):
			corked = strings.EqualFold(strings.TrimSpace(t[len("Corked:"):]), "yes")
		case strings.Contains(t, "application.process.binary"):
			if inSet(unquoteProp(t)) {
				wallpaper = true
			}
		case strings.Contains(t, "media.role"):
			if strings.EqualFold(unquoteProp(t), "wallpaper") {
				wallpaper = true
			}
		case strings.Contains(t, "application.name"):
			if v := strings.ToLower(unquoteProp(t)); strings.Contains(v, "ryogami") || strings.Contains(v, "wallpaper") {
				wallpaper = true
			}
		}
	}
	flush()
	return other
}

func unquoteProp(line string) string {
	i := strings.Index(line, "=")
	if i < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(line[i+1:]), "\"")
}

func otherAudioPlaying(ctx context.Context) bool {
	bin := duckPactl.lookPath()
	if bin == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, duckListLimit)
	defer cancel()
	out, err := duckPactl.list(ctx, bin)
	if err != nil {
		return false
	}
	return parseOtherAudio(string(out), wallpaperAudioBins)
}

func (d *daemon) evaluateDuck(ctx context.Context) {
	duckState.mu.Lock()
	defer duckState.mu.Unlock()
	want := d.settingBool(duckSettingKey) && otherAudioPlaying(ctx)
	if want == duckState.ducked {
		return
	}
	duckState.ducked = want
	if want {
		m := true
		d.surface.setAudio(&m, nil, nil)
		if d.paper != nil {
			_ = d.paper.setAudio(&m, nil, nil)
		}
		return
	}
	d.restoreDuckedAudio()
}

func (d *daemon) restoreDuckedAudio() {
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(d.config().cacheDir(), "outputs.json"), &state)
	def := wallAudioDefaults()
	if len(state) == 0 {
		m, vol := def.mute, def.volume
		d.surface.setAudio(&m, &vol, nil)
		return
	}
	for out, e := range state {
		m, vol := entryAudio(e, def)
		var outs []string
		if out != "*" {
			outs = []string{out}
		}
		d.surface.setAudio(&m, &vol, outs)
		if d.paper != nil {
			_ = d.paper.setAudio(&m, &vol, outs)
		}
	}
}

// duckWatcher owns the one pactl subscribe; it runs only while the setting is on.
var duckWatcher struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func (d *daemon) syncDuckWatcher() {
	if !d.settingBool(duckSettingKey) {
		stopDuckWatcher()
		return
	}
	duckWatcher.mu.Lock()
	defer duckWatcher.mu.Unlock()
	if duckWatcher.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	duckWatcher.cancel, duckWatcher.done = cancel, done
	go func() {
		defer close(done)
		d.watchOtherAudio(ctx)
	}()
}

// Waiting for the watcher means no pactl child is left once the setting reads off.
func stopDuckWatcher() {
	duckWatcher.mu.Lock()
	defer duckWatcher.mu.Unlock()
	if duckWatcher.cancel == nil {
		return
	}
	duckWatcher.cancel()
	<-duckWatcher.done
	duckWatcher.cancel, duckWatcher.done = nil, nil
}

// restartBackoff spaces pactl restarts so a missing audio server costs nothing.
type restartBackoff struct{ next time.Duration }

// after returns the wait before the next start, given how long the last run lasted.
func (b *restartBackoff) after(ran time.Duration) time.Duration {
	if b.next == 0 || ran >= duckHealthyRun {
		b.next = duckRestartMin
	}
	wait := b.next
	b.next = min(b.next*2, duckRestartMax)
	return wait
}

func sleepCtx(ctx context.Context, wait time.Duration) bool {
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (d *daemon) watchOtherAudio(ctx context.Context) {
	var backoff restartBackoff
	for {
		began := time.Now()
		d.subscribeOtherAudio(ctx)
		if !sleepCtx(ctx, backoff.after(time.Since(began))) {
			return
		}
	}
}

func (d *daemon) subscribeOtherAudio(ctx context.Context) {
	bin := duckPactl.lookPath()
	if bin == "" {
		return
	}
	stdout, wait, err := duckPactl.subscribe(ctx, bin)
	if err != nil {
		return
	}
	// Evaluated after the subscribe starts, so a change during the gap is not missed.
	d.evaluateDuck(ctx)
	pending := make(chan struct{}, 1)
	var debounced sync.WaitGroup
	debounced.Add(1)
	go func() {
		defer debounced.Done()
		for range pending {
			if !sleepCtx(ctx, duckDebounce) {
				return
			}
			select {
			case <-pending:
			default:
			}
			d.evaluateDuck(ctx)
		}
	}()
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if !strings.Contains(sc.Text(), "sink-input") {
			continue
		}
		select {
		case pending <- struct{}{}:
		default:
		}
	}
	close(pending)
	debounced.Wait()
	_ = wait()
}

func init() {
	onStart(func(d *daemon) { d.syncDuckWatcher() })
	watchSetting(duckSettingKey, func(d *daemon, _ string, _ interface{}) {
		d.syncDuckWatcher()
		if !d.settingBool(duckSettingKey) {
			d.evaluateDuck(context.Background())
		}
	})
}
