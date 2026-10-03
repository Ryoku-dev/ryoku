package updater

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/sys"
)

// pacmanOp reports whether any recorded call runs `pacman <op>`.
func pacmanOp(calls [][]string, op string) bool {
	for _, c := range calls {
		for i, a := range c {
			if a == "pacman" && i+1 < len(c) && c[i+1] == op {
				return true
			}
		}
	}
	return false
}

// #291: reverting (or tracking) onto a channel that does not serve an installed
// split meta -- ryoku-desktop-hyprland pins ryoku-desktop=<exact> -- must drop
// that meta first and then move the WHOLE set in one transaction. The old boot
// guard ran `pacman -S ryoku-desktop` alone, so the meta's exact pin failed the
// downgrade; this test fails on that behaviour.
func TestMoveRyokuSetDropsUnservedSplitMeta(t *testing.T) {
	var priv [][]string
	oldPriv := privileged
	privileged = func(argv ...string) error { priv = append(priv, argv); return nil }
	t.Cleanup(func() { privileged = oldPriv })

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
	if !pacmanOp(priv, "-Syy") {
		t.Fatalf("no forced -Syy refresh before the move; calls: %v", priv)
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
	oldPriv := privileged
	privileged = func(argv ...string) error {
		if len(argv) >= 2 && argv[0] == "pacman" && argv[1] == "-Syy" {
			return fmt.Errorf("no route to host")
		}
		return nil
	}
	t.Cleanup(func() { privileged = oldPriv })

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

// A box migrating onto packages from a retired checkout has no ryoku-desktop
// installed, and the installed-only set would move nothing: the track message
// promises the switch installs the base. The move must add the umbrella plus
// the active compositor variant, or the box is left with its source lane gone
// and no packaged base to update from.
func TestMoveRyokuSetAddsPackagedBaseWhenMigrating(t *testing.T) {
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
