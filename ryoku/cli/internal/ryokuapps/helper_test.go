package ryokuapps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
)

// fakeGpk writes a gpk stand-in that records its argv and environment in its
// HOME and then runs body.
func fakeGpk(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gpk")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/argv\"\nenv > \"$HOME/env\"\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func testHelper(t *testing.T, manager host.PackageManager, gpk string) (*Helper, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	h := &Helper{
		Manager: manager,
		Gpk:     gpk,
		GpkHome: filepath.Join(t.TempDir(), "gpk-home"),
		Stdout:  stdout,
		Stderr:  stderr,
		Query:   func(string, ...string) (string, int) { return "", 1 },
		Release: func(context.Context, apprelease.App, bool, apprelease.Options) (apprelease.Status, error) {
			t.Fatal("the release lane ran")
			return apprelease.Status{}, nil
		},
	}
	return h, stdout, stderr
}

type event struct {
	Event  string `json:"event"`
	Action string `json:"action"`
	Steps  []Step `json:"steps"`
	Index  int    `json:"index"`
	Phase  string `json:"phase"`
	Line   string `json:"line"`
	OK     bool   `json:"ok"`
	Exit   int    `json:"exit"`
	Error  string `json:"error"`
}

func readEvents(t *testing.T, out *bytes.Buffer) []event {
	t.Helper()
	var events []event
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("stdout line %q is not an event: %v", line, err)
		}
		events = append(events, e)
	}
	return events
}

