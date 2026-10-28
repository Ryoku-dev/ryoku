package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type flatpakFixture struct {
	provider flatpakProvider
	calls    [][]string
	failing  bool
	body     map[string]any
	remotes  map[string]string
	found    map[string]any
	searched []map[string]any
}

func hit(id, name, kind string, verified bool, arches ...string) map[string]any {
	return map[string]any{
		"app_id": id, "name": name, "summary": name + " summary", "type": kind,
		"description":     "A  long\n  description of " + name,
		"icon":            "https://dl.flathub.org/media/" + id + ".png",
		"project_license": "GPL-3.0+", "developer_name": "Dev",
		"main_categories": "graphics", "sub_categories": []string{"Photography"},
		"verification_verified": verified, "arches": arches,
		"added_at": 1493820137, "installs_last_month": 42,
	}
}

func newFlatpakFixture(t *testing.T, installed []installedFlatpak) *flatpakFixture {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	arch := flathubArch()
	fx := &flatpakFixture{remotes: map[string]string{"system": flathubRepoURL, "user": flathubRepoURL, "extra": flathubRepoURL}}
	fx.body = map[string]any{"hits": []any{
		hit("org.example.Alpha", "Alpha", "desktop-application", true, arch, "other"),
		hit("org.example.Beta", "Beta", "desktop-application", false, arch),
		hit("org.example.Alpha", "Alpha again", "desktop-application", true, arch),
		hit("org.example.Cli", "Cli", "console-application", true, arch),
		hit("org.example.Foreign", "Foreign", "desktop-application", true, "riscv64"),
		hit("--evil", "Evil", "desktop-application", true, arch),
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fx.failing {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collection/popular":
			_ = json.NewEncoder(w).Encode(fx.body)
		case r.Method == http.MethodPost && r.URL.Path == "/search":
			var sent map[string]any
			_ = json.NewDecoder(r.Body).Decode(&sent)
			fx.searched = append(fx.searched, sent)
			_ = json.NewEncoder(w).Encode(fx.found)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	fx.provider = flatpakProvider{
		api:     server.URL,
		present: func() bool { return true },
		installed: func(context.Context) ([]installedFlatpak, error) {
			return installed, nil
		},
		run: func(_ context.Context, args ...string) ([]byte, error) {
			fx.calls = append(fx.calls, args)
			if args[0] == "remotes" {
				installation := strings.TrimPrefix(args[2], "--installation=")
				if args[2] == "--user" {
					installation = "user"
				} else if args[2] == "--system" {
					installation = "system"
				}
				if remote := fx.remotes[installation]; remote != "" {
					return []byte("community\thttps://example.org/repo/\nflathub\t" + remote + "\n"), nil
				}
			}
			return nil, nil
		},
	}
	return fx
}

func TestParseFlatpakList(t *testing.T) {
	out := []byte("org.example.Alpha\tAlpha\t1.2\tsystem\tflathub\n" +
		"not-an-id\tBad\t1\tuser\tflathub\n" +
		"org.example.Short\tShort\n" +
		"org.example.Beta\tBeta\t\tuser\tflathub-beta\n")
	got := parseFlatpakList(out)
	want := []installedFlatpak{
		{ID: "org.example.Alpha", Name: "Alpha", Version: "1.2", Installation: "system", Origin: "flathub"},
		{ID: "org.example.Beta", Name: "Beta", Installation: "user", Origin: "flathub-beta"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFlatpakList = %+v, want %+v", got, want)
	}
}

func TestFlatpakLoadFiltersAndMarksInstalled(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Name: "Alpha", Version: "1.2", Installation: "system", Origin: "flathub"},
		{ID: "org.example.Zeta", Name: "Zeta", Version: "9", Installation: "user", Origin: "flathub"},
		{ID: "org.example.Beta", Name: "Beta", Version: "3", Installation: "user", Origin: "flathub-beta"},
	})
	items, state, err := fx.provider.Load(context.Background(), true)
	if err != nil || state.Offline {
		t.Fatalf("Load = %v, offline %v", err, state.Offline)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	wantIDs := []string{"org.example.Alpha", "org.example.Beta", "org.example.Zeta"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("ids = %v, want %v (console apps, other arches, duplicates and invalid ids are dropped; installed outside the ranking is appended)", ids, wantIDs)
	}

	alpha, beta, zeta := items[0], items[1], items[2]
	if !alpha.Installed || alpha.InstalledVersion != "1.2" || alpha.Metadata["installation"] != "system" {
		t.Errorf("alpha installed state wrong: %+v", alpha)
	}
	if beta.Installed {
		t.Errorf("beta is installed only from another remote, got Installed")
	}
	if !zeta.Installed || zeta.Name != "Zeta" || zeta.Metadata["installation"] != "user" {
		t.Errorf("zeta should be listed as an installed extra: %+v", zeta)
	}
	if alpha.VerificationStatus != "verified" || beta.VerificationStatus != "unverified" {
		t.Errorf("verification = %q / %q", alpha.VerificationStatus, beta.VerificationStatus)
	}
	if zeta.VerificationStatus != "unknown" {
		t.Errorf("installed-only verification must not be guessed: %q", zeta.VerificationStatus)
	}
	if alpha.Description != "A long description of Alpha" {
		t.Errorf("description whitespace not collapsed: %q", alpha.Description)
	}
	if alpha.Upstream != "https://flathub.org/apps/org.example.Alpha" || alpha.Kind != "flatpak" || alpha.License != "GPL-3.0+" {
		t.Errorf("provenance fields wrong: %+v", alpha)
	}
	if !reflect.DeepEqual(alpha.Tags, []string{"graphics", "photography"}) {
		t.Errorf("tags = %v", alpha.Tags)
	}
	if alpha.Unavailable {
		t.Errorf("alpha should be available")
	}
}

func TestFlatpakCategoryCountsInstalled(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Name: "Alpha", Installation: "user", Origin: "flathub"},
	})
	cat := BuildCatalog(context.Background(), []Provider{fx.provider}, true)
	if len(cat.Categories) != 1 || cat.Categories[0].ID != flatpakCategory {
		t.Fatalf("categories = %+v", cat.Categories)
	}
	if got := cat.Categories[0]; got.Count != 2 || got.InstalledCount != 1 || got.Error != "" {
		t.Fatalf("category = %+v", got)
	}
}

