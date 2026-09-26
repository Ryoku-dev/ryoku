package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"ryoku-cli/internal/sys"
	wm "ryoku-wm"
	"slices"
	"strings"
	"testing"
)

// materialize must never overwrite a per-machine generated seed (monitors.lua,
// gpu.lua) or a user file, so `ryoku update` can't change a user's settings
// while it refreshes the managed config the package ships.
func TestMaterializePreservesGeneratedAndUserFiles(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Package ships a managed module and seeds for the generated drop-ins
	// plus the user-owned keyboard layout and fastfetch readout.
	writeFile(t, filepath.Join(base, "hypr/hyprland.lua"), "require(\"monitors\")\n")
	writeFile(t, filepath.Join(base, "hypr/monitors.lua"), "-- seed\n")
	writeFile(t, filepath.Join(base, "hypr/gpu.lua"), "-- seed\n")
	writeFile(t, filepath.Join(base, "hypr/keyboard.lua"), "kb_layout = \"us\"\n")
	writeFile(t, filepath.Join(base, "hypr/user.lua"), "-- seed header\n")
	writeFile(t, filepath.Join(base, "fastfetch/config.jsonc"), "\"source\": \"ryoku\"\n")
	writeFile(t, filepath.Join(base, "kitty/current-theme.conf"), "background #16110b\n")

	// fresh install: every file lands, seeds included so first boot works.
	if err := Materialize(); err != nil {
		t.Fatalf("fresh materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/monitors.lua"), "-- seed")
	wantFile(t, filepath.Join(dest, "hypr/gpu.lua"), "-- seed")
	wantFile(t, filepath.Join(dest, "hypr/keyboard.lua"), "kb_layout = \"us\"")
	wantFile(t, filepath.Join(dest, "hypr/user.lua"), "seed header")
	wantFile(t, filepath.Join(dest, "fastfetch/config.jsonc"), "ryoku")
	wantFile(t, filepath.Join(dest, "kitty/current-theme.conf"), "16110b")
	// the shell's JSON stores live in ~/.config/ryoku, which no shipped file
	// creates; materialize guarantees it so the QML self-seed can write there.
	if st, err := os.Stat(filepath.Join(dest, "ryoku")); err != nil || !st.IsDir() {
		t.Errorf("materialize did not create the ryoku config dir: %v", err)
	}

	// Runtime regenerates the drop-in seeds; user adds extra keyboard layouts,
	// a user file, and customizes the fastfetch readout.
	writeFile(t, filepath.Join(dest, "hypr/monitors.lua"), "DISPLAY\n")
	writeFile(t, filepath.Join(dest, "hypr/gpu.lua"), "GPUPIN\n")
	writeFile(t, filepath.Join(dest, "hypr/keyboard.lua"), "kb_layout = \"us,ru,de,fr\"\n")
	writeFile(t, filepath.Join(dest, "hypr/user.lua"), "USER\n")
	writeFile(t, filepath.Join(dest, "fastfetch/config.jsonc"), "\"source\": \"my-custom-logo\"\n")
	writeFile(t, filepath.Join(dest, "kitty/current-theme.conf"), "background #3a5f8a\n") // matugen from the wallpaper
	// later release changes the managed module and reworks the shipped readout.
	writeFile(t, filepath.Join(base, "hypr/hyprland.lua"), "require(\"monitors_user\")\n")
	writeFile(t, filepath.Join(base, "fastfetch/config.jsonc"), "\"source\": \"ryoku-redesigned\"\n")

	// update: managed file is refreshed; the generated seeds, the user file,
	// and the customized fastfetch readout stay exactly as the machine had them.
	if err := Materialize(); err != nil {
		t.Fatalf("update materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/hyprland.lua"), "monitors_user")
	wantFile(t, filepath.Join(dest, "hypr/monitors.lua"), "DISPLAY")
	wantFile(t, filepath.Join(dest, "hypr/gpu.lua"), "GPUPIN")
	wantFile(t, filepath.Join(dest, "hypr/user.lua"), "USER")
	wantFile(t, filepath.Join(dest, "hypr/keyboard.lua"), "us,ru,de,fr")
	wantFile(t, filepath.Join(dest, "fastfetch/config.jsonc"), "my-custom-logo")
	wantFile(t, filepath.Join(dest, "kitty/current-theme.conf"), "3a5f8a")
}

