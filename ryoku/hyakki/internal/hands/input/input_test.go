package input

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAllowedRunsOnlyTheTwoExactShapes(t *testing.T) {
	forty := strings.Repeat("é", MaxChunk)
	ok := [][]string{
		{"-s", Settle, "--", "hello"},
		{"-s", Settle, "--", forty},
		{"-s", Settle, "--", "-d is text after the separator"},
		{"-s", Settle, "-k", "Return"},
		{"-s", Settle, "-M", "ctrl", "-k", "l", "-m", "ctrl"},
		{"-s", Settle, "-M", "ctrl", "-M", "shift", "-k", "Tab", "-m", "shift", "-m", "ctrl"},
		{"-s", Settle, "-M", "logo", "-k", "F4", "-m", "logo"},
	}
	for _, args := range ok {
		if !Allowed(Tool, args...) {
			t.Errorf("refused %q", args)
		}
	}
	bad := map[string][]string{
		"no settle":             {"--", "hi"},
		"other settle":          {"-s", "0", "--", "hi"},
		"delay option":          {"-s", Settle, "-d", "10", "--", "hi"},
		"second text":           {"-s", Settle, "--", "hi", "there"},
		"text without --":       {"-s", Settle, "hi"},
		"option-looking chunk":  {"-s", Settle, "-x"},
		"empty chunk":           {"-s", Settle, "--", ""},
		"too long":              {"-s", Settle, "--", forty + "e"},
		"newline":               {"-s", Settle, "--", "a\nb"},
		"tab":                   {"-s", Settle, "--", "a\tb"},
		"carriage return":       {"-s", Settle, "--", "a\rb"},
		"escape":                {"-s", Settle, "--", "a\x1bb"},
		"line separator":        {"-s", Settle, "--", "a\u2028b"},
		"invalid utf-8":         {"-s", Settle, "--", "a\xffb"},
		"dash key":              {"-s", Settle, "-k", "-d"},
		"two keys":              {"-s", Settle, "-k", "a", "-k", "b"},
		"unknown modifier":      {"-s", Settle, "-M", "hyper", "-k", "a", "-m", "hyper"},
		"held modifier":         {"-s", Settle, "-M", "ctrl", "-k", "a"},
		"wrong release order":   {"-s", Settle, "-M", "ctrl", "-M", "alt", "-k", "a", "-m", "ctrl", "-m", "alt"},
		"twice held":            {"-s", Settle, "-M", "ctrl", "-M", "ctrl", "-k", "a", "-m", "ctrl", "-m", "ctrl"},
		"release never pressed": {"-s", Settle, "-k", "a", "-m", "ctrl"},
		"key with a space":      {"-s", Settle, "-k", "a b"},
		"text after a key":      {"-s", Settle, "-k", "a", "--", "x"},
	}
	for name, args := range bad {
		if Allowed(Tool, args...) {
			t.Errorf("%s: allowed %q", name, args)
		}
	}
	for _, tool := range []string{"sh", "ydotool", "/usr/bin/" + Tool, ""} {
		if Allowed(tool, "-s", Settle, "--", "hi") {
			t.Errorf("allowed tool %q", tool)
		}
	}
}

func TestChordsBecomePressesInOrder(t *testing.T) {
	cases := map[string]string{
		"ctrl+shift+t": "-M ctrl -M shift -k t -m shift -m ctrl",
		"Ctrl+L":       "-M ctrl -k l -m ctrl",
		"alt+f4":       "-M alt -k F4 -m alt",
		"super+Enter":  "-M logo -k Return -m logo",
		"shift+Tab":    "-M shift -k Tab -m shift",
		"Page_Up":      "-k Page_Up",
		"esc":          "-k Escape",
		"A":            "-k A",
	}
	for chord, want := range cases {
		args, err := KeyArgs(chord)
		if err != nil || strings.Join(args, " ") != want {
			t.Errorf("%s = %v, %v; want %s", chord, args, err, want)
		}
		if !Allowed(Tool, append([]string{"-s", Settle}, args...)...) {
			t.Errorf("%s: its own argv is refused", chord)
		}
	}
	for _, chord := range []string{"", "ctrl+", "hyper+x", "ctrl+ctrl+x", "ctrl+,", "--help", "-d", "ctrl+a b", "a\nb"} {
		if args, err := KeyArgs(chord); !errors.Is(err, ErrKey) {
			t.Errorf("%q gave %v, %v; want ErrKey", chord, args, err)
		}
	}
}

func liveVars(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../../../wm/oniwindow/testdata/session-handles.txt")
	if err != nil {
		t.Fatal(err)
	}
	handles := strings.Fields(string(data))
	if len(handles) != 2 {
		t.Fatalf("session fixture must name both handles: %q", handles)
	}
	return append(handles, "DBUS_SESSION_BUS_ADDRESS", "DISPLAY", "WAYLAND_SOCKET")
}