func TestFlatpakOfflineServesArchive(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	if _, _, err := fx.provider.Load(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	fx.failing = true
	items, state, err := fx.provider.Load(context.Background(), true)
	if err != nil || !state.Offline || state.CachedAt == "" || len(items) != 2 {
		t.Fatalf("archive fallback = %d items, state %+v, err %v", len(items), state, err)
	}
}

func TestFlatpakFirstFetchFailureIsIsolatedToTheCategory(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.failing = true
	if _, _, err := fx.provider.Load(context.Background(), true); err == nil {
		t.Fatal("expected an error with no network, no archive and nothing installed")
	}

	fx2 := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Alpha", Name: "Alpha", Installation: "user", Origin: "flathub"}})
	fx2.failing = true
	items, _, err := fx2.provider.Load(context.Background(), true)
	if err != nil || len(items) != 1 || !items[0].Installed {
		t.Fatalf("installed apps must stay listed (and removable) offline: %v %+v", err, items)
	}
}

func TestFlatpakAbsentGreysItemsOut(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.provider.present = func() bool { return false }
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil || len(items) == 0 {
		t.Fatalf("Load = %d items, %v", len(items), err)
	}
	for _, it := range items {
		if !it.Unavailable || !it.UnavailableEdition || !strings.Contains(it.UnavailableReason, "Flatpak") {
			t.Fatalf("%s should be unavailable without flatpak: %+v", it.ID, it)
		}
	}
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err == nil {
		t.Fatal("Install must refuse without flatpak")
	}
}

func TestFlatpakInstallUsesSystemInstallation(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"remotes", "--columns=name,url", "--system"},
		{"install", "--system", "--noninteractive", "--assumeyes", "flathub", "org.example.Alpha"},
	}
	if !reflect.DeepEqual(fx.calls, want) {
		t.Fatalf("calls = %v, want %v", fx.calls, want)
	}
}

func TestFlatpakFallsBackToExistingUserRemote(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"}})
	delete(fx.remotes, "system")
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Unavailable || !items[2].Installed || items[2].Unavailable {
		t.Fatalf("canonical user remote should keep apps available: %+v", items)
	}
	fx.calls = nil
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"remotes", "--columns=name,url", "--system"},
		{"remotes", "--columns=name,url", "--user"},
		{"install", "--user", "--noninteractive", "--assumeyes", "flathub", "org.example.Alpha"},
	}
	if !reflect.DeepEqual(fx.calls, want) {
		t.Fatalf("new app must use existing user remote, without adding one: %v", fx.calls)
	}
}