// A user who symlinks a seed slot (hypr/user.lua, keyboard.lua, ...) at its live
// path into a dotfiles repo owns it. materialize must leave the symlink alone --
// even when the link dangles because the repo is not mounted yet at this point
// in boot -- instead of laying the shipped default over it. The old check
// followed the link (os.Stat), read the missing target as an empty slot, and
// clobbered the symlink with Ryoku's default.
func TestMaterializeKeepsSymlinkedSeed(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "hypr/hyprland.lua"), "pcall(require, \"user\")\n")
	writeFile(t, filepath.Join(base, "hypr/user.lua"), "-- shipped default\n")

	// The user's dotfiles symlink into ~/.config/hypr, pointing at a target that
	// only appears later, so at materialize time the link dangles.
	target := filepath.Join(t.TempDir(), "user.lua")
	link := filepath.Join(dest, "hypr/user.lua")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := Materialize(); err != nil {
		t.Fatalf("materialize over a dangling symlinked seed: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("symlinked seed vanished: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("materialize clobbered the user's symlinked user.lua with the shipped default")
	}
	if got, _ := os.Readlink(link); got != target {
		t.Fatalf("symlink now points at %q, want %q", got, target)
	}

	// Once the dotfiles repo is mounted the link resolves to the user's file.
	writeFile(t, target, "-- my binds\n")
	wantFile(t, link, "my binds")
}

// nvim seeds once like ghostty: the shipped LazyVim starting point lands on a
// fresh install, then the config is the user's. A later release must not reset
// their edits, and LazyVim's own state that lives under ~/.config/nvim (never
// shipped) must be left alone. This is the "toggles reset every update" fix.
func TestMaterializeSeedsNvimOnce(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "nvim/init.lua"), "-- ryoku defaults\n")
	writeFile(t, filepath.Join(base, "nvim/lua/config/options.lua"), "opt.wrap = false\n")
	writeFile(t, filepath.Join(base, "nvim/lua/plugins/99-ryoku-user.lua"), "return {}\n")

	if err := Materialize(); err != nil {
		t.Fatalf("fresh materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "nvim/init.lua"), "ryoku defaults")

	// user tweaks options and their plugin slot, and LazyVim writes its state.
	writeFile(t, filepath.Join(dest, "nvim/lua/config/options.lua"), "opt.wrap = true\n")
	writeFile(t, filepath.Join(dest, "nvim/lua/plugins/99-ryoku-user.lua"), "return { \"mine\" }\n")
	writeFile(t, filepath.Join(dest, "nvim/lazyvim.json"), "{\"extras\":[\"lang.go\"]}\n")
	// a later release reworks its shipped defaults.
	writeFile(t, filepath.Join(base, "nvim/init.lua"), "-- ryoku defaults v2\n")
	writeFile(t, filepath.Join(base, "nvim/lua/config/options.lua"), "opt.wrap = false\nopt.number = true\n")

	if err := Materialize(); err != nil {
		t.Fatalf("update materialize: %v", err)
	}
	// every nvim path the machine had stays exactly as the user left it.
	wantFile(t, filepath.Join(dest, "nvim/init.lua"), "ryoku defaults\n")
	wantFile(t, filepath.Join(dest, "nvim/lua/config/options.lua"), "opt.wrap = true")
	wantFile(t, filepath.Join(dest, "nvim/lua/plugins/99-ryoku-user.lua"), "mine")
	wantFile(t, filepath.Join(dest, "nvim/lazyvim.json"), "lang.go")
}

// A managed file dropped from a release is pruned; a generated seed is never
// pruned, even after the base stops shipping it.
func TestMaterializePrunesManagedNotSeeds(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "hypr/hyprland.lua"), "require(\"monitors\")\n")
	writeFile(t, filepath.Join(base, "kitty/old.conf"), "x\n")
	writeFile(t, filepath.Join(base, "hypr/monitors.lua"), "-- seed\n")
	if err := Materialize(); err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	writeFile(t, filepath.Join(dest, "hypr/monitors.lua"), "DISPLAY\n") // runtime-regenerated

	// next release drops both the managed file and the monitors seed. The managed
	// file is chosen outside any provider config dir: a compositor's tree is kept
	// whole when the box switches away from it (see the switched-away test).
	os.Remove(filepath.Join(base, "kitty/old.conf"))
	os.Remove(filepath.Join(base, "hypr/monitors.lua"))
	if err := Materialize(); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	if sys.Exists(filepath.Join(dest, "kitty/old.conf")) {
		t.Error("a managed file dropped from the release should be pruned")
	}
	wantFile(t, filepath.Join(dest, "hypr/monitors.lua"), "DISPLAY") // seed survives
}

