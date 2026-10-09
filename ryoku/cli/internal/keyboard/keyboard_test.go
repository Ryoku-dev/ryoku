package keyboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The three sources rank: an X11 layout was set deliberately, the console keymap
// is what the installer was told, and the locale is a hint of last resort. A
// French speaker on a US board is common, so a locale must never beat a keymap.
func TestDetectSourcePrecedence(t *testing.T) {
	cases := []struct {
		name                   string
		x11, console, locale   string
		wantLayout, wantSource string
	}{
		{"x11 outranks everything", "de", "fr-latin1", "fr_FR.UTF-8", "de", "the X11 keymap"},
		{"keymap outranks locale", "", "fr-latin1", "en_US.UTF-8", "fr", "the console keymap"},
		{"locale only when nothing else", "", "", "fr_FR.UTF-8", "fr", "the system locale"},
		{"a bare keymap is its own code", "", "fr", "en_US.UTF-8", "fr", "the console keymap"},
		{"charset suffix stripped", "", "fr-latin9", "en_US.UTF-8", "fr", "the console keymap"},
		{"uk keymap is the gb layout", "", "uk", "", "gb", "the console keymap"},
		{"swiss german keymap", "", "sg", "", "ch", "the console keymap"},
		{"a second x11 layout is ignored", "fr,us", "", "", "fr", "the X11 keymap"},
		{"nothing recorded says nothing", "", "", "", "", ""},
		{"an English locale stays unmapped", "", "", "en_IE.UTF-8", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.x11, c.console, c.locale)
			if got.Layout != c.wantLayout || got.Source != c.wantSource {
				t.Errorf("Detect(%q,%q,%q) = %q via %q, want %q via %q",
					c.x11, c.console, c.locale, got.Layout, got.Source, c.wantLayout, c.wantSource)
			}
		})
	}
}

// "us" is an answer, not silence: the caller tells "the box says US" from "the
// box says nothing" only by the layout being empty.
func TestDetectUsIsAnAnswerNotSilence(t *testing.T) {
	if got := Detect("", "us", "en_US.UTF-8"); got.Layout != "us" {
		t.Errorf("us keymap = %q, want us", got.Layout)
	}
}