func TestFlatpakWithoutCanonicalRemoteExplainsDoctorSetup(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	delete(fx.remotes, "system")
	delete(fx.remotes, "user")
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil || !items[0].Unavailable || !strings.Contains(items[0].UnavailableReason, "ryoku doctor") {
		t.Fatalf("missing remotes must explain system setup: %+v, %v", items, err)
	}
	fx.calls = nil
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err == nil || !strings.Contains(err.Error(), "ryoku doctor") {
		t.Fatalf("new installation without remote = %v", err)
	}
	if !reflect.DeepEqual(fx.calls, [][]string{{"remotes", "--columns=name,url", "--system"}, {"remotes", "--columns=name,url", "--user"}}) {
		t.Fatalf("no remote-add or install is allowed: %v", fx.calls)
	}
}

func TestFlatpakUserInstallWorksWithoutSystemRemote(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"}})
	delete(fx.remotes, "system")
	if err := fx.provider.Install(context.Background(), "org.example.Zeta"); err != nil {
		t.Fatal(err)
	}
	if err := fx.provider.Remove(context.Background(), "org.example.Zeta"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"remotes", "--columns=name,url", "--user"},
		{"update", "--noninteractive", "--assumeyes", "--user", "org.example.Zeta"},
		{"remotes", "--columns=name,url", "--user"},
		{"uninstall", "--noninteractive", "--assumeyes", "--user", "org.example.Zeta"},
	}
	if !reflect.DeepEqual(fx.calls, want) {
		t.Fatalf("existing user app actions = %v, want %v", fx.calls, want)
	}
}

func TestFlatpakRemoteIdentityRejectsImpostors(t *testing.T) {
	for _, remote := range []string{"https://evil.example/repo/", "http://dl.flathub.org/repo/", "https://dl.flathub.org/repo/extra", "https://dl.flathub.org/repo//"} {
		t.Run(remote, func(t *testing.T) {
			fx := newFlatpakFixture(t, nil)
			fx.remotes["system"] = remote
			delete(fx.remotes, "user")
			items, _, err := fx.provider.Load(context.Background(), true)
			if err != nil {
				t.Fatal(err)
			}
			if !items[0].Unavailable || !strings.Contains(items[0].UnavailableReason, "official Flathub") {
				t.Fatalf("spoofed remote must be unavailable: %+v", items[0])
			}
			fx.calls = nil
			if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err == nil || !strings.Contains(err.Error(), "official Flathub") {
				t.Fatalf("spoofed installation accepted: %v", err)
			}
			if !reflect.DeepEqual(fx.calls, [][]string{{"remotes", "--columns=name,url", "--system"}, {"remotes", "--columns=name,url", "--user"}}) {
				t.Fatalf("spoofed install ran other commands: %v", fx.calls)
			}
		})
	}
}

func TestFlatpakUsesCanonicalUserRemoteWhenSystemIsImpostor(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.remotes["system"] = "https://example.org/repo/"
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil || items[0].Unavailable {
		t.Fatalf("canonical user remote should still be usable: %+v, %v", items, err)
	}
	fx.calls = nil
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err != nil {
		t.Fatal(err)
	}
	if got := fx.calls[len(fx.calls)-1]; !reflect.DeepEqual(got, []string{"install", "--user", "--noninteractive", "--assumeyes", flatpakRemote, "org.example.Alpha"}) {
		t.Fatalf("installation should use canonical user remote: %v", fx.calls)
	}
}

func TestFlatpakAcceptsCanonicalRemoteWithoutTrailingSlash(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.remotes["system"] = strings.TrimSuffix(flathubRepoURL, "/")
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err != nil {
		t.Fatal(err)
	}
}

