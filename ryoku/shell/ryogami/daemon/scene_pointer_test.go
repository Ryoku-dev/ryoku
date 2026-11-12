package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// stands in for skwd-wall-vk, its stdin held open the way paper holds a worker's
type fakeRenderer struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
}

func startFakeRenderer(t *testing.T) *fakeRenderer {
	t.Helper()
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := &fakeRenderer{cmd: cmd, stdin: stdin, lines: make(chan string, 64)}
	go func() {
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			r.lines <- scan.Text()
		}
		close(r.lines)
	}()
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return r
}

func (r *fakeRenderer) next(t *testing.T) string {
	t.Helper()
	select {
	case line := <-r.lines:
		return line
	case <-time.After(2 * time.Second):
		t.Fatal("renderer received no pointer line")
		return ""
	}
}

func (r *fakeRenderer) silent(t *testing.T) {
	t.Helper()
	select {
	case line := <-r.lines:
		t.Fatalf("renderer received %q, want nothing", line)
	case <-time.After(3 * scenePointerInterval):
	}
}

type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

func newTestFeed(workers func() ([]paperWorker, error)) (*scenePointerFeed, *fakeClock) {
	clock := &fakeClock{at: time.Unix(1000, 0)}
	f := newScenePointerFeed(workers)
	f.now = clock.now
	return f, clock
}

func sceneWorker(pid int, outputs ...string) paperWorker {
	return paperWorker{Outputs: outputs, Source: paperSource{Kind: "we", Path: "/we/1"}, Pid: pid}
}

func TestScenePointerReachesTheRendererOnItsOutput(t *testing.T) {
	left := startFakeRenderer(t)
	right := startFakeRenderer(t)
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{
			sceneWorker(left.cmd.Process.Pid, "DP-1"),
			sceneWorker(right.cmd.Process.Pid, "HDMI-A-1"),
		}, nil
	})

	f.move("DP-1", 0, 1)
	if got, want := left.next(t), `{"pointer":{"x":0,"y":65535,"buttons":0}}`; got != want {
		t.Fatalf("DP-1 renderer got %s, want %s", got, want)
	}
	clock.advance(time.Second)
	f.move("HDMI-A-1", 0.5, 0.25)
	if got, want := right.next(t), `{"pointer":{"x":32768,"y":16384,"buttons":0}}`; got != want {
		t.Fatalf("HDMI-A-1 renderer got %s, want %s", got, want)
	}
	left.silent(t)

	// Out-of-range input clamps to the position already sent, so nothing moves.
	clock.advance(time.Second)
	f.move("DP-1", -3, 7)
	left.silent(t)
}

func TestScenePointerSkipsRepeatsAndCoalescesBursts(t *testing.T) {
	r := startFakeRenderer(t)
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{sceneWorker(r.cmd.Process.Pid, "*")}, nil
	})

	f.move("DP-1", 0.1, 0.1)
	if got := r.next(t); !strings.Contains(got, `"x":6554,"y":6554`) {
		t.Fatalf("first position = %s", got)
	}
	f.move("DP-1", 0.1, 0.1)
	r.silent(t)

	// A burst inside one interval leaves only its newest position.
	f.move("DP-1", 0.2, 0.2)
	f.move("DP-1", 0.3, 0.3)
	f.move("DP-1", 0.4, 0.4)
	if got := r.next(t); !strings.Contains(got, `"x":26214,"y":26214`) {
		t.Fatalf("burst flushed %s, want the newest position", got)
	}
	r.silent(t)

	clock.advance(time.Second)
	f.move("eDP-1", 0.4, 0.4)
	if got := r.next(t); !strings.Contains(got, `"x":26214`) {
		t.Fatalf("a scene on every output did not follow the pointer onto eDP-1: %s", got)
	}
}

func TestScenePointerIgnoresOutputsWithoutAScene(t *testing.T) {
	r := startFakeRenderer(t)
	asks := 0
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		asks++
		video := sceneWorker(r.cmd.Process.Pid, "HDMI-A-1")
		video.Source.Kind = "video"
		return []paperWorker{sceneWorker(r.cmd.Process.Pid, "DP-1"), video}, nil
	})

	for i := range 20 {
		clock.advance(scenePointerInterval)
		f.move("HDMI-A-1", float64(i)/20, 0.5)
	}
	r.silent(t)
	if asks != 1 {
		t.Fatalf("paper was asked %d times while hovering an output without a scene, want 1", asks)
	}
	clock.advance(scenePointerRetry)
	f.move("HDMI-A-1", 0.9, 0.5)
	if asks != 2 {
		t.Fatalf("paper was asked %d times after the retry window, want 2", asks)
	}
}

