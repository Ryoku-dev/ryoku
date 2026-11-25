package nest

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// hostEnv makes the test's environment look like a live desktop session,
// with every handle the desktop must never pass on.
func hostEnv(t *testing.T) {
	t.Helper()
	socketHandle, instanceHandle := sessionHandles(t)
	for name, value := range map[string]string{
		"WAYLAND_DISPLAY":          "wayland-host",
		"WAYLAND_SOCKET":           "9",
		"DISPLAY":                  ":0",
		"DBUS_SESSION_BUS_ADDRESS": "unix:path=/host/run/bus",
		"XDG_RUNTIME_DIR":          "/host/run",
		socketHandle:               "/host/run/niri.wayland-host.1.sock",
		instanceHandle:             "host",
		"SWAYSOCK":                 "/host/run/sway.sock",
		"XDG_CONFIG_HOME":          "/host/home/.config",
		"SHELL":                    "/bin/bash",
		"LANG":                     "C.UTF-8",
		"TZ":                       "UTC",
	} {
		t.Setenv(name, value)
	}
	for _, name := range []string{"LC_ALL", "LANGUAGE", "USER", "LOGNAME", "LIBGL_ALWAYS_SOFTWARE", "GALLIUM_DRIVER", "MESA_LOADER_DRIVER_OVERRIDE"} {
		t.Setenv(name, "")
		_ = os.Unsetenv(name)
	}
}

func start(t *testing.T, o Options) *Workspace {
	t.Helper()
	w, err := Start(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return w
}

func options(t *testing.T) Options {
	t.Helper()
	return Options{Dir: shortDir(t), Home: filepath.Join(t.TempDir(), "home")}
}

func sorted(env []string) []string {
	out := slices.Clone(env)
	slices.Sort(out)
	return out
}

func TestAvailableSaysWhatIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if r := Available(); r.OK() || r.String() != "weston, niri and grim are not installed" {
		t.Fatalf("nothing installed: %+v %q", r, r.String())
	}
	bin := fakes(t, nil, "weston", "grim")
	if r := Available(); r.OK() || !slices.Equal(r.Missing, []string{nestedProviderName}) || r.String() != "niri is not installed" {
		t.Fatalf("no niri: %+v %q", r, r.String())
	}
	fakes(t, nil, "weston")
	if r := Available(); r.String() != "niri and grim are not installed" {
		t.Fatalf("weston only: %q", r.String())
	}
	fakes(t, nil)
	if r := Available(); !r.OK() || r.String() != "" {
		t.Fatalf("all installed: %+v", r)
	}
	if rs := records(t, bin, "weston"); len(rs) != 0 {
		t.Fatalf("Available started weston %d times", len(rs))
	}
}

func TestStartRefusesWhenAProgramIsMissing(t *testing.T) {
	bin := fakes(t, nil, "weston", "grim")
	o := options(t)
	_, err := Start(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "niri is not installed") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the runtime dir was made: %v", err)
	}
	if rs := records(t, bin, "weston"); len(rs) != 0 {
		t.Fatal("weston started without niri")
	}
}

func TestStartRunsExactArgvInABuiltEnvironment(t *testing.T) {
	hostEnv(t)
	bin := fakes(t, nil)
	o := options(t)
	o.Width, o.Height = 1024, 768
	w := start(t, o)

	weston := awaitRecord(t, bin, "weston")
	wantArgv := []string{"weston", "--backend=headless", "--shell=kiosk", "--idle-time=0", "--no-config",
		"--width=1024", "--height=768", "--socket=wayland-parent"}
	if !slices.Equal(weston.Argv, wantArgv) {
		t.Errorf("weston argv = %q", weston.Argv)
	}
	base := []string{"PATH=" + bin, "HOME=" + o.Home, "XDG_RUNTIME_DIR=" + o.Dir, "LANG=C.UTF-8"}
	if !slices.Equal(sorted(weston.Env), sorted(base)) {
		t.Errorf("weston env = %q", weston.Env)
	}
	if weston.Cwd != o.Dir {
		t.Errorf("weston cwd = %s", weston.Cwd)
	}

	niri := awaitRecord(t, bin, nestedProviderName)
	if !slices.Equal(niri.Argv, []string{nestedProviderName, "-c", filepath.Join(o.Dir, "niri.kdl")}) {
		t.Errorf("niri argv = %q", niri.Argv)
	}
	if !slices.Equal(sorted(niri.Env), sorted(append(slices.Clone(base), "WAYLAND_DISPLAY=wayland-parent"))) {
		t.Errorf("niri env = %q", niri.Env)
	}

	for path, mode := range map[string]os.FileMode{o.Dir: 0o700 | os.ModeDir, filepath.Join(o.Dir, "niri.kdl"): 0o600, o.Home: 0o700 | os.ModeDir} {
		if info, err := os.Stat(path); err != nil || info.Mode() != mode {
			t.Errorf("%s: %v %v", path, info.Mode(), err)
		}
	}

	if w.Display() != "wayland-1" || w.Socket() != filepath.Join(o.Dir, "wayland-1") {
		t.Errorf("display %s socket %s", w.Display(), w.Socket())
	}
	if width, height := w.Size(); width != 1024 || height != 768 {
		t.Errorf("size %dx%d", width, height)
	}
	socketHandle, _ := sessionHandles(t)
	wantEnv := []string{"PATH=" + bin, "XDG_RUNTIME_DIR=" + o.Dir, "WAYLAND_DISPLAY=wayland-1",
		socketHandle + "=" + filepath.Join(o.Dir, "niri.wayland-1."+strconv.Itoa(niri.Pid)+".sock")}
	if !slices.Equal(w.Env(), wantEnv) {
		t.Errorf("Env = %q", w.Env())
	}
}

