package window

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

func TestAllowlistIsReadVerbsOnly(t *testing.T) {
	allowed := [][]string{
		{"niri", "msg", "--json", "windows"},
		{"niri", "msg", "--json", "outputs"},
		{"niri", "msg", "--json", "workspaces"},
		{"niri", "msg", "--json", "event-stream"},
		{"hyprctl", "-j", "clients"},
		{"hyprctl", "-j", "monitors"},
		{"hyprctl", "-j", "cursorpos"},
		{"hyprctl", "-j", "layers"},
		{"hyprctl", "--batch", "j/monitors;j/clients"},
		{"ryoku", "wm", "state"},
		{"ryoku", "wm", "caps"},
		{"ryoku", "wm", "watch"},
		{"ryoku", "wm", "watch", "windows", "outputs", "workspaces", "focus"},
	}
	for _, a := range allowed {
		if !Allowed(a[0], a[1:]...) {
			t.Errorf("%q refused", a)
		}
	}
	refused := [][]string{
		{"niri", "msg", "action", "focus-window", "--id", "12"},
		{"niri", "msg", "action", "screenshot-window"},
		{"niri", "msg", "--json", "focused-window", "extra"},
		{"niri", "msg", "windows"},
		{"hyprctl", "dispatch", "movewindowpixel", "exact 0 0,address:0x1"},
		{"hyprctl", "-j", "clients", "--batch", "dispatch exec wtype hi"},
		{"hyprctl", "keyword", "input:kb_layout", "us"},
		{"hyprctl", "--batch", "-j clients ; dispatch killactive"},
		{"hyprctl", "--batch", "j/monitors;j/clients;dispatch killactive"},
		{"hyprctl", "--batch", "j/monitors;j/clients", "--batch", "dispatch killactive"},
		{"ryoku", "wm", "act", "pointer.locate"},
		{"ryoku", "wm", "apply"},
		{"ryoku", "wm", "watch", "windows", "windows"},
		{"ryoku", "wm", "watch", "keyboard"},
		{"ryoku", "wm"},
		{"grim", "-"},
		{"wtype", "hello"},
		{"ydotool", "click", "0xC0"},
		{"/usr/bin/niri", "msg", "--json", "windows"},
		{"sh", "-c", "niri msg --json windows"},
	}
	for _, a := range refused {
		if Allowed(a[0], a[1:]...) {
			t.Errorf("%q allowed", a)
		}
	}
	// A Hyprland batch can chain any request, so an allowlisted batch may hold
	// only reads that are each allowlisted on their own.
	batches := 0
	for _, argv := range allowlist["hyprctl"] {
		if argv[0] != "--batch" {
			continue
		}
		batches++
		if len(argv) != 2 {
			t.Errorf("batch %q is not one request string", argv)
			continue
		}
		for _, c := range strings.Split(argv[1], ";") {
			verb, ok := strings.CutPrefix(c, "j/")
			if !ok || !Allowed("hyprctl", "-j", verb) {
				t.Errorf("batch %q holds %q, which is not an allowlisted read", argv[1], c)
			}
		}
	}
	if batches == 0 {
		t.Error("no batch checked")
	}
}