// The default-app map used to be laid at ~/.config/mimeapps.list, which is the
// file "Set as default" writes: an update copied Ryoku's map over the user's
// picks. Ryoku ships it to the vendor layer now, and the prune must not take the
// user's file with it when it leaves the release, or the update would still
// throw the picks away.
func TestMaterializeKeepsRetiredMimeappsList(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "mimeapps.list"), "[Default Applications]\ntext/html=ryoku-nvim.desktop\n")
	if err := Materialize(); err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	// the user then makes Firefox their browser, which writes this same file.
	writeFile(t, filepath.Join(dest, "mimeapps.list"), "[Default Applications]\nx-scheme-handler/http=firefox.desktop\n")

	// the release stops shipping the map: it moved to /usr/share/applications.
	os.Remove(filepath.Join(base, "mimeapps.list"))
	if err := Materialize(); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "mimeapps.list"), "firefox.desktop")
}

// ~/.config/quickshell converges against the shipped tree even when the
// manifest never recorded a stale file (a lost state file, an old deploy.sh
// run): leftovers are swept, files outside quickshell stay manifest-ruled.
func TestMaterializeSweepsStaleQuickshell(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "quickshell/shell/shell.qml"), "NEW\n")
	// stale leftovers no manifest knows about, plus an unmanaged file outside
	// quickshell that must survive.
	writeFile(t, filepath.Join(dest, "quickshell/shell/Removed.qml"), "OLD\n")
	writeFile(t, filepath.Join(dest, "quickshell/plugins/shell.qml"), "OLD\n")
	writeFile(t, filepath.Join(dest, "hypr/user.lua"), "USER\n")

	if err := Materialize(); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "quickshell/shell/shell.qml"), "NEW")
	if sys.Exists(filepath.Join(dest, "quickshell/shell/Removed.qml")) {
		t.Error("stale quickshell file should be swept without a manifest entry")
	}
	if sys.Exists(filepath.Join(dest, "quickshell/plugins")) {
		t.Error("emptied stale quickshell dir should be pruned")
	}
	wantFile(t, filepath.Join(dest, "hypr/user.lua"), "USER")
}

// A withdrawn Ryoku drop-in is swept with no manifest entry and WirePlumber
// restarts, while the user's own drop-in in the same directory survives.
func TestMaterializeSweepsWithdrawnRyokuDropIn(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	const dir = "wireplumber/wireplumber.conf.d"
	writeFile(t, filepath.Join(base, dir, "51-ryoku-bluetooth.conf"), "SHIPPED\n")
	writeFile(t, filepath.Join(dest, dir, "50-ryoku-alsa-soft-mixer.conf"), "OLD\n")
	writeFile(t, filepath.Join(dest, dir, "99-my-own.conf"), "MINE\n")

	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "systemctl.log")
	t.Setenv("RYOKU_TEST_SYSTEMCTL_LOG", log)
	writeFile(t, filepath.Join(bin, "systemctl"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$RYOKU_TEST_SYSTEMCTL_LOG\"\n")
	if err := os.Chmod(filepath.Join(bin, "systemctl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := Materialize(); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if sys.Exists(filepath.Join(dest, dir, "50-ryoku-alsa-soft-mixer.conf")) {
		t.Error("withdrawn Ryoku drop-in should be swept without a manifest entry")
	}
	wantFile(t, filepath.Join(dest, dir, "99-my-own.conf"), "MINE")
	wantFile(t, filepath.Join(dest, dir, "51-ryoku-bluetooth.conf"), "SHIPPED")

	restarts, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("WirePlumber was not restarted after the sweep: %v", err)
	}
	if !strings.Contains(string(restarts), "--user try-restart wireplumber.service") {
		t.Errorf("sweeping a drop-in should restart WirePlumber; log: %q", restarts)
	}
}

