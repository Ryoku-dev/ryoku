package installerplan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func editionTable(t *testing.T, edition string) []byte {
	t.Helper()
	if edition == "arch" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join("..", "..", edition, "packages", "translations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func unavailableZenFixture(t *testing.T, edition string) (string, string) {
	t.Helper()
	lines := strings.Split(string(editionTable(t, edition)), "\n")
	reason := "Fixture " + edition + " Zen package is unavailable."
	found := false
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || fields[0] != "zen-browser-bin" {
			continue
		}
		fields[1] = "-"
		fields[3] = reason
		lines[i] = strings.Join(fields, "\t")
		found = true
	}
	if !found {
		t.Fatal("zen-browser-bin row not found")
	}
	path := filepath.Join(t.TempDir(), "translations.tsv")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, reason
}

func resolvedPlan(t *testing.T, edition string) EditionPlan {
	t.Helper()
	if edition == "arch" {
		return ResolveSource("arch")
	}
	plan, err := ResolveEdition(editionTable(t, edition))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestEditionChoicesDefaultsAndDrops(t *testing.T) {
	for _, test := range []struct {
		edition      string
		browsers     string
		shells       string
		fileManagers string
		compositors  string
	}{
		{edition: "arch", browsers: "firefox,chromium,zen,none", shells: "fish,zsh,bash", fileManagers: "nautilus,dolphin,thunar", compositors: "hyprland,niri,wayfire"},
		{edition: "void", browsers: "firefox,chromium,none", shells: "fish,zsh,bash", fileManagers: "nautilus,dolphin,thunar", compositors: "niri"},
		{edition: "fedora", browsers: "firefox,chromium,none", shells: "fish,zsh,bash", fileManagers: "nautilus,dolphin,thunar", compositors: "hyprland,niri"},
	} {
		t.Run(test.edition, func(t *testing.T) {
			plan := resolvedPlan(t, test.edition)
			if got := strings.Join(plan.AvailableIDs(Browser), ","); test.browsers != "" && got != test.browsers {
				t.Fatalf("browsers = %q, want %q", got, test.browsers)
			}
			if got := strings.Join(plan.AvailableIDs(LoginShell), ","); got != test.shells {
				t.Fatalf("shells = %q, want %q", got, test.shells)
			}
			if got := strings.Join(plan.AvailableIDs(FileManager), ","); got != test.fileManagers {
				t.Fatalf("file managers = %q, want %q", got, test.fileManagers)
			}
			if got := strings.Join(plan.AvailableIDs(Compositor), ","); got != test.compositors {
				t.Fatalf("compositors = %q, want %q", got, test.compositors)
			}
			if got := FreshDefault(Browser); got != "firefox" {
				t.Fatalf("fresh browser = %q, want firefox", got)
			}
			if got := FreshDefault(LoginShell); got != "fish" {
				t.Fatalf("fresh shell = %q, want fish", got)
			}
			if got := FreshDefault(FileManager); got != "nautilus" {
				t.Fatalf("fresh file manager = %q, want nautilus", got)
			}
			for _, kind := range []string{Browser, LoginShell, FileManager} {
				id := FreshDefault(kind)
				availability, ok := plan.Lookup(kind, id)
				if !ok || !availability.Available {
					t.Fatalf("fresh %s default %q is unavailable: %+v", kind, id, availability)
				}
			}
			if got, available := FreshSelection(Compositor, plan.AvailableIDs(Compositor)), plan.AvailableIDs(Compositor); len(available) == 0 || got != available[0] {
				t.Fatalf("fresh compositor = %q, available %v", got, available)
			}
			if got := ExistingLoginShell("/usr/bin/zsh", plan.AvailableIDs(LoginShell)); got != "zsh" {
				t.Fatalf("supported existing shell = %q, want zsh", got)
			}
			if got := ExistingLoginShell("/usr/bin/nu", plan.AvailableIDs(LoginShell)); got != "fish" {
				t.Fatalf("unsupported existing shell = %q, want fish", got)
			}
			if got := ExistingFileManager("/usr/bin/dolphin", plan.AvailableIDs(FileManager)); got != "dolphin" {
				t.Fatalf("supported existing file manager = %q, want dolphin", got)
			}
			if got := ExistingFileManager("/usr/bin/pcmanfm", plan.AvailableIDs(FileManager)); got != "nautilus" {
				t.Fatalf("unsupported existing file manager = %q, want nautilus", got)
			}
			wantDrops := "chromium,zen-browser-bin,zsh,zsh-autosuggestions,zsh-history-substring-search,zsh-syntax-highlighting,ryoku-oh-my-zsh,blesh"
			if got := strings.Join(DropPackages("firefox", "fish"), ","); got != wantDrops {
				t.Fatalf("drops = %q, want %q", got, wantDrops)
			}
		})
	}
}

func TestUnavailableBrowserFixturesCarryTableReason(t *testing.T) {
	for _, edition := range []string{"void", "fedora"} {
		t.Run(edition, func(t *testing.T) {
			path, reason := unavailableZenFixture(t, edition)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := ResolveEdition(data)
			if err != nil {
				t.Fatal(err)
			}
			availability, ok := plan.Lookup(Browser, "zen")
			if !ok || availability.Available || availability.Reason != reason {
				t.Fatalf("zen availability = %+v, found %v", availability, ok)
			}
		})
	}
}

func TestBashResolverReadsTheSameManifest(t *testing.T) {
	manifest, err := filepath.Abs("choices.tsv")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := filepath.Abs(filepath.Join("..", "backend", "lib", "choices.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, edition := range []string{"arch", "void", "fedora"} {
		t.Run(edition, func(t *testing.T) {
			plan := resolvedPlan(t, edition)
			table := ""
			if edition != "arch" {
				table, err = filepath.Abs(filepath.Join("..", "..", edition, "packages", "translations.tsv"))
				if err != nil {
					t.Fatal(err)
				}
			}
			script := `set -euo pipefail
` +
				`die() { printf "$1\n" "${@:2}" >&2; exit 1; }
` +
				`source "$HELPER"
` +
				`RYOKU_BROWSER= RYOKU_LOGIN_SHELL= RYOKU_FILE_MANAGER= RYOKU_COMPOSITOR= RYOKU_DROP_PACKAGES=
` +
				`ryoku_choice_apply_fresh "$EDITION" "$TABLE"
` +
				`printf 'browser=%s\nshell=%s\nfiles=%s\ndrops=%s\n' "$RYOKU_BROWSER" "$RYOKU_LOGIN_SHELL" "$RYOKU_FILE_MANAGER" "$RYOKU_DROP_PACKAGES"
` +
				`printf 'browsers='; for id in "${RYOKU_BROWSER_CHOICES[@]}"; do [[ ${RYOKU_CHOICE_AVAILABLE[browser:$id]} == 1 ]] && printf '%s,' "$id"; done; printf '\n'
` +
				`printf 'shells='; for id in "${RYOKU_LOGIN_SHELL_CHOICES[@]}"; do [[ ${RYOKU_CHOICE_AVAILABLE[login-shell:$id]} == 1 ]] && printf '%s,' "$id"; done; printf '\n'
` +
				`printf 'files='; for id in "${RYOKU_FILE_MANAGER_CHOICES[@]}"; do [[ ${RYOKU_CHOICE_AVAILABLE[file-manager:$id]} == 1 ]] && printf '%s,' "$id"; done; printf '\n'
` +
				`printf 'compositors='; for id in "${RYOKU_COMPOSITOR_CHOICES[@]}"; do [[ ${RYOKU_CHOICE_AVAILABLE[compositor:$id]} == 1 ]] && printf '%s,' "$id"; done; printf '\n'
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(),
				"HELPER="+helper,
				"RYOKU_INSTALLER_CHOICES_FILE="+manifest,
				"EDITION="+edition,
				"TABLE="+table,
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("bash resolver: %v\n%s", err, output)
			}
			drop := append(DropPackages("firefox", "fish"), DropPackagesFor(FileManager, "nautilus")...)
			if rashin, ok := plan.Component("rashin"); ok &&
				!componentSelected(rashin, plan.DefaultComponents()) {
				drop = append(drop, rashin.Component.Packages...)
			}
			want := "browser=firefox\nshell=fish\nfiles=nautilus\ndrops=" + strings.Join(drop, ",") + "\n" +
				"browsers=" + strings.Join(plan.AvailableIDs(Browser), ",") + ",\n" +
				"shells=" + strings.Join(plan.AvailableIDs(LoginShell), ",") + ",\n" +
				"files=" + strings.Join(plan.AvailableIDs(FileManager), ",") + ",\n" +
				"compositors=" + strings.Join(plan.AvailableIDs(Compositor), ",") + ",\n"
			if string(output) != want {
				t.Fatalf("bash plan:\n%s\nwant:\n%s", output, want)
			}
		})
	}
}

func TestBashResolverRejectsUnavailableBrowser(t *testing.T) {
	manifest, err := filepath.Abs("choices.tsv")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := filepath.Abs(filepath.Join("..", "backend", "lib", "choices.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, edition := range []string{"void", "fedora"} {
		t.Run(edition, func(t *testing.T) {
			table, reason := unavailableZenFixture(t, edition)
			script := `set -euo pipefail
die() { printf "$1\n" "${@:2}" >&2; exit 1; }
source "$HELPER"
RYOKU_BROWSER=zen RYOKU_LOGIN_SHELL=fish RYOKU_FILE_MANAGER=nautilus RYOKU_COMPOSITOR= RYOKU_DROP_PACKAGES=
ryoku_choice_apply_fresh "$EDITION" "$TABLE"
`
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(),
				"HELPER="+helper,
				"RYOKU_INSTALLER_CHOICES_FILE="+manifest,
				"EDITION="+edition,
				"TABLE="+table,
			)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("zen was accepted")
			}
			message := string(output)
			if !strings.Contains(strings.ToLower(message), edition) ||
				!strings.Contains(message, "'zen'") ||
				!strings.Contains(message, reason) {
				t.Fatalf("unavailable browser error = %q", message)
			}
		})
	}
}

func TestComponentAvailabilityByEdition(t *testing.T) {
	for _, test := range []struct {
		edition      string
		aurAvailable bool
		gamingID     string
		needsNetwork bool
	}{
		{edition: "arch", aurAvailable: true, gamingID: "cachyos-layer", needsNetwork: true},
		{edition: "cachyos", aurAvailable: true, gamingID: "cachyos-gaming"},
		{edition: "void", gamingID: "void-gaming"},
		{edition: "fedora"},
	} {
		t.Run(test.edition, func(t *testing.T) {
			var plan EditionPlan
			if test.edition == "cachyos" {
				plan = ResolveSource("cachyos")
			} else {
				plan = resolvedPlan(t, test.edition)
			}
			for _, id := range []string{"ryotunes", "ryoport", "ryomanager", "node"} {
				availability, ok := plan.Component(id)
				if !ok || !availability.Available {
					t.Errorf("%s availability = %+v, found %v", id, availability, ok)
				}
			}
			for _, id := range []string{"localsend"} {
				availability, ok := plan.Component(id)
				if !ok || availability.Available != test.aurAvailable {
					t.Errorf("%s availability = %+v, want available %v", id, availability, test.aurAvailable)
				}
			}
			gaming, ok := plan.GamingComponent()
			if test.gamingID == "" {
				if ok {
					t.Errorf("gaming component = %+v, want none", gaming)
				}
			} else if !ok || gaming.Component.ID != test.gamingID ||
				gaming.Component.Step != StepGaming ||
				gaming.Component.DefaultOn ||
				gaming.Component.NeedsNetwork != test.needsNetwork {
				t.Errorf("gaming component = %+v, want id=%q network=%v", gaming, test.gamingID, test.needsNetwork)
			}
			rashin, ok := plan.Component("rashin")
			if !ok || !rashin.Available || rashin.Locked || rashin.Component.DefaultOn {
				t.Errorf("rashin availability = %+v, want an available, unlocked, default-off choice", rashin)
			}
			terminal, ok := plan.Component("terminal")
			if !ok || !terminal.Available || !terminal.Locked ||
				terminal.Reason != "the Ryoku terminal role: the launcher, Settings, and open in terminal use it" {
				t.Errorf("required terminal row = %+v", terminal)
			}
		})
	}
}

func TestGamingComponentPackagesAndDrops(t *testing.T) {
	tests := []struct {
		edition  string
		id       string
		packages string
		dropNo   bool
	}{
		{
			edition: "arch", id: "cachyos-layer",
			packages: "linux-cachyos,linux-cachyos-headers,cachyos-settings,scx-scheds,cachyos-gaming-meta,steam",
		},
		{
			edition: "cachyos", id: "cachyos-gaming",
			packages: "steam,cachyos-gaming-meta", dropNo: true,
		},
		{
			edition: "void", id: "void-gaming",
			packages: "steam,lib32-mesa,lib32-vulkan-icd-loader,lib32-gcc-libs,lib32-libglvnd,lib32-libdrm,lib32-libva", dropNo: true,
		},
	}
	for _, test := range tests {
		t.Run(test.edition, func(t *testing.T) {
			plan := ResolveSource(test.edition)
			gaming, ok := plan.GamingComponent()
			if !ok || gaming.Component.ID != test.id {
				t.Fatalf("gaming component = %+v, want %s", gaming, test.id)
			}
			if got := strings.Join(gaming.Component.Packages, ","); got != test.packages {
				t.Fatalf("packages = %q, want %q", got, test.packages)
			}
			no := plan.DefaultComponents()
			no[test.id] = false
			gotNo := strings.Join(plan.DropComponentPackages(no), ",")
			if test.dropNo && !strings.Contains(gotNo, test.packages) {
				t.Errorf("No drop list = %q, want gaming packages %q", gotNo, test.packages)
			}
			if !test.dropNo && strings.Contains(gotNo, "linux-cachyos") {
				t.Errorf("network-only Arch gaming package appears in No drops: %q", gotNo)
			}
			yes := plan.DefaultComponents()
			yes[test.id] = true
			gotYes := strings.Join(plan.DropComponentPackages(yes), ",")
			for _, pkg := range gaming.Component.Packages {
				if strings.Contains(","+gotYes+",", ","+pkg+",") {
					t.Errorf("Yes drop list %q contains %s", gotYes, pkg)
				}
			}
		})
	}
}

func TestComponentDropAndHideRules(t *testing.T) {
	contains := func(values []string, want string) bool {
		for _, value := range values {
			if value == want {
				return true
			}
		}
		return false
	}

	arch := ResolveSource("arch")
	arch.components = append(arch.components, ComponentAvailability{
		Component: Component{
			ID: "default-off-payload", DefaultOn: false,
			Packages: []string{"not-installed"}, DesktopIDs: []string{"not-installed.desktop"},
		},
		Available: true,
	})
	selected := arch.DefaultComponents()
	selected["ryoport"] = false
	selected["ryomanager"] = false
	selected["rashin"] = false
	drop := arch.DropComponentPackages(selected)
	for _, want := range []string{"ryoport", "ryomanager", "ryoku-rashin"} {
		if !contains(drop, want) {
			t.Errorf("drop packages %v missing %s", drop, want)
		}
	}
	for _, absent := range []string{"linux-cachyos", "not-installed"} {
		if contains(drop, absent) {
			t.Errorf("default-off package %s appears in drops %v", absent, drop)
		}
	}
	if got := strings.Join(arch.HiddenDesktopIDs(selected), ","); got != "" {
		t.Fatalf("hidden desktop ids = %q, want none", got)
	}

	for _, edition := range []string{"void", "fedora"} {
		plan := resolvedPlan(t, edition)
		planSelected := plan.DefaultComponents()
		planSelected["rashin"] = false
		if !contains(plan.DropComponentPackages(planSelected), "ryoku-rashin") {
			t.Fatalf("%s kept ryoku-rashin after Rashin was turned off", edition)
		}
	}

	if got, want := strings.Join(DropPackagesFor(FileManager, "nautilus"), ","), "dolphin,thunar"; got != want {
		t.Fatalf("nautilus file-manager drops = %q, want %q", got, want)
	}
	if got, want := strings.Join(DropPackagesFor(FileManager, "dolphin"), ","), "nautilus,nautilus-python,thunar"; got != want {
		t.Fatalf("dolphin file-manager drops = %q, want %q", got, want)
	}
}