// Every provider path, driven through recording fakes, asks only for
// allowlisted argv; and a disallowed argv never reaches any Runner, fake or
// real.
func TestProvidersStayInsideTheAllowlist(t *testing.T) {
	ctx := context.Background()
	var all [][]string

	nd := niriDesk()
	np := newNiri(nd.run, nd.stream)
	if _, err := np.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	np.Pointer(ctx)
	collect(t, np)
	all = append(all, nd.calls...)

	hd := hyprDesk()
	hp := newHyprland(hd.run, nil)
	if _, err := hp.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := hp.Pointer(ctx); err != nil {
		t.Fatal(err)
	}
	all = append(all, hd.calls...)

	sd := newDesk(map[string]string{"ryoku wm state": "seam/state-niri.json", "ryoku wm caps": "seam/caps-niri.json"})
	sd.streams["ryoku wm watch windows outputs workspaces focus"] = "seam/watch-niri.jsonl"
	sp, err := DetectWith(nil, sd.run, sd.stream, HyprlandIPC{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	sp.Pointer(ctx)
	collect(t, sp)
	all = append(all, sd.calls...)

	if len(all) < 10 {
		t.Fatalf("only %d calls recorded; the walk exercised nothing", len(all))
	}
	for _, c := range all {
		if !Allowed(c[0], c[1:]...) {
			t.Errorf("provider ran %q outside the allowlist", c)
		}
	}

	reached := false
	spy := func(context.Context, string, ...string) ([]byte, error) { reached = true; return nil, nil }
	spyStream := func(context.Context, string, ...string) (io.ReadCloser, error) { reached = true; return nil, nil }
	if _, err := guard(spy)(ctx, "hyprctl", "dispatch", "exec", "x"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("guard: %v", err)
	}
	if _, err := guardStream(spyStream)(ctx, "niri", "msg", "action", "quit"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("guardStream: %v", err)
	}
	if reached {
		t.Fatal("a refused argv reached the runner")
	}
	// The real runners refuse before starting anything.
	if _, err := Exec(nil)(ctx, "wtype", "x"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("Exec: %v", err)
	}
	if _, err := ExecStream(nil)(ctx, "hyprctl", "dispatch", "killactive"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("ExecStream: %v", err)
	}
}

// Only allow.go may start a process, and no source string names an input
// tool or a compositor action verb. The one Hyprland batch is held to reads
// by TestAllowlistIsReadVerbsOnly.
func TestPackageSourceHasNoOtherExecPath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	banned := []string{"wtype", "ydotool", "uinput", "virtual_pointer", "virtual-keyboard", "dispatch", "action", "keyword"}
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if (p == "os/exec" || p == "syscall") && name != "allow.go" {
				t.Errorf("%s imports %s; only allow.go may start a process", name, p)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			for _, b := range banned {
				if strings.Contains(strings.ToLower(s), b) {
					t.Errorf("%s: string %q names %q", fset.Position(lit.Pos()), s, b)
				}
			}
			return true
		})
	}
	if checked < 6 {
		t.Fatalf("checked %d files", checked)
	}
}

func TestDetectOrder(t *testing.T) {
	seamDesk := func() *fakeDesk { return newDesk(map[string]string{"ryoku wm caps": "seam/caps-hyprland.json"}) }
	noSeam := newDesk(map[string]string{})
	cases := []struct {
		name string
		env  []string
		desk *fakeDesk
		want string
	}{
		{"seam first", []string{"NIRI_SOCKET=/run/user/1000/niri.sock", "WAYLAND_DISPLAY=wayland-1"}, seamDesk(), "ryoku"},
		{"niri", []string{"NIRI_SOCKET=/run/user/1000/niri.sock", "WAYLAND_DISPLAY=wayland-1"}, noSeam, "niri"},
		{"hyprland", []string{"HYPRLAND_INSTANCE_SIGNATURE=0e7f5c1a_1763800000_1234567", "WAYLAND_DISPLAY=wayland-1"}, noSeam, "hyprland"},
		{"generic wlr", []string{"WAYLAND_DISPLAY=wayland-1", "XDG_CURRENT_DESKTOP=sway"}, noSeam, "wlr"},
		{"empty seam answer", []string{"WAYLAND_DISPLAY=wayland-1"}, newDesk(map[string]string{"ryoku wm caps": "seam/empty.json"}), "wlr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := DetectWith(c.env, c.desk.run, c.desk.stream, HyprlandIPC{Requests: c.desk.run})
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Caps().Provider; got != c.want {
				t.Fatalf("provider %q, want %q", got, c.want)
			}
		})
	}
	if _, err := Detect([]string{"HOME=/tmp/h"}, noSeam.run); !errors.Is(err, ErrNoDesktop) {
		t.Fatalf("no desktop: %v", err)
	}
	for _, c := range noSeam.calls {
		if !slices.Equal(c, []string{"ryoku", "wm", "caps"}) {
			t.Fatalf("detect ran %q; it probes only the seam caps", c)
		}
	}
}

func TestWlrDegradedNamesWindowsButNeverPlacesThem(t *testing.T) {
	p := NewWlr()
	if c := p.Caps(); c.Geometry != GeomNone || c.Pointer {
		t.Fatalf("caps %+v", c)
	}
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan Snapshot, 4)
	done := make(chan error, 1)
	go func() { done <- p.Watch(ctx, func(s Snapshot) { got <- s }) }()
	if s := <-got; len(s.Windows) != 0 {
		t.Fatalf("initial %+v", s)
	}
	p.Feed(Snapshot{Outputs: twoOutputs(), Windows: []Window{
		{ID: "t1", App: "foot", Title: "foot", Output: "eDP-1", Focused: true, Visible: true, Box: &Box{W: 800, H: 600}},
	}})
	s := <-got
	w := byID(t, s, "t1")
	wantUnplaced(t, w, UnplacedNoGeom)
	if w.App != "foot" || !w.Focused {
		t.Fatalf("identity lost: %+v", w)
	}
	var tr Tracker
	if _, ok := tr.HitTest(spec.Point{X: 10, Y: 10}, s); ok {
		t.Fatal("a degraded desktop must never hit a window")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
