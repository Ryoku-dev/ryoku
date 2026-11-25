package apprules

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/window"
)

// Every key the rules name, by name: the safe set Hands presses, Enter as
// the person's submit, paste and desktop keys refused, the rest handed off.
func TestEveryKeyByName(t *testing.T) {
	cases := []struct {
		chord string
		want  Kind
		why   string
	}{
		{"Tab", KindSafe, WhySafe},
		{"shift+tab", KindSafe, WhySafe},
		{"Shift+Tab", KindSafe, WhySafe},
		{"left", KindSafe, WhySafe},
		{"Right", KindSafe, WhySafe},
		{"ArrowUp", KindSafe, WhySafe},
		{"down", KindSafe, WhySafe},
		{"Home", KindSafe, WhySafe},
		{"End", KindSafe, WhySafe},
		{"PageUp", KindSafe, WhySafe},
		{"page_down", KindSafe, WhySafe},
		{"BackSpace", KindSafe, WhySafe},
		{"Ctrl+Z", KindSafe, WhySafe},
		{"control+z", KindSafe, WhySafe},

		{"Enter", KindHandoff, WhySubmit},
		{"Return", KindHandoff, WhySubmit},
		{"ctrl+Enter", KindHandoff, WhySubmit},
		{"shift+Return", KindHandoff, WhySubmit},
		{"kp_enter", KindHandoff, WhySubmit},

		{"ctrl+v", KindRefused, WhyPaste},
		{"Ctrl+Shift+V", KindRefused, WhyPaste},
		{"shift+Insert", KindRefused, WhyPaste},
		{"super+l", KindRefused, WhySystemKey},
		{"Super+Tab", KindRefused, WhySystemKey},
		{"win+d", KindRefused, WhySystemKey},
		{"meta+Return", KindRefused, WhySystemKey},
		{"ctrl+alt+F2", KindRefused, WhySystemKey},
		{"ctrl+alt+BackSpace", KindRefused, WhySystemKey},
		{"ctrl+alt+Delete", KindRefused, WhySystemKey},

		{"Delete", KindHandoff, WhyRiskyKey},
		{"del", KindHandoff, WhyRiskyKey},
		{"alt+F4", KindHandoff, WhyRiskyKey},
		{"ctrl+w", KindHandoff, WhyRiskyKey},
		{"ctrl+q", KindHandoff, WhyRiskyKey},
		{"F5", KindHandoff, WhyRiskyKey},
		{"F12", KindHandoff, WhyRiskyKey},
		{"Escape", KindHandoff, WhyRiskyKey},
		{"ctrl+a", KindHandoff, WhyRiskyKey},
		{"ctrl+shift+z", KindHandoff, WhyRiskyKey},
		{"shift+Left", KindHandoff, WhyRiskyKey},
		{"space", KindHandoff, WhyRiskyKey},
		{"a", KindHandoff, WhyRiskyKey},
		{"Menu", KindHandoff, WhyRiskyKey},

		{"", KindRefused, WhySystemKey},
		{"ctrl+", KindRefused, WhySystemKey},
		{"ctrl+ctrl+z", KindRefused, WhySystemKey},
		{"hyper+z", KindRefused, WhySystemKey},
		{"Tab Tab", KindRefused, WhySystemKey},
		{"ctrl+zz", KindRefused, WhySystemKey},
		{"XF86PowerOff", KindRefused, WhySystemKey},
	}
	for _, tc := range cases {
		_, v := PressChord(tc.chord)
		if v.Kind != tc.want || v.Why != tc.why {
			t.Errorf("PressChord(%q) = %v %s, want %v %s", tc.chord, v.Kind, v.Why, tc.want, tc.why)
		}
		if Safe(tc.chord) != (tc.want == KindSafe) {
			t.Errorf("Safe(%q) = %v", tc.chord, !(tc.want == KindSafe))
		}
	}
}