// The user overlay lays user_edits over the freshly materialized base: a fork
// wins over the base file it shadows, a file base never shipped still lands (the
// overlay is sparse), and a base file the user did not touch stays base's, so an
// update keeps delivering fixes everywhere the user has not overridden.
func TestMaterializeUserEditsOverlay(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "hypr/modules/binds.lua"), "-- base binds v1\n")
	writeFile(t, filepath.Join(base, "hypr/modules/window_rules.lua"), "-- base rules\n")
	writeFile(t, filepath.Join(base, "hypr/user.lua"), "-- seed header\n")                    // live-owned seed
	writeFile(t, filepath.Join(base, "fastfetch/config.jsonc"), "\"source\": \"ryoku\"\n")    // hub-edited seed
	writeFile(t, filepath.Join(dest, "fastfetch/config.jsonc"), "\"source\": \"my-remix\"\n") // the hub edited it in place

	edits := sys.UserEditsDir()
	writeFile(t, filepath.Join(edits, "hypr/modules/binds.lua"), "-- my binds\n") // fork
	writeFile(t, filepath.Join(edits, "hypr/settings.lua"), "-- my settings\n")   // addition (a Hub file)
	writeFile(t, filepath.Join(edits, "hypr/user.lua"), "-- overlay junk\n")      // live-owned: must be ignored
	// the retired adopt step froze a fastfetch snapshot into the overlay; laying
	// it back was "updates keep resetting my fastfetch".
	writeFile(t, filepath.Join(edits, "fastfetch/config.jsonc"), "\"source\": \"frozen-2025\"\n")

	if err := Materialize(); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/modules/binds.lua"), "my binds")          // fork wins
	wantFile(t, filepath.Join(dest, "hypr/settings.lua"), "my settings")            // addition lands
	wantFile(t, filepath.Join(dest, "hypr/modules/window_rules.lua"), "base rules") // untouched = base
	// a live-owned file is NEVER laid from the overlay: the live seed stands, so a
	// stale overlay copy cannot wipe the user's in-place edits (if it had clobbered,
	// the file would read "overlay junk", which does not contain "seed header").
	wantFile(t, filepath.Join(dest, "hypr/user.lua"), "seed header")
	// the hub's in-place readout edit survives: the frozen overlay snapshot is
	// never laid over a live-edited seed.
	wantFile(t, filepath.Join(dest, "fastfetch/config.jsonc"), "my-remix")

	// a later base changes the forked file: the fork still wins (the user owns it).
	writeFile(t, filepath.Join(base, "hypr/modules/binds.lua"), "-- base binds v2 (a fix)\n")
	if err := Materialize(); err != nil {
		t.Fatalf("re-materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/modules/binds.lua"), "my binds")

	// dropping the fork reverts live to base on the next materialize.
	if err := os.Remove(filepath.Join(edits, "hypr/modules/binds.lua")); err != nil {
		t.Fatal(err)
	}
	if err := Materialize(); err != nil {
		t.Fatalf("revert materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/modules/binds.lua"), "base binds v2")
}

