package main

import (
	"bufio"
	"encoding/json"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// dayNightRotation alternates between a "day" and a "night" video pool on a
// timer, choosing the pool by the real sunrise/sunset the shell already computes
// for the weather widget (#247). It is a second dimension of the same rotation
// the random mode runs: a ticker whose tick picks a clip and applies it. The
// pools are directories on disk; a clip already shown today for its pool is
// skipped until the whole pool has been seen, then the day's set resets.
type dayNightRotation struct {
	mu       sync.Mutex
	stopCh   chan struct{}
	interval time.Duration
	dayDir   string
	nightDir string
	noRepeat bool
	shown    map[string]map[string]bool // pool -> paths shown since the last reset
	dayKey   string                     // local date the shown sets belong to
	forced   string                     // "" | "day" | "night" manual override (testing)
}

func newDayNightRotation() *dayNightRotation {
	return &dayNightRotation{shown: map[string]map[string]bool{}}
}

// dayNightConfig mirrors the picker's daynight block in config.json. The daemon
// reads it straight from the wall-ui file so the GUI's Save is the only writer.
type dayNightConfig struct {
	Enabled  bool   `json:"enabled"`
	DayDir   string `json:"dayDir"`
	NightDir string `json:"nightDir"`
	Interval int    `json:"rotateIntervalMinutes"`
	NoRepeat *bool  `json:"noRepeatWithinDay"`
}

// dayNightFromWall reads the daynight block from the picker's config.json.
// Absent or malformed yields a disabled config.
func dayNightFromWall() dayNightConfig {
	var c dayNightConfig
	var data struct {
		DayNight *dayNightConfig `json:"daynight"`
	}
	loadJSON(filepath.Join(ryogamiWallConfigDir(), "config.json"), &data)
	if data.DayNight != nil {
		c = *data.DayNight
	}
	return c
}

// isDay reports whether it is currently daytime, taken from the shell daemon's
// weather topic rather than a second geolocation/sunrise call. The topic is a
// state stream, so a subscribe replays the current frame immediately; the first
// frame's current.isDay is the answer. Any failure (no shell daemon, no weather
// yet) returns the caller's fallback so rotation never stalls on a missing
// network source.
func isDay(fallback bool) bool {
	conn, err := net.DialTimeout("unix", shellSocketPath(), 500*time.Millisecond)
	if err != nil {
		return fallback
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("subscribe weather\n")); err != nil {
		return fallback
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var frame struct {
			Current *struct {
				IsDay *bool `json:"isDay"`
			} `json:"current"`
		}
		if json.Unmarshal(sc.Bytes(), &frame) != nil {
			continue
		}
		if frame.Current != nil && frame.Current.IsDay != nil {
			return *frame.Current.IsDay
		}
	}
	return fallback
}

// shellSocketPath is ryoku-shell's control socket, where the weather topic lives.
func shellSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "ryoku-shell.sock")
}

// poolFor returns the active pool directory for the current phase.
func (r *dayNightRotation) poolFor(day bool) string {
	if day {
		return r.dayDir
	}
	return r.nightDir
}

// pick returns a clip from the active pool that has not been shown today,
// resetting the day's set once the pool is exhausted. videoFiles lists the pool;
// an empty pool yields "".
func (r *dayNightRotation) pick(day bool) string {
	pool := r.poolFor(day)
	if pool == "" {
		return ""
	}
	clips := videoFiles(pool)
	if len(clips) == 0 {
		return ""
	}
	key := pool
	if !r.noRepeat {
		return clips[rand.IntN(len(clips))]
	}
	today := time.Now().Format("2006-01-02")
	if r.dayKey != today {
		r.shown = map[string]map[string]bool{}
		r.dayKey = today
	}
	seen := r.shown[key]
	if seen == nil {
		seen = map[string]bool{}
		r.shown[key] = seen
	}
	unseen := make([]string, 0, len(clips))
	for _, c := range clips {
		if !seen[c] {
			unseen = append(unseen, c)
		}
	}
	if len(unseen) == 0 {
		// whole pool shown today: refill and start the next pass.
		for _, c := range clips {
			seen[c] = false
		}
		unseen = clips
	}
	pick := unseen[rand.IntN(len(unseen))]
	seen[pick] = true
	return pick
}

// videoFiles lists the media files directly in dir (not recursive), sorted-ish by
// readdir order; only the extensions the picker treats as video are kept.
func videoFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if typeOf(e.Name()) == "video" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// start begins rotating between the two pools. intervalMinutes floors to 1; a
// missing pool is not fatal (that phase simply has nothing to show until the
// user fills it). tick applies the chosen clip; isDayFn is injectable for tests.
func (r *dayNightRotation) start(cfg dayNightConfig, tick func(path string), isDayFn func(bool) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
	mins := cfg.Interval
	if mins < 1 {
		mins = 1
	}
	r.interval = time.Duration(mins) * time.Minute
	r.dayDir = resolvePath(cfg.DayDir)
	r.nightDir = resolvePath(cfg.NightDir)
	r.noRepeat = cfg.NoRepeat == nil || *cfg.NoRepeat
	r.shown = map[string]map[string]bool{}
	r.dayKey = ""
	r.forced = ""
	if isDayFn == nil {
		isDayFn = isDay
	}
	stop := make(chan struct{})
	r.stopCh = stop
	go func() {
		// one immediate pass so enabling takes effect without waiting a full
		// interval, then the steady tick.
		if p := r.pickAt(isDayFn); p != "" {
			tick(p)
		}
		t := time.NewTicker(r.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if p := r.pickAt(isDayFn); p != "" {
					tick(p)
				}
			case <-stop:
				return
			}
		}
	}()
}

// pickAt resolves the current phase (honouring a manual override) and picks.
func (r *dayNightRotation) pickAt(isDayFn func(bool) bool) string {
	r.mu.Lock()
	forced := r.forced
	r.mu.Unlock()
	var day bool
	switch forced {
	case "day":
		day = true
	case "night":
		day = false
	default:
		day = isDayFn(true)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pick(day)
}

func (r *dayNightRotation) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *dayNightRotation) stopLocked() {
	if r.stopCh != nil {
		close(r.stopCh)
		r.stopCh = nil
	}
}

// force pins the active phase for manual testing; "" returns to real sunrise.
func (r *dayNightRotation) force(phase string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if phase != "day" && phase != "night" {
		phase = ""
	}
	r.forced = phase
}

func (r *dayNightRotation) status() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopCh == nil {
		return map[string]interface{}{"running": false}
	}
	return map[string]interface{}{
		"running":     true,
		"interval":    int(r.interval / time.Minute),
		"dayDir":      r.dayDir,
		"nightDir":    r.nightDir,
		"no_repeat":   r.noRepeat,
		"forced":      r.forced,
		"shown_today": r.countShown(),
	}
}

func (r *dayNightRotation) countShown() map[string]int {
	out := map[string]int{}
	for pool, seen := range r.shown {
		n := 0
		for _, v := range seen {
			if v {
				n++
			}
		}
		out[filepath.Base(pool)] = n
	}
	return out
}
