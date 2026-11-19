package updater

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/sys"

	wm "ryoku-wm"
)

func stubMoveRepoSync(t *testing.T, fn func(bool) error) {
	t.Helper()
	old := syncRepoForMove
	syncRepoForMove = fn
	t.Cleanup(func() { syncRepoForMove = old })
}

// #291: reverting (or tracking) onto a channel that does not serve an installed
// split meta -- ryoku-desktop-hyprland pins ryoku-desktop=<exact> -- must drop
// that meta first and then move the WHOLE set in one transaction. The old boot
// guard ran `pacman -S ryoku-desktop` alone, so the meta's exact pin failed the
// downgrade; this test fails on that behaviour.
func TestMoveRyokuSetDropsUnservedSplitMeta(t *testing.T) {
	synced := false
	stubMoveRepoSync(t, func(force bool) error {
		synced = force
		return nil
	})

	var movedSet []string
	oldMove := runRyokuMove
	runRyokuMove = func(set []string) error { movedSet = set; return nil }
	t.Cleanup(func() { runRyokuMove = oldMove })

	oldSet, oldServed, oldPkg := ryokuSetForMove, servedSetForMove, pkgInstalledForMove
	ryokuSetForMove = func() ([]string, error) {
		return []string{"ryoku/ryoku-desktop", "ryoku/ryogami"}, nil
	}
	servedSetForMove = func() map[string]bool {
		// the target channel serves the set but NOT the compositor split meta.
		return map[string]bool{"ryoku-desktop": true, "ryogami": true}
	}
	pkgInstalledForMove = func(string) bool { return true } // packaged box: no base add
	t.Cleanup(func() {
		ryokuSetForMove, servedSetForMove, pkgInstalledForMove = oldSet, oldServed, oldPkg
	})

	var dropped []string
	oldInstalled, oldRemove := splitMetaInstalled, splitMetaRemove
	splitMetaInstalled = func(name string) bool { return name == "ryoku-desktop-hyprland" }
	splitMetaRemove = func(name string) error { dropped = append(dropped, name); return nil }
	t.Cleanup(func() { splitMetaInstalled, splitMetaRemove = oldInstalled, oldRemove })

	set, err := moveRyokuSetToChannel()
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if want := []string{"ryoku-desktop-hyprland"}; !reflect.DeepEqual(dropped, want) {
		t.Fatalf("dropped split metas = %v, want %v", dropped, want)
	}
	if want := []string{"ryoku/ryoku-desktop", "ryoku/ryogami"}; !reflect.DeepEqual(set, want) {
		t.Fatalf("moved set = %v, want %v", set, want)
	}
	if !synced {
		t.Fatal("move did not force-refresh the target repository")
	}
	// the transaction moves the WHOLE set (the old revert moved ryoku-desktop
	// alone, which is exactly what left the split meta's pin unsatisfiable).
	if want := []string{"ryoku/ryoku-desktop", "ryoku/ryogami"}; !reflect.DeepEqual(movedSet, want) {
		t.Fatalf("move transaction set = %v, want %v", movedSet, want)
	}
}

// A refresh that cannot reach the channel surfaces as errChannelUnreachable, the
// signal the boot guard uses to retry next boot rather than escalate.
func TestMoveRyokuSetUnreachableRefresh(t *testing.T) {
	stubMoveRepoSync(t, func(bool) error {
		return fmt.Errorf("no route to host")
	})

	if _, err := moveRyokuSetToChannel(); err == nil || !errors.Is(err, errChannelUnreachable) {
		t.Fatalf("err = %v, want errChannelUnreachable", err)
	}
}

