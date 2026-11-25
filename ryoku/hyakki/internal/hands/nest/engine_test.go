//go:build nestengine

package nest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestEngineSeparateDesktop runs the real weston, niri and grim: it starts a
// desktop in /dev/shm, opens a small client on it, captures one PNG of it
// and stops it, and then nothing that held the desktop's runtime dir is
// left. Run it once, inside the test sandbox:
//
//	go-gate.sh <unit> 3G test -tags nestengine -run Engine ./internal/hands/nest/
func TestEngineSeparateDesktop(t *testing.T) {
	if r := Available(); !r.OK() {
		t.Fatalf("the separate desktop is unavailable: %s", r)
	}
	client, err := exec.LookPath("weston-simple-shm")
	if err != nil {
		t.Fatalf("no trivial client: %v", err)
	}
	dir := fmt.Sprintf("/dev/shm/hk-nest-%d", os.Getpid())
	home := dir + "-home"
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	w, err := Start(ctx, Options{Dir: dir, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			w.Stop()
		}
	})
	if err := w.Open([]string{client}); err != nil {
		t.Fatal(err)
	}

	var windows []struct {
		Layout struct {
			Position *[2]float64 `json:"tile_pos_in_workspace_view"`
			Offset   [2]float64  `json:"window_offset_in_tile"`
			Size     [2]int      `json:"window_size"`
		} `json:"layout"`
	}
	for len(windows) == 0 && ctx.Err() == nil {
		out, err := runIn(ctx, w, nestedProviderName, "msg", "--json", "windows")
		if err == nil && json.Unmarshal(out, &windows) != nil {
			windows = nil
		}
		if len(windows) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if len(windows) != 1 {
		t.Fatalf("the client's window never mapped: %d windows", len(windows))
	}

	app := w.apps[0].cmd.Process.Pid
	environ, err := os.ReadFile("/proc/" + strconv.Itoa(app) + "/environ")
	if err != nil {
		t.Fatal(err)
	}
	vars := strings.Split(string(environ), "\x00")
	for _, want := range []string{"XDG_RUNTIME_DIR=" + dir, "WAYLAND_DISPLAY=" + w.Display(), "HOME=" + home} {
		if !slices.Contains(vars, want) {
			t.Errorf("the client lacks %s", want)
		}
	}
	socketHandle, instanceHandle := sessionHandles(t)
	for _, v := range vars {
		name, _, _ := strings.Cut(v, "=")
		switch name {
		case "DISPLAY", "DBUS_SESSION_BUS_ADDRESS", socketHandle, "WAYLAND_SOCKET", instanceHandle, "SWAYSOCK":
			t.Errorf("the client holds %s", v)
		}
	}

	var frame image.Image
	for ctx.Err() == nil {
		shot, err := w.Capture(ctx)
		if err != nil {
			t.Fatal(err)
		}
		frame, err = png.Decode(bytes.NewReader(shot))
		if err != nil {
			t.Fatal(err)
		}
		if !uniform(frame) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if frame == nil {
		t.Fatal("no capture before the deadline")
	}
	if b := frame.Bounds(); b.Dx() != DefaultWidth || b.Dy() != DefaultHeight {
		t.Errorf("the capture is %dx%d", b.Dx(), b.Dy())
	}
	if uniform(frame) {
		t.Error("the capture never showed the client")
	}

	placed := windows[0].Layout
	if placed.Position == nil {
		t.Fatal("the separate desktop's mapped window has no position for checked clicks")
	}
	x, y := placed.Position[0]+placed.Offset[0], placed.Position[1]+placed.Offset[1]
	width, height := placed.Size[0], placed.Size[1]
	if width <= 0 || height <= 0 || x < 0 || y < 0 ||
		x+float64(width) > float64(frame.Bounds().Dx()) || y+float64(height) > float64(frame.Bounds().Dy()) {
		t.Fatalf("the reported client box leaves the captured desktop: %.2f, %.2f %dx%d", x, y, width, height)
	}

	pids := []int{w.weston.cmd.Process.Pid, w.niri.cmd.Process.Pid, app}
	w.Stop()
	stopped = true
	if err := w.Wait(); err != nil {
		t.Errorf("Wait = %v", err)
	}
	for _, pid := range pids {
		gone(t, pid)
	}
	if held := holders(t, dir); len(held) != 0 {
		t.Errorf("processes %v still hold the runtime dir", held)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the runtime dir is left: %v", err)
	}
}

func runIn(ctx context.Context, w *Workspace, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = w.Env()
	return cmd.Output()
}

// uniform is true when every sampled pixel of the frame has one colour.
func uniform(frame image.Image) bool {
	b := frame.Bounds()
	r0, g0, b0, _ := frame.At(b.Min.X, b.Min.Y).RGBA()
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			if r, g, bl, _ := frame.At(x, y).RGBA(); r != r0 || g != g0 || bl != b0 {
				return false
			}
		}
	}
	return true
}
