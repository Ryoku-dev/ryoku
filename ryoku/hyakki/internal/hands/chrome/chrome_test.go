package chrome

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as a fake browser: started with the pipe flag it
// answers every command with what it was started with.
func TestMain(m *testing.M) {
	if slices.Contains(os.Args, flagPipe) {
		fakeBrowser()
		return
	}
	os.Exit(m.Run())
}

func fakeBrowser() {
	in, out := os.NewFile(3, "in"), os.NewFile(4, "out")
	stubborn := slices.Contains(os.Args, flagClass+"stubborn")
	if stubborn {
		signal.Ignore(syscall.SIGTERM)
	}
	stdin, _ := os.Readlink("/proc/self/fd/0")
	br := bufio.NewReader(in)
	for {
		msg, err := br.ReadBytes(0)
		if err != nil {
			break
		}
		var m struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(msg[:len(msg)-1], &m)
		raw, _ := json.Marshal(map[string]any{"id": m.ID, "result": map[string]any{
			"args": os.Args, "env": os.Environ(), "stdin": stdin,
		}})
		if _, err := out.Write(append(raw, 0)); err != nil {
			break
		}
	}
	if stubborn {
		time.Sleep(time.Hour)
	}
	os.Exit(0)
}

// tree builds a fake root with the files named: a value of "launcher" is a
// small ELF program, "script" a shell script, "browser" a large ELF program
// (sparse), and "->X" a symlink to X inside the tree.
func tree(t *testing.T, files map[string]string) resolver {
	t.Helper()
	root := t.TempDir()
	for p, kind := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasPrefix(kind, "->"):
			if err := os.Symlink(filepath.Join(root, kind[2:]), full); err != nil {
				t.Fatal(err)
			}
		case kind == "script":
			if err := os.WriteFile(full, []byte("#!/bin/sh\nexec /usr/lib/x/real $(cat ~/.config/chromium-flags.conf) \"$@\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		case kind == "launcher", kind == "browser":
			if err := os.WriteFile(full, []byte("\x7fELF chromium-flags.conf"), 0o755); err != nil {
				t.Fatal(err)
			}
			if kind == "browser" {
				if err := os.Truncate(full, MinBrowserBytes+1); err != nil {
					t.Fatal(err)
				}
			}
		default:
			t.Fatalf("kind %q", kind)
		}
	}
	return resolver{root: root, path: "/usr/bin:/bin"}
}

var good = Options{ProfileDir: "/state/hyakki/hands-browser", Class: "hyakki-hands-1", StartURL: "about:blank"}

func with(exec ...string) Options {
	o := good
	o.Exec = exec
	return o
}

