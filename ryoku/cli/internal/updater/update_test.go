package updater

import (
	"encoding/json"
	"errors"
	"github.com/godbus/dbus/v5"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	wm "ryoku-wm"
	"strings"
	"testing"
	"time"
)

// wantedSnapperHelpers gates the offer. no btrfs+snapper -> nothing,
// limine-snapper-sync only on Limine.
func TestWantedSnapperHelpers(t *testing.T) {
	ready := snapHelpers{rootBtrfs: true, snapper: true}

	both := ready
	both.limine = true
	if got := wantedSnapperHelpers(both); len(got) != 2 || got[0] != "snap-pac" || got[1] != "limine-snapper-sync" {
		t.Fatalf("both missing + limine: got %v, want [snap-pac limine-snapper-sync]", got)
	}
	if got := wantedSnapperHelpers(ready); len(got) != 1 || got[0] != "snap-pac" {
		t.Fatalf("no limine: got %v, want [snap-pac]", got)
	}

	hasSnapPac := both
	hasSnapPac.snapPac = true
	if got := wantedSnapperHelpers(hasSnapPac); len(got) != 1 || got[0] != "limine-snapper-sync" {
		t.Fatalf("snap-pac present: got %v, want [limine-snapper-sync]", got)
	}

	allPresent := hasSnapPac
	allPresent.limineSync = true
	if got := wantedSnapperHelpers(allPresent); got != nil {
		t.Fatalf("all present: got %v, want nil", got)
	}

	if got := wantedSnapperHelpers(snapHelpers{snapper: true}); got != nil {
		t.Fatalf("non-btrfs root must offer nothing, got %v", got)
	}
	if got := wantedSnapperHelpers(snapHelpers{rootBtrfs: true}); got != nil {
		t.Fatalf("snapper absent must offer nothing (a separate doctor warn), got %v", got)
	}
}