func lastDone(t *testing.T, events []event) event {
	t.Helper()
	if len(events) == 0 || events[len(events)-1].Event != "done" {
		t.Fatalf("stream does not end with done: %+v", events)
	}
	for _, e := range events[:len(events)-1] {
		if e.Event == "done" {
			t.Fatalf("two done events: %+v", events)
		}
	}
	return events[len(events)-1]
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// install forwards gpk's own events untouched, wraps stray output as log
// events, and runs gpk with escalation off in its own home.
func TestInstallForwardsGpkEvents(t *testing.T) {
	gpk := fakeGpk(t, `echo '{"event":"plan","action":"install","steps":[{"manager":"pacman","name":"cowsay","version":"3.8.4-1"}]}'
echo '{"event":"step","index":0,"phase":"start"}'
echo 'not json from a child'
echo '{"event":"progress","index":0,"done":1,"total":1}'
echo '{"event":"step","index":0,"phase":"done","exit":0}'
echo '{"event":"done","ok":true,"exit":0,"error":""}'`)
	h, stdout, _ := testHelper(t, host.Pacman, gpk)

	if code := h.Run([]string{"install", "--manager", "pacman", "cowsay", "cowsay"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	events := readEvents(t, stdout)
	if got := len(events); got != 6 {
		t.Fatalf("%d events: %+v", got, events)
	}
	if events[0].Event != "plan" || events[0].Steps[0].Name != "cowsay" {
		t.Fatalf("plan = %+v", events[0])
	}
	if events[2].Event != "log" || events[2].Line != "not json from a child" {
		t.Fatalf("stray line = %+v", events[2])
	}
	if done := lastDone(t, events); !done.OK || done.Exit != 0 {
		t.Fatalf("done = %+v", done)
	}
	argv := readFile(t, filepath.Join(h.GpkHome, "argv"))
	if want := "install\ncowsay\n--manager\npacman\n--yes\n--events\n--privilege\nnone\n"; argv != want {
		t.Fatalf("gpk argv = %q, want %q", argv, want)
	}
	env := readFile(t, filepath.Join(h.GpkHome, "env"))
	for _, want := range []string{"HOME=" + h.GpkHome + "\n", "GPK_PRIVILEGE=none\n", "XDG_DATA_HOME=" + h.GpkHome + "/data\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("gpk env lacks %q", want)
		}
	}
	if strings.Contains(env, "XDG_RUNTIME_DIR=") || strings.Contains(env, "DBUS_SESSION_BUS_ADDRESS=") {
		t.Errorf("gpk inherited the caller's session: %s", env)
	}
	if info, err := os.Stat(h.GpkHome); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("gpk home mode: %v %v", info, err)
	}
}

// A gpk that dies without a result still ends the stream, with its exit code.
func TestGpkWithoutDoneEventStillEnds(t *testing.T) {
	gpk := fakeGpk(t, `echo '{"event":"plan","action":"remove","steps":[]}'
echo 'error: target not found: nosuch' >&2
exit 1`)
	h, stdout, stderr := testHelper(t, host.Pacman, gpk)

	if code := h.Run([]string{"remove", "--manager", "pacman", "--with-deps", "nosuch"}); code != 1 {
		t.Fatalf("exit %d", code)
	}
	done := lastDone(t, readEvents(t, stdout))
	if done.OK || done.Exit != 1 || done.Error == "" {
		t.Fatalf("done = %+v", done)
	}
	if !strings.Contains(stderr.String(), "target not found") {
		t.Fatalf("gpk stderr not passed on: %q", stderr.String())
	}
	if argv := readFile(t, filepath.Join(h.GpkHome, "argv")); !strings.Contains(argv, "--with-deps\n") {
		t.Fatalf("--with-deps not passed: %q", argv)
	}
}

func TestMissingGpk(t *testing.T) {
	h, stdout, _ := testHelper(t, host.Pacman, filepath.Join(t.TempDir(), "absent"))
	if code := h.Run([]string{"install", "--manager", "pacman", "sl"}); code != 127 {
		t.Fatalf("exit %d", code)
	}
	if done := lastDone(t, readEvents(t, stdout)); done.OK || done.Exit != 127 {
		t.Fatalf("done = %+v", done)
	}
}

// A cancel reaches gpk as the signal the helper got, and gpk's own final event
// is the one the caller reads.
func TestSignalsReachGpk(t *testing.T) {
	gpk := fakeGpk(t, `trap 'echo "{\"event\":\"done\",\"ok\":false,\"exit\":130,\"error\":\"cancelled\"}"; exit 130' TERM
echo '{"event":"plan","action":"install","steps":[]}'
sleep 5 >/dev/null 2>&1 &
wait`)
	h, stdout, _ := testHelper(t, host.Pacman, gpk)
	signals := make(chan os.Signal, 1)
	h.Signals = signals
	go func() {
		time.Sleep(300 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()
	start := time.Now()
	if code := h.Run([]string{"install", "--manager", "pacman", "sl"}); code != 130 {
		t.Fatalf("exit %d", code)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("gpk was not interrupted")
	}
	if done := lastDone(t, readEvents(t, stdout)); done.OK || done.Exit != 130 || done.Error != "cancelled" {
		t.Fatalf("done = %+v", done)
	}
}

// Every refusal is exit 3 or 2 with a done event, and gpk never runs.
func TestRefusals(t *testing.T) {
	never := fakeGpk(t, "echo ran > \"$HOME/ran\"")
	cases := []struct {
		args []string
		code int
		says string
	}{
		{[]string{"install", "--manager", "aur", "yay-bin"}, 3, "terminal"},
		{[]string{"remove", "--manager", "aur", "yay-bin"}, 3, "terminal"},
		{[]string{"install", "--manager", "flatpak", "org.gnome.Maps"}, 2, "Flatpak"},
		{[]string{"install", "--manager", "dnf", "sl"}, 2, "pacman"},
		{[]string{"install", "--manager", "brew", "sl"}, 2, "Unknown"},
		{[]string{"install", "sl"}, 2, "--manager"},
		{[]string{"install", "--manager", "pacman"}, 2, "at least one"},
		{[]string{"install", "--manager", "pacman", "--overwrite=*", "sl"}, 2, "not a package name"},
		{[]string{"install", "--manager", "pacman", "-Syu"}, 2, "not a package name"},
		{[]string{"install", "--manager", "pacman", "../sl"}, 2, "not a package name"},
		{[]string{"install", "--manager", "pacman", "--with-deps", "sl"}, 2, "not a package name"},
		{[]string{"remove", "--manager", "pacman", "ryoku-desktop"}, 3, "part of the Ryoku desktop"},
		{[]string{"remove", "--manager", "pacman", "ryoku-desktop-niri"}, 3, "part of the Ryoku desktop"},
		{[]string{"remove", "--manager", "pacman", "sl", "gpk"}, 3, "gpk is part"},
		{[]string{"ryoku", "remove", "ryostore"}, 3, "Ryostore is part"},
		{[]string{"ryoku", "install", "ryostore"}, 3, "Ryostore is part"},
		{[]string{"ryoku", "update", "ryostore"}, 3, "Ryostore is part"},
		{[]string{"ryoku", "install", "firefox"}, 2, "not a Ryoku app"},
		{[]string{"ryoku", "purge", "ryotunes"}, 2, "Unknown action"},
		{[]string{"ryoku", "install"}, 2, "apps ryoku"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			h, stdout, stderr := testHelper(t, host.Pacman, never)
			if code := h.Run(c.args); code != c.code {
				t.Fatalf("exit %d, want %d (stderr %q)", code, c.code, stderr.String())
			}
			done := lastDone(t, readEvents(t, stdout))
			if done.OK || done.Exit != c.code || !strings.Contains(done.Error, c.says) {
				t.Fatalf("done = %+v, want error mentioning %q", done, c.says)
			}
			if !strings.Contains(stderr.String(), c.says) {
				t.Fatalf("stderr %q lacks %q", stderr.String(), c.says)
			}
			if _, err := os.Stat(filepath.Join(h.GpkHome, "ran")); err == nil {
				t.Fatal("gpk ran for a refused request")
			}
		})
	}
}

// A dependency of the installed ryoku-desktop is refused; a leaf is not.
func TestRemoveRefusesDesktopClosure(t *testing.T) {
	gpk := fakeGpk(t, `echo '{"event":"done","ok":true,"exit":0,"error":""}'`)
	h, stdout, _ := testHelper(t, host.Pacman, gpk)
	h.Query = func(name string, args ...string) (string, int) {
		if name+" "+strings.Join(args, " ") != "pacman -Qi" {
			return "", 1
		}
		return "Name            : ryoku-desktop\nDepends On      : quickshell>=0.2  ryoku-desktop-compositor\n\n" +
			"Name            : ryoku-desktop-niri\nProvides        : ryoku-desktop-compositor\nDepends On      : niri\n\n" +
			"Name            : niri\nDepends On      : None\n\n" +
			"Name            : quickshell\nDepends On      : qt6-base\n\n" +
			"Name            : qt6-base\nDepends On      : None\n\n" +
			"Name            : sl\nDepends On      : ncurses\n", 0
	}
	for _, name := range []string{"quickshell", "qt6-base", "niri"} {
		stdout.Reset()
		if code := h.Run([]string{"remove", "--manager", "pacman", name}); code != 3 {
			t.Fatalf("remove %s: exit %d", name, code)
		}
	}
	stdout.Reset()
	if code := h.Run([]string{"remove", "--manager", "pacman", "sl"}); code != 0 {
		t.Fatalf("remove sl: exit %d", code)
	}
}

// Ryoku apps: removal and repository lanes go through gpk; an update on a
// repository lane is gpk's upgrade, since DNF does not reinstall on install.
func TestRyokuVerbsOnRepositoryLanes(t *testing.T) {
	cases := []struct {
		manager host.PackageManager
		args    []string
		argv    string
	}{
		{host.DNF, []string{"ryoku", "install", "ryotunes"}, "install ryotunes --manager dnf"},
		{host.DNF, []string{"ryoku", "update", "ryotunes"}, "upgrade ryotunes --manager dnf"},
		{host.XBPS, []string{"ryoku", "update", "ryoport"}, "upgrade ryoport --manager xbps"},
		{host.Pacman, []string{"ryoku", "install", "ryomanager"}, "install ryomanager --manager pacman"},
		{host.Pacman, []string{"ryoku", "update", "ryomanager"}, "upgrade ryomanager --manager pacman"},
		{host.Pacman, []string{"ryoku", "remove", "ryotunes"}, "remove ryotunes --manager pacman"},
		{host.Pacman, []string{"ryoku", "remove", "ryoport"}, "remove ryoport --manager aur"},
		{host.Pacman, []string{"ryoku", "remove", "ryomanager"}, "remove ryomanager --manager pacman"},
	}
	for _, c := range cases {
		t.Run(string(c.manager)+" "+strings.Join(c.args, " "), func(t *testing.T) {
			gpk := fakeGpk(t, `echo '{"event":"done","ok":true,"exit":0,"error":""}'`)
			h, _, _ := testHelper(t, c.manager, gpk)
			// ryoport was installed from its GitHub release: foreign to pacman's sync databases.
			h.Query = func(name string, args ...string) (string, int) {
				if name+" "+strings.Join(args, " ") == "pacman -Qqm -- ryoport" {
					return "ryoport\n", 0
				}
				return "", 1
			}
			if code := h.Run(c.args); code != 0 {
				t.Fatalf("exit %d", code)
			}
			argv := strings.ReplaceAll(strings.TrimSpace(readFile(t, filepath.Join(h.GpkHome, "argv"))), "\n", " ")
			if want := c.argv + " --yes --events --privilege none"; argv != want {
				t.Fatalf("gpk argv %q, want %q", argv, want)
			}
		})
	}
}

// On pacman, Ryotunes installs through the verified release lane on the
// caller's line, and its output becomes the same event shapes.
func TestRyokuInstallOnReleaseLane(t *testing.T) {
	h, stdout, _ := testHelper(t, host.Pacman, fakeGpk(t, "exit 9"))
	h.Line = apprelease.HyperionLine
	var got apprelease.App
	var gotUpdate bool
	h.Release = func(_ context.Context, app apprelease.App, update bool, opts apprelease.Options) (apprelease.Status, error) {
		got, gotUpdate = app, update
		opts.Resolved("", "1:2.0.0-1")
		fmt.Fprint(opts.Output, "loading packages...\rresolving dependencies...\n:: Processing package changes...\npartial")
		return apprelease.Status{Installed: "1:2.0.0-1", Latest: "1:2.0.0-1", Updated: true}, nil
	}
	if code := h.Run([]string{"ryoku", "install", "ryotunes"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got.Name != "ryotunes" || !got.Prerelease() || gotUpdate {
		t.Fatalf("release lane got %+v update=%v", got, gotUpdate)
	}
	events := readEvents(t, stdout)
	var shape []string
	for _, e := range events {
		shape = append(shape, e.Event+":"+e.Phase+e.Line)
	}
	want := "plan:,step:start,log:loading packages...,log:resolving dependencies...,log::: Processing package changes...,log:partial,step:done,done:"
	if strings.Join(shape, ",") != want {
		t.Fatalf("events %v\nwant %s", shape, want)
	}
	if events[0].Action != "install" || events[0].Steps[0] != (Step{Manager: "apprelease", Name: "ryotunes", Version: "1:2.0.0-1"}) {
		t.Fatalf("plan = %+v", events[0])
	}
	if done := lastDone(t, events); !done.OK {
		t.Fatalf("done = %+v", done)
	}
}

func TestRyokuUpdateOnReleaseLane(t *testing.T) {
	cases := []struct {
		name   string
		status apprelease.Status
		err    error
		code   int
		events string
	}{
		{"current", apprelease.Status{Installed: "1:2.0.0-1", Latest: "1:2.0.0-1"}, nil, 0, "plan,log,done"},
		{"absent", apprelease.Status{}, nil, 1, "done"},
		{"failed", apprelease.Status{}, errors.New("sha256 mismatch"), 1, "done"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, stdout, _ := testHelper(t, host.Pacman, fakeGpk(t, "exit 9"))
			h.Release = func(_ context.Context, app apprelease.App, update bool, _ apprelease.Options) (apprelease.Status, error) {
				if !update || app.Prerelease() {
					t.Fatalf("update=%v line=%q", update, app.Line)
				}
				return c.status, c.err
			}
			if code := h.Run([]string{"ryoku", "update", "ryotunes"}); code != c.code {
				t.Fatalf("exit %d, want %d", code, c.code)
			}
			events := readEvents(t, stdout)
			var names []string
			for _, e := range events {
				names = append(names, e.Event)
			}
			if strings.Join(names, ",") != c.events {
				t.Fatalf("events %v, want %s", names, c.events)
			}
			if done := lastDone(t, events); done.OK != (c.code == 0) {
				t.Fatalf("done = %+v", done)
			}
		})
	}
}

// Under --stdin-cancel the end of stdin, or a "cancel" line, stops gpk the way
// a signal would; without the flag stdin is never read.
func TestStdinCancel(t *testing.T) {
	script := `trap 'echo "{\"event\":\"done\",\"ok\":false,\"exit\":130,\"error\":\"cancelled\"}"; exit 130' TERM
echo '{"event":"plan","action":"install","steps":[]}'
sleep 3 >/dev/null 2>&1 &
wait
echo '{"event":"done","ok":true,"exit":0,"error":""}'`
	for name, stdin := range map[string]string{"eof": "", "cancel line": "keep\ncancel\n"} {
		t.Run(name, func(t *testing.T) {
			h, stdout, _ := testHelper(t, host.Pacman, fakeGpk(t, script))
			h.Stdin = strings.NewReader(stdin)
			start := time.Now()
			if code := h.Run([]string{"--stdin-cancel", "install", "--manager", "pacman", "sl"}); code != 130 {
				t.Fatalf("exit %d", code)
			}
			if time.Since(start) > 2500*time.Millisecond {
				t.Fatal("stdin cancel did not stop gpk")
			}
			if done := lastDone(t, readEvents(t, stdout)); done.OK || done.Exit != 130 {
				t.Fatalf("done = %+v", done)
			}
		})
	}
	t.Run("without the flag", func(t *testing.T) {
		h, stdout, _ := testHelper(t, host.Pacman, fakeGpk(t, `echo '{"event":"done","ok":true,"exit":0,"error":""}'`))
		h.Stdin = strings.NewReader("cancel\n")
		if code := h.Run([]string{"install", "--manager", "pacman", "sl"}); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if done := lastDone(t, readEvents(t, stdout)); !done.OK {
			t.Fatalf("done = %+v", done)
		}
	})
	t.Run("release lane", func(t *testing.T) {
		h, stdout, _ := testHelper(t, host.Pacman, fakeGpk(t, "exit 9"))
		h.Stdin = strings.NewReader("")
		h.Release = func(ctx context.Context, _ apprelease.App, _ bool, opts apprelease.Options) (apprelease.Status, error) {
			opts.Resolved("", "1:2.0.0-1")
			<-ctx.Done()
			return apprelease.Status{}, ctx.Err()
		}
		if code := h.Run([]string{"--stdin-cancel", "ryoku", "install", "ryotunes"}); code != 130 {
			t.Fatalf("exit %d", code)
		}
		events := readEvents(t, stdout)
		if done := lastDone(t, events); done.OK || done.Exit != 130 || done.Error != "cancelled" {
			t.Fatalf("done = %+v", done)
		}
		if step := events[len(events)-2]; step.Event != "step" || step.Phase != "done" || step.Exit != 130 {
			t.Fatalf("step = %+v", step)
		}
	})
}
