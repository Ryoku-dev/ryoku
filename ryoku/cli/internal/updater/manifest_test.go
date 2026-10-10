package updater

import (
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-cli/internal/ryokumanifest"
)

func TestPlanManifestFirstRunRecordsBaselineOnly(t *testing.T) {
	next := &ryokumanifest.Manifest{
		Base:       []string{"base"},
		FirstParty: []string{"ryoku-desktop"},
	}
	plan := PlanManifest(nil, nil, next, map[string]bool{"base": true})
	if len(plan.Install) != 0 || len(plan.AUR) != 0 || len(plan.UserGone) != 0 {
		t.Fatalf("first run changed state: %+v", plan)
	}
	// The gap the box has against the release is reported, not acted on.
	if !reflect.DeepEqual(plan.Untracked, []string{"ryoku-desktop"}) {
		t.Errorf("untracked = %v, want [ryoku-desktop]", plan.Untracked)
	}
}

func TestPlanManifestDeliversNeverPresentNames(t *testing.T) {
	// The user's complaint in one function: a package the release always named
	// but this box never received must self-heal, not wait for a release that
	// happens to "add" it.
	prev := &ryokumanifest.Manifest{Base: []string{"base"}, Dev: []string{"go"}}
	next := &ryokumanifest.Manifest{Base: []string{"base"}, Dev: []string{"go"}}
	installed := map[string]bool{"base": true} // go never landed here
	plan := PlanManifest(prev, map[string]bool{"base": true}, next, installed)
	if !reflect.DeepEqual(plan.Install, []string{"go"}) {
		t.Errorf("install = %v, want [go]", plan.Install)
	}
}

func TestPlanManifestRespectsUserRemovalAfterBaseline(t *testing.T) {
	prev := &ryokumanifest.Manifest{Base: []string{"base"}, Dev: []string{"go"}}
	next := prev
	// go was present when the baseline was saved and is gone now: the user
	// removed it, and no later update puts it back.
	plan := PlanManifest(prev, map[string]bool{"base": true, "go": true}, next,
		map[string]bool{"base": true})
	if len(plan.Install) != 0 {
		t.Errorf("install = %v; a user removal must stay removed", plan.Install)
	}
	if !reflect.DeepEqual(plan.UserGone, []string{"go"}) {
		t.Errorf("userGone = %v, want [go]", plan.UserGone)
	}
}

func TestPlanManifestRoutesAURLaneBestEffort(t *testing.T) {
	prev := &ryokumanifest.Manifest{}
	next := &ryokumanifest.Manifest{AUR: []string{"some-aur-tool"}}
	plan := PlanManifest(prev, map[string]bool{}, next, map[string]bool{})
	if !reflect.DeepEqual(plan.AUR, []string{"some-aur-tool"}) {
		t.Errorf("aur = %v, want [some-aur-tool]", plan.AUR)
	}
	if len(plan.Install) != 0 {
		t.Errorf("install = %v; AUR names must not go through pacman", plan.Install)
	}
}

func TestPlanManifestLeavesProvisionedLaneToShippedApps(t *testing.T) {
	// reconcileShippedApps owns the deliver-once apps. If PlanManifest also
	// installed them, one update would run two pacman transactions over the
	// same names.
	prev := &ryokumanifest.Manifest{}
	next := &ryokumanifest.Manifest{Provisioned: []string{"kitty"}}
	plan := PlanManifest(prev, map[string]bool{}, next, map[string]bool{})
	if len(plan.Install) != 0 {
		t.Errorf("install = %v; the provisioned lane is not this reconciler's", plan.Install)
	}
}

func TestPlanManifestReportsRetiredNames(t *testing.T) {
	prev := &ryokumanifest.Manifest{Dev: []string{"dropped-tool"}}
	next := &ryokumanifest.Manifest{}
	plan := PlanManifest(prev, map[string]bool{"dropped-tool": true}, next,
		map[string]bool{"dropped-tool": true})
	if !reflect.DeepEqual(plan.Retired, []string{"dropped-tool"}) {
		t.Errorf("retired = %v, want [dropped-tool]", plan.Retired)
	}
	if len(plan.UserGone) != 0 {
		t.Errorf("userGone = %v; a retired name is the release's decision, not the user's", plan.UserGone)
	}
}

func TestFedoraPackageSetsUseRPMAndDNFUserInstalled(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "dnf")
	bin := t.TempDir()
	writeExec(t, filepath.Join(bin, "rpm"), "#!/bin/sh\nprintf 'ryoku-desktop\\nquickshell\\n'\n")
	writeExec(t, filepath.Join(bin, "dnf5"), "#!/bin/sh\n[ \"$1 $2\" = 'repoquery -y' ] || exit 2\nprintf 'ryoku-desktop\\n'\n")
	t.Setenv("PATH", bin)

	installed, err := packageSet(false)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]bool{"ryoku-desktop": true, "quickshell": true}; !reflect.DeepEqual(installed, want) {
		t.Fatalf("installed = %#v, want %#v", installed, want)
	}
	explicit, err := packageSet(true)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]bool{"ryoku-desktop": true}; !reflect.DeepEqual(explicit, want) {
		t.Fatalf("explicit = %#v, want %#v", explicit, want)
	}
}