// WirePlumber reads fragments only at startup. Materialize restarts it when the
// effective Ryoku-owned Bluetooth policy changes, but leaves audio uninterrupted
// on ordinary updates that copy identical bytes.
func TestMaterializeRestartsWirePlumberOnlyWhenPolicyChanges(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	const policy = "wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf"
	writeFile(t, filepath.Join(base, policy), "wireplumber.settings = { bluetooth.autoswitch-to-headset-profile = false }\n")

	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "systemctl.log")
	t.Setenv("RYOKU_TEST_SYSTEMCTL_LOG", log)
	writeFile(t, filepath.Join(bin, "systemctl"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$RYOKU_TEST_SYSTEMCTL_LOG\"\n")
	if err := os.Chmod(filepath.Join(bin, "systemctl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := Materialize(); err != nil {
		t.Fatalf("fresh materialize: %v", err)
	}
	first, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("WirePlumber was not restarted after the policy landed: %v", err)
	}
	if got := strings.Count(string(first), "--user try-restart wireplumber.service"); got != 1 {
		t.Fatalf("fresh policy caused %d WirePlumber restarts, want 1; log: %q", got, first)
	}

	if err := Materialize(); err != nil {
		t.Fatalf("unchanged materialize: %v", err)
	}
	unchanged, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(first) {
		t.Fatalf("unchanged policy restarted WirePlumber again: before %q, after %q", first, unchanged)
	}

	writeFile(t, filepath.Join(base, policy), "# revised\nwireplumber.settings = { bluetooth.autoswitch-to-headset-profile = false }\n")
	if err := Materialize(); err != nil {
		t.Fatalf("changed materialize: %v", err)
	}
	changed, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(changed), "--user try-restart wireplumber.service"); got != 2 {
		t.Fatalf("policy revision caused %d total WirePlumber restarts, want 2; log: %q", got, changed)
	}
}

func wantFile(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(b), want) {
		t.Errorf("%s = %q, want substring %q", path, string(b), want)
	}
}

// activeFlags: the lines a chromium-flags.conf actually applies. Asserting on
// these rather than the whole file means a commented-out flag reads as absent,
// and adding a comment does not fail the build.
func activeFlags(conf string) []string {
	var flags []string
	for _, ln := range strings.Split(conf, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		flags = append(flags, ln)
	}
	return flags
}

// chromium-flags.conf is a root-level config: chromium reads
// $XDG_CONFIG_HOME/chromium-flags.conf at launch, so it is shipped under the
// base config dir and laid at ~/.config/chromium-flags.conf on install and every
// update. This pins the flags and that materialize routes it to the config root
// as a managed file, not a one-time seed. Unlike the default-app map, nothing
// else writes it, so clobbering it takes nothing from the user.
func TestMaterializeDeliversChromiumFlags(t *testing.T) {
	// Both are load-bearing for sharing a screen: gnome-libsecret so chromium
	// shares the keyring the desktop unlocks, wayland so it reaches the PipeWire
	// capturer at all.
	want := []string{"--password-store=gnome-libsecret", "--ozone-platform=wayland"}

	_, thisFile, _, _ := runtime.Caller(0)
	shipped := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "apps", "chromium-flags.conf")
	src, err := os.ReadFile(shipped)
	if err != nil {
		t.Fatalf("read shipped chromium-flags.conf: %v", err)
	}
	if got := activeFlags(string(src)); !slices.Equal(got, want) {
		t.Fatalf("shipped chromium-flags.conf applies %q, want exactly %q", got, want)
	}

	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	writeFile(t, filepath.Join(base, "chromium-flags.conf"), string(src))

	// install/update routes the base file to the config root, byte for byte.
	if err := Materialize(); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	routed := filepath.Join(dest, "chromium-flags.conf")
	if b, err := os.ReadFile(routed); err != nil || string(b) != string(src) {
		t.Fatalf("chromium-flags.conf not routed to ~/.config: got %q err %v", b, err)
	}

	// managed, not a seed: a later `ryoku update` re-lays it. A copy an older
	// deploy left (no manifest hash) is restored to the shipped flags; a copy
	// the user edited by hand is kept as a fork instead.
	os.Remove(materializeStatePath())
	writeFile(t, routed, "--password-store=basic\n")
	if err := Materialize(); err != nil {
		t.Fatalf("re-materialize: %v", err)
	}
	if b, err := os.ReadFile(routed); err != nil || string(b) != string(src) {
		t.Fatalf("update did not re-deliver chromium-flags.conf: got %q err %v", b, err)
	}
	writeFile(t, routed, string(src)+"--my-flag\n")
	if err := Materialize(); err != nil {
		t.Fatalf("re-materialize after edit: %v", err)
	}
	wantFile(t, routed, "--my-flag")
	wantFile(t, filepath.Join(sys.UserEditsDir(), "chromium-flags.conf"), "--my-flag")
}