func TestScenePointerFollowsReplacedWorkers(t *testing.T) {
	first := startFakeRenderer(t)
	second := startFakeRenderer(t)
	current := first
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{sceneWorker(current.cmd.Process.Pid, "DP-1")}, nil
	})

	f.move("DP-1", 0.25, 0.25)
	first.next(t)

	current = second
	f.invalidate()
	clock.advance(time.Second)
	f.move("DP-1", 0.25, 0.25)
	if got := second.next(t); !strings.Contains(got, `"x":16384`) {
		t.Fatalf("replacement renderer got %s", got)
	}
	first.silent(t)
}

func TestScenePointerRefusesARecycledPid(t *testing.T) {
	r := startFakeRenderer(t)
	impostor := startFakeRenderer(t)
	pipes := map[int]string{
		r.cmd.Process.Pid: "/proc/" + strconv.Itoa(r.cmd.Process.Pid) + "/fd/0",
	}
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{sceneWorker(r.cmd.Process.Pid, "DP-1")}, nil
	})
	f.stdin = func(pid int) string { return pipes[pid] }

	f.move("DP-1", 0.5, 0.5)
	r.next(t)

	// The pid now names a different pipe, as after the worker exited and the
	// number was reused.
	pipes[r.cmd.Process.Pid] = "/proc/" + strconv.Itoa(impostor.cmd.Process.Pid) + "/fd/0"
	clock.advance(time.Second)
	f.move("DP-1", 0.75, 0.75)
	impostor.silent(t)
	r.silent(t)
}

func TestScenePointerLeavesTheRendererStoppable(t *testing.T) {
	r := startFakeRenderer(t)
	f, clock := newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{sceneWorker(r.cmd.Process.Pid, "DP-1")}, nil
	})
	f.move("DP-1", 0.5, 0.5)
	r.next(t)
	clock.advance(time.Second)
	f.move("DP-1", 0.6, 0.6)
	r.next(t)

	// paper stops a renderer by closing its stdin; the feed must not hold it open.
	_ = r.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("renderer kept running after its owner closed stdin")
	}
}

func TestPointerVerbFeedsTheScene(t *testing.T) {
	r := startFakeRenderer(t)
	d := &daemon{}
	d.scenePointer, _ = newTestFeed(func() ([]paperWorker, error) {
		return []paperWorker{sceneWorker(r.cmd.Process.Pid, "DP-1")}, nil
	})
	if got := d.dispatchVerb("pointer DP-1 0.5 0.5"); got != "ok" {
		t.Fatalf("pointer verb = %q", got)
	}
	if got := r.next(t); got != `{"pointer":{"x":32768,"y":32768,"buttons":0}}` {
		t.Fatalf("renderer got %s", got)
	}
	if got := d.dispatchVerb("pointer DP-1 left top"); !strings.HasPrefix(got, "err") {
		t.Fatalf("bad position answered %q", got)
	}
}

func TestSceneFramesAskForThePointer(t *testing.T) {
	w := newWallSurface()
	w.showScene("/we/1/preview.jpg", "fill")
	if f := w.snapshot(); !f.Default.Pointer {
		t.Fatal("a scene on every output did not ask for the pointer")
	}
	w.show("/walls/a.png", "fill", nil, false, false, videoClip{})
	if f := w.snapshot(); f.Default.Pointer {
		t.Fatal("a still kept asking for the pointer")
	}
	w.showOutputScene("DP-1", "/we/1/preview.jpg", "fill")
	w.showOutput("HDMI-A-1", "/walls/b.png", "fill", nil, false, false, videoClip{})
	f := w.snapshot()
	if !f.Outputs["DP-1"].Pointer || f.Outputs["HDMI-A-1"].Pointer || f.Default.Pointer {
		t.Fatalf("per-output pointer flags = DP-1 %v, HDMI-A-1 %v, default %v",
			f.Outputs["DP-1"].Pointer, f.Outputs["HDMI-A-1"].Pointer, f.Default.Pointer)
	}
}

func TestOnlySceneProjectsAskForThePointer(t *testing.T) {
	library := t.TempDir()
	for id, kind := range map[string]string{"1": "scene", "2": "video"} {
		dir := filepath.Join(library, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(`{"type":"`+kind+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !weIsScene(filepath.Join(library, "1")) {
		t.Fatal("scene project not recognised")
	}
	if weIsScene(filepath.Join(library, "2")) {
		t.Fatal("video project treated as a scene")
	}
}
