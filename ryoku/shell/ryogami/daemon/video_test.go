package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	wm "ryoku-wm"
)

func TestLiveFps(t *testing.T) {
	cases := []struct {
		name string
		tier string
		src  liveShape
		want string
	}{
		{"low is capped", "low", liveShape{fps: 60}, "24"},
		{"medium caps 60 to 30", "medium", liveShape{fps: 60}, "30"},
		{"24fps is never padded", "medium", liveShape{fps: 24}, "24"},
		{"high keeps 60", "high", liveShape{fps: 60}, "60"},
		{"high keeps a 30fps source at 30", "high", liveShape{fps: 30}, "30"},
		{"ntsc rates round up", "high", liveShape{fps: 59.94}, "60"},
		{"unreadable rate falls back to 30", "high", liveShape{}, "30"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := liveFps(tc.tier, tc.src); got != tc.want {
				t.Fatalf("liveFps(%q, %+v) = %q, want %q", tc.tier, tc.src, got, tc.want)
			}
		})
	}
}

func TestLiveDirect(t *testing.T) {
	fits := liveShape{codec: "h264", pixFmt: "yuv420p", width: 1920, fps: 30}
	if !liveDirect(fits, 2048, "30") {
		t.Fatal("a fitting h264 clip must play directly")
	}
	hevc := fits
	hevc.codec = "hevc"
	if liveDirect(hevc, 2048, "30") {
		t.Fatal("hevc must transcode")
	}
	tenBit := fits
	tenBit.pixFmt = "yuv420p10le"
	if liveDirect(tenBit, 2048, "30") {
		t.Fatal("10-bit must transcode")
	}
	wide := fits
	wide.width = 3840
	if liveDirect(wide, 2048, "30") {
		t.Fatal("a 4K clip must downscale")
	}
	fast := fits
	fast.fps = 60
	if liveDirect(fast, 2048, "30") {
		t.Fatal("a 60fps clip must transcode to a 30fps budget")
	}
}

func TestLiveFit(t *testing.T) {
	if got := liveFit("Contain"); got != "fit" {
		t.Fatalf("Contain = %q, want fit", got)
	}
	for _, cf := range []string{"Cover", "Fill", "ScaleDown", ""} {
		if got := liveFit(cf); got != "fill" {
			t.Fatalf("liveFit(%q) = %q, want fill", cf, got)
		}
	}
}

// fakeLiveTools installs stub ryogami-live / ffprobe binaries on PATH and seeds
// the output cache with two monitors. The player records its argv to marker;
// ffprobe reports a small direct-playable h264 clip so Play never transcodes.
func fakeLiveTools(t *testing.T, marker string, exitFast bool) {
	t.Helper()
	dir := t.TempDir()
	player := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + marker + "\n"
	if exitFast {
		player += "exit 0\n"
	} else {
		player += "sleep 30\n"
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("ryogami-live", player)
	write("ffprobe", "#!/bin/sh\nprintf 'codec_name=h264\\npix_fmt=yuv420p\\nwidth=1280\\nr_frame_rate=30/1\\n'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	seedOutputs(t, []wm.Output{
		{Name: "DP-1", Width: 2560, Scale: 1.25},
		{Name: "DP-2", Width: 1920, Scale: 1.0},
	})
}

// seedOutputs pins the output cache so liveSlots/liveCapWidth read a known
// monitor set instead of probing a compositor the test has none of.
func seedOutputs(t *testing.T, outs []wm.Output) {
	t.Helper()
	outputs.mu.Lock()
	outputs.outs = outs
	outputs.probed = true
	outputs.mu.Unlock()
	t.Cleanup(func() {
		outputs.mu.Lock()
		outputs.outs = nil
		outputs.probed = false
		outputs.mu.Unlock()
	})
}