// publishPrompt/awaitAnswer = the Hub consent back-channel. publish clears
// stale answers, run-state carries the prompt, awaitAnswer reads + consumes.
func TestPromptAnswerRoundTrip(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	// stale answer from a previous prompt must not satisfy this one.
	if err := os.WriteFile(answerPath(), []byte("Install"), 0o644); err != nil {
		t.Fatal(err)
	}
	publishPrompt("snapper-helpers", "Enable snapshot helpers?", "detail", []string{"Install", "Skip"})
	if _, err := os.Stat(answerPath()); !os.IsNotExist(err) {
		t.Fatal("publishPrompt must clear a stale answer")
	}

	b, err := os.ReadFile(runStatePath())
	if err != nil || !strings.Contains(string(b), `"phase":"prompt"`) || !strings.Contains(string(b), "snapper-helpers") {
		t.Fatalf("run-state missing the prompt: %s (err %v)", b, err)
	}

	// no answer in the window = decline.
	if choice, ok := awaitAnswer(150 * time.Millisecond); ok {
		t.Fatalf("awaitAnswer should time out with no answer, got %q", choice)
	}

	// written answer: read, then consume.
	if err := os.WriteFile(answerPath(), []byte("Install\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if choice, ok := awaitAnswer(2 * time.Second); !ok || choice != "Install" {
		t.Fatalf("awaitAnswer = %q, %v; want Install, true", choice, ok)
	}
	if _, err := os.Stat(answerPath()); !os.IsNotExist(err) {
		t.Fatal("awaitAnswer must consume the answer file")
	}
}

// An up-to-date packaged box (installed == latest) reports nothing pending but
// lists the recent history the installed commit contains, so the Hub's Updates
// page still has content. packagedStatus is the pure core of buildStatus, so
// this exercises the sha branching and the recent lookup without pacman.
func TestPackagedStatusUpToDatePopulatesRecent(t *testing.T) {
	srv, _ := stubRecent(t, [][2]string{
		{"latest work", "86a91f4aaaa"},
		{"earlier work", "1184abcdddd"},
	})
	t.Setenv("RYOKU_GITHUB_API", srv.URL)
	t.Setenv("RYOKU_REPO_SLUG", "owner/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	ver := "0.12.6.r1184.g86a91f4-1" // installed == latest: shortCommit -> 86a91f4
	r := packagedStatus(ver, ver)

	if r.Behind != 0 {
		t.Errorf("pendingUpdates = %d, want 0 when up to date", r.Behind)
	}
	if r.Available {
		t.Error("available = true, want false when up to date")
	}
	if len(r.Updates) != 0 {
		t.Errorf("updates = %d, want 0 (nothing incoming)", len(r.Updates))
	}
	if len(r.Recent) != 2 {
		t.Fatalf("recent = %d, want 2 (from the stub)", len(r.Recent))
	}
	if r.Recent[0].Name != "latest work" || r.Recent[0].New != "86a91f4" {
		t.Errorf("recent[0] = %+v, want {latest work, 86a91f4}", r.Recent[0])
	}
	// The Hub reads these JSON keys off `ryoku status --json`; pin them.
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if js := string(b); !strings.Contains(js, `"pendingUpdates":0`) || !strings.Contains(js, `"recent":[{`) {
		t.Errorf("status JSON = %s, want pendingUpdates:0 and a non-empty recent[]", js)
	}
}

// Offline / rate-limited: the recent lookup fails, so the up-to-date report
// degrades to an empty (non-nil) recent list with no error or hang, keeping the
// JSON shape stable.
func TestPackagedStatusUpToDateOfflineEmptyRecent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_GITHUB_API", srv.URL)
	t.Setenv("RYOKU_REPO_SLUG", "owner/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	ver := "0.12.6.r1184.g86a91f4-1"
	r := packagedStatus(ver, ver)

	if r.Behind != 0 {
		t.Errorf("pendingUpdates = %d, want 0 when up to date", r.Behind)
	}
	if r.Recent == nil {
		t.Error("recent = nil, want a non-nil empty slice so the JSON stays stable")
	}
	if len(r.Recent) != 0 {
		t.Errorf("recent = %d, want 0 on a failed lookup", len(r.Recent))
	}
}

// The Ryoku lane must never be a sysupgrade: the kernel and the base come from
// the distribution the box was installed from, on the user's schedule. It must
// also --overwrite the Ryoku system paths the ISO installer and deploy.sh seed
// unowned (ryoku-dns / ryoku-wifi-powersave + their polkit rules, the Plymouth
// splash theme, the initcpio hook, the logind lid drop-in). Once a package owns
// one of those paths a
// file conflict otherwise aborts the whole transaction and blocks every user
// update, so pin them here.
func TestRyokuInstallArgsStayInTheRyokuLane(t *testing.T) {
	set := []string{"ryoku/ryoku-desktop", "ryoku/ryogami"}
	args := ryokuInstallArgs(set)
	joined := strings.Join(args, " ")
	for _, want := range []string{"pacman -S", "--needed", "--noconfirm", "--overwrite",
		"RYOKU_MANAGED_UPDATE=1", "ryoku/ryoku-desktop", "ryoku/ryogami"} {
		if !strings.Contains(joined, want) {
			t.Errorf("ryokuInstallArgs missing %q: %v", want, args)
		}
	}
	// -Su/-Syu here would upgrade the whole system, kernel included, which is
	// exactly what this lane exists not to do.
	for _, banned := range []string{"-Su", "-Syu", "-Syyu", "-u"} {
		for _, a := range args {
			if a == banned {
				t.Errorf("ryokuInstallArgs runs %q: that is a system upgrade, not the Ryoku set", banned)
			}
		}
	}
	// The database refresh is its own step, and a channel move forces it: pacman
	// skips a db that is not newer than its cache, and a frozen release is older
	// than the channel the box just left.
	if got := strings.Join(refreshDBArgs(false), " "); !strings.Contains(got, "pacman -Sy") {
		t.Errorf("refreshDBArgs = %q, want a -Sy refresh", got)
	}
	if got := strings.Join(refreshDBArgs(true), " "); !strings.Contains(got, "pacman -Syy") {
		t.Errorf("refreshDBArgs(force) = %q, want -Syy so a frozen release's db is refetched", got)
	}

	var glob string
	for i, a := range args {
		if a == "--overwrite" && i+1 < len(args) {
			glob = args[i+1]
		}
	}
	if glob == "" {
		t.Fatalf("no --overwrite glob in %v", args)
	}
	for _, p := range []string{
		"/usr/bin/ryoku-dns",
		"/usr/bin/ryoku-wifi-powersave",
		"/usr/share/polkit-1/rules.d/50-ryoku-dns.rules",
		"/usr/share/polkit-1/rules.d/49-ryoku-wifi-powersave.rules",
		"/usr/share/plymouth/themes/ryoku/bullet.png",
		"/usr/share/plymouth/themes/ryoku/logo.png",
		"/usr/lib/systemd/system/ryoku-network-kill-guard.service",
		"/usr/lib/initcpio/install/ryoku-gpu-trim",
		"/usr/share/ryoku/boot/default.conf",
		"/etc/systemd/logind.conf.d/10-ryoku-lid.conf",
	} {
		covered := false
		for _, g := range strings.Split(glob, ",") {
			if ok, _ := filepath.Match(g, p); ok {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("--overwrite %q does not cover deploy.sh-seeded path %q", glob, p)
		}
	}

	// The opt-in lane is the only place a sysupgrade may appear.
	if got := strings.Join(systemUpgradeArgs(), " "); !strings.Contains(got, "pacman -Syu") {
		t.Errorf("systemUpgradeArgs = %q, want the full -Syu it exists for", got)
	} else if !strings.Contains(got, "RYOKU_MANAGED_UPDATE=1") {
		t.Errorf("systemUpgradeArgs = %q, want first-rollout scheduling owned by stage2", got)
	}
}

// prowlDecide is the pure core of prowlRefresh: a dev install (on PATH, not
// pacman-owned) self-updates; a pacman-owned copy is left to `pacman -Syu`; an
// absent binary is a no-op. Pinned so the dev-vs-packaged branch cannot regress.
func TestProwlDecide(t *testing.T) {
	cases := []struct {
		name        string
		onPath      bool
		pacmanOwned bool
		want        prowlAction
	}{
		{"absent does nothing", false, false, prowlNoop},
		{"packaged is left to pacman", true, true, prowlManaged},
		{"dev install self-updates", true, false, prowlSelfUpdate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := prowlDecide(c.onPath, c.pacmanOwned); got != c.want {
				t.Fatalf("prowlDecide(onPath=%v, owned=%v) = %v, want %v", c.onPath, c.pacmanOwned, got, c.want)
			}
		})
	}
}

func TestLogin1GraphicalUserProperties(t *testing.T) {
	properties := func(sessionType, sessionClass, desktop string) map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"Type":    dbus.MakeVariant(sessionType),
			"Class":   dbus.MakeVariant(sessionClass),
			"Desktop": dbus.MakeVariant(desktop),
		}
	}
	for _, tc := range []struct {
		name       string
		properties map[string]dbus.Variant
		want       bool
	}{
		{"wayland Hyprland user", properties("wayland", "user", "Hyprland"), true},
		{"x11 niri early user", properties("x11", "user-early", "niri"), false},
		{"tty user", properties("tty", "user", "Hyprland"), false},
		{"wayland greeter", properties("wayland", "greeter", "Hyprland"), false},
		{"wayland lock screen", properties("wayland", "lock-screen", "Hyprland"), false},
		{"other desktop", properties("wayland", "user", "GNOME"), false},
		{"missing class", map[string]dbus.Variant{
			"Type": dbus.MakeVariant("wayland"), "Desktop": dbus.MakeVariant("Hyprland"),
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := login1GraphicalUserProperties(tc.properties); got != tc.want {
				t.Fatalf("login1GraphicalUserProperties() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasUpdateSleepGuardRequiresOwnedBlock(t *testing.T) {
	const uid = 1000
	for _, tc := range []struct {
		name       string
		inhibitors []login1Inhibitor
		want       bool
	}{
		{"owned sleep block", []login1Inhibitor{{
			What: "sleep:shutdown", Who: "ryoku-session-cutover", Mode: "block", UID: uid,
		}}, true},
		{"wrong user", []login1Inhibitor{{
			What: "sleep", Who: "ryoku-session-cutover", Mode: "block", UID: 1001,
		}}, false},
		{"delay is not a block", []login1Inhibitor{{
			What: "sleep", Who: "ryoku-session-cutover", Mode: "delay", UID: uid,
		}}, false},
		{"lid-only block", []login1Inhibitor{{
			What: "handle-lid-switch", Who: "ryoku-session-cutover", Mode: "block", UID: uid,
		}}, false},
		{"foreign owner", []login1Inhibitor{{
			What: "sleep", Who: "other", Mode: "block", UID: uid,
		}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasUpdateSleepGuard(tc.inhibitors, uid); got != tc.want {
				t.Fatalf("hasUpdateSleepGuard() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPackagePowerCutoverRetriesIncompleteAdoption(t *testing.T) {
	for _, tc := range []struct {
		name            string
		markerPresent   bool
		rootGuardActive bool
		want            bool
	}{
		{"first rollout", false, false, true},
		{"completed hook", true, false, false},
		{"stale marker with emergency guard", true, true, true},
		{"failed unmarked adoption", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsPackagePowerCutover(tc.markerPresent, tc.rootGuardActive); got != tc.want {
				t.Fatalf("needsPackagePowerCutover(%v, %v) = %v, want %v",
					tc.markerPresent, tc.rootGuardActive, got, tc.want)
			}
		})
	}
}

func TestConfigReloadResultAcceptsProviderWithoutReload(t *testing.T) {
	if err := configReloadResult(wm.ErrUnsupported); err != nil {
		t.Fatalf("unsupported reload should defer to provider file watching: %v", err)
	}
	providerErr := errors.New("provider unavailable")
	if err := configReloadResult(providerErr); !errors.Is(err, providerErr) {
		t.Fatalf("live provider error was suppressed: %v", err)
	}
}

func TestDbRejection(t *testing.T) {
	cases := map[string]bool{
		"error: failed to commit transaction (conflicting files)": false,
		"could not satisfy dependencies":                          false,
		"target not found: ryoku-desktop":                         false,
		"ryoku.db: invalid or corrupted package":                  true,
		"signature from repository is unknown":                    true,
		"could not read db file":                                  true,
	}
	for msg, want := range cases {
		if got := dbRejection(errors.New(msg)); got != want {
			t.Fatalf("dbRejection(%q) = %v, want %v", msg, got, want)
		}
	}
	if dbRejection(nil) {
		t.Fatal("a nil error is not a database rejection")
	}
}

func TestDropSplitMetasNotServed(t *testing.T) {
	installed := map[string]bool{"ryoku-desktop-hyprland": true, "ryoku-desktop-niri": false}
	var removed []string
	oldI, oldR := splitMetaInstalled, splitMetaRemove
	splitMetaInstalled = func(n string) bool { return installed[n] }
	splitMetaRemove = func(n string) error { removed = append(removed, n); return nil }
	t.Cleanup(func() { splitMetaInstalled, splitMetaRemove = oldI, oldR })

	// A release that predates the split serves neither meta: the installed one
	// goes, the absent one is untouched.
	if got := dropSplitMetasNotServed(map[string]bool{"ryoku-desktop": true}); len(got) != 1 || got[0] != "ryoku-desktop-hyprland" {
		t.Fatalf("across the split the installed meta must be dropped, got %v", got)
	}
	// A release that serves the metas keeps them.
	removed = nil
	if got := dropSplitMetasNotServed(map[string]bool{"ryoku-desktop-hyprland": true}); len(got) != 0 || len(removed) != 0 {
		t.Fatalf("a served meta must stay, got %v removed %v", got, removed)
	}
}
