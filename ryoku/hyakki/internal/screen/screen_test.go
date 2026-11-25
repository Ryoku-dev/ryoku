package screen

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ryoku-rashin/internal/hands/input"
)

func TestCommandLinesSplitWithoutAShell(t *testing.T) {
	cases := map[string][]string{
		"kitty":                               {"kitty"},
		"firefox https://example.org/?q=a":    {"firefox", "https://example.org/?q=a"},
		"  gedit   notes.txt  ":               {"gedit", "notes.txt"},
		`gedit "my notes.txt" 'it''s'`:        {"gedit", "my notes.txt", "its"},
		`echo "say \"hi\" \\ \n"`:             {"echo", `say "hi" \ \n`},
		`touch a\ b`:                          {"touch", "a b"},
		"nautilus ~ ~/Documents a~b '~'":      {"nautilus", "/screen/home", "/screen/home/Documents", "a~b", "~"},
		"firefox 'https://x/?a=1&b=$2' x#y":   {"firefox", "https://x/?a=1&b=$2", "x#y"},
		"/usr/bin/kitty --title=Agent --hold": {"/usr/bin/kitty", "--title=Agent", "--hold"},
	}
	for line, want := range cases {
		got, err := Argv(line, "/screen/home")
		if err != nil || strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("%s = %q, %v; want %q", line, got, err, want)
		}
	}
	refused := map[string]error{
		"":                    errEmpty,
		"   ":                 errEmpty,
		"''":                  errEmpty,
		"kitty | tee log":     errShell,
		"cd /tmp && kitty":    errShell,
		"kitty; rm -rf ~":     errShell,
		"kitty > out":         errShell,
		"kitty &":             errShell,
		"echo $HOME":          errShell,
		"echo `id`":           errShell,
		`echo "$(id)"`:        errShell,
		"(kitty)":             errShell,
		"# kitty":             errShell,
		"FOO=bar kitty":       errShell,
		"sh -c kitty":         errShellRun,
		"/bin/bash -lc kitty": errShellRun,
		"env FOO=1 kitty":     errShellRun,
		"busybox sh":          errShellRun,
		"gedit 'open":         errQuote,
		`gedit "open`:         errQuote,
		`gedit end\`:          errQuote,
		"kitty\nrm -rf ~":     errControl,
		"kitty\x1b[2J":        errControl,
	}
	for line, want := range refused {
		if got, err := Argv(line, "/screen/home"); !errors.Is(err, want) {
			t.Errorf("%q = %q, %v; want %v", line, got, err, want)
		}
	}
}

// keyLog is a fake Runner: it logs what the screen asks the keyboard tool
// to do, and runs nothing.
type keyLog []string

func (l *keyLog) run(_ context.Context, name string, args ...string) error {
	if !input.Allowed(name, args...) {
		*l = append(*l, "REFUSED")
		return input.ErrNotAllowed
	}
	if args[2] == "--" {
		*l = append(*l, "type:"+args[3])
	} else {
		*l = append(*l, "key:"+args[len(args)/2+1])
	}
	return nil
}

// Typing keeps what the screen always did with newlines and tabs, pressing
// them as keys, while every typed argument stays a bounded, key-free chunk.
func TestScreenTypesLinesAndTabsAsKeys(t *testing.T) {
	var log keyLog
	s := &Screen{keys: input.Keyboard{Run: log.run}}
	long := strings.Repeat("x", 45)
	if err := s.Type(context.Background(), "ab\tc\r\n"+long+"\n\nend"); err != nil {
		t.Fatal(err)
	}
	want := "type:ab,key:Tab,type:c,key:Return,type:" + long[:40] + ",type:" + long[40:] + ",key:Return,key:Return,type:end"
	if strings.Join(log, ",") != want {
		t.Fatalf("log = %q", log)
	}
}

func TestScreenRefusesOtherControlCharactersBeforeTyping(t *testing.T) {
	var log keyLog
	s := &Screen{keys: input.Keyboard{Run: log.run}}
	for _, text := range []string{"first\nsecond\x1b:q!", "a\rb", "bad\xff"} {
		if err := s.Type(context.Background(), text); !errors.Is(err, input.ErrText) {
			t.Errorf("%q: %v; want ErrText", text, err)
		}
	}
	if len(log) != 0 {
		t.Fatalf("typed %q", log)
	}
}

func TestScreenKeysGoThroughTheChordAllowlist(t *testing.T) {
	var log keyLog
	s := &Screen{keys: input.Keyboard{Run: log.run}}
	for _, chord := range []string{"ctrl+l", "alt+f4", "Return"} {
		if err := s.Key(context.Background(), chord); err != nil {
			t.Fatalf("%s: %v", chord, err)
		}
	}
	if err := s.Key(context.Background(), "ctrl+-d"); !errors.Is(err, input.ErrKey) {
		t.Fatalf("ctrl+-d = %v", err)
	}
	if strings.Join(log, ",") != "key:l,key:F4,key:Return" {
		t.Fatalf("log = %q", log)
	}
}