// Drift is compared in xkb terms, so a console spelling of the same layout is
// not mistaken for a different one.
func TestConsoleAsXkb(t *testing.T) {
	cases := map[string]string{
		"uk": "gb", "fr-latin1": "fr", "sg": "ch", "br-abnt2": "br",
		"fr": "fr", "fr-latin9": "fr", "": "", "some_long_name": "",
	}
	for in, want := range cases {
		if got := ConsoleAsXkb(in); got != want {
			t.Errorf("ConsoleAsXkb(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocaleCountry(t *testing.T) {
	cases := map[string]string{
		"fr_FR.UTF-8": "FR", "de_DE@euro": "DE", "pt_BR": "BR",
		"C": "", "": "", "en_US.UTF-8": "US",
	}
	for in, want := range cases {
		if got := localeCountry(in); got != want {
			t.Errorf("localeCountry(%q) = %q, want %q", in, got, want)
		}
	}
}

// A login screen and a boot prompt have no way to switch, so a "fr,us" pair
// hands them the first.
func TestLayoutPrimary(t *testing.T) {
	if got := (Layout{Layout: "fr,us"}).Primary(); got != "fr" {
		t.Errorf("Primary() = %q, want fr", got)
	}
	if got := (Layout{}).Primary(); got != "" {
		t.Errorf("empty Primary() = %q, want empty", got)
	}
}

func TestApplySystemUsesLocalectlUnderSystemd(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "systemd")
	if err := os.Mkdir(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "calls")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >\"$CALL_LOG\"\n"
	if err := os.WriteFile(filepath.Join(bin, "localectl"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	oldRuntime, oldRC := systemdRuntimeDir, voidRCConfPath
	systemdRuntimeDir, voidRCConfPath = runtimeDir, filepath.Join(dir, "untouched-rc.conf")
	t.Cleanup(func() { systemdRuntimeDir, voidRCConfPath = oldRuntime, oldRC })
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := ApplySystem(Layout{Layout: "de", Variant: "nodeadkeys", Options: "ctrl:nocaps"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "set-x11-keymap de  nodeadkeys ctrl:nocaps\n"; string(got) != want {
		t.Fatalf("localectl args = %q, want %q", got, want)
	}
	if _, err := os.Stat(voidRCConfPath); !os.IsNotExist(err) {
		t.Fatalf("systemd branch wrote Void rc.conf: %v", err)
	}
}

func TestApplySystemWritesVoidRCConfWithoutSystemd(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, "rc.conf")
	initial := "# console settings\nKEYMAP=\"us\"\nHARDWARECLOCK=\"UTC\"\n"
	if err := os.WriteFile(rc, []byte(initial), 0o640); err != nil {
		t.Fatal(err)
	}
	oldRuntime, oldRC := systemdRuntimeDir, voidRCConfPath
	systemdRuntimeDir, voidRCConfPath = filepath.Join(dir, "no-systemd"), rc
	t.Cleanup(func() { systemdRuntimeDir, voidRCConfPath = oldRuntime, oldRC })

	if err := ApplySystem(Layout{Layout: "fr"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	want := "# console settings\nKEYMAP=\"fr\"\nHARDWARECLOCK=\"UTC\"\n"
	if string(got) != want {
		t.Fatalf("rc.conf = %q, want %q", got, want)
	}
}

func TestWriteVoidKeymapAppendsAndRejectsInvalidValues(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "rc.conf")
	if err := os.WriteFile(rc, []byte("HARDWARECLOCK=\"UTC\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeVoidKeymap(rc, "gb"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "HARDWARECLOCK=\"UTC\"\nKEYMAP=\"gb\"\n"; string(got) != want {
		t.Fatalf("appended rc.conf = %q, want %q", got, want)
	}
	if err := writeVoidKeymap(rc, "us\nMALICIOUS=yes"); err == nil {
		t.Fatal("invalid keymap was accepted")
	}
}

// `keyboard apply` escalates through pkexec from the Hub (no tty for sudo, so the
// boot-image rebuild used to fail and the apply reported FAILED, #177). The
// re-exec hands the fully-resolved layout across with --resolved so the root
// pass reads no per-user config; the flag must round-trip the layout, variant,
// and options exactly, and malformed input must error rather than silently drop.
func TestParseApplyArgs(t *testing.T) {
	t.Run("resolved carries the full layout across the privilege boundary", func(t *testing.T) {
		want, noBoot, resolved, err := parseApplyArgs([]string{"--resolved", "fr", "azerty", "ctrl:nocaps", "--no-boot"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want != "" {
			t.Errorf("want layout = %q, expected empty when resolved", want)
		}
		if !noBoot {
			t.Error("--no-boot not parsed")
		}
		if resolved == nil || *resolved != (Layout{Layout: "fr", Variant: "azerty", Options: "ctrl:nocaps"}) {
			t.Fatalf("resolved = %+v, want {fr azerty ctrl:nocaps}", resolved)
		}
	})
	t.Run("a bare layout is the desktop override, not resolved", func(t *testing.T) {
		want, _, resolved, err := parseApplyArgs([]string{"us"})
		if err != nil || want != "us" || resolved != nil {
			t.Fatalf("parseApplyArgs(us) = (%q, %v, %v)", want, resolved, err)
		}
	})
	for _, bad := range [][]string{
		{"--resolved", "us", ""}, // missing the options field
		{"--bogus"},              // unknown flag
		{"us", "de"},             // two layouts
	} {
		if _, _, _, err := parseApplyArgs(bad); err == nil {
			t.Errorf("parseApplyArgs(%q) accepted malformed input", bad)
		}
	}
}

func touch(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

// A fallback image is regenerated on its own schedule, so it must never be the
// image freshness is judged against.
func TestBootImageTimeIgnoresFallback(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-2 * time.Hour)
	touch(t, filepath.Join(dir, "ryoku_linux.efi"), base)
	touch(t, filepath.Join(dir, "ryoku_linux-cachyos.efi"), base.Add(30*time.Minute))
	touch(t, filepath.Join(dir, "initramfs-linux-fallback.img"), base.Add(90*time.Minute))

	orig := BootImageGlobs
	BootImageGlobs = []string{filepath.Join(dir, "*.efi"), filepath.Join(dir, "initramfs-*.img")}
	t.Cleanup(func() { BootImageGlobs = orig })

	when, which := BootImageTime()
	if filepath.Base(which) != "ryoku_linux-cachyos.efi" {
		t.Errorf("picked %q, want the newest non-fallback image", filepath.Base(which))
	}
	if !when.Equal(base.Add(30 * time.Minute)) {
		t.Errorf("time = %v, want the newest non-fallback mtime", when)
	}
}

// The trap this whole package exists for: every file under /etc can agree while
// the passphrase prompt is still wrong, because the image carries its own copy.
func TestBootStaleWhenVconsoleIsNewer(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	img := filepath.Join(dir, "ryoku_linux.efi")
	vc := filepath.Join(dir, "vconsole.conf")
	touch(t, img, base)
	touch(t, vc, base.Add(10*time.Minute))

	origG, origV := BootImageGlobs, VconsolePath
	BootImageGlobs = []string{filepath.Join(dir, "*.efi")}
	VconsolePath = vc
	t.Cleanup(func() { BootImageGlobs, VconsolePath = origG, origV })

	stale, got := BootStale()
	if !stale {
		t.Error("a boot image older than vconsole.conf must read as stale")
	}
	if filepath.Base(got) != "ryoku_linux.efi" {
		t.Errorf("image = %q", got)
	}

	// and the other way: rebuilt after the keymap change, so the prompt is current
	touch(t, img, base.Add(20*time.Minute))
	if stale, _ := BootStale(); stale {
		t.Error("a boot image newer than vconsole.conf must not read as stale")
	}
}

// A French box, end to end through Read(): the greeter is unset (nothing has
// pushed to it yet), the console says fr, and the image predates the change.
// This is the shape of the bug users hit and cannot see.
func TestReadReportsDriftOnAFrenchBox(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	img := filepath.Join(dir, "ryoku_linux.efi")
	vc := filepath.Join(dir, "vconsole.conf")
	x11 := filepath.Join(dir, "00-keyboard.conf")
	touch(t, img, base)
	if err := os.WriteFile(vc, []byte("KEYMAP=fr-latin1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(vc, base.Add(10*time.Minute), base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(x11, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	og, ov, ox := BootImageGlobs, VconsolePath, X11Path
	BootImageGlobs = []string{filepath.Join(dir, "*.efi")}
	VconsolePath, X11Path = vc, x11
	t.Cleanup(func() { BootImageGlobs, VconsolePath, X11Path = og, ov, ox })

	if got := ConsoleKeymap(); got != "fr-latin1" {
		t.Fatalf("console = %q, want fr-latin1", got)
	}
	if got := ConsoleAsXkb(ConsoleKeymap()); got != "fr" {
		t.Errorf("console as layout = %q, want fr", got)
	}
	// the console spells it differently from the layout, and that must not
	// read as drift once normalised
	if got := Detect(X11Layout(), ConsoleKeymap(), ""); got.Layout != "fr" {
		t.Errorf("detected %q, want fr", got.Layout)
	}
	stale, _ := BootStale()
	if !stale {
		t.Error("the boot image predates the keymap change and must read as stale")
	}
}