func TestTheArgvIsTheOneShape(t *testing.T) {
	r := tree(t, map[string]string{"/opt/b/browser": "browser"})
	argv, err := r.command(with("/opt/b/browser"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{r.root + "/opt/b/browser", "--user-data-dir=/state/hyakki/hands-browser", "--remote-debugging-pipe",
		"--class=hyakki-hands-1", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "about:blank"}
	if !slices.Equal(argv, want) {
		t.Fatalf("argv\n got %q\nwant %q", argv, want)
	}
	if !Allowed(argv) {
		t.Fatal("its own argv refused")
	}
	o := with("/opt/b/browser")
	o.Headless = true
	argv, _ = r.command(o)
	if argv[7] != "--headless" || len(argv) != 9 || !Allowed(argv) {
		t.Fatalf("headless argv %q", argv)
	}
	for _, a := range argv {
		if strings.Contains(a, "remote-debugging-port") {
			t.Fatalf("argv names a port: %q", argv)
		}
	}
}

func TestAllowedRefusesEveryOtherArgv(t *testing.T) {
	base := []string{"/usr/lib/chromium/chromium", "--user-data-dir=/s/hands-browser", "--remote-debugging-pipe",
		"--class=c", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "about:blank"}
	if !Allowed(base) {
		t.Fatal("base refused")
	}
	mut := func(f func([]string) []string) []string { return f(slices.Clone(base)) }
	bad := [][]string{
		mut(func(a []string) []string { return slices.Insert(a, 7, "--remote-debugging-port=9222") }),
		mut(func(a []string) []string { a[2] = "--remote-debugging-port=0"; return a }),
		mut(func(a []string) []string { return slices.Insert(a, 7, "--load-extension=/x") }),
		mut(func(a []string) []string { a[6] = "--enable-extensions"; return a }),
		mut(func(a []string) []string { a[4], a[5] = a[5], a[4]; return a }),
		mut(func(a []string) []string { a[7] = "--headless"; return a }),
		mut(func(a []string) []string { return append(a, "about:blank") }),
		mut(func(a []string) []string { a[7] = "-x"; return a }),
		mut(func(a []string) []string { a[7] = "data:text/html,hi"; return a }),
		mut(func(a []string) []string { a[7] = "javascript:alert(1)"; return a }),
		mut(func(a []string) []string { a[7] = "chrome://settings"; return a }),
		mut(func(a []string) []string { a[1] = "--user-data-dir=/home/u/.config/chromium"; return a }),
		mut(func(a []string) []string { a[1] = "--user-data-dir=rel/hands-browser"; return a }),
		mut(func(a []string) []string { a[3] = "--class=a b"; return a }),
		mut(func(a []string) []string { a[0] = "chromium"; return a }),
		mut(func(a []string) []string { return slices.Insert(a, 0, "/usr/bin/flatpak") }),
		nil,
	}
	for _, a := range bad {
		if Allowed(a) {
			t.Errorf("allowed %q", a)
		}
	}
}

func TestOptionsOutsideTheShapeAreRefused(t *testing.T) {
	r := tree(t, map[string]string{"/opt/b/browser": "browser"})
	for _, o := range []Options{
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/home/u/.config/chromium", Class: "c", StartURL: "about:blank"},
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/s/../hands-browser", Class: "c", StartURL: "about:blank"},
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/s/hands-browser", Class: "--x", StartURL: "about:blank"},
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/s/hands-browser", Class: "c", StartURL: "--remote-debugging-port=1"},
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/s/hands-browser", Class: "c", StartURL: "https://u:p@x.test/"},
		{Exec: []string{"/opt/b/browser"}, ProfileDir: "/s/hands-browser", Class: "c", StartURL: "file://host/x"},
	} {
		if _, err := r.command(o); !errors.Is(err, ErrOptions) {
			t.Errorf("%+v: %v", o, err)
		}
	}
}

func TestAFlagReadingLauncherIsSkippedForItsRealProgram(t *testing.T) {
	r := tree(t, map[string]string{
		"/usr/bin/chromium":                "launcher", // Arch's small ELF launcher reads chromium-flags.conf
		"/usr/lib/chromium/chromium":       "browser",
		"/usr/bin/google-chrome-stable":    "->/opt/google/chrome/google-chrome",
		"/opt/google/chrome/google-chrome": "script",
		"/opt/google/chrome/chrome":        "browser",
	})
	flags := filepath.Join(r.root, "home/.config/chromium-flags.conf")
	os.MkdirAll(filepath.Dir(flags), 0o755)
	os.WriteFile(flags, []byte("--remote-debugging-port=9222\n--load-extension=/x\n"), 0o644)
	for exec, real := range map[string]string{
		"/usr/bin/chromium":             "/usr/lib/chromium/chromium",
		"chromium":                      "/usr/lib/chromium/chromium",
		"google-chrome-stable":          "/opt/google/chrome/chrome",
		"/usr/bin/google-chrome-stable": "/opt/google/chrome/chrome",
	} {
		argv, err := r.command(with(exec))
		if err != nil {
			t.Fatalf("%s: %v", exec, err)
		}
		if argv[0] != r.root+real {
			t.Errorf("%s started %s, want %s", exec, argv[0], real)
		}
		for _, a := range argv {
			if strings.Contains(a, "9222") || strings.Contains(a, "load-extension") {
				t.Fatalf("a user flag reached the argv: %q", argv)
			}
		}
	}
}

func TestUnknownLaunchersSandboxesAndShellsAreRefused(t *testing.T) {
	r := tree(t, map[string]string{
		"/usr/bin/mybrowser":        "script",
		"/usr/bin/tinyelf":          "launcher",
		"/usr/bin/snap":             "launcher",
		"/usr/bin/chromium-browser": "->/usr/bin/snap",
		"/snap/bin/chromium":        "browser",
		"/var/lib/flatpak/exports/bin/org.chromium.Chromium": "browser",
		"/usr/bin/flatpak": "launcher",
		"/bin/sh":          "browser",
		"/usr/bin/env":     "launcher",
		"/usr/bin/shim":    "->/bin/sh",
	})
	cases := map[string]error{
		"/usr/bin/mybrowser": ErrWrapper,
		"/usr/bin/tinyelf":   ErrWrapper,
		"chromium-browser":   ErrSandboxed,
		"/snap/bin/chromium": ErrSandboxed,
		"/var/lib/flatpak/exports/bin/org.chromium.Chromium": ErrSandboxed,
		"flatpak":          ErrSandboxed,
		"/bin/sh":          ErrShell,
		"env":              ErrShell,
		"/usr/bin/shim":    ErrShell,
		"/usr/bin/absent":  ErrExec,
		"relative/browser": ErrExec,
		"-browser":         ErrExec,
	}
	for exec, want := range cases {
		if _, err := r.command(with(exec)); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", exec, err, want)
		}
	}
	for _, exec := range [][]string{
		{"/usr/bin/flatpak", "run", "org.chromium.Chromium"},
		{"/usr/lib/chromium/chromium", "--incognito"},
		{"sh", "-c", "chromium"},
		{},
	} {
		if _, err := r.command(with(exec...)); !errors.Is(err, ErrExec) {
			t.Errorf("%q: %v", exec, err)
		}
	}
}

func TestTheEnvironmentCarriesNoFlags(t *testing.T) {
	got := Environ([]string{"CHROMIUM_FLAGS=--remote-debugging-port=1", "CHROMIUM_USER_FLAGS=--x", "CHROME_FLAGS=--y",
		"GOOGLE_API_KEY=k", "LD_PRELOAD=/x.so", "WAYLAND_DISPLAY=wayland-1", "LC_ALL=C", "HOME=/h", "DBUS_SESSION_BUS_ADDRESS=unix:x"})
	if !slices.Equal(got, []string{"WAYLAND_DISPLAY=wayland-1", "LC_ALL=C", "HOME=/h"}) {
		t.Fatalf("env %q", got)
	}
}

type started struct {
	Args  []string `json:"args"`
	Env   []string `json:"env"`
	Stdin string   `json:"stdin"`
}

func helper(t *testing.T, class string) (*Browser, Options) {
	t.Helper()
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), ProfileName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	o := Options{Exec: []string{self}, ProfileDir: dir, Class: class, StartURL: "about:blank",
		Env: []string{"PATH=/usr/bin", "CHROMIUM_FLAGS=--remote-debugging-port=9222", "WAYLAND_DISPLAY=w"}}
	b, err := launch(context.Background(), o, resolver{min: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b, o
}

func TestLaunchStartsExactlyTheShapeOverThePipe(t *testing.T) {
	b, o := helper(t, "polite")
	var s started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := b.Conn().Call(ctx, "", "Target.getTargets", nil, &s); err != nil {
		t.Fatalf("call over the pipe: %v", err)
	}
	if !slices.Equal(s.Args, b.Argv()) || !Allowed(s.Args) {
		t.Fatalf("started %q, recorded %q", s.Args, b.Argv())
	}
	if !slices.Equal(s.Env, []string{"PATH=/usr/bin", "WAYLAND_DISPLAY=w"}) {
		t.Fatalf("env %q", s.Env)
	}
	if s.Stdin != "/dev/null" {
		t.Fatalf("stdin is %q", s.Stdin)
	}
	cmdline, err := os.ReadFile("/proc/" + strconv.Itoa(b.PID()) + "/cmdline")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cmdline), "--remote-debugging-pipe\x00") || strings.Contains(string(cmdline), "port") {
		t.Fatalf("cmdline %q", cmdline)
	}
	fi, err := os.Stat(o.ProfileDir)
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("profile mode %v, %v", fi.Mode(), err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-b.Wait():
		if err != nil {
			t.Fatalf("exit %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no exit")
	}
}

func TestCloseStopsABrowserThatIgnoresThePipe(t *testing.T) {
	closeGrace = 300 * time.Millisecond
	t.Cleanup(func() { closeGrace = CloseGrace })
	b, _ := helper(t, "stubborn")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := b.Conn().Call(ctx, "", "Target.getTargets", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-b.Wait(); err == nil {
		t.Fatal("a killed browser reported a clean exit")
	}
	if err := syscall.Kill(b.PID(), 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("browser still there: %v", err)
	}
}