func TestFlatpakUserRemoteImpostorIsNotAFlathubInstall(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Alpha", Installation: "user", Origin: "flathub"}})
	fx.remotes["user"] = "https://example.org/repo/"
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Installed || items[0].Metadata["installation"] != nil {
		t.Fatalf("spoofed user install claimed as Flathub: %+v", items[0])
	}
	fx.calls = nil
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err == nil || !strings.Contains(err.Error(), "official Flathub") {
		t.Fatalf("spoofed update accepted: %v", err)
	}
	if err := fx.provider.Remove(context.Background(), "org.example.Alpha"); err == nil || !strings.Contains(err.Error(), "official Flathub") {
		t.Fatalf("spoofed removal accepted: %v", err)
	}
	for _, call := range fx.calls {
		if call[0] == "update" || call[0] == "uninstall" || call[0] == "install" {
			t.Fatalf("spoofed app mutated: %v", fx.calls)
		}
	}
	fx.found = searchResult(hit("org.example.Alpha", "Alpha", "desktop-application", true, flathubArch()))
	results, err := fx.provider.Search(context.Background(), "alpha")
	if err != nil || len(results) != 1 || results[0].Installed {
		t.Fatalf("search claimed spoofed installation: %+v, %v", results, err)
	}
}

func TestFlatpakRemoteProbeFailureRefusesInstallation(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.provider.run = func(_ context.Context, args ...string) ([]byte, error) {
		fx.calls = append(fx.calls, args)
		return nil, errors.New("flatpak remotes failed")
	}
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil || !items[0].Unavailable || !strings.Contains(items[0].UnavailableReason, "Could not check") {
		t.Fatalf("remote probe failure must be unavailable: %+v, %v", items, err)
	}
	fx.calls = nil
	if err := fx.provider.Install(context.Background(), "org.example.Alpha"); err == nil || !strings.Contains(err.Error(), "Could not check") {
		t.Fatalf("remote probe failure accepted: %v", err)
	}
	if len(fx.calls) != 2 || fx.calls[0][0] != "remotes" || fx.calls[1][0] != "remotes" {
		t.Fatalf("probe failure reached installation: %v", fx.calls)
	}
}

func TestFlatpakSnapshotRechecksRemoteIdentity(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"}})
	fx.provider.updates = nil
	var first strings.Builder
	if err := runCatalog(&first, []Provider{fx.provider}, nil); err != nil {
		t.Fatal(err)
	}
	fx.remotes["user"] = "https://example.org/repo/"
	delete(fx.remotes, "system")
	var second strings.Builder
	if err := runCatalog(&second, []Provider{fx.provider}, nil); err != nil {
		t.Fatal(err)
	}
	var cat Catalog
	if err := json.Unmarshal([]byte(second.String()), &cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Items) != 2 || cat.Items[0].UnavailableReason == "" {
		t.Fatalf("snapshot retained a stale install or missed the missing system remote: %+v", cat.Items)
	}
	fx.remotes["system"] = flathubRepoURL
	fx.remotes["user"] = flathubRepoURL
	var third strings.Builder
	if err := runCatalog(&third, []Provider{fx.provider}, nil); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(third.String()), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Items[0].Unavailable || !cat.Items[2].Installed {
		t.Fatalf("snapshot pinned bad remote after repair: %+v", cat.Items)
	}
}

func TestFlatpakRefusesIDsThatCouldBeFlags(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	for _, id := range []string{"--remove-all", "--evil.org.App", "-y", "a.b", "org.example.Alpha --user", "", "org/example/Alpha"} {
		if err := fx.provider.Install(context.Background(), id); err == nil {
			t.Errorf("Install(%q) should fail", id)
		}
		if err := fx.provider.Remove(context.Background(), id); err == nil {
			t.Errorf("Remove(%q) should fail", id)
		}
	}
	if len(fx.calls) != 0 {
		t.Fatalf("flatpak must not run for a refused id: %v", fx.calls)
	}
}

func TestFlatpakInstallReportsLastOutputLine(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	run := fx.provider.run
	fx.provider.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] == "install" {
			return []byte("Looking for matches…\nerror: No remote refs found for 'org.example.Alpha'\n"), errors.New("exit status 1")
		}
		return run(ctx, args...)
	}
	err := fx.provider.Install(context.Background(), "org.example.Alpha")
	if err == nil || !strings.Contains(err.Error(), "No remote refs found") {
		t.Fatalf("error = %v", err)
	}
}

func TestFlatpakRemoveTargetsTheHoldingInstallation(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Installation: "system", Origin: "flathub"},
		{ID: "org.example.Beta", Installation: "user", Origin: "flathub"},
	})
	for id, scope := range map[string]string{"org.example.Alpha": "--system", "org.example.Beta": "--user"} {
		fx.calls = nil
		if err := fx.provider.Remove(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		want := [][]string{
			{"remotes", "--columns=name,url", scope},
			{"uninstall", "--noninteractive", "--assumeyes", scope, id},
		}
		if !reflect.DeepEqual(fx.calls, want) {
			t.Fatalf("Remove(%s) calls = %v, want %v", id, fx.calls, want)
		}
	}
	if err := fx.provider.Remove(context.Background(), "org.example.Missing"); err == nil {
		t.Fatal("removing an app that is not installed should fail")
	}
}

