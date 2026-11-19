package ryokuapps

import (
	"sort"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

func names(p protection) string {
	var list []string
	for name := range p {
		list = append(list, name)
	}
	sort.Strings(list)
	return strings.Join(list, " ")
}

// Continuation lines, virtual provides and version constraints all resolve to
// installed package names.
func TestPacmanClosure(t *testing.T) {
	db := parsePacmanDatabase("Name            : ryoku-desktop\n" +
		"Version         : 1.1.0-1\n" +
		"Depends On      : gpk  glibc>=2.40\n" +
		"                  ryoku-desktop-compositor  libfoo.so=1-64\n" +
		"Optional Deps   : kitty: terminal\n\n" +
		"Name            : ryoku-desktop-hyprland\n" +
		"Provides        : ryoku-desktop-compositor=1.1.0\n" +
		"Depends On      : hyprland\n\n" +
		"Name            : hyprland\nDepends On      : None\n\n" +
		"Name            : gpk\nDepends On      : pacman\n\n" +
		"Name            : pacman\nDepends On      : glibc\n\n" +
		"Name            : glibc\nProvides        : None\nDepends On      : None\n\n" +
		"Name            : foo\nProvides        : libfoo.so=1-64\nDepends On      : None\n\n" +
		"Name            : kitty\nDepends On      : None\n\n" +
		"Name            : sl\nDepends On      : ncurses\n")
	got := names(closure(db))
	if want := "foo glibc gpk hyprland pacman ryoku-desktop ryoku-desktop-hyprland"; got != want {
		t.Fatalf("closure = %q, want %q", got, want)
	}
	if got := names(closure(parsePacmanDatabase("Name : sl\nDepends On : ncurses\n"))); got != "" {
		t.Fatalf("closure without ryoku-desktop = %q", got)
	}
}

func TestRPMClosure(t *testing.T) {
	provides := "ryoku-desktop\tryoku-desktop\nryoku-desktop\tryoku-desktop(x86-64)\n" +
		"quickshell\tquickshell\nglibc\tlibc.so.6()(64bit)\nglibc\tglibc\nsl\tsl\n"
	requires := "ryoku-desktop\tquickshell >= 0.2\nryoku-desktop\t/usr/bin/sh\nryoku-desktop\t(foo or bar)\n" +
		"ryoku-desktop\trpmlib(CompressedFileNames)\nquickshell\tlibc.so.6()(64bit)\nsl\tncurses\n"
	got := names(closure(parseRPMPairs(provides, requires)))
	if want := "glibc quickshell ryoku-desktop"; got != want {
		t.Fatalf("closure = %q, want %q", got, want)
	}
}

func TestXBPSProtection(t *testing.T) {
	query := func(name string, args ...string) (string, int) {
		if name+" "+strings.Join(args, " ") != "xbps-query --fulldeptree -x ryoku-desktop" {
			return "", 1
		}
		return "quickshell-0.2.0_1\nglibc-2.41_1\nryoku-desktop-niri-1.1.0_1\n", 0
	}
	got := names(protectedPackages(host.XBPS, query))
	if want := "glibc quickshell ryoku-desktop ryoku-desktop-niri"; got != want {
		t.Fatalf("protection = %q, want %q", got, want)
	}
	if got := names(protectedPackages(host.XBPS, func(string, ...string) (string, int) { return "", 2 })); got != "" {
		t.Fatalf("failed query protection = %q", got)
	}
}

// The fixed set holds even when the database says nothing.
func TestAlwaysProtected(t *testing.T) {
	p := protection{}
	for _, name := range []string{"ryoku-desktop", "ryoku-desktop-niri", "gpk", "pacman", "dnf5", "xbps", "polkit"} {
		if !p.covers(name) {
			t.Errorf("%s not protected", name)
		}
	}
	for _, name := range []string{"sl", "cowsay", "ryotunes", "ryoport", "ryomanager", "ryoku-dictation"} {
		if p.covers(name) {
			t.Errorf("%s protected", name)
		}
	}
}

func TestXBPSName(t *testing.T) {
	for in, want := range map[string]string{"glibc-2.41_1": "glibc", "ryoku-desktop-niri-1.1.0_1": "ryoku-desktop-niri", "foo>=1.0": "foo", "bare": "bare"} {
		if got := xbpsName(in); got != want {
			t.Errorf("xbpsName(%q) = %q, want %q", in, got, want)
		}
	}
}