func TestStartTakesTheDefaultSize(t *testing.T) {
	bin := fakes(t, nil)
	w := start(t, options(t))
	if width, height := w.Size(); width != DefaultWidth || height != DefaultHeight {
		t.Errorf("size %dx%d", width, height)
	}
	argv := awaitRecord(t, bin, "weston").Argv
	if !slices.Contains(argv, "--width=1280") || !slices.Contains(argv, "--height=800") {
		t.Errorf("weston argv = %q", argv)
	}
}

func TestStartThatNeverReportsFailsAndCleansUp(t *testing.T) {
	bin := fakes(t, map[string]string{"weston": "hang"})
	o := options(t)
	o.Ready = 300 * time.Millisecond
	began := time.Now()
	_, err := Start(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "did not come up in 300ms") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(began) > 5*time.Second {
		t.Errorf("the start took %s", time.Since(began))
	}
	gone(t, awaitRecord(t, bin, "weston").Pid)
	if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the runtime dir is left: %v", err)
	}
	if rs := records(t, bin, nestedProviderName); len(rs) != 0 {
		t.Error("niri started on a weston that never came up")
	}
}

func TestStartEndsWithItsContext(t *testing.T) {
	bin := fakes(t, map[string]string{"weston": "hang"})
	o := options(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := Start(ctx, o); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	gone(t, awaitRecord(t, bin, "weston").Pid)
	if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the runtime dir is left: %v", err)
	}
}

func TestStartReportsACompositorThatExits(t *testing.T) {
	for _, c := range []struct{ fake, want string }{
		{"weston", "weston exited: exit status 3: fake weston: no backend"},
		{nestedProviderName, "niri exited: exit status 4: fake niri: no config"},
	} {
		t.Run(c.fake, func(t *testing.T) {
			bin := fakes(t, map[string]string{c.fake: "die"})
			o := options(t)
			_, err := Start(context.Background(), o)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v", err)
			}
			gone(t, awaitRecord(t, bin, "weston").Pid)
			if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the runtime dir is left: %v", err)
			}
		})
	}
}

func TestOptionsAreChecked(t *testing.T) {
	bin := fakes(t, nil)
	home := filepath.Join(t.TempDir(), "home")
	for name, o := range map[string]Options{
		"relative dir":     {Dir: "run/nest", Home: home},
		"long dir":         {Dir: "/" + strings.Repeat("d", 80), Home: home},
		"root dir":         {Dir: "/", Home: home},
		"session's dir":    {Dir: "/host/run", Home: home},
		"relative home":    {Dir: "/tmp/nest-x", Home: "home"},
		"home inside dir":  {Dir: "/tmp/nest-x", Home: "/tmp/nest-x/home"},
		"too small":        {Dir: "/tmp/nest-x", Home: home, Width: 100, Height: 100},
		"too large":        {Dir: "/tmp/nest-x", Home: home, Width: 9000, Height: 800},
		"one side omitted": {Dir: "/tmp/nest-x", Home: home, Width: 1280},
	} {
		t.Setenv("XDG_RUNTIME_DIR", "/host/run")
		if _, err := Start(context.Background(), o); err == nil {
			t.Errorf("%s: started", name)
		}
	}
	if rs := records(t, bin, "weston"); len(rs) != 0 {
		t.Fatalf("weston started %d times", len(rs))
	}
}