func TestFlatpakIsRegistered(t *testing.T) {
	if _, ok := providerFor(providers(), flatpakCategory); !ok {
		t.Fatal("flatpak provider is not registered")
	}
	if !storeSection(flatpakCategory) {
		t.Fatal("flatpak is not an openable store section")
	}
	if shouldWarmAssets(Item{Category: flatpakCategory}) {
		t.Fatal("flatpak icons must load lazily, not warm for every listed app")
	}
}

func TestInstallationFlag(t *testing.T) {
	for installation, want := range map[string]string{
		"user": "--user", "system": "--system", "": "--system", "extra": "--installation=extra",
	} {
		if got := installationFlag(installation); got != want {
			t.Errorf("installationFlag(%q) = %q, want %q", installation, got, want)
		}
	}
}

func TestPendingFlatpakUpdatesAsksEachInstallation(t *testing.T) {
	var asked [][]string
	run := func(_ context.Context, args ...string) ([]byte, error) {
		asked = append(asked, args)
		switch args[len(args)-1] {
		case "--user":
			return []byte("org.example.Alpha\tflathub\norg.example.Beta\tcommunity\nnot-an-id\tflathub\n"), nil
		case "--system":
			return nil, errors.New("exit status 1")
		}
		return []byte("org.example.Gamma\tflathub\n"), nil
	}
	got := pendingFlatpakUpdates(context.Background(), run, []string{"user", "system", "extra"})
	want := map[string]bool{"user\x00org.example.Alpha": true, "extra\x00org.example.Gamma": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending = %v, want %v (a failing installation is skipped, bad lines are ignored)", got, want)
	}
	wantAsked := [][]string{
		{"remote-ls", "--updates", "--app", "--columns=application:full,origin:full", "--user"},
		{"remote-ls", "--updates", "--app", "--columns=application:full,origin:full", "--system"},
		{"remote-ls", "--updates", "--app", "--columns=application:full,origin:full", "--installation=extra"},
	}
	if !reflect.DeepEqual(asked, wantAsked) {
		t.Fatalf("asked = %v, want %v", asked, wantAsked)
	}
}

func TestFlatpakLoadFlagsOnlyInstalledAppsWithAnUpdate(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Name: "Alpha", Installation: "system", Origin: "flathub"},
		{ID: "org.example.Zeta", Name: "Zeta", Installation: "user", Origin: "flathub"},
	})
	var installations []string
	fx.provider.updates = func(_ context.Context, names []string) map[string]bool {
		installations = names
		return map[string]bool{"system\x00org.example.Alpha": true, "user\x00org.example.Zeta": true, "user\x00org.example.Beta": true}
	}
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.ID] = it.UpdateAvailable
	}
	want := map[string]bool{"org.example.Alpha": true, "org.example.Beta": false, "org.example.Zeta": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("updateAvailable = %v, want %v (Beta is not installed, so an update is meaningless)", got, want)
	}
	if !reflect.DeepEqual(installations, []string{"system", "user"}) {
		t.Fatalf("installations probed = %v", installations)
	}
}

func TestFlatpakUpdateBelongsToSelectedInstallation(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Installation: "user", Origin: "flathub"},
		{ID: "org.example.Alpha", Installation: "system", Origin: "flathub"},
	})
	fx.provider.updates = func(_ context.Context, _ []string) map[string]bool {
		return map[string]bool{"system\x00org.example.Alpha": true}
	}
	items, _, err := fx.provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].UpdateAvailable || items[0].Metadata["installation"] != "user" {
		t.Fatalf("system update presented for user installation: %+v", items[0])
	}
}

func TestFlatpakLoadSkipsTheUpdateProbeWhenNothingIsInstalled(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.provider.updates = func(context.Context, []string) map[string]bool {
		t.Fatal("no installed app, so no installation needs probing")
		return nil
	}
	if _, _, err := fx.provider.Load(context.Background(), true); err != nil {
		t.Fatal(err)
	}
}