// #291 follow-up (raised in review): the split-meta drop is a separate committed
// -Rdd, so if the move transaction then fails, restoring the pin alone would
// leave the box permanently missing ryoku-desktop-hyprland/niri (the ordinary
// update only moves installed AND served names). A failed move must reinstall the
// dropped metas from the restored channel. This fails on a revert that only
// restores the pin.
func TestRetargetChannelReinstallsDroppedMetaOnFailure(t *testing.T) {
	stubMoveRepoSync(t, func(bool) error { return nil })
	isolateHome(t)
	packagedConf(t, "stable") // the restored channel serves the split metas

	var priv [][]string
	oldPriv := privileged
	privileged = func(argv ...string) error { priv = append(priv, argv); return nil }
	t.Cleanup(func() { privileged = oldPriv })

	oldSet, oldServed, oldPkg := ryokuSetForMove, servedSetForMove, pkgInstalledForMove
	ryokuSetForMove = func() ([]string, error) { return []string{"ryoku/ryoku-desktop"}, nil }
	servedSetForMove = func() map[string]bool { return map[string]bool{"ryoku-desktop": true} } // target predates the split
	pkgInstalledForMove = func(string) bool { return true }                                     // packaged box: no base add
	t.Cleanup(func() {
		ryokuSetForMove, servedSetForMove, pkgInstalledForMove = oldSet, oldServed, oldPkg
	})

	oldInstalled, oldRemove := splitMetaInstalled, splitMetaRemove
	splitMetaInstalled = func(name string) bool { return name == "ryoku-desktop-hyprland" }
	splitMetaRemove = func(string) error { return nil } // the drop commits
	t.Cleanup(func() { splitMetaInstalled, splitMetaRemove = oldInstalled, oldRemove })

	oldMove := runRyokuMove
	runRyokuMove = func([]string) error { return fmt.Errorf("could not satisfy dependencies") }
	t.Cleanup(func() { runRyokuMove = oldMove })

	err := retargetChannel("v0.63.1-beta.19", func() error { _, e := moveRyokuSetToChannel(); return e })
	if err == nil {
		t.Fatal("expected the failed move to surface an error")
	}
	if got := sys.PackagedChannel(); got != "stable" {
		t.Fatalf("channel = %q after failed move, want the previous stable restored", got)
	}
	reinstalled := false
	for _, c := range priv {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, " -S ") && strings.Contains(joined, ryokuRepo+"/ryoku-desktop-hyprland") {
			reinstalled = true
		}
	}
	if !reinstalled {
		t.Fatalf("dropped meta not reinstalled from the restored channel; privileged calls: %v", priv)
	}
}

func TestRetargetChannelKeepsTargetWhenDNFTransactionApplied(t *testing.T) {
	isolateHome(t)
	packagedConf(t, "stable")
	synced := false
	stubMoveRepoSync(t, func(bool) error {
		synced = true
		return nil
	})
	oldApplied := channelMoveApplied
	channelMoveApplied = func(channel string) bool { return channel == sys.ChannelTesting }
	t.Cleanup(func() { channelMoveApplied = oldApplied })

	err := retargetChannel(sys.ChannelTesting, func() error {
		lastDroppedMetas = []string{"ryoku-desktop-hyprland"}
		return fmt.Errorf("post-transaction scriptlet failed")
	})
	if err != nil {
		t.Fatalf("committed DNF transaction surfaced as a failed channel move: %v", err)
	}
	if got := sys.PackagedChannel(); got != sys.ChannelTesting {
		t.Fatalf("channel = %q, want target %q kept", got, sys.ChannelTesting)
	}
	if synced {
		t.Fatal("committed DNF transaction restored and synced the previous channel")
	}
	if len(lastDroppedMetas) != 0 {
		t.Fatalf("dropped target metas were retained for rollback: %v", lastDroppedMetas)
	}
}

