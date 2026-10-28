package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkSkin(t *testing.T, dir, slug string, withPreview bool) {
	t.Helper()
	d := filepath.Join(dir, slug)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "Main.qml"), []byte("Rectangle{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withPreview {
		if err := os.WriteFile(filepath.Join(d, "preview.gif"), []byte("GIF89a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func useGreeterThemeInstaller(t *testing.T) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "lockscreen", "qylock", "sddm-host", "install-greeter-theme"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_GREETER_THEME_INSTALLER", path)
}

func findSkin(skins []LockSkin, slug string) *LockSkin {
	for i := range skins {
		if skins[i].Slug == slug {
			return &skins[i]
		}
	}
	return nil
}

func TestListLockSkinsIn(t *testing.T) {
	dir := t.TempDir()
	mkSkin(t, dir, "clockwork/orbital", true)
	mkSkin(t, dir, "clockwork/tape", false)
	// folder without Main.qml: not a skin, must be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "clockwork", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}

	resp := listLockSkinsIn(dir, "clockwork/tape", "")

	if resp.Active != "clockwork/tape" {
		t.Fatalf("active = %q, want clockwork/tape", resp.Active)
	}
	if len(resp.Skins) != 2 {
		t.Fatalf("got %d skins, want 2: %+v", len(resp.Skins), resp.Skins)
	}

	orb := findSkin(resp.Skins, "clockwork/orbital")
	tape := findSkin(resp.Skins, "clockwork/tape")
	if orb == nil || tape == nil {
		t.Fatalf("missing a skin: %+v", resp.Skins)
	}
	if orb.Name != "Orbital" || orb.Theme != "Clockwork" {
		t.Errorf("curated metadata not applied: %+v", orb)
	}
	if orb.Preview == "" {
		t.Errorf("orbital should report its preview.gif path")
	}
	if tape.Preview != "" {
		t.Errorf("tape has no preview.gif but reported %q", tape.Preview)
	}
	if orb.Active {
		t.Errorf("orbital marked active, but tape is selected")
	}
	if !tape.Active {
		t.Errorf("tape should be the active skin")
	}
}

func TestListLockSkinsInDerivesUncurated(t *testing.T) {
	dir := t.TempDir()
	// single-level theme, not in the curated map: name comes from the folder,
	// summary from metadata.desktop.
	mkSkin(t, dir, "nier-automata", false)
	desk := "[SddmGreeterTheme]\nName=nier\nDescription=A lonely android keeps the time\n"
	if err := os.WriteFile(filepath.Join(dir, "nier-automata", "metadata.desktop"), []byte(desk), 0o644); err != nil {
		t.Fatal(err)
	}

	resp := listLockSkinsIn(dir, "", "")
	s := findSkin(resp.Skins, "nier-automata")
	if s == nil {
		t.Fatalf("single-level skin not found: %+v", resp.Skins)
	}
	if s.Name != "Nier Automata" {
		t.Errorf("derived name = %q, want Nier Automata", s.Name)
	}
	if s.Summary != "A lonely android keeps the time" {
		t.Errorf("summary not read from metadata.desktop: %q", s.Summary)
	}
}

func TestSetLockSkinIn(t *testing.T) {
	dir := t.TempDir()
	mkSkin(t, dir, "clockwork/orbital", false)
	pref := filepath.Join(t.TempDir(), "qylock", "theme")

	if err := setLockSkinIn(dir, pref, "clockwork/orbital"); err != nil {
		t.Fatalf("set valid skin: %v", err)
	}
	if got := readLockPref(pref); got != "clockwork/orbital" {
		t.Fatalf("pref = %q, want clockwork/orbital", got)
	}

	// unknown slug -> reject. a bad value must never land in the pref.
	if err := setLockSkinIn(dir, pref, "ghost/none"); err == nil {
		t.Fatalf("setting an unknown skin should error")
	}
	if got := readLockPref(pref); got != "clockwork/orbital" {
		t.Fatalf("pref changed after a rejected set: %q", got)
	}
}

func TestInstallGreeter(t *testing.T) {
	useGreeterThemeInstaller(t)
	src := t.TempDir()
	mkSkin(t, src, "material-you", false)
	themes := t.TempDir()
	conf := filepath.Join(t.TempDir(), "sddm.conf.d", "99-ryoku.conf")

	if err := installGreeter(src, themes, conf, "material-you", false); err != nil {
		t.Fatalf("install greeter: %v", err)
	}
	if !fileExists(filepath.Join(themes, greeterTheme, "Main.qml")) {
		t.Fatalf("greeter theme %q not installed under %s", greeterTheme, themes)
	}
	if !fileExists(filepath.Join(themes, greeterTheme, "theme", "Main.qml")) {
		t.Fatalf("selected skin not installed beneath the greeter host")
	}
	themeConfig, err := os.ReadFile(filepath.Join(themes, greeterTheme, "theme.conf"))
	if err != nil {
		t.Fatalf("read greeter theme config: %v", err)
	}
	if !strings.Contains(string(themeConfig), "clock24h=false") {
		t.Fatalf("greeter theme config did not receive 12-hour preference: %q", themeConfig)
	}
	b, err := os.ReadFile(conf)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	if !strings.Contains(string(b), "Current="+greeterTheme) {
		t.Fatalf("conf does not select the greeter theme: %q", b)
	}

	// second skin overwrites the same fixed greeter dir; nothing orphans.
	mkSkin(t, src, "clockwork/orbital", false)
	if err := installGreeter(src, themes, conf, "clockwork/orbital", true); err != nil {
		t.Fatalf("reinstall greeter: %v", err)
	}
	if !fileExists(filepath.Join(themes, greeterTheme, "Main.qml")) {
		t.Fatalf("greeter theme missing after switch")
	}
	themeConfig, err = os.ReadFile(filepath.Join(themes, greeterTheme, "theme.conf"))
	if err != nil {
		t.Fatalf("read reinstalled greeter theme config: %v", err)
	}
	if !strings.Contains(string(themeConfig), "clock24h=true") {
		t.Fatalf("greeter theme config did not receive 24-hour preference: %q", themeConfig)
	}

	// unknown skin -> error before touching anything privileged.
	if err := installGreeter(src, themes, conf, "ghost/none", false); err == nil {
		t.Fatalf("installing an unknown skin should error")
	}
}

func TestValidSlug(t *testing.T) {
	for _, ok := range []string{"clockwork/orbital", "material-you", "pixel-coffee"} {
		if err := validSlug(ok); err != nil {
			t.Errorf("validSlug(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "/etc/passwd", "../../etc", "a/../b"} {
		if err := validSlug(bad); err == nil {
			t.Errorf("validSlug(%q) should error", bad)
		}
	}
}

// A skin pulled from the catalogue lands 0700 user-owned (os.MkdirTemp), but the
// greeter runs as the unprivileged `sddm` user: installGreeter must widen the
// copy to world-readable, else SDDM can't read the theme and silently falls back
// to its embedded one on every boot.
func TestInstallGreeterMakesThemeWorldReadable(t *testing.T) {
	useGreeterThemeInstaller(t)
	src := t.TempDir()
	mkSkin(t, src, "video/tape", false)
	// mimic a catalogue download: owner-only perms on the skin tree.
	skin := filepath.Join(src, "video", "tape")
	if err := os.Chmod(filepath.Join(skin, "Main.qml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(skin, 0o700); err != nil {
		t.Fatal(err)
	}
	themes := t.TempDir()
	conf := filepath.Join(t.TempDir(), "sddm.conf.d", "99-ryoku.conf")

	if err := installGreeter(src, themes, conf, "video/tape", false); err != nil {
		t.Fatalf("install greeter: %v", err)
	}

	dir := filepath.Join(themes, greeterTheme)
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm()&0o005 != 0o005 {
		t.Errorf("greeter dir mode = %o, want world read+execute (o+rx)", di.Mode().Perm())
	}
	mi, err := os.Stat(filepath.Join(dir, "Main.qml"))
	if err != nil {
		t.Fatal(err)
	}
	if mi.Mode().Perm()&0o004 == 0 {
		t.Errorf("Main.qml mode = %o, want world readable (o+r)", mi.Mode().Perm())
	}
}

func TestRunLockRejectsMovedStoreCommands(t *testing.T) {
	for _, command := range []string{"catalog", "install", "cache"} {
		if err := runLock([]string{command}); err == nil {
			t.Errorf("lock %s remained on the Hub command surface", command)
		}
	}
}

// writeLockReceipt drops a minimal RyoStore lockscreen receipt: the fields
// receiptLockSlug reads (destination + file destinations), matching the on-disk
// shape ryostore writes.
func writeLockReceipt(t *testing.T, receiptsDir, id, destination string, fileDests ...string) {
	t.Helper()
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := make([]string, len(fileDests))
	for i, d := range fileDests {
		files[i] = fmt.Sprintf(`{"source":%q,"destination":%q}`, d, d)
	}
	body := fmt.Sprintf(`{"category":"lockscreens","id":%q,"version":"1.0.0","destination":%q,"files":[%s]}`,
		id, destination, strings.Join(files, ","))
	if err := os.WriteFile(filepath.Join(receiptsDir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A skin installed as a symlink to a directory (a stow/dotfile setup, a manual
// link) must still list: os.ReadDir reports the symlink with IsDir()==false, so
// the pre-fix scan dropped it and the skin vanished from Settings though its
// files were on disk.
func TestScanLockSlugsFollowsSymlinkedThemeDir(t *testing.T) {
	dir := t.TempDir()
	store := t.TempDir()

	// top-level symlinked theme: store/moon -> linked as themes/moon
	moon := filepath.Join(store, "moon")
	if err := os.MkdirAll(moon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moon, "Main.qml"), []byte("Rectangle{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moon, filepath.Join(dir, "moon")); err != nil {
		t.Fatal(err)
	}

	// level-two symlinked variant: real family dir, symlinked variant inside it.
	if err := os.MkdirAll(filepath.Join(dir, "clockwork"), 0o755); err != nil {
		t.Fatal(err)
	}
	tape := filepath.Join(store, "tape")
	if err := os.MkdirAll(tape, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tape, "Main.qml"), []byte("Rectangle{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(tape, filepath.Join(dir, "clockwork", "tape")); err != nil {
		t.Fatal(err)
	}

	slugs, err := scanLockSlugs(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	got := map[string]bool{}
	for _, s := range slugs {
		got[s] = true
	}
	if !got["moon"] {
		t.Errorf("symlinked top-level theme not listed: %v", slugs)
	}
	if !got["clockwork/tape"] {
		t.Errorf("symlinked variant not listed: %v", slugs)
	}
}

// A missing themes dir is "nothing installed", not a failure: no error, no skins.
func TestScanLockSlugsMissingDir(t *testing.T) {
	slugs, err := scanLockSlugs(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(slugs) != 0 {
		t.Fatalf("missing dir should yield no slugs, got %v", slugs)
	}
}

// An unreadable themes dir is a listing failure: listLockSkinsIn must report it
// via Error so the page can say "couldn't read the list" instead of "nothing
// installed". Root bypasses permission bits, so skip there.
func TestListLockSkinsInReportsReadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "themes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	resp := listLockSkinsIn(dir, "", "")
	if resp.Error == "" {
		t.Fatalf("unreadable themes dir should report an error, got %+v", resp)
	}
	if len(resp.Skins) != 0 {
		t.Fatalf("failed read should yield no skins, got %+v", resp.Skins)
	}
}

// The receipt source lists a store product the folder scan misses (here nested
// deeper than the two-level walk) as long as its Main.qml is on disk.
func TestListLockSkinsInUnionsReceiptScanMissed(t *testing.T) {
	dir := t.TempDir()
	receipts := t.TempDir()
	// Main.qml three levels down: scan walks only theme + theme/variant.
	mkSkin(t, dir, "pack/inner/deep", false)
	writeLockReceipt(t, receipts, "pack", "qylock/themes/pack", "inner/deep/Main.qml")

	// Without receipts the scan cannot reach it.
	if s := findSkin(listLockSkinsIn(dir, "", "").Skins, "pack/inner/deep"); s != nil {
		t.Fatalf("scan should not reach a three-level skin: %+v", s)
	}
	// With the receipt it lists.
	resp := listLockSkinsIn(dir, "", receipts)
	if s := findSkin(resp.Skins, "pack/inner/deep"); s == nil {
		t.Fatalf("receipt-owned skin not unioned in: %+v", resp.Skins)
	}
}

// A receipt for a skin the scan already found must not duplicate it, and a
// receipt whose files are gone from disk must not be listed at all.
func TestListLockSkinsInReceiptDedupesAndSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	receipts := t.TempDir()
	mkSkin(t, dir, "clockwork-tape", false)
	writeLockReceipt(t, receipts, "clockwork-tape", "qylock/themes/clockwork-tape", "Main.qml")
	// ghost: receipt present, files removed -> not listed.
	writeLockReceipt(t, receipts, "ghost", "qylock/themes/ghost", "Main.qml")

	resp := listLockSkinsIn(dir, "", receipts)
	if len(resp.Skins) != 1 {
		t.Fatalf("want exactly one skin, got %d: %+v", len(resp.Skins), resp.Skins)
	}
	if resp.Skins[0].Slug != "clockwork-tape" {
		t.Fatalf("slug = %q, want clockwork-tape", resp.Skins[0].Slug)
	}
}

func TestReceiptLockSlug(t *testing.T) {
	receipts := t.TempDir()
	cases := []struct {
		id, destination, mainDest, want string
	}{
		{"tape", "qylock/themes/clockwork-tape", "Main.qml", "clockwork-tape"},
		{"wrapped", "qylock/themes/wrapped", "content/Main.qml", "wrapped/content"},
		{"foreign", "ryoku/rices/foo", "Main.qml", ""},
	}
	for _, c := range cases {
		writeLockReceipt(t, receipts, c.id, c.destination, c.mainDest)
		if got := receiptLockSlug(filepath.Join(receipts, c.id+".json")); got != c.want {
			t.Errorf("receiptLockSlug(%s) = %q, want %q", c.id, got, c.want)
		}
	}
}