func TestChordsHaveOneSpelling(t *testing.T) {
	for in, want := range map[string]string{
		"Shift+Tab": "shift+Tab", "shift + tab": "shift+Tab", "Ctrl+Z": "ctrl+z", "shift+ctrl+v": "ctrl+shift+v",
		"PageDown": "Page_Down", "ArrowLeft": "Left", "cmd+alt+q": "alt+logo+q", "Backspace": "BackSpace",
	} {
		c, err := ParseChord(in)
		if err != nil {
			t.Fatalf("ParseChord(%q): %v", in, err)
		}
		if c.String() != want {
			t.Errorf("ParseChord(%q) = %q, want %q", in, c, want)
		}
		again, err := ParseChord(c.String())
		if err != nil || again.String() != want {
			t.Errorf("the canonical %q does not read back: %v %q", want, err, again)
		}
	}
}

// The zero verdict is a handoff: nothing unjudged is ever run.
func TestTheZeroKindIsAHandoff(t *testing.T) {
	if (Verdict{}).Kind != KindHandoff || KindHandoff.String() != "handoff" {
		t.Fatal("the zero verdict must be a handoff")
	}
}

func TestTextRules(t *testing.T) {
	cases := []struct {
		text string
		want Kind
		why  string
	}{
		{"hello", KindSafe, WhySafe},
		{"Ada Lovelace, 10 Downing St", KindSafe, WhySafe},
		{"naïve café 東京 👩‍👩‍👧", KindSafe, WhySafe},
		{"in 2026", KindSafe, WhySafe},
		{"order 12", KindSafe, WhySafe},

		{"hello\nworld", KindHandoff, WhySubmit},
		{"hello\r", KindHandoff, WhySubmit},
		{"one\u2028two", KindHandoff, WhySubmit},

		{"tab\there", KindRefused, WhyControl},
		{"esc\x1b", KindRefused, WhyControl},
		{"right\u202eto left", KindRefused, WhyControl},
		{"bad \xff", KindRefused, WhyControl},
		{"", KindRefused, WhyEmpty},
		{"   ", KindRefused, WhyEmpty},

		{"my password is swordfish", KindRefused, WhySecret},
		{"4111 1111 1111 1111", KindRefused, WhySecret},
		{"482913", KindRefused, WhySecret},
		{"482 913", KindRefused, WhySecret},
		{"48291375", KindRefused, WhySecret},
		{"sk-proj-abcdefghijklmnop", KindRefused, WhySecret},
		{"token ghp_aaaaBBBBccccDDDD1234", KindRefused, WhySecret},
		{"AKIAIOSFODNN7EXAMPLE", KindRefused, WhySecret},
		{"Xk9fQ2mZ7pL4wR8tY1vB3nC6", KindRefused, WhySecret},
		{"-----BEGIN OPENSSH PRIVATE KEY-----", KindRefused, WhySecret}, // rashin-reviewer:allow private-key - header-only rejection fixture
		{"a secret with a newline\n", KindRefused, WhySecret},
	}
	for _, tc := range cases {
		v := Text(tc.text)
		if v.Kind != tc.want || v.Why != tc.why {
			t.Errorf("Text(%q) = %v %s, want %v %s", tc.text, v.Kind, v.Why, tc.want, tc.why)
		}
		if strings.Contains(v.Why, tc.text) && tc.text != "" {
			t.Errorf("the rule code repeats the text")
		}
	}
}

func TestChunksStayWithinFortyRunesAndKeepClusters(t *testing.T) {
	long := strings.Repeat("abcdefghij", 13)
	for _, text := range []string{
		long,
		strings.Repeat("é", 41),
		strings.Repeat("e\u0301", 30),
		strings.Repeat("👩‍👩‍👧", 9),
		strings.Repeat("🇯🇵", 25),
		"short",
	} {
		chunks := Chunks(text, MaxChunk)
		if strings.Join(chunks, "") != text {
			t.Fatalf("chunks of %q do not rejoin", text)
		}
		for _, c := range chunks {
			if n := utf8.RuneCountInString(c); n == 0 || n > MaxChunk {
				t.Fatalf("a chunk of %d runes", n)
			}
			if strings.HasPrefix(c, "\u0301") || strings.HasPrefix(c, "\u200d") {
				t.Fatalf("a chunk opens inside a cluster: %q", c)
			}
		}
	}
	if got := len(Chunks(long, MaxChunk)); got != 4 {
		t.Fatalf("130 runes in %d chunks, want 4", got)
	}
	if got := Chunks("abc", 0); len(got) != 3 {
		t.Fatalf("max 0 cuts per rune: %q", got)
	}
}