func TestMaterializeSkipsDirectorySymlinks(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "quickshell/hub/SettingsSheet.qml"), "Item {}\n")
	target := filepath.Join(base, "quickshell/lockscreen/imports/QtGraphicalEffects/private-target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "quickshell/lockscreen/imports/QtGraphicalEffects/private")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := Materialize(); err != nil {
		t.Fatalf("materialize with a directory symlink: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "quickshell/lockscreen/imports/QtGraphicalEffects/private")); !os.IsNotExist(err) {
		t.Fatalf("directory symlink was materialized: %v", err)
	}
}

func TestMaterializeKeepsHandEditsAsForks(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	writeFile(t, filepath.Join(base, "hypr/modules/window_rules.lua"), "-- base rules v1\n")
	writeFile(t, filepath.Join(base, "hypr/modules/binds.lua"), "-- base binds v1\n")
	writeFile(t, filepath.Join(base, "quickshell/shell/shell.qml"), "// shell v1\n")
	if err := Materialize(); err != nil {
		t.Fatalf("first materialize: %v", err)
	}

	// the user edits a shipped file in place; the shell tree is Ryoku's
	writeFile(t, filepath.Join(dest, "hypr/modules/window_rules.lua"), "-- base rules v1\n-- my rule\n")
	writeFile(t, filepath.Join(dest, "quickshell/shell/shell.qml"), "// hacked\n")
	writeFile(t, filepath.Join(base, "hypr/modules/window_rules.lua"), "-- base rules v2\n")
	writeFile(t, filepath.Join(base, "hypr/modules/binds.lua"), "-- base binds v2\n")
	if err := Materialize(); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/modules/window_rules.lua"), "my rule")
	wantFile(t, filepath.Join(sys.UserEditsDir(), "hypr/modules/window_rules.lua"), "my rule")
	wantFile(t, filepath.Join(dest, "hypr/modules/binds.lua"), "base binds v2")
	wantFile(t, filepath.Join(dest, "quickshell/shell/shell.qml"), "shell v1")
	if _, err := os.Stat(filepath.Join(sys.UserEditsDir(), "quickshell/shell/shell.qml")); !os.IsNotExist(err) {
		t.Fatal("the shell tree must never be forked")
	}

	// dropping the fork takes the shipped version again
	if err := os.Remove(filepath.Join(sys.UserEditsDir(), "hypr/modules/window_rules.lua")); err != nil {
		t.Fatal(err)
	}
	if err := Materialize(); err != nil {
		t.Fatalf("third materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/modules/window_rules.lua"), "base rules v2")
}

func TestManifestWithoutHashesStillPrunes(t *testing.T) {
	state := filepath.Join(t.TempDir(), "manifest")
	os.WriteFile(state, []byte("a/b.lua\nc.conf\n"), 0o644)
	if rels := readManifest(state); len(rels) != 2 || rels[0] != "a/b.lua" {
		t.Fatalf("old manifest rels: %v", rels)
	}
	if h := readManifestHashes(state); len(h) != 0 {
		t.Fatalf("old manifest must carry no hashes, got %v", h)
	}
	if err := writeManifest(state, []string{"x"}, map[string]string{"x": "abc"}); err != nil {
		t.Fatal(err)
	}
	if h := readManifestHashes(state); h["x"] != "abc" {
		t.Fatalf("hash round trip: %v", h)
	}
}