func TestFlatpakInstallOnAnInstalledAppUpdatesIt(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Alpha", Installation: "system", Origin: "flathub"},
		{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"},
	})
	for id, scope := range map[string]string{"org.example.Alpha": "--system", "org.example.Zeta": "--user"} {
		fx.calls = nil
		if err := fx.provider.Install(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		want := [][]string{
			{"remotes", "--columns=name,url", scope},
			{"update", "--noninteractive", "--assumeyes", scope, id},
		}
		if !reflect.DeepEqual(fx.calls, want) {
			t.Fatalf("Install(%s) calls = %v, want %v", id, fx.calls, want)
		}
	}
}

func TestFlatpakUpdateFailureNamesTheCause(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Alpha", Installation: "user", Origin: "flathub"}})
	run := fx.provider.run
	fx.provider.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] == "update" {
			return []byte("error: Failed to update: no space left\n"), errors.New("exit status 1")
		}
		return run(ctx, args...)
	}
	err := fx.provider.Install(context.Background(), "org.example.Alpha")
	if err == nil || !strings.Contains(err.Error(), "could not update org.example.Alpha") || !strings.Contains(err.Error(), "no space left") {
		t.Fatalf("error = %v", err)
	}
}

func TestFlatpakRemoveInACustomInstallation(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Alpha", Installation: "extra", Origin: "flathub"}})
	if err := fx.provider.Remove(context.Background(), "org.example.Alpha"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"remotes", "--columns=name,url", "--installation=extra"},
		{"uninstall", "--noninteractive", "--assumeyes", "--installation=extra", "org.example.Alpha"},
	}
	if !reflect.DeepEqual(fx.calls, want) {
		t.Fatalf("calls = %v, want %v", fx.calls, want)
	}
}

func TestFlatpakRemoveDoesNotUninstallOtherRemotes(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Zeta", Installation: "user", Origin: "community"}})
	if err := fx.provider.Remove(context.Background(), "org.example.Zeta"); err == nil {
		t.Fatal("a non-Flathub app must not be removed through the Flathub category")
	}
	if len(fx.calls) != 0 {
		t.Fatalf("unexpected commands: %v", fx.calls)
	}
}

func TestFlatpakDuplicateInstallationsPreferUser(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"},
		{ID: "org.example.Zeta", Installation: "system", Origin: "flathub"},
	})
	if err := fx.provider.Install(context.Background(), "org.example.Zeta"); err != nil {
		t.Fatal(err)
	}
	if err := fx.provider.Remove(context.Background(), "org.example.Zeta"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"remotes", "--columns=name,url", "--user"},
		{"update", "--noninteractive", "--assumeyes", "--user", "org.example.Zeta"},
		{"remotes", "--columns=name,url", "--user"},
		{"uninstall", "--noninteractive", "--assumeyes", "--user", "org.example.Zeta"},
	}
	if !reflect.DeepEqual(fx.calls, want) {
		t.Fatalf("calls = %v, want %v", fx.calls, want)
	}
}

func TestFlatpakRemoveFailure(t *testing.T) {
	fx := newFlatpakFixture(t, []installedFlatpak{{ID: "org.example.Zeta", Installation: "user", Origin: "flathub"}})
	run := fx.provider.run
	fx.provider.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] == "uninstall" {
			fx.calls = append(fx.calls, args)
			return []byte("error: not removable\n"), errors.New("exit status 1")
		}
		return run(ctx, args...)
	}
	if err := fx.provider.Remove(context.Background(), "org.example.Zeta"); err == nil {
		t.Fatal("expected the removal error")
	}
	if len(fx.calls) != 2 {
		t.Fatalf("failed removal ran extra commands: %v", fx.calls)
	}
}

func searchResult(hits ...map[string]any) map[string]any {
	list := make([]any, len(hits))
	for i, h := range hits {
		list[i] = h
	}
	return map[string]any{"hits": list}
}