func TestEnvNamesOneDisplayAndNothingElse(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-live")
	for _, name := range liveVars(t) {
		t.Setenv(name, "live")
	}
	env, err := Env("/run/user/1000/hyakki/nest", "wayland-9")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	if strings.Join(names, ",") != "PATH,XDG_RUNTIME_DIR,WAYLAND_DISPLAY,LC_ALL" ||
		env[1] != "XDG_RUNTIME_DIR=/run/user/1000/hyakki/nest" || env[2] != "WAYLAND_DISPLAY=wayland-9" {
		t.Fatalf("env = %q", env)
	}
	for _, c := range [][2]string{
		{"relative/dir", "wayland-1"}, {"", "wayland-1"}, {"/run/user/1000/../0", "wayland-1"},
		{"/run/user/1000", ""}, {"/run/user/1000", "../wayland-0"}, {"/run/user/1000", "/run/user/1000/wayland-0"},
		{"/run/user/1000", ".."}, {"/run/user/1000", "wayland 1"},
	} {
		if _, err := Env(c[0], c[1]); !errors.Is(err, ErrEnv) {
			t.Errorf("Env(%q, %q) = %v; want ErrEnv", c[0], c[1], err)
		}
	}
}

func TestExecRefusesBeforeStartingAnything(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	marker := filepath.Join(dir, "ran")
	fakeTool(t, bin, "#!/bin/sh\n: > "+marker+"\n")
	t.Setenv("PATH", bin)
	env, err := Env(dir, "wayland-9")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, extra := range append(liveVars(t), "WAYLAND_DISPLAY=wayland-0", "LC_ALL=C", "XDG_RUNTIME_DIR=/run/user/1000") {
		if !strings.Contains(extra, "=") {
			extra += "=live"
		}
		if err := Exec(append(env[:len(env):len(env)], extra))(ctx, Tool, "-s", Settle, "--", "x"); !errors.Is(err, ErrEnv) {
			t.Errorf("env with %s: %v; want ErrEnv", extra, err)
		}
	}
	if err := Exec(env[:3])(ctx, Tool, "-s", Settle, "--", "x"); !errors.Is(err, ErrEnv) {
		t.Errorf("env without a locale: %v; want ErrEnv", err)
	}
	for _, argv := range [][]string{{Tool, "--", "x"}, {"sh", "-c", Tool}, {Tool, "-s", Settle, "--", "a\nb"}} {
		if err := Exec(env)(ctx, argv[0], argv[1:]...); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v; want ErrNotAllowed", argv, err)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a refused call started the tool")
	}
}

// The tool runs directly, with exactly the argv and only the display's
// environment, even when the daemon's own holds the live session's.
func TestExecRunsTheToolWithOnlyTheDisplay(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	fakeTool(t, bin, "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > \"$XDG_RUNTIME_DIR/args\"\nexport -p > \"$XDG_RUNTIME_DIR/env\"\n")
	t.Setenv("PATH", bin)
	t.Setenv("WAYLAND_DISPLAY", "wayland-live")
	for _, name := range liveVars(t) {
		t.Setenv(name, "live")
	}
	env, err := Env(dir, "wayland-9")
	if err != nil {
		t.Fatal(err)
	}
	if err := Exec(env)(context.Background(), Tool, "-s", Settle, "--", "-n héllo"); err != nil {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(filepath.Join(dir, "args"))
	if string(args) != "-s\n"+Settle+"\n--\n-n héllo\n" {
		t.Fatalf("argv = %q", args)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "env"))
	got := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^(?:declare -x|export) ([A-Za-z_][A-Za-z0-9_]*)`).FindAllStringSubmatch(string(raw), -1) {
		got[m[1]] = true
	}
	for _, name := range liveVars(t) {
		if got[name] {
			t.Errorf("the tool saw %s", name)
		}
	}
	for _, name := range []string{"XDG_RUNTIME_DIR", "WAYLAND_DISPLAY", "LC_ALL"} {
		if !got[name] {
			t.Errorf("the tool missed %s: %s", name, raw)
		}
	}
	if !strings.Contains(string(raw), "wayland-9") || strings.Contains(string(raw), "wayland-live") {
		t.Errorf("the tool saw another display: %s", raw)
	}
}

func fakeTool(t *testing.T, bin, script string) {
	t.Helper()
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, Tool), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestChunksKeepCharactersWhole(t *testing.T) {
	family := "👩\u200d👩\u200d👧\u200d👦"
	text := strings.Repeat("a", 38) + family + "🇯🇵🇯🇵" + strings.Repeat("e\u0301", 30)
	chunks := Chunks(text, MaxChunk)
	if strings.Join(chunks, "") != text {
		t.Fatal("chunks lose text")
	}
	for _, c := range chunks {
		if n := utf8.RuneCountInString(c); n == 0 || n > MaxChunk {
			t.Errorf("chunk of %d runes", n)
		}
		if strings.HasPrefix(c, "\u200d") || strings.HasPrefix(c, "\u0301") {
			t.Errorf("chunk starts inside a character: %q", c)
		}
	}
	if chunks[0] != strings.Repeat("a", 38) || !strings.HasPrefix(chunks[1], family+"🇯🇵🇯🇵e\u0301") {
		t.Errorf("chunks = %q", chunks[:2])
	}
}
