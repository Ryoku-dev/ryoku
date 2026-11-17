package keyboard

import (
	"os"
	"path/filepath"
	"strings"
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

// fakeBootBox points the rebuild at a temp dir of fake generators and the
// staleness check at temp files, so nothing here reaches sudo or /boot.
func fakeBootBox(t *testing.T, tools map[string]string) (dir, log string) {
	t.Helper()
	dir = t.TempDir()
	log = filepath.Join(dir, "ran.log")
	for name, body := range tools {
		script := "#!/bin/sh\nLOG=" + log + "\necho " + name + " \"$@\" >>\"$LOG\"\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	globs, vconsole, stamp := BootImageGlobs, VconsolePath, BootKeymapStamp
	dirs, asRoot, timeout := bootToolDirs, bootImageAsRoot, bootImageTimeout
	t.Cleanup(func() {
		BootImageGlobs, VconsolePath, BootKeymapStamp = globs, vconsole, stamp
		bootToolDirs, bootImageAsRoot, bootImageTimeout = dirs, asRoot, timeout
	})
	BootImageGlobs = []string{filepath.Join(dir, "*.efi")}
	VconsolePath = filepath.Join(dir, "vconsole.conf")
	BootKeymapStamp = filepath.Join(dir, "boot-keymap")
	bootToolDirs = []string{dir}
	bootImageAsRoot = func() []string { return nil }
	return dir, log
}

func readLog(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestBootImageCommandSelectsHostGenerator(t *testing.T) {
	t.Run("limine-mkinitcpio beats mkinitcpio", func(t *testing.T) {
		_, log := fakeBootBox(t, map[string]string{"limine-mkinitcpio": "", "mkinitcpio": ""})
		if err := RebuildBootImage(); err != nil {
			t.Fatal(err)
		}
		// mkinitcpio -P builds nothing on a Limine UKI box (no presets)
		if got := readLog(t, log); got != "limine-mkinitcpio" {
			t.Fatalf("ran %q, want limine-mkinitcpio alone", got)
		}
		if got := BootImageRebuildHint(); got != "sudo limine-mkinitcpio" {
			t.Errorf("hint = %q", got)
		}
	})

	t.Run("mkinitcpio by absolute path", func(t *testing.T) {
		dir, _ := fakeBootBox(t, map[string]string{"mkinitcpio": "", "dracut": ""})
		argv, err := bootImageCommand()
		if err != nil {
			t.Fatal(err)
		}
		// a bare name lets sudo's secure_path pick the /usr/local/bin wrapper
		if strings.Join(argv, " ") != filepath.Join(dir, "mkinitcpio")+" -P" {
			t.Fatalf("rebuild command = %q", argv)
		}
	})

	t.Run("dracut is the fallback", func(t *testing.T) {
		dir, _ := fakeBootBox(t, map[string]string{"dracut": ""})
		argv, err := bootImageCommand()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(argv, " ") != filepath.Join(dir, "dracut")+" --regenerate-all --force" {
			t.Fatalf("rebuild command = %q", argv)
		}
	})

	t.Run("missing generators name both tools", func(t *testing.T) {
		fakeBootBox(t, nil)
		err := RebuildBootImage()
		if err == nil || !strings.Contains(err.Error(), "mkinitcpio") || !strings.Contains(err.Error(), "dracut") {
			t.Fatalf("missing generator error = %v", err)
		}
	})
}

// The update hides child output, so a generator that asks a question on stdin
// hung it forever (#748). The rebuild must never hand over the terminal.
func TestRebuildBootImageNeverReadsTheTerminal(t *testing.T) {
	_, log := fakeBootBox(t, map[string]string{
		"limine-mkinitcpio": `if read -r answer; then echo "answered $answer" >>"$LOG"; else echo eof >>"$LOG"; fi`,
	})
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	keep := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = keep })

	if err := RebuildBootImage(); err != nil {
		t.Fatal(err)
	}
	if got := readLog(t, log); got != "limine-mkinitcpio\neof" {
		t.Fatalf("generator log %q, want it to see end of file on stdin", got)
	}
}

func TestRebuildBootImageTimesOut(t *testing.T) {
	fakeBootBox(t, map[string]string{"limine-mkinitcpio": "exec sleep 30"})
	bootImageTimeout = 200 * time.Millisecond
	start := time.Now()
	err := RebuildBootImage()
	if err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("error = %v, want a timeout", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("rebuild took %v past its deadline", took)
	}
}

func TestApplySystemDelegatesToHostKeymap(t *testing.T) {
	old := applySystemKeymap
	t.Cleanup(func() { applySystemKeymap = old })
	var layout, variant, options string
	applySystemKeymap = func(gotLayout, gotVariant, gotOptions string) int {
		layout, variant, options = gotLayout, gotVariant, gotOptions
		return 0
	}

	if err := ApplySystem(Layout{Layout: "de,us", Variant: "nodeadkeys", Options: "ctrl:nocaps"}); err != nil {
		t.Fatal(err)
	}
	if layout != "de" || variant != "nodeadkeys" || options != "ctrl:nocaps" {
		t.Fatalf("host keymap args = %q %q %q", layout, variant, options)
	}
}

func TestApplySystemReportsHostFailure(t *testing.T) {
	old := applySystemKeymap
	t.Cleanup(func() { applySystemKeymap = old })
	applySystemKeymap = func(string, string, string) int { return 1 }
	if err := ApplySystem(Layout{Layout: "fr"}); err == nil {
		t.Fatal("host keymap failure was ignored")
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

	origG, origV, origS := BootImageGlobs, VconsolePath, BootKeymapStamp
	BootImageGlobs = []string{filepath.Join(dir, "*.efi")}
	VconsolePath, BootKeymapStamp = vc, filepath.Join(dir, "boot-keymap")
	t.Cleanup(func() { BootImageGlobs, VconsolePath, BootKeymapStamp = origG, origV, origS })

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

// A reproducible rebuild leaves the image byte-identical with its old mtime, so
// a vconsole.conf rewritten with the same content (or stamped by a clock that
// ran ahead) stayed "newer" forever and every update healed it again (#748).
func TestBootStaleClearsAfterARebuildWithSameContent(t *testing.T) {
	dir, _ := fakeBootBox(t, map[string]string{"limine-mkinitcpio": ""})
	base := time.Now().Add(-time.Hour)
	touch(t, filepath.Join(dir, "ryoku_linux.efi"), base)
	if err := os.WriteFile(VconsolePath, []byte("KEYMAP=us\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale, _ := BootStale(); !stale {
		t.Fatal("vconsole.conf written after the image must read as stale before any rebuild")
	}
	if err := RebuildBootImage(); err != nil {
		t.Fatal(err)
	}
	if stale, _ := BootStale(); stale {
		t.Fatal("a successful rebuild must clear the finding even when the image keeps its mtime")
	}
	// the same layout written again moves only the mtime
	if err := os.WriteFile(VconsolePath, []byte("KEYMAP=us\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale, _ := BootStale(); stale {
		t.Error("a same-content rewrite of vconsole.conf must not read as stale")
	}
	if err := os.WriteFile(VconsolePath, []byte("KEYMAP=fr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale, _ := BootStale(); !stale {
		t.Error("a new keymap after the last rebuild must read as stale")
	}
}

func TestBootStaleIgnoresAFutureMtime(t *testing.T) {
	dir, _ := fakeBootBox(t, nil)
	touch(t, filepath.Join(dir, "ryoku_linux.efi"), time.Now().Add(-time.Hour))
	touch(t, VconsolePath, time.Now().Add(8*time.Hour))
	if stale, _ := BootStale(); stale {
		t.Error("a vconsole.conf dated in the future proves nothing about the image")
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

	og, ov, ox, ost := BootImageGlobs, VconsolePath, X11Path, BootKeymapStamp
	BootImageGlobs = []string{filepath.Join(dir, "*.efi")}
	VconsolePath, X11Path, BootKeymapStamp = vc, x11, filepath.Join(dir, "boot-keymap")
	t.Cleanup(func() { BootImageGlobs, VconsolePath, X11Path, BootKeymapStamp = og, ov, ox, ost })

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