func TestSelfAndPrivateApps(t *testing.T) {
	for _, app := range []string{"dev.ryoku.Hyakki", "DEV.RYOKU.HYAKKI", "dev.ryoku.Hyakki.Oni"} {
		if !Self(app) {
			t.Errorf("Self(%q) = false", app)
		}
	}
	for _, app := range []string{"dev.ryoku.HyakkiX", "dev.ryoku.Hub", "", "org.gnome.TextEditor"} {
		if Self(app) {
			t.Errorf("Self(%q) = true", app)
		}
	}
	for _, app := range []string{"org.keepassxc.KeePassXC", "com.bitwarden.desktop", "1Password", "polkit-gnome-authentication-agent-1",
		"hyprpolkitagent", "org.kde.polkit-kde-authentication-agent-1", "lxqt-policykit-agent", "gcr-prompter",
		"ssh-askpass", "swaylock", "hyprlock", "org.kde.kscreenlocker_greet", "pinentry-qt", "org.gnome.seahorse.Application"} {
		if !PrivateApp(app) {
			t.Errorf("PrivateApp(%q) = false", app)
		}
	}
	for _, app := range []string{"org.gnome.TextEditor", "firefox", "kitty", "", "com.github.pinta"} {
		if PrivateApp(app) {
			t.Errorf("PrivateApp(%q) = true", app)
		}
	}
}

// Executables are shown, never run: each kind of program, script or
// launcher gives the folder it sits in, and so does a document.
func TestExecutableShownNotRun(t *testing.T) {
	roots := []string{"/home/u/Downloads", "/home/u/Documents"}
	cases := []struct {
		name string
		f    File
		exec bool
	}{
		{"exec bit", File{Path: "/home/u/Downloads/tool", Mode: 0o755}, true},
		{"group exec bit", File{Path: "/home/u/Downloads/tool", Mode: 0o610}, true},
		{"shebang", File{Path: "/home/u/Downloads/notes", Mode: 0o644, Head: []byte("#!/bin/sh\nrm -rf ~\n")}, true},
		{"elf", File{Path: "/home/u/Downloads/blob", Mode: 0o644, Head: []byte("\x7fELF\x02\x01")}, true},
		{"desktop", File{Path: "/home/u/Downloads/app.desktop", Mode: 0o644}, true},
		{"appimage", File{Path: "/home/u/Downloads/App-x86_64.AppImage", Mode: 0o644}, true},
		{"sh", File{Path: "/home/u/Downloads/install.sh", Mode: 0o644}, true},
		{"run", File{Path: "/home/u/Downloads/driver.run", Mode: 0o644}, true},
		{"python", File{Path: "/home/u/Documents/x.PY", Mode: 0o644}, true},
		{"document", File{Path: "/home/u/Documents/report.pdf", Mode: 0o644, Head: []byte("%PDF-1.7")}, false},
	}
	for _, tc := range cases {
		if got := Executable(tc.f); got != tc.exec {
			t.Errorf("%s: Executable = %v", tc.name, got)
		}
		dir, err := Show(tc.f, roots)
		if err != nil {
			t.Errorf("%s: Show: %v", tc.name, err)
			continue
		}
		if want := tc.f.Path[:strings.LastIndexByte(tc.f.Path, '/')]; dir != want {
			t.Errorf("%s: Show = %q, want the folder %q", tc.name, dir, want)
		}
		if dir == tc.f.Path {
			t.Errorf("%s: Show named the file itself", tc.name)
		}
	}
	dir, err := Show(File{Path: "/home/u/Documents/sub", Mode: fs.ModeDir | 0o755}, roots)
	if err != nil || dir != "/home/u/Documents/sub" {
		t.Fatalf("a folder shows itself: %q %v", dir, err)
	}
}