func TestStartNeverEmptiesADirThatIsNotADesktops(t *testing.T) {
	fakes(t, nil)
	o := options(t)
	if err := os.Mkdir(o.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(o.Dir, "keep")
	if err := os.WriteFile(keep, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(context.Background(), o); err == nil || !strings.Contains(err.Error(), "not a separate desktop's dir") {
		t.Fatalf("err = %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "mine" {
		t.Fatalf("the dir was touched: %q %v", data, err)
	}

	link := shortDir(t)
	if err := os.Symlink(o.Dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(context.Background(), Options{Dir: link, Home: o.Home}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("symlink: err = %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("the symlink's target was touched")
	}
}

func TestStartReplacesAnEarlierDesktopsDir(t *testing.T) {
	bin := fakes(t, nil)
	o := options(t)
	first := start(t, o)
	first.Stop()
	if err := os.Mkdir(o.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{marker, "wayland-1", "stale"} {
		if err := os.WriteFile(filepath.Join(o.Dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	w := start(t, o)
	if _, err := os.Stat(filepath.Join(o.Dir, "stale")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the earlier desktop's files are left")
	}
	if w.Display() != "wayland-1" || len(records(t, bin, "weston")) != 2 {
		t.Error("the second desktop did not start")
	}
}

func TestStopLeavesNothing(t *testing.T) {
	bin := fakes(t, map[string]string{nestedProviderName: "escape"})
	o := options(t)
	w := start(t, o)
	if err := w.Open([]string{"fakeapp"}); err != nil {
		t.Fatal(err)
	}
	pids := []int{
		awaitRecord(t, bin, "weston").Pid, awaitRecord(t, bin, nestedProviderName).Pid,
		awaitRecord(t, bin, "fakeapp").Pid, awaitRecord(t, bin, "leftover").Pid,
	}
	w.Stop()
	for _, pid := range pids {
		gone(t, pid)
	}
	if held := holders(t, o.Dir); len(held) != 0 {
		t.Errorf("processes %v still hold the runtime dir", held)
	}
	if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the runtime dir is left: %v", err)
	}
	select {
	case <-w.Done():
	default:
		t.Fatal("Done is open after Stop")
	}
	if err := w.Wait(); err != nil {
		t.Errorf("Wait after Stop = %v", err)
	}
	w.Stop()
	if err := w.Open([]string{"fakeapp"}); err == nil {
		t.Error("Open after Stop ran a program")
	}
	if _, err := w.Capture(context.Background()); err == nil {
		t.Error("Capture after Stop captured")
	}
	if n := len(records(t, bin, "fakeapp")); n != 1 {
		t.Errorf("fakeapp ran %d times", n)
	}
}

func TestStopKillsWhatWillNotEnd(t *testing.T) {
	bin := fakes(t, map[string]string{"weston": "stubborn"})
	o := options(t)
	o.StopGrace = 200 * time.Millisecond
	w := start(t, o)
	pid := awaitRecord(t, bin, "weston").Pid
	began := time.Now()
	w.Stop()
	if time.Since(began) > 3*time.Second {
		t.Errorf("Stop took %s", time.Since(began))
	}
	gone(t, pid)
}

func TestACompositorExitingEndsTheDesktop(t *testing.T) {
	bin := fakes(t, nil)
	o := options(t)
	w := start(t, o)
	weston, niri := awaitRecord(t, bin, "weston").Pid, awaitRecord(t, bin, nestedProviderName).Pid
	if err := syscall.Kill(niri, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the desktop outlived niri")
	}
	if err := w.Wait(); err == nil || !strings.Contains(err.Error(), "niri exited: signal: killed") {
		t.Errorf("Wait = %v", err)
	}
	gone(t, weston)
	if _, err := os.Stat(o.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the runtime dir is left: %v", err)
	}
}

func TestADaemonKilledTakesTheDesktopWithIt(t *testing.T) {
	fakes(t, nil)
	dir := shortDir(t)
	t.Cleanup(func() { _ = os.RemoveAll(dir + "-home") })
	daemon, weston, niri := startDaemon(t, dir)
	if err := daemon.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = daemon.Wait()
	gone(t, weston)
	gone(t, niri)
	if err := Sweep(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Sweep left the runtime dir: %v", err)
	}
}

func TestSweepEndsWhatADesktopLeftBehind(t *testing.T) {
	bin := fakes(t, nil)
	dir, other := shortDir(t), shortDir(t)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, marker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(runtime string) *exec.Cmd {
		cmd := exec.Command(filepath.Join(bin, "leftover"))
		cmd.Env = []string{"XDG_RUNTIME_DIR=" + runtime}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	left, kept := run(dir), run(other)
	t.Cleanup(func() { _ = kept.Process.Kill(); _ = kept.Wait() })
	exited := make(chan struct{})
	go func() { _ = left.Wait(); close(exited) }()
	awaitN(t, bin, "leftover", 2)
	if err := Sweep(dir); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("Sweep left the process")
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Sweep left the dir: %v", err)
	}
	if err := kept.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("Sweep ended another dir's process: %v", err)
	}

	foreign := shortDir(t)
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Sweep(foreign); err == nil {
		t.Error("Sweep emptied a dir that is not a desktop's")
	}
}

func awaitN(t *testing.T, bin, name string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(records(t, bin, name)) < n {
		if time.Now().After(deadline) {
			t.Fatalf("%s ran fewer than %d times", name, n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCaptureReadsTheDesktopOnly(t *testing.T) {
	hostEnv(t)
	bin := fakes(t, nil)
	o := options(t)
	w := start(t, o)
	frame, err := w.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(frame)); err != nil {
		t.Fatalf("the capture is no PNG: %v", err)
	}
	grim := awaitRecord(t, bin, "grim")
	if !slices.Equal(grim.Argv, []string{"grim", "-t", "png", "-l", "3", "-"}) {
		t.Errorf("grim argv = %q", grim.Argv)
	}
	if !slices.Equal(grim.Env, w.Env()) {
		t.Errorf("grim env = %q", grim.Env)
	}
	if grim.Cwd != o.Dir {
		t.Errorf("grim cwd = %s", grim.Cwd)
	}

	if err := os.WriteFile(filepath.Join(bin, "mode.grim"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Capture(context.Background()); err == nil || !strings.Contains(err.Error(), "not a PNG") {
		t.Errorf("garbage capture: %v", err)
	}
}

func TestOpenRunsExactlyTheArgv(t *testing.T) {
	hostEnv(t)
	t.Setenv("LC_TIME", "C")
	bin := fakes(t, nil)
	o := options(t)
	w := start(t, o)
	argv := []string{"fakeapp", "two words", "$HOME; rm -rf ~", "`id`", "--flag=*"}
	if err := w.Open(argv); err != nil {
		t.Fatal(err)
	}
	app := awaitRecord(t, bin, "fakeapp")
	if !slices.Equal(app.Argv, argv) {
		t.Errorf("argv = %q", app.Argv)
	}
	if app.Cwd != o.Home {
		t.Errorf("cwd = %s", app.Cwd)
	}
	want := []string{
		"PATH=" + bin, "LANG=C.UTF-8", "TZ=UTC", "LC_TIME=C",
		"HOME=" + o.Home, "XDG_RUNTIME_DIR=" + o.Dir, "WAYLAND_DISPLAY=wayland-1", "XDG_SESSION_TYPE=wayland",
		"MOZ_ENABLE_WAYLAND=1", "QT_QPA_PLATFORM=wayland", "GDK_BACKEND=wayland", "SDL_VIDEODRIVER=wayland",
		"ELECTRON_OZONE_PLATFORM_HINT=wayland",
	}
	if !slices.Equal(sorted(app.Env), sorted(want)) {
		t.Errorf("env = %q", app.Env)
	}
}

func TestOpenRefusesShellsAndItsOwnFiles(t *testing.T) {
	bin := fakes(t, nil)
	o := options(t)
	w := start(t, o)
	planted := filepath.Join(o.Home, "tool")
	copyFile(t, filepath.Join(bin, "fakeapp"), planted)
	inDir := filepath.Join(o.Dir, "tool")
	copyFile(t, filepath.Join(bin, "fakeapp"), inDir)
	for name, argv := range map[string][]string{
		"nothing":        nil,
		"empty name":     {""},
		"shell on PATH":  {"bash", "-c", "true"},
		"shell by path":  {"/bin/sh", "-c", "true"},
		"program runner": {"/usr/bin/env", "fakeapp"},
		"relative path":  {"./fakeapp"},
		"not installed":  {"nothere"},
		"in its home":    {planted},
		"in its dir":     {inDir},
		"NUL":            {"fakeapp", "a\x00b"},
	} {
		if err := w.Open(argv); err == nil {
			t.Errorf("%s: opened", name)
		}
	}
	time.Sleep(100 * time.Millisecond)
	for _, name := range []string{"fakeapp", "bash"} {
		if n := len(records(t, bin, name)); n != 0 {
			t.Errorf("%s ran %d times", name, n)
		}
	}
}
