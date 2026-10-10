package wm

import (
	"reflect"
	"testing"
)

// The removal set is the one computation a switch can break a machine with, so
// it is pinned across the four cases that matter: a satellite the incoming
// compositor also uses stays, a package a surviving package needs stays, a
// package only the outgoing compositor uses goes, and a box with nothing
// installed reclaims nothing and says so.

// reclaimEnv swaps the pacman seams for a hand-built model and restores them.
type reclaimEnv struct {
	packages map[string][]string // provider -> declared packages
	// plan models `pacman -Rs` for a target list: names[t] is what removing t
	// alone would drop (t plus its private orphans), and needs[t] is the
	// surviving package that keeps t installed, if any.
	private map[string][]string
	needs   map[string]string
	size    map[string]int64
}

func (e reclaimEnv) install(t *testing.T) {
	t.Helper()
	oldPkg, oldInst, oldOnce, oldSize := providerPackages, pkgInstalled, removalOnce, installedSizes
	t.Cleanup(func() {
		providerPackages, pkgInstalled, removalOnce, installedSizes = oldPkg, oldInst, oldOnce, oldSize
	})
	installed := map[string]bool{}
	for p := range e.private {
		installed[p] = true
	}
	providerPackages = func(name string) ([]string, error) { return e.packages[name], nil }
	pkgInstalled = func(name string) bool { return installed[name] }
	installedSizes = func(names []string) (int64, error) {
		var total int64
		for _, n := range names {
			total += e.size[n]
		}
		return total, nil
	}
	removalOnce = func(targets []string) (set, blocked []string, err error) {
		for _, t := range targets {
			if n := e.needs[t]; n != "" && !contains(targets, n) {
				blocked = append(blocked, t)
			}
		}
		if len(blocked) > 0 {
			return nil, blocked, nil
		}
		seen := map[string]bool{}
		for _, t := range targets {
			for _, p := range append([]string{t}, e.private[t]...) {
				if !seen[p] {
					seen[p] = true
					set = append(set, p)
				}
			}
		}
		return set, nil, nil
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestReclaimKeepsSatelliteSharedWithIncoming(t *testing.T) {
	reclaimEnv{
		packages: map[string][]string{
			"out": {"out-core", "shared-portal"},
			"in":  {"in-core", "shared-portal"},
		},
		private: map[string][]string{
			"out-core":      {"out-lib"},
			"shared-portal": nil,
		},
		size: map[string]int64{"out-core": 100, "out-lib": 50},
	}.install(t)

	rs, err := Reclaim("out", "in")
	if err != nil {
		t.Fatal(err)
	}
	if contains(rs.Packages, "shared-portal") {
		t.Errorf("shared-portal must stay: %v", rs.Packages)
	}
	if !contains(rs.Packages, "out-core") || !contains(rs.Packages, "out-lib") {
		t.Errorf("out-core and its orphan must go: %v", rs.Packages)
	}
	if rs.Size != 150 {
		t.Errorf("size = %d, want 150", rs.Size)
	}
}

func TestReclaimKeepsPackageASurvivorNeeds(t *testing.T) {
	reclaimEnv{
		packages: map[string][]string{
			"out": {"out-core", "out-portal"},
			"in":  {"in-core"},
		},
		private: map[string][]string{
			"out-core":   nil,
			"out-portal": nil,
		},
		// a surviving app still needs out-portal, so pacman refuses to remove it
		needs: map[string]string{"out-portal": "some-app"},
		size:  map[string]int64{"out-core": 200, "out-portal": 80},
	}.install(t)

	rs, err := Reclaim("out", "in")
	if err != nil {
		t.Fatal(err)
	}
	if contains(rs.Packages, "out-portal") {
		t.Errorf("out-portal is needed by a survivor and must stay: %v", rs.Packages)
	}
	if contains(rs.Targets, "out-portal") {
		t.Errorf("out-portal must be dropped from the named targets: %v", rs.Targets)
	}
	if !contains(rs.Packages, "out-core") {
		t.Errorf("out-core must still be removed: %v", rs.Packages)
	}
	if rs.Size != 200 {
		t.Errorf("size = %d, want 200 (out-portal excluded)", rs.Size)
	}
}

func TestReclaimRemovesOutgoingOnlyPackage(t *testing.T) {
	reclaimEnv{
		packages: map[string][]string{
			"out": {"out-core"},
			"in":  {"in-core"},
		},
		private: map[string][]string{
			"out-core": {"out-lib"},
		},
		size: map[string]int64{"out-core": 300, "out-lib": 25},
	}.install(t)

	rs, err := Reclaim("out", "in")
	if err != nil {
		t.Fatal(err)
	}
	if !rs.Removable {
		t.Fatal("expected a removable set")
	}
	want := []string{"out-core", "out-lib"}
	if !reflect.DeepEqual(rs.Packages, want) {
		t.Errorf("packages = %v, want %v", rs.Packages, want)
	}
	if rs.Count != 2 || rs.Size != 325 {
		t.Errorf("count/size = %d/%d, want 2/325", rs.Count, rs.Size)
	}
}

func TestReclaimNothingInstalledIsHonestlyEmpty(t *testing.T) {
	reclaimEnv{
		packages: map[string][]string{
			"out": {"out-core", "out-portal"},
			"in":  {"in-core"},
		},
		private: map[string][]string{}, // nothing installed
		size:    map[string]int64{},
	}.install(t)

	rs, err := Reclaim("out", "in")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Removable {
		t.Errorf("nothing is installed, so nothing is removable: %+v", rs)
	}
	if len(rs.Packages) != 0 || rs.Count != 0 || rs.Size != 0 {
		t.Errorf("empty set expected, got %+v", rs)
	}
}

// removalSet must keep dropping targets a survivor needs until the plan is
// clean, so a chain of blocked targets still yields the safe remainder.
func TestRemovalSetDropsEachBlockedTarget(t *testing.T) {
	blocked := map[string]bool{"a": true, "b": true}
	plan := func(targets []string) (set, block []string, err error) {
		for _, t := range targets {
			if blocked[t] {
				block = append(block, t)
			}
		}
		if len(block) > 0 {
			return nil, block, nil
		}
		return append([]string{}, targets...), nil, nil
	}
	removed, kept, err := removalSet([]string{"a", "b", "c", "d"}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(removed, []string{"c", "d"}) {
		t.Errorf("removed = %v, want [c d]", removed)
	}
	if !reflect.DeepEqual(kept, []string{"c", "d"}) {
		t.Errorf("kept targets = %v, want [c d]", kept)
	}
}

func TestDNFBlockedTargetsKeepsRequiredRPMs(t *testing.T) {
	tests := []struct {
		name      string
		targets   []string
		native    map[string][]string
		provides  map[string][]string
		requires  map[string][]string
		providers map[string][]string
		want      []string
	}{
		{
			name:     "surviving package requires target capability",
			targets:  []string{"out-core", "out-portal"},
			native:   map[string][]string{"out-core": {"core-rpm"}, "out-portal": {"portal-rpm"}},
			provides: map[string][]string{"core-rpm": {"core-api"}, "portal-rpm": {"portal-api"}},
			requires: map[string][]string{"core-api": {"portal-rpm"}, "portal-api": {"desktop-app"}},
			providers: map[string][]string{
				"core-api":   {"core-rpm"},
				"portal-api": {"portal-rpm"},
			},
			want: []string{"out-portal"},
		},
		{
			name:      "surviving provider satisfies shared capability",
			targets:   []string{"out-core"},
			native:    map[string][]string{"out-core": {"out-rpm"}},
			provides:  map[string][]string{"out-rpm": {"ryoku-desktop-compositor"}},
			requires:  map[string][]string{"ryoku-desktop-compositor": {"ryoku-desktop"}},
			providers: map[string][]string{"ryoku-desktop-compositor": {"out-rpm", "in-rpm"}},
			want:      nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := func(values map[string][]string) func(string) ([]string, error) {
				return func(key string) ([]string, error) {
					return append([]string(nil), values[key]...), nil
				}
			}
			got, err := dnfBlockedTargets(tc.targets, tc.native, query(tc.provides), query(tc.requires), query(tc.providers))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("blocked = %v, want %v", got, tc.want)
			}
		})
	}
}

// VerifyRemoval is the guard run right before the destructive removal: it must
// refuse when pacman's live plan no longer matches the reviewed set.
func TestVerifyRemovalRefusesOnDivergence(t *testing.T) {
	rs := ReclaimSet{Outgoing: "out", Targets: []string{"out-core"}, Packages: []string{"out-core", "out-lib"}, Removable: true}

	cases := []struct {
		name string
		set  []string
		blk  []string
		ok   bool
	}{
		{"matches", []string{"out-core", "out-lib"}, nil, true},
		{"pulls in an extra package", []string{"out-core", "out-lib", "shared-lib"}, nil, false},
		{"would keep one from the set", []string{"out-core"}, nil, false},
		{"a target is now needed", nil, []string{"out-core"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			old := removalOnce
			t.Cleanup(func() { removalOnce = old })
			removalOnce = func([]string) (set, blocked []string, err error) { return c.set, c.blk, nil }
			err := VerifyRemoval(rs)
			if c.ok && err != nil {
				t.Errorf("expected pass, got %v", err)
			}
			if !c.ok && err == nil {
				t.Error("expected refusal, got nil")
			}
		})
	}
}

func TestVerifyRemovalSkipsEmptySet(t *testing.T) {
	if err := VerifyRemoval(ReclaimSet{Removable: false}); err != nil {
		t.Errorf("nothing to remove must not refuse: %v", err)
	}
}

// pacman writes its refusal lines to stdout, not stderr. The parser must read
// the stream the refusals actually arrive on, or a packaged box -- where the
// variant package owns every satellite -- sees "nothing to remove" and the
// switch loses its keep-or-remove choice. This is the seam the mocked tests
// cannot catch, so it is pinned on the real transcript shape.
func TestParseRemovalPlanReadsStdoutRefusals(t *testing.T) {
	targets := []string{"ryoku-desktop-hyprland", "hyprland"}
	stdout := ":: removing hyprland breaks dependency 'hyprland' required by ryoku-desktop-hyprland\n" +
		":: removing ryoku-desktop-hyprland breaks dependency 'ryoku-desktop-compositor' required by ryoku-desktop\n"
	_, blocked := parseRemovalPlan(stdout, "", targets)
	if len(blocked) != 2 {
		t.Fatalf("blocked = %v, want both targets", blocked)
	}
}

func TestParseRemovalPlanCleanPrintList(t *testing.T) {
	targets := []string{"hyprland"}
	stdout := "hyprland\nhyprcursor\naquamarine\n"
	set, blocked := parseRemovalPlan(stdout, "", targets)
	if blocked != nil {
		t.Fatalf("blocked = %v, want none", blocked)
	}
	if !reflect.DeepEqual(set, []string{"hyprland", "hyprcursor", "aquamarine"}) {
		t.Errorf("set = %v", set)
	}
}
