package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestIsVideo(t *testing.T) {
	cases := map[string]bool{
		"/x/a.mp4": true, "/x/a.MP4": true, "/x/a.webm": true,
		"/x/a.mkv": true, "/x/a.mov": true,
		"/x/a.jpg": false, "/x/a.png": false, "/x/a.gif": false, "/x/plain": false,
	}
	for p, want := range cases {
		if got := isVideo(p); got != want {
			t.Errorf("isVideo(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestFrameOffset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	tune := filepath.Join(dir, "ryoku-ryowalls.json")
	video := "/home/x/Pictures/livewalls/clip.mp4"

	// no tune -> the auto default
	if got := frameOffset(video); got != "1" {
		t.Fatalf("no tune: got %q want 1", got)
	}
	// a tune for this video -> its chosen second
	_ = os.WriteFile(tune, []byte(`{"image":"`+video+`","frame":3.5}`), 0o644)
	if got := frameOffset(video); got != "3.50" {
		t.Fatalf("matching tune: got %q want 3.50", got)
	}
	// a tune keyed to another video never bleeds across
	if got := frameOffset("/home/x/other.mp4"); got != "1" {
		t.Fatalf("other video: got %q want 1", got)
	}
	// frame 0 falls back to the default
	_ = os.WriteFile(tune, []byte(`{"image":"`+video+`","frame":0}`), 0o644)
	if got := frameOffset(video); got != "1" {
		t.Fatalf("zero frame: got %q want 1", got)
	}
}

// liveFrame caches one still per clip + mtime + offset. The still used to be a
// single shared file, which is how a preview of one clip could hand the reader
// another clip's frame.
func TestLiveFramePerClip(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	bin := t.TempDir()
	runs := filepath.Join(state, "ffmpeg.runs")
	body := `printf x >> "` + runs + `"; for a in "$@"; do o="$a"; done; : > "$o"`
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	clips := t.TempDir()
	one := filepath.Join(clips, "one.mp4")
	two := filepath.Join(clips, "two.mp4")
	for _, c := range []string{one, two} {
		if err := os.WriteFile(c, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	first := liveFrame(one)
	if first == "" || !strings.Contains(first, "ryoku-live-frames") {
		t.Fatalf("still for one.mp4 = %q, want a file under ryoku-live-frames", first)
	}
	if again := liveFrame(one); again != first {
		t.Errorf("second call = %q, want the cached %q", again, first)
	}
	if b, _ := os.ReadFile(runs); len(b) != 1 {
		t.Errorf("ffmpeg ran %d times for one clip, want 1", len(b))
	}
	if other := liveFrame(two); other == first {
		t.Errorf("two.mp4 reused one.mp4's still (%q): a shared still is the bug", other)
	}

	// The ryowalls frame slider picks a different second: that is a different
	// still, not an overwrite of the one already on screen.
	tune := filepath.Join(state, "ryoku-ryowalls.json")
	if err := os.WriteFile(tune, []byte(`{"image":"`+one+`","frame":4}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if moved := liveFrame(one); moved == first {
		t.Errorf("offset 4 reused the offset 1 still (%q)", moved)
	}
}

// TestReadLivePreview: the marker survives while its Hub lives and is dropped
// once it dies, so a palette reload keeps re-asserting an in-progress preview
// but reverts an orphaned one to disk.
func TestReadLivePreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".desktop-preview.json")
	write := func(pid int) {
		b, _ := json.Marshal(map[string]any{"pid": pid, "draft": map[string]any{"desktop": map[string]any{"input": map[string]any{"followMouse": 1}}}})
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// A live Hub (this test process) keeps the draft.
	write(os.Getpid())
	draft, ok := readLivePreview(path)
	if !ok || draft == nil {
		t.Fatalf("a preview owned by a live pid must survive, got ok=%v", ok)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the marker must stay for the next reload: %v", err)
	}

	// A dead Hub drops it, so the next reload reverts to disk.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if syscall.Kill(dead, 0) == nil {
		t.Skip("pid still live; cannot test the orphan path")
	}
	write(dead)
	if _, ok := readLivePreview(path); ok {
		t.Fatal("a preview owned by a dead pid must be dropped")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the orphaned marker must be removed")
	}

	// A marker whose owner was reparented to init cannot attest to a live Hub
	// either: kill(1,0) succeeds on permission grounds.
	write(1)
	if _, ok := readLivePreview(path); ok {
		t.Fatal("a reparented (pid 1) marker must be dropped, not trusted")
	}

	// A malformed marker is removed rather than trusted.
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readLivePreview(path); ok {
		t.Fatal("a malformed marker must not be treated as a live preview")
	}
}
