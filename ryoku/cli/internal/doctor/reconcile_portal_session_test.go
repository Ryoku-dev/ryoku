package doctor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// statLine builds a /proc/<pid>/stat line with `comm` as the process name and
// `start` as field 22. Fields 3..21 are filler, which is what the parser has to
// skip to reach starttime.
func statLine(comm string, start uint64) string {
	f := []string{"4242", "(" + comm + ")", "S"}
	for i := 4; i <= 21; i++ {
		f = append(f, "0")
	}
	return strings.Join(append(f, strconv.FormatUint(start, 10), "trailing", "fields"), " ") + "\n"
}

func TestParseStartTicksReadsFieldTwentyTwo(t *testing.T) {
	got, ok := parseStartTicks(statLine("systemd", 60273731))
	if !ok || got != 60273731 {
		t.Fatalf("plain comm: got (%d, %v), want (60273731, true)", got, ok)
	}
}

// A process name is free-form and lands inside parentheses unescaped, so a
// whitespace split over the whole line shifts every field after it. Counting
// from the last ')' is what keeps field 22 field 22.
func TestParseStartTicksSurvivesCommWithSpacesAndParens(t *testing.T) {
	got, ok := parseStartTicks(statLine("xdg desktop (portal)", 77488096))
	if !ok || got != 77488096 {
		t.Fatalf("hostile comm: got (%d, %v), want (77488096, true)", got, ok)
	}
}

func TestParseStartTicksRejectsUnusableLines(t *testing.T) {
	for name, stat := range map[string]string{
		"no comm parens": "4242 systemd S 0 0\n",
		"truncated":      "4242 (systemd) S 0 0 0\n",
		"empty":          "",
		"unparsable":     statLine("systemd", 0)[:strings.LastIndex(statLine("systemd", 0), " 0 trailing")] + " notanumber trailing fields\n",
	} {
		if _, ok := parseStartTicks(stat); ok {
			t.Errorf("%s: parsed as usable, want rejected", name)
		}
	}
}

// The parser has to agree with the kernel's real format, not just the fixture:
// /proc/self/stat is the one stat file every test run is guaranteed to have.
func TestParseStartTicksMatchesRealProcSelf(t *testing.T) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Skipf("no /proc/self/stat: %v", err)
	}
	got, ok := parseStartTicks(string(b))
	if !ok {
		t.Fatalf("could not parse /proc/self/stat: %q", b)
	}
	// Independent extraction: the kernel's own field 22, counted from the last ')'.
	want := strings.Fields(string(b)[strings.LastIndexByte(string(b), ')')+1:])[19]
	if strconv.FormatUint(got, 10) != want {
		t.Fatalf("got %d, want %s", got, want)
	}
	if got == 0 {
		t.Fatal("start time of a live process is 0")
	}
}

// portalShims puts a fake busctl (printing `introspect`), systemctl and pacman
// on PATH so the reconciler sees exactly one portal state, with no live session.
func portalShims(t *testing.T, introspect string, busctl bool, pkgInstalled bool) {
	t.Helper()
	bin := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// PATH is wiped to this directory, so the shims may only use shell builtins:
	// a shim that shells out to cat exits 127 and reads as having no bus.
	if busctl {
		body := "#!/bin/sh\n"
		for _, line := range strings.Split(introspect, "\n") {
			if line != "" {
				body += "echo '" + line + "'\n"
			}
		}
		write("busctl", body)
	}
	// No frontend unit running, so the lifetime branch stays out of the way.
	write("systemctl", "#!/bin/sh\n[ \"$3\" = MainPID ] && echo 0\nexit 0\n")
	rc := "1"
	if pkgInstalled {
		rc = "0"
	}
	write("pacman", "#!/bin/sh\nexit "+rc+"\n")
	t.Setenv("PATH", bin)
}

// portalCaps points the seam at a compositor declaring `backend`, so the fault
// names that backend's package rather than another compositor's.
func portalCaps(t *testing.T, provider, backend string) {
	t.Helper()
	bin := t.TempDir()
	caps := "#!/bin/sh\n[ \"$1\" = caps ] && echo '{\"name\":\"" + provider +
		"\",\"portalBackend\":\"" + backend + "\",\"supports\":[],\"workspaceModel\":\"dynamic\"}'\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-"+provider), []byte(caps), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RYOKU_WM", provider)
}

// The point of probing: a frontend that publishes ScreenCast is healthy.
// Screenshot must never satisfy it -- an app asking to share a screen cannot
// use it, and the two names differ by one character.
func TestPortalSessionPassesOnlyWhenScreenCastIsPublished(t *testing.T) {
	portalShims(t, `org.freedesktop.portal.Screenshot   interface - - -
org.freedesktop.portal.ScreenCast   interface - - -
`, true, false)
	if got := reconcilePortalSession(true); got.status != recOK {
		t.Fatalf("published ScreenCast: status=%v detail=%q, want ok", got.status, got.detail)
	}

	portalShims(t, "org.freedesktop.portal.Screenshot   interface - - -\n", true, false)
	if got := reconcilePortalSession(true); got.status == recOK {
		t.Fatalf("Screenshot must not pass for ScreenCast: %q", got.detail)
	}
}

// The measured niri failure: the declared backend package is missing, so
// ScreenCast is never published and every app silently gets no picker.
func TestPortalSessionNamesTheMissingBackendPackage(t *testing.T) {
	portalShims(t, "org.freedesktop.portal.Screenshot   interface - - -\n", true, false)
	portalCaps(t, "niri", "gnome")

	got := reconcilePortalSession(true)
	if got.status != recWarn {
		t.Fatalf("missing backend: status=%v, want warn", got.status)
	}
	if !strings.Contains(got.remedy, "xdg-desktop-portal-gnome") {
		t.Fatalf("remedy %q should name the backend this compositor declares", got.remedy)
	}
	if strings.Contains(got.remedy, "hyprland") {
		t.Fatalf("remedy %q points at another compositor's portal", got.remedy)
	}
}

// Installed but publishing nothing is a different fault: telling the user to
// install the package again would send them in circles.
func TestPortalSessionDistinguishesADeadBackendFromAMissingOne(t *testing.T) {
	portalShims(t, "org.freedesktop.portal.Screenshot   interface - - -\n", true, true)
	portalCaps(t, "niri", "gnome")

	got := reconcilePortalSession(true)
	if got.status != recWarn {
		t.Fatalf("dead backend: status=%v, want warn", got.status)
	}
	if strings.Contains(got.remedy, "pacman -S") {
		t.Fatalf("remedy %q tells the user to reinstall a package that is already there", got.remedy)
	}
}

// No busctl means no bus to interrogate. That is a headless or dev shell, not a
// broken desktop, so the probe must not manufacture a finding.
func TestPortalSessionStaysQuietWithoutABus(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "systemctl"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_SESSION_ID", "")

	if got := reconcilePortalSession(false); got.status != recOK {
		t.Fatalf("no bus: status=%v detail=%q, want ok", got.status, got.detail)
	}
}