func TestShowRefusesOutsideTheRoots(t *testing.T) {
	roots := []string{"/home/u/Downloads"}
	cases := []struct {
		f    File
		want error
	}{
		{File{Path: "/home/u/Downloads/../.ssh/id_ed25519", Mode: 0o600}, ErrPath},
		{File{Path: "/home/u/Downloads/./a", Mode: 0o644}, ErrPath},
		{File{Path: "Downloads/a", Mode: 0o644}, ErrPath},
		{File{Path: "", Mode: 0o644}, ErrPath},
		{File{Path: "/home/u/.bashrc", Mode: 0o644}, ErrOutside},
		{File{Path: "/home/u/Downloads-evil/a", Mode: 0o644}, ErrOutside},
		{File{Path: "/home/u/Downloads", Mode: 0o644}, ErrOutside},
		{File{Path: "/home/u/Downloads/pipe", Mode: fs.ModeNamedPipe}, ErrKind},
		{File{Path: "/home/u/Downloads/dev", Mode: fs.ModeDevice}, ErrKind},
		{File{Path: "/home/u/Downloads/link", Mode: fs.ModeSymlink}, ErrKind},
	}
	for _, tc := range cases {
		if _, err := Show(tc.f, roots); !errors.Is(err, tc.want) {
			t.Errorf("Show(%q) = %v, want %v", tc.f.Path, err, tc.want)
		}
	}
	if Within("/home/u/Downloads/a", []string{"/home/u/Downloads/"}) {
		t.Fatal("a root that is not clean is never a root")
	}
}

func TestQueryNamesFilesNeverAPlace(t *testing.T) {
	for _, q := range []string{"report", "tax 2026", "*.pdf", "résumé"} {
		if err := Query(q); err != nil {
			t.Errorf("Query(%q): %v", q, err)
		}
	}
	for _, q := range []string{"", " ", "../etc", "/etc/passwd", `a\b`, "a..b", "two\nlines", strings.Repeat("x", MaxQueryRunes+1)} {
		if err := Query(q); !errors.Is(err, ErrQuery) {
			t.Errorf("Query(%q) = %v", q, err)
		}
	}
}

func TestClickOnlyOnAPlacedUncoveredWindow(t *testing.T) {
	target := window.Window{ID: "1", App: "notes", Output: "DP-1", Visible: true, Floating: true, Box: &window.Box{X: 100, Y: 50, W: 400, H: 200}}
	mid := Region{X: 0.4, Y: 0.4, W: 0.2, H: 0.2}
	x, y, err := Click(target, mid, nil)
	if err != nil || x != 300 || y != 150 {
		t.Fatalf("Click = %v,%v %v; want 300,150", x, y, err)
	}

	tiled := target
	tiled.Box, tiled.Floating = nil, false
	other := window.Window{ID: "2", Output: "DP-1", Visible: true, Box: &window.Box{X: 250, Y: 100, W: 100, H: 100}}
	away := window.Window{ID: "3", Output: "DP-1", Visible: true, Box: &window.Box{X: 900, Y: 0, W: 100, H: 100}}
	elsewhere := window.Window{ID: "4", Output: "HDMI-1", Visible: true, Box: &window.Box{X: 250, Y: 100, W: 100, H: 100}}
	hiddenOther := other
	hiddenOther.Visible = false
	unplacedFloat := window.Window{ID: "5", Output: "DP-1", Visible: true, Floating: true}
	unplacedTiled := window.Window{ID: "6", Output: "DP-1", Visible: true}
	hidden := target
	hidden.Visible = false
	minimized := target
	minimized.Minimized = true

	cases := []struct {
		name   string
		target window.Window
		region Region
		others []window.Window
		want   error
	}{
		{"tiled without a box", tiled, mid, nil, ErrUnplaced},
		{"not shown", hidden, mid, nil, ErrHidden},
		{"minimized", minimized, mid, nil, ErrHidden},
		{"past the right edge", target, Region{X: 0.9, Y: 0.1, W: 0.2, H: 0.1}, nil, ErrRegion},
		{"negative", target, Region{X: -0.1, Y: 0.1, W: 0.05, H: 0.1}, nil, ErrRegion},
		{"covered", target, mid, []window.Window{away, other}, ErrCovered},
		{"an unplaced floating window may be on top", target, mid, []window.Window{unplacedFloat}, ErrCovered},
		{"clear", target, mid, []window.Window{target, away, elsewhere, hiddenOther, unplacedTiled}, nil},
	}
	for _, tc := range cases {
		if _, _, err := Click(tc.target, tc.region, tc.others); !errors.Is(err, tc.want) {
			t.Errorf("%s: Click = %v, want %v", tc.name, err, tc.want)
		}
	}
}