// A compositor the machine switched away from keeps its config tree. Its variant
// package leaves the base, so the manifest prune used to delete the tree and a
// switch back booted an autogenerated config: no keybinds, no shell autostart.
func TestMaterializeKeepsTheTreeOfASwitchedAwayCompositor(t *testing.T) {
	base, dest := t.TempDir(), t.TempDir()
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("XDG_CONFIG_HOME", dest)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// A Hyprland box: the variant ships its tree, materialize lays it.
	writeFile(t, filepath.Join(base, "hypr/hyprland.lua"), "require(\"settings\")\n")
	writeFile(t, filepath.Join(base, "hypr/theme.lua"), "-- theme\n")
	writeFile(t, filepath.Join(base, "hypr/stale.lua"), "-- dropped later\n")
	if err := Materialize(); err != nil {
		t.Fatalf("hyprland materialize: %v", err)
	}
	wantFile(t, filepath.Join(dest, "hypr/hyprland.lua"), "require(\"settings\")")

	// Switched to niri: the base now ships the niri tree and nothing of hyprland.
	for _, rel := range []string{"hypr/hyprland.lua", "hypr/theme.lua", "hypr/stale.lua"} {
		if err := os.Remove(filepath.Join(base, rel)); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(base, "niri/config.kdl"), "include \"settings.kdl\"\n")
	if err := Materialize(); err != nil {
		t.Fatalf("switch materialize: %v", err)
	}

	// The niri tree lands and the hyprland tree survives whole, so switching back
	// restores the desktop instead of an autogenerated config.
	wantFile(t, filepath.Join(dest, "niri/config.kdl"), "include \"settings.kdl\"")
	wantFile(t, filepath.Join(dest, "hypr/hyprland.lua"), "require(\"settings\")")
	wantFile(t, filepath.Join(dest, "hypr/theme.lua"), "theme")

	// A file dropped *within* a tree the box still ships still prunes: the guard
	// is the switched-away compositor, not a blanket keep.
	if err := os.Remove(filepath.Join(base, "niri/config.kdl")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(base, "niri/other.kdl"), "x\n")
	if err := Materialize(); err != nil {
		t.Fatalf("prune materialize: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "niri/config.kdl")); !os.IsNotExist(err) {
		t.Fatal("a dropped file inside a shipped tree must still prune")
	}
}

// TestApplyGeneratedCompletesLaidTrees pins the switch-breaking gap: a
// compositor hard-includes the files its provider generates, the packages ship
// only the static seeds, and nothing between a hyprland-to-niri switch and the
// first login of the target used to run that target's apply. Every name here
// comes from the seam, so the test names no compositor.
func TestApplyGeneratedCompletesLaidTrees(t *testing.T) {
	providers := wm.Providers()
	if len(providers) == 0 {
		t.Skip("no providers in the seam")
	}
	primary := func(name string) []string {
		dir := wm.ConfigDir(name)
		var out []string
		for _, rel := range wm.GeneratedConfig(name) {
			if d, _, ok := strings.Cut(rel, "/"); ok && d == dir {
				out = append(out, rel)
			}
		}
		return out
	}
	// lay builds a config home: every provider's tree dir exists, the neutral
	// store exists unless withStore is false, and the generated primaries are
	// written for every provider except skipGenerated.
	lay := func(t *testing.T, withStore bool, skipGenerated string) string {
		t.Helper()
		home := t.TempDir()
		for _, name := range providers {
			dir := wm.ConfigDir(name)
			if dir == "" {
				continue
			}
			if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
				t.Fatal(err)
			}
			if name == skipGenerated {
				continue
			}
			for _, rel := range primary(name) {
				if err := os.WriteFile(filepath.Join(home, rel), []byte("// generated\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		if withStore {
			if err := os.MkdirAll(filepath.Join(home, "ryoku"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "ryoku", "desktop.json"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return home
	}
	run := func(t *testing.T, home string) []string {
		t.Helper()
		var calls []string
		old := applyProvider
		applyProvider = func(name, store string) error {
			calls = append(calls, name)
			return nil
		}
		t.Cleanup(func() { applyProvider = old })
		applyGenerated(home)
		return calls
	}

	incomplete := providers[0]
	if got := run(t, lay(t, true, incomplete)); len(got) != 1 || got[0] != incomplete {
		t.Fatalf("a laid tree without its generated files must be completed, got %v want %v", got, incomplete)
	}
	if got := run(t, lay(t, true, "")); len(got) != 0 {
		t.Fatalf("complete trees must not re-apply, got %v", got)
	}
	home := lay(t, true, incomplete)
	for _, name := range providers {
		if dir := wm.ConfigDir(name); dir != "" {
			if err := os.RemoveAll(filepath.Join(home, dir)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := run(t, home); len(got) != 0 {
		t.Fatalf("absent trees must not be created, got %v", got)
	}
	if got := run(t, lay(t, false, incomplete)); len(got) != 0 {
		t.Fatalf("no neutral store must not trigger an apply, got %v", got)
	}
}
