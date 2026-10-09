package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// pacman progress repaints end in \r, not \n; they must surface live as
// transient events (throttled), while only newline-terminated lines are final.
func TestCmdReaderSplitsCarriageReturns(t *testing.T) {
	e := &engine{events: make(chan any, 64)}
	err := e.cmd("", nil, "sh", "-c", `printf 'dl 1%%\rdl 2%%\rdl done\n'; printf 'crlf line\r\n'`)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	close(e.events)
	var finals, transients []string
	for msg := range e.events {
		if ln, ok := msg.(evLine); ok {
			s := strings.TrimSpace(ln.line)
			if strings.HasPrefix(s, "$") {
				continue // the echoed command line
			}
			if ln.transient {
				transients = append(transients, s)
			} else {
				finals = append(finals, s)
			}
		}
	}
	if len(transients) != 1 || transients[0] != "dl 1%" {
		t.Errorf("transients = %v, want the first repaint only (throttle eats the second)", transients)
	}
	if len(finals) != 2 || finals[0] != "dl done" || finals[1] != "crlf line" {
		t.Errorf("finals = %v, want [dl done, crlf line] (\\r\\n is one line ending)", finals)
	}
}

// a shell-converted box keeps its own bootloader + initramfs, so the limine
// hook packages -- which pull limine + mkinitcpio back as depends and ship
// pacman hooks that collide with a host's own (Garuda's garuda-hooks, #58) --
// must be filtered out of the install set alongside limine/mkinitcpio/snapper.
func TestReadBasePackagesSkipsBootChain(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "system/packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "# a comment\nlimine\nlimine-mkinitcpio-hook\nlimine-snapper-sync\nmkinitcpio\nsnapper\nkitty\nryoku-desktop\n"
	if err := os.WriteFile(filepath.Join(dir, "system/packages/base.packages"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (&engine{payload: dir}).readBasePackages()
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, skip := range []string{"limine", "limine-mkinitcpio-hook", "limine-snapper-sync", "mkinitcpio", "snapper"} {
		if set[skip] {
			t.Errorf("%s must be skipped on a shell-converted box, got %v", skip, got)
		}
	}
	for _, keep := range []string{"kitty", "ryoku-desktop"} {
		if !set[keep] {
			t.Errorf("%s must install, missing from %v", keep, got)
		}
	}
}

func TestVoidDefaultPackageSet(t *testing.T) {
	e := &engine{
		payload: "..",
		p:       &plan{browser: "firefox", shell: "fish"},
	}
	base, err := e.readBasePackages()
	if err != nil {
		t.Fatal(err)
	}
	pkgs := sourcePackageSet(voidLinux, base)
	set := packageSet(pkgs)
	for _, required := range []string{
		"fish-shell", "NetworkManager", "ImageMagick", "ffmpeg6",
		"niri", "quickshell", "turnstile", "elogind", "sddm",
		"qt6-base-devel", "xwayland-satellite",
	} {
		if !set[required] {
			t.Errorf("Void package set is missing %s", required)
		}
	}
	for _, absent := range []string{
		"fish", "chromium", "hypridle", "limine", "mkinitcpio",
		"pipewire-pulse", "ttf-material-symbols-variable", "vimix-cursors",
	} {
		if set[absent] {
			t.Errorf("Void package set must not request %s", absent)
		}
	}
	for start := 0; start < len(pkgs); start += 12 {
		end := min(start+12, len(pkgs))
		t.Log(strings.Join(pkgs[start:end], " "))
	}
}

func TestVoidDefaultsToPackagedProvider(t *testing.T) {
	p := defaultPlan(&facts{distro: voidLinux})
	if p.compositor != wm.ProviderNiri {
		t.Fatalf("Void provider = %q", p.compositor)
	}
}
