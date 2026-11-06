package profile

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fakeResolver struct {
	refs      []PackageRef
	installed map[string]bool
	required  map[string][]string
	err       error
}

func (f fakeResolver) Resolve(targets []string, repos []string) ([]PackageRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]PackageRef{}, f.refs...), nil
}
func (f fakeResolver) Installed(names []string) (map[string]bool, error)    { return f.installed, f.err }
func (f fakeResolver) Required(names []string) (map[string][]string, error) { return f.required, f.err }

func pinnedFixture() Pinned {
	var p Pinned
	p.Profile.ID, p.Profile.Version, p.Profile.ManifestSHA256 = "hacker", "1.0.0", "manifest"
	p.Account.Name, p.Account.Role, p.Account.Shell = "hacker", "standard", "/bin/bash"
	p.Account.Permissions = []string{"devices", "audio"}
	p.Account.Groups = []string{"wireshark", "profile-hacker"}
	p.Repos = []string{"blackarch"}
	p.Targets = []string{"nmap", "burpsuite"}
	p.Removals = []string{"zaproxy"}
	p.Capabilities = []CapabilitySpec{{Tool: "nmap", Caps: AllowedCapabilities["nmap"]}}
	p.Wordlists.Links = []WordlistLink{{Name: "seclists", Target: "/usr/share/seclists"}}
	p.Hosts.Suffixes = []string{"thm", "htb"}
	p.Env = []EnvVar{{Key: "WORDLISTS", Value: WordlistsRoot}}
	p.Path = []string{"@PROFILE_BIN@", "@HOME@/go/bin"}
	p.Files = []FileHash{{Path: "home/a", SHA256: "a"}, {Path: "home/b", SHA256: "b"}}
	return p
}

func TestPinnedDigestSortsContractSets(t *testing.T) {
	a := pinnedFixture()
	b := pinnedFixture()
	reverseStrings(b.Account.Permissions)
	reverseStrings(b.Account.Groups)
	reverseStrings(b.Targets)
	reverseStrings(b.Hosts.Suffixes)
	b.Files[0], b.Files[1] = b.Files[1], b.Files[0]
	if a.Digest() != b.Digest() {
		t.Fatal("digest changed when sorted contract sets changed order")
	}
}

func TestPinnedDigestIncludesEveryPinnedField(t *testing.T) {
	base := pinnedFixture()
	mutations := []func(*Pinned){
		func(p *Pinned) { p.Profile.ID = "other" }, func(p *Pinned) { p.Profile.Version = "2.0.0" }, func(p *Pinned) { p.Profile.ManifestSHA256 = "other" },
		func(p *Pinned) { p.Account.Name = "other" }, func(p *Pinned) { p.Account.Role = "other" }, func(p *Pinned) { p.Account.Permissions[0] = "containers" },
		func(p *Pinned) { p.Account.Groups[0] = "other" }, func(p *Pinned) { p.Account.Shell = "/bin/zsh" }, func(p *Pinned) { p.Repos[0] = "other" },
		func(p *Pinned) { p.Targets[0] = "other" }, func(p *Pinned) { p.Removals[0] = "other" }, func(p *Pinned) { p.Capabilities[0].Caps = "other" },
		func(p *Pinned) { p.Wordlists.Links[0].Target = "/usr/share/other" }, func(p *Pinned) { p.Hosts.Suffixes[0] = "vl" }, func(p *Pinned) { p.Env[0].Value = "other" },
		func(p *Pinned) { p.Path[0] = "/other" }, func(p *Pinned) { p.Files[0].SHA256 = "other" },
	}
	want := base.Digest()
	for i, mutate := range mutations {
		candidate := pinnedFixture()
		mutate(&candidate)
		if candidate.Digest() == want {
			t.Fatalf("mutation %d did not change digest", i)
		}
	}
}

func TestBuildRemovalPlanAppliesPackageRules(t *testing.T) {
	r := &SystemReceipt{}
	r.Profile.ID = "hacker"
	r.Account.Name = "hacker"
	r.Packages.Introduced = []string{"remove-me", "shared", "required"}
	r.Packages.PreExisting = []string{"existing"}
	r.Repos = []RepoClaim{{ID: "blackarch", EnabledByProfile: true}}
	other := SystemReceipt{}
	other.Account.Name = "student"
	other.Packages.Introduced = []string{"shared"}
	other.Repos = []RepoClaim{{ID: "blackarch"}}
	resolver := fakeResolver{required: map[string][]string{"remove-me": {}, "shared": {}, "required": {"consumer"}}, refs: []PackageRef{{Name: "remove-me", Repo: "extra", Installed: 10}}}
	plan, err := BuildRemovalPlan(r, []SystemReceipt{other}, resolver, RemoveOptions{}, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Packages) != 1 || plan.Packages[0].Name != "remove-me" {
		t.Fatalf("unexpected removals: %#v", plan.Packages)
	}
	got := map[string]string{}
	for _, item := range plan.KeptPackages {
		got[item.Name] = item.Reason
	}
	want := map[string]string{"shared": "used by another profile account", "required": "required by consumer", "existing": "installed before the profile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kept = %#v", got)
	}
	if plan.Repos[0].Action != "keep" {
		t.Fatalf("shared repo action = %s", plan.Repos[0].Action)
	}
}