func readMarker(t *testing.T, marker string) []string {
	t.Helper()
	data, err := os.ReadFile(marker)
	if err != nil {
		return nil
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func waitFor(t *testing.T, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func clip(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVideoPlayExplicitOutputs(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	p := newVideoPlayer()
	defer p.Stop()
	c := clip(t)

	p.Play([]string{"DP-1", "DP-2"}, c, "fill", "medium", nil)
	if !waitFor(t, func() bool { return len(readMarker(t, marker)) == 2 }) {
		t.Fatalf("players did not spawn: %q", readMarker(t, marker))
	}
	// The two players write their argv markers concurrently, so the file order
	// is not the spawn order; compare as a set.
	got := readMarker(t, marker)
	sort.Strings(got)
	// A 1280-wide direct-playable clip: no transcode, source path verbatim,
	// medium cap resolved from the fake monitors' physical width (2560; the
	// compositor upscales to physical pixels, so scale never divides it).
	want := []string{c + " 2560 fill DP-1", c + " 2560 fill DP-2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("spawn argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !p.Playing() {
		t.Fatal("Playing() = false with live players")
	}
	path, outs := p.Current()
	if path != c || len(outs) != 2 {
		t.Fatalf("Current() = %q %q", path, outs)
	}
}

func TestVideoPlayReplacesOnlyTargetOutput(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	p := newVideoPlayer()
	defer p.Stop()
	first := clip(t)
	second := clip(t)

	p.Play([]string{"DP-1"}, first, "fill", "medium", nil)
	p.Play([]string{"DP-2"}, first, "fill", "medium", nil)
	if !waitFor(t, func() bool { return len(readMarker(t, marker)) == 2 }) {
		t.Fatalf("initial players did not spawn: %q", readMarker(t, marker))
	}
	p.Play([]string{"DP-1"}, second, "fill", "medium", nil)
	if !waitFor(t, func() bool { return len(readMarker(t, marker)) == 3 }) {
		t.Fatalf("replacement player did not spawn: %q", readMarker(t, marker))
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.procs["DP-1"] == nil || p.procs["DP-1"].file != second {
		t.Fatalf("DP-1 player = %+v, want replacement %q", p.procs["DP-1"], second)
	}
	if p.procs["DP-2"] == nil || p.procs["DP-2"].file != first {
		t.Fatalf("DP-2 player = %+v, want original %q", p.procs["DP-2"], first)
	}
}

func TestVideoPlayBroadcastSpansMonitors(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	p := newVideoPlayer()
	defer p.Stop()
	c := clip(t)

	p.Play(nil, c, "fit", "medium", nil)
	if !waitFor(t, func() bool { return len(readMarker(t, marker)) == 2 }) {
		t.Fatalf("broadcast did not span both monitors: %q", readMarker(t, marker))
	}
	got := readMarker(t, marker)
	sort.Strings(got)
	want := []string{c + " 2560 fit DP-1", c + " 2560 fit DP-2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("spawn argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	_, outs := p.Current()
	if len(outs) != 1 || outs[0] != "*" {
		t.Fatalf("Current outputs = %q, want [*]", outs)
	}
}

func TestVideoStopEndsPlayback(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	p := newVideoPlayer()
	c := clip(t)

	p.Play([]string{"DP-1"}, c, "fill", "medium", nil)
	if !waitFor(t, func() bool { return p.Playing() }) {
		t.Fatal("player never spawned")
	}
	p.Stop()
	if !waitFor(t, func() bool { return !p.Playing() }) {
		t.Fatal("Stop left players tracked")
	}
	if path, outs := p.Current(); path != "" || len(outs) != 0 {
		t.Fatalf("Current after Stop = %q %q", path, outs)
	}
}

func TestVideoStopCancelsInFlightLaunch(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	dir := t.TempDir()
	// A stalling ffprobe holds Play inside the async prepare stage.
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"),
		[]byte("#!/bin/sh\nsleep 0.4\nprintf 'codec_name=h264\\npix_fmt=yuv420p\\nwidth=1280\\nr_frame_rate=30/1\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	p := newVideoPlayer()
	defer p.Stop()
	c := clip(t)

	p.Play([]string{"DP-1"}, c, "fill", "medium", nil)
	p.Stop() // switch away before the probe lands
	time.Sleep(700 * time.Millisecond)
	if lines := readMarker(t, marker); len(lines) != 0 {
		t.Fatalf("stale generation spawned a player: %q", lines)
	}
}

func TestVideoCrashIsUntracked(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, true) // player exits immediately
	p := newVideoPlayer()
	defer p.Stop()
	c := clip(t)

	p.Play([]string{"DP-1"}, c, "fill", "medium", nil)
	if !waitFor(t, func() bool { return len(readMarker(t, marker)) == 1 }) {
		t.Fatal("player never spawned")
	}
	if !waitFor(t, func() bool { return !p.Playing() }) {
		t.Fatal("crashed player still tracked")
	}
}

func TestVideoReadyHandshake(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argv")
	fakeLiveTools(t, marker, false)
	dir := t.TempDir()
	// A player that paints (READY), lives briefly, then dies.
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + marker + "\necho READY\nsleep 0.3\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "ryogami-live"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	p := newVideoPlayer()
	defer p.Stop()

	flips := make(chan bool, 4)
	p.Play([]string{"DP-1"}, clip(t), "fill", "medium", func(l bool) { flips <- l })
	select {
	case l := <-flips:
		if !l {
			t.Fatal("first flip = false, want true on READY")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("READY never yielded the painter")
	}
	select {
	case l := <-flips:
		if l {
			t.Fatal("second flip = true, want false when the player dies")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("player death never brought the still back")
	}
}