func TestFlatpakSearchFindsAppsOutsideTheRanking(t *testing.T) {
	arch := flathubArch()
	fx := newFlatpakFixture(t, []installedFlatpak{
		{ID: "org.example.Rare", Name: "Rare", Version: "2", Installation: "user", Origin: "flathub"},
	})
	fx.found = searchResult(
		hit("org.example.Rare", "Rare", "desktop-application", true, arch),
		hit("org.example.Obscure", "Obscure", "desktop-application", false, arch),
		hit("org.example.Cli", "Cli", "console-application", true, arch),
		hit("org.example.Foreign", "Foreign", "desktop-application", true, "riscv64"),
		hit("--evil", "Evil", "desktop-application", true, arch),
	)
	items, err := fx.provider.Search(context.Background(), "  rare   thing ")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if want := []string{"org.example.Rare", "org.example.Obscure"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	if !items[0].Installed || items[0].InstalledVersion != "2" || items[1].Installed {
		t.Fatalf("installed state not joined: %+v / %+v", items[0], items[1])
	}
	if len(fx.searched) != 1 || fx.searched[0]["query"] != "rare thing" || fx.searched[0]["hits_per_page"] != float64(flathubSearchHits) {
		t.Fatalf("request = %v", fx.searched)
	}
}

func TestFlatpakSearchIgnoresTooShortQueriesWithoutAsking(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	for _, q := range []string{"", " ", "a", "  b "} {
		items, err := fx.provider.Search(context.Background(), q)
		if err != nil || len(items) != 0 || items == nil {
			t.Fatalf("Search(%q) = %v, %v; want an empty non-nil list", q, items, err)
		}
	}
	if len(fx.searched) != 0 {
		t.Fatalf("a short query must not reach Flathub: %v", fx.searched)
	}
}

func TestFlatpakSearchCapsTheQueryLength(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.found = searchResult()
	if _, err := fx.provider.Search(context.Background(), strings.Repeat("é", 500)); err != nil {
		t.Fatal(err)
	}
	if got := fx.searched[0]["query"].(string); len([]rune(got)) != flathubQueryMax {
		t.Fatalf("query is %d runes, want %d", len([]rune(got)), flathubQueryMax)
	}
}

func TestFlatpakSearchReportsAnUnreachableFlathub(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.failing = true
	if _, err := fx.provider.Search(context.Background(), "discord"); err == nil || !strings.Contains(err.Error(), "flathub search") {
		t.Fatalf("error = %v", err)
	}
}

type plainProvider struct{}

func (plainProvider) Category() Category { return Category{ID: "plain"} }
func (plainProvider) Load(context.Context, bool) ([]Item, SourceState, error) {
	return nil, SourceState{}, nil
}
func (plainProvider) Install(context.Context, string) error { return nil }
func (plainProvider) Remove(context.Context, string) error  { return nil }

func TestRunSearchPrintsItemsAndRejectsBadInvocations(t *testing.T) {
	fx := newFlatpakFixture(t, nil)
	fx.found = searchResult(hit("org.example.Obscure", "Obscure", "desktop-application", false, flathubArch()))
	provs := []Provider{fx.provider, plainProvider{}}

	var out strings.Builder
	if err := runSearch(&out, provs, []string{"flatpak", "-obscure"}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Category string `json:"category"`
		Query    string `json:"query"`
		Items    []Item `json:"items"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Category != "flatpak" || got.Query != "-obscure" || len(got.Items) != 1 || got.Items[0].ID != "org.example.Obscure" {
		t.Fatalf("output = %+v (a query that looks like a flag is still just the query)", got)
	}

	for name, args := range map[string][]string{
		"missing query":      {"flatpak"},
		"unknown category":   {"nope", "x"},
		"no search support":  {"plain", "xx"},
		"too many arguments": {"flatpak", "a", "b"},
	} {
		if err := runSearch(&strings.Builder{}, provs, args); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestFlatpakIconRejectsUntrustedOrigins(t *testing.T) {
	for _, icon := range []string{
		"http://dl.flathub.org/icon.png",
		"https://dl.flathub.org.evil.test/icon.png",
		"file:///etc/passwd",
		"https://127.0.0.1/private",
	} {
		if got := flathubIcon(icon); got != "" {
			t.Errorf("icon %q accepted as %q", icon, got)
		}
	}
	const icon = "https://dl.flathub.org/media/icon.png"
	if got := flathubIcon(icon); got != icon {
		t.Errorf("Flathub icon rejected: %q", got)
	}
}

func TestFlathubRejectsCrossOriginRedirect(t *testing.T) {
	fetched := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		fetched = true
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/private", http.StatusFound)
	}))
	defer source.Close()
	p := flatpakProvider{api: source.URL}
	if _, err := p.fetchPopular(context.Background()); err == nil {
		t.Fatal("cross-origin redirect accepted")
	}
	if fetched {
		t.Fatal("cross-origin target received a request")
	}
}