func TestBuildRemovalPlanKeepsOnlyExternallyRequiredPackages(t *testing.T) {
	r := &SystemReceipt{}
	r.Account.Name = "hacker"
	r.Packages.Introduced = []string{"target", "dep", "subdep", "shared-dep", "shared-subdep"}
	resolver := fakeResolver{
		required: map[string][]string{
			"target":        {},
			"dep":           {"target"},
			"subdep":        {"dep"},
			"shared-dep":    {"unrelated"},
			"shared-subdep": {"shared-dep"},
		},
		refs: []PackageRef{{Name: "target"}, {Name: "dep"}, {Name: "subdep"}},
	}
	plan, err := BuildRemovalPlan(r, nil, resolver, RemoveOptions{}, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	removed := map[string]bool{}
	for _, item := range plan.Packages {
		removed[item.Name] = true
	}
	if !reflect.DeepEqual(removed, map[string]bool{"target": true, "dep": true, "subdep": true}) {
		t.Fatalf("removable packages = %#v", removed)
	}
	kept := map[string]string{}
	for _, item := range plan.KeptPackages {
		kept[item.Name] = item.Reason
	}
	want := map[string]string{
		"shared-dep":    "required by unrelated",
		"shared-subdep": "required by shared-dep",
	}
	if !reflect.DeepEqual(kept, want) {
		t.Fatalf("kept packages = %#v", kept)
	}
}

func TestBuildRemovalPlanCountsFilesFromStagedPayload(t *testing.T) {
	staged, manifest := validTestManifest(t)
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staged, "profile.json"), string(body))
	r := &SystemReceipt{Staged: staged}
	r.Account.Name = "hacker"
	plan, err := BuildRemovalPlan(r, nil, fakeResolver{required: map[string][]string{}}, RemoveOptions{}, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Files.Managed != 2 || plan.Files.SeedOnce != 1 {
		t.Fatalf("staged file counts = %#v", plan.Files)
	}
}

func TestBuildPlanTurnsResolverErrorIntoBlocker(t *testing.T) {
	_, m := validTestManifest(t)
	plan, err := BuildPlan(m, Source{}, AccountTarget{Name: "hacker"}, Selection{}, HostFacts{PackageManager: "pacman", ManagedHomes: true, Shells: []string{"/bin/bash"}}, nil, fakeResolver{err: errors.New("conflicting dependencies")})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blockers) == 0 {
		t.Fatal("resolver error was not a blocker")
	}
}

func TestBuildPlanTotalsExcludePreExistingPackages(t *testing.T) {
	_, manifest := validTestManifest(t)
	resolver := fakeResolver{
		refs: []PackageRef{
			{Name: "nmap", Repo: "extra", Download: 25, Installed: 100, Target: true, PreExisting: true},
			{Name: "libnew", Repo: "extra", Download: 5, Installed: 40},
		},
		installed: map[string]bool{"nmap": true},
	}
	host := HostFacts{PackageManager: "pacman", ManagedHomes: true, Shells: []string{"/bin/bash"}}
	plan, err := BuildPlan(manifest, Source{}, AccountTarget{Name: "hacker"}, Selection{}, host, nil, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Packages) != 2 || !plan.Packages[0].PreExisting {
		t.Fatalf("package state was not preserved: %#v", plan.Packages)
	}
	if plan.Totals.Download != 5 || plan.Totals.Installed != 40 || plan.Totals.Targets != 0 || plan.Totals.Dependencies != 1 {
		t.Fatalf("totals include pre-existing packages: %#v", plan.Totals)
	}
}

func TestBuildPlanBleedExcludesPreInstalledComponents(t *testing.T) {
	_, manifest := validTestManifest(t)
	host := HostFacts{PackageManager: "pacman", ManagedHomes: true, Shells: []string{"/bin/bash"}}
	resolver := fakeResolver{
		refs:      []PackageRef{{Name: "nmap", Repo: "extra", Target: true}},
		installed: map[string]bool{"nmap": true},
	}
	plan, err := BuildPlan(manifest, Source{}, AccountTarget{Name: "hacker"}, Selection{}, host, nil, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bleed) != 0 {
		t.Fatalf("pre-installed component added bleed entries: %#v", plan.Bleed)
	}

	resolver.installed["nmap"] = false
	plan, err = BuildPlan(manifest, Source{}, AccountTarget{Name: "hacker"}, Selection{}, host, nil, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bleed) != 1 || plan.Bleed[0].DesktopID != "nmap.desktop" || plan.Bleed[0].Name != "nmap" || plan.Bleed[0].Package != "nmap" {
		t.Fatalf("unexpected bleed entry: %#v", plan.Bleed)
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