func TestDNFMoveReachedTargetUsesInstalledState(t *testing.T) {
	targetBuild := "v0.95.9-beta.20.dev.5+g90673ad"
	tests := []struct {
		name      string
		channel   string
		installed string
		target    string
		release   sys.Release
		want      bool
	}{
		{
			name:      "package EVR reached target",
			channel:   sys.ChannelTesting,
			installed: "0:0.95.9-1.fc44",
			target:    "0:0.95.9-1.fc44",
			want:      true,
		},
		{
			name:    "frozen marker reached target",
			channel: targetBuild,
			release: sys.Release{Release: targetBuild},
			want:    true,
		},
		{
			name:      "nothing moved",
			channel:   sys.ChannelTesting,
			installed: "0:0.95.8-1.fc44",
			target:    "0:0.95.9-1.fc44",
			release:   sys.Release{Channel: sys.ChannelTesting},
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dnfMoveReachedTarget(tc.channel, tc.installed, tc.target, tc.release); got != tc.want {
				t.Fatalf("dnfMoveReachedTarget() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A box migrating onto packages from a retired checkout has no ryoku-desktop
// installed, and the installed-only set would move nothing: the track message
// promises the switch installs the base. The move must add the umbrella plus
// the active compositor variant, or the box is left with its source lane gone
// and no packaged base to update from.
func TestMoveRyokuSetAddsPackagedBaseWhenMigrating(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
	stubMoveRepoSync(t, func(bool) error { return nil })
	var movedSet []string
	oldMove := runRyokuMove
	runRyokuMove = func(set []string) error { movedSet = set; return nil }
	t.Cleanup(func() { runRyokuMove = oldMove })

	oldPriv := privileged
	privileged = func(...string) error { return nil }
	t.Cleanup(func() { privileged = oldPriv })

	oldSet, oldServed, oldPkg, oldDetect :=
		ryokuSetForMove, servedSetForMove, pkgInstalledForMove, detectCompositorForMove
	ryokuSetForMove = func() ([]string, error) { return []string{"ryoku/ryogami"}, nil }
	servedSetForMove = func() map[string]bool {
		return map[string]bool{"ryoku-desktop": true, "ryoku-desktop-niri": true, "ryogami": true}
	}
	pkgInstalledForMove = func(name string) bool { return name != ryokuDesktopPkg } // no base yet
	detectCompositorForMove = func() string { return "niri" }
	t.Cleanup(func() {
		ryokuSetForMove, servedSetForMove, pkgInstalledForMove, detectCompositorForMove =
			oldSet, oldServed, oldPkg, oldDetect
	})

	set, err := moveRyokuSetToChannel()
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	want := []string{
		ryokuRepo + "/ryoku-desktop",
		ryokuRepo + "/ryoku-desktop-niri",
		ryokuRepo + "/ryogami",
	}
	if !reflect.DeepEqual(movedSet, want) {
		t.Fatalf("moved set = %v, want the base + detected variant first, then %v", movedSet, want)
	}
	if !reflect.DeepEqual(set, want) {
		t.Fatalf("returned set = %v, want %v", set, want)
	}
}

func TestMoveRyokuSetMigratesRetiredVariant(t *testing.T) {
	for _, legacy := range wm.RetiredCompositors() {
		t.Run(legacy.VariantPackage, func(t *testing.T) {
			t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
			t.Setenv(legacy.SessionHandle, "")
			stubMoveRepoSync(t, func(bool) error { return nil })
			var movedSet, dropped []string

			oldPriv := privileged
			privileged = func(...string) error { return nil }
			t.Cleanup(func() { privileged = oldPriv })

			oldMove := runRyokuMove
			runRyokuMove = func(set []string) error {
				movedSet = append([]string(nil), set...)
				return nil
			}
			t.Cleanup(func() { runRyokuMove = oldMove })

			oldSet, oldServed, oldPkg, oldDetect :=
				ryokuSetForMove, servedSetForMove, pkgInstalledForMove, detectCompositorForMove
			ryokuSetForMove = func() ([]string, error) {
				return []string{ryokuRepo + "/" + ryokuDesktopPkg, ryokuRepo + "/ryogami"}, nil
			}
			servedSetForMove = func() map[string]bool {
				return map[string]bool{
					ryokuDesktopPkg:                         true,
					ryokuDesktopPkg + "-" + wm.ProviderNiri: true,
					"ryogami":                               true,
				}
			}
			pkgInstalledForMove = func(name string) bool { return name == ryokuDesktopPkg }
			detectCompositorForMove = func() string { return wm.ProviderNiri }
			t.Cleanup(func() {
				ryokuSetForMove, servedSetForMove, pkgInstalledForMove, detectCompositorForMove =
					oldSet, oldServed, oldPkg, oldDetect
			})

			oldInstalled, oldRemove := splitMetaInstalled, splitMetaRemove
			splitMetaInstalled = func(name string) bool { return name == legacy.VariantPackage }
			splitMetaRemove = func(name string) error {
				dropped = append(dropped, name)
				return nil
			}
			t.Cleanup(func() { splitMetaInstalled, splitMetaRemove = oldInstalled, oldRemove })

			set, err := moveRyokuSetToChannel()
			if err != nil {
				t.Fatalf("move: %v", err)
			}
			wantSet := []string{
				ryokuRepo + "/" + ryokuDesktopPkg + "-" + wm.ProviderNiri,
				ryokuRepo + "/" + ryokuDesktopPkg,
				ryokuRepo + "/ryogami",
			}
			if !reflect.DeepEqual(set, wantSet) || !reflect.DeepEqual(movedSet, wantSet) {
				t.Fatalf("moved set = %v, returned set = %v, want %v", movedSet, set, wantSet)
			}
			if want := []string{legacy.VariantPackage}; !reflect.DeepEqual(dropped, want) {
				t.Fatalf("dropped variants = %v, want %v", dropped, want)
			}
		})
	}
}

func TestMoveRyokuSetKeepsLiveRetiredSession(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
	legacy := wm.RetiredCompositors()[1]
	t.Setenv(legacy.SessionHandle, "/tmp/retired-compositor-test")
	stubMoveRepoSync(t, func(bool) error { return nil })
	oldSet, oldInstalled, oldMove, oldRemove := ryokuSetForMove, splitMetaInstalled, runRyokuMove, splitMetaRemove
	t.Cleanup(func() {
		ryokuSetForMove, splitMetaInstalled, runRyokuMove, splitMetaRemove =
			oldSet, oldInstalled, oldMove, oldRemove
	})
	ryokuSetForMove = func() ([]string, error) { return []string{"ryoku/ryoku-desktop"}, nil }
	splitMetaInstalled = func(name string) bool { return name == legacy.VariantPackage }
	runRyokuMove = func([]string) error { t.Fatal("must not move while retired session is live"); return nil }
	splitMetaRemove = func(string) error { t.Fatal("must not drop live compositor variant"); return nil }
	if _, err := moveRyokuSetToChannel(); err == nil || !strings.Contains(err.Error(), "TTY") {
		t.Fatalf("live session migration error = %v, want TTY instruction", err)
	}
}
