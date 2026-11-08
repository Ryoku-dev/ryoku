package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	installerplan "ryoku-installerplan"
)

func packageSet(pkgs []string) map[string]bool {
	set := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		set[pkg] = true
	}
	return set
}

func unavailableZenTable(t *testing.T, edition string) ([]byte, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", edition, "packages", "translations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
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
	return []byte(strings.Join(lines, "\n")), reason
}

func TestChoicePackageFiltering(t *testing.T) {
	all := []string{
		"firefox", "chromium", "zen-browser-bin",
		"fish", "zsh", "zsh-autosuggestions", "zsh-history-substring-search",
		"zsh-syntax-highlighting", "ryoku-oh-my-zsh", "blesh",
		"bash", "starship", "fastfetch", "zoxide", "fzf", "eza", "bat", "mise",
	}
	for _, browser := range browserChoices {
		for _, shell := range loginShellChoices {
			name := browser + "/" + shell
			t.Run(name, func(t *testing.T) {
				p := &plan{browser: browser, shell: shell}
				got := packageSet(filterChoicePackages(all, p))
				for _, candidate := range browserChoices {
					// None has no package, so there is nothing to find or miss.
					pkg, installs := browserPackage[candidate]
					if !installs {
						continue
					}
					want := candidate == browser
					if got[pkg] != want {
						t.Errorf("browser package %s present=%v, want %v", pkg, got[pkg], want)
					}
				}
				for _, candidate := range loginShellChoices {
					want := candidate == shell
					for _, pkg := range shellStackPackages[candidate] {
						if got[pkg] != want {
							t.Errorf("shell package %s present=%v, want %v", pkg, got[pkg], want)
						}
					}
				}
				for _, pkg := range []string{"bash", "starship", "fastfetch", "zoxide", "fzf", "eza", "bat", "mise"} {
					if !got[pkg] {
						t.Errorf("shared package %s was filtered", pkg)
					}
				}
			})
		}
	}
}

func TestProvisionedLedgerSkipsInstalledAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	e := &engine{
		f: &facts{homeDir: home, distro: archLinux},
		p: &plan{browser: "firefox", shell: "fish"},
		installedPkg: func(pkg string) bool {
			return pkg == "chromium"
		},
	}
	if err := e.writeProvisionedLedger(); err != nil {
		t.Fatal(err)
	}
	if err := e.writeProvisionedLedger(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".local/state/ryoku/provisioned"))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, pkg := range strings.Fields(string(data)) {
		counts[pkg]++
	}
	if counts["chromium"] != 0 {
		t.Fatal("an already-installed package must not be recorded")
	}
	for _, pkg := range choiceDropPackages(e.p) {
		if pkg == "chromium" {
			continue
		}
		if counts[pkg] != 1 {
			t.Errorf("ledger count for %s = %d, want 1", pkg, counts[pkg])
		}
	}
}

func TestChoiceValidationAndEnvironmentDefaults(t *testing.T) {
	for _, browser := range browserChoices {
		if err := validateBrowser(browser); err != nil {
			t.Errorf("validateBrowser(%q): %v", browser, err)
		}
	}
	if err := validateBrowser("opera"); err == nil || !strings.Contains(err.Error(), "firefox, chromium, zen") {
		t.Fatalf("invalid browser error = %v", err)
	}
	for _, shell := range loginShellChoices {
		if err := validateLoginShell(shell); err != nil {
			t.Errorf("validateLoginShell(%q): %v", shell, err)
		}
	}
	if err := validateLoginShell("nu"); err == nil || !strings.Contains(err.Error(), "fish, zsh, bash") {
		t.Fatalf("invalid shell error = %v", err)
	}
	for _, fileManager := range fileManagerChoices {
		if err := validateFileManager(fileManager); err != nil {
			t.Errorf("validateFileManager(%q): %v", fileManager, err)
		}
	}
	if err := validateFileManager("pcmanfm"); err == nil || !strings.Contains(err.Error(), "nautilus, dolphin, thunar") {
		t.Fatalf("invalid file manager error = %v", err)
	}

	t.Setenv("RYOKU_BROWSER", "chromium")
	if got := envOr("RYOKU_BROWSER", "firefox"); got != "chromium" {
		t.Fatalf("browser env default = %q", got)
	}
	t.Setenv("RYOKU_BROWSER", "invalid")
	if err := validateBrowser(envOr("RYOKU_BROWSER", "firefox")); err == nil {
		t.Fatal("invalid browser from the environment must be rejected")
	}
	t.Setenv("RYOKU_LOGIN_SHELL", "invalid")
	if err := validateLoginShell(os.Getenv("RYOKU_LOGIN_SHELL")); err == nil {
		t.Fatal("invalid shell from the environment must be rejected")
	}
	t.Setenv("RYOKU_RASHIN", "0")
	if got, err := envToggle("RYOKU_RASHIN", true); err != nil || got {
		t.Fatalf("Rashin env toggle = %v, %v", got, err)
	}
	t.Setenv("RYOKU_CACHYOS_LAYER", "yes")
	if _, err := envToggle("RYOKU_CACHYOS_LAYER", false); err == nil {
		t.Fatal("invalid CachyOS layer environment value was accepted")
	}
	t.Setenv("RYOKU_GAMING", "yes")
	if _, err := envToggle("RYOKU_GAMING", false); err == nil {
		t.Fatal("invalid gaming environment value was accepted")
	}
	without, err := parseWithout("ryotunes, ryoport,ryotunes")
	if err != nil || strings.Join(without, ",") != "ryotunes,ryoport" {
		t.Fatalf("parsed --without = %v, %v", without, err)
	}
}

func TestEditionFlagsRejectUnavailableZen(t *testing.T) {
	for _, edition := range []string{"fedora", "void"} {
		t.Run(edition, func(t *testing.T) {
			data, reason := unavailableZenTable(t, edition)
			plan, err := parseEditionCompositors(data)
			if err != nil {
				t.Fatal(err)
			}
			err = validateEditionFlags(planOverrides{browser: "zen", shell: "fish", rashin: true}, plan)
			if err == nil {
				t.Fatal("--browser zen was accepted")
			}
			message := err.Error()
			if !strings.Contains(strings.ToLower(message), edition) ||
				!strings.Contains(message, "\"zen\"") ||
				!strings.Contains(message, reason) {
				t.Fatalf("unavailable browser error = %q", message)
			}
		})
	}
}

func TestShellFrontendMatchesSharedEditionPlan(t *testing.T) {
	for _, edition := range []string{"arch", "void", "fedora"} {
		t.Run(edition, func(t *testing.T) {
			var plan editionCompositorPlan
			if edition == "arch" {
				plan = sourceEditionChoices("Arch")
			} else {
				data, err := os.ReadFile(filepath.Join("..", edition, "packages", "translations.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				plan, err = parseEditionCompositors(data)
				if err != nil {
					t.Fatal(err)
				}
			}
			p := defaultPlanForEdition(&facts{userShell: "/usr/bin/zsh"}, plan)
			if p.browser != installerplan.FreshDefault(installerplan.Browser) || p.shell != "zsh" ||
				p.fileManager != installerplan.FreshDefault(installerplan.FileManager) {
				t.Fatalf("defaults = browser %q, shell %q, file manager %q", p.browser, p.shell, p.fileManager)
			}
			items := buildItems(&facts{}, p, plan)
			wantByLabel := map[string]string{
				"Compositor":   strings.Join(plan.choices.AvailableIDs(installerplan.Compositor), ","),
				"Browser":      strings.Join(plan.choices.AvailableIDs(installerplan.Browser), ","),
				"Login shell":  strings.Join(plan.choices.AvailableIDs(installerplan.LoginShell), ","),
				"File manager": strings.Join(plan.choices.AvailableIDs(installerplan.FileManager), ","),
			}
			for _, row := range items {
				want, ok := wantByLabel[row.label]
				if !ok {
					continue
				}
				var got []string
				for _, option := range row.options {
					got = append(got, option.key)
				}
				if strings.Join(got, ",") != want {
					t.Errorf("%s choices = %q, want %q", row.label, strings.Join(got, ","), want)
				}
				delete(wantByLabel, row.label)
			}
			if len(wantByLabel) != 0 {
				t.Fatalf("missing choice rows: %v", wantByLabel)
			}
			gotDrops := strings.Join(choiceDropPackages(p), ",")
			wantDrops := installerplan.DropPackages(p.browser, p.shell)
			wantDrops = append(wantDrops, installerplan.DropPackagesFor(installerplan.FileManager, p.fileManager)...)
			wantDrops = append(wantDrops, plan.choices.DropComponentPackages(p.componentSelections())...)
			wantDrops = uniquePackages(wantDrops)
			if gotDrops != strings.Join(wantDrops, ",") {
				t.Fatalf("choice drops = %q, want %q", gotDrops, strings.Join(wantDrops, ","))
			}
			if gaming, ok := plan.choices.GamingComponent(); ok {
				p.setComponent(gaming.Component.ID, true)
				drops := packageSet(choiceDropPackages(p))
				for _, pkg := range gaming.Component.Packages {
					if drops[pkg] {
						t.Errorf("selected gaming package %s was dropped", pkg)
					}
				}
			}
		})
	}
}

func TestZenForcesAURAndAddsItsPackage(t *testing.T) {
	p := &plan{browser: "firefox", shell: "fish", aur: false}
	applyPlanChoices(p, "zen", "", "")
	if !p.aur {
		t.Fatal("Zen must force the AUR step on")
	}
	if !packageSet(selectedAURPackages(p))["zen-browser-bin"] {
		t.Fatal("Zen selection must add zen-browser-bin to the AUR transaction")
	}

	items := buildItems(&facts{kbLayout: "de"}, p)
	aur := -1
	for i := range items {
		if items[i].label == "AUR extras" {
			aur = i
			break
		}
	}
	if aur < 0 || !items[aur].locked {
		t.Fatal("Zen must lock the AUR toggle on")
	}
	m := model{state: "plan", p: p, items: items, sel: aur}
	next, _ := m.onKey(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Text: " "}))
	if !next.(model).p.aur {
		t.Fatal("the locked AUR row must ignore the toggle key")
	}
}
func TestDefaultPlanUsesSupportedCurrentShell(t *testing.T) {
	for _, shell := range loginShellChoices {
		p := defaultPlan(&facts{userShell: "/usr/bin/" + shell})
		if p.shell != shell {
			t.Errorf("current shell %s selected %s", shell, p.shell)
		}
	}
	if got := defaultPlan(&facts{userShell: "/usr/bin/nu"}).shell; got != "fish" {
		t.Fatalf("unsupported current shell falls back to %q, want fish", got)
	}
}

func TestDefaultRiceMarkerOnlyForFreshAccount(t *testing.T) {
	home := t.TempDir()
	e := &engine{f: &facts{homeDir: home}, freshAccount: true}
	if err := e.writeDefaultRiceMarker(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".local/state/ryoku/default-rice-pending")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "default\n" {
		t.Fatalf("marker = %q, want default newline", data)
	}

	other := t.TempDir()
	e = &engine{f: &facts{homeDir: other}}
	if err := e.writeDefaultRiceMarker(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, ".local/state/ryoku/default-rice-pending")); !os.IsNotExist(err) {
		t.Fatalf("non-fresh account marker stat error = %v, want not exist", err)
	}
}

func TestDefaultPlanPreselectsInstalledFileManager(t *testing.T) {
	edition := sourceEditionChoices("Arch")
	for _, current := range []string{"nautilus", "dolphin", "thunar"} {
		p := defaultPlanForEdition(&facts{fileManager: current}, edition)
		if p.fileManager != current {
			t.Errorf("installed file manager %q selected %q", current, p.fileManager)
		}
	}
	if got := defaultPlanForEdition(&facts{fileManager: "pcmanfm"}, edition).fileManager; got != "nautilus" {
		t.Fatalf("unsupported file manager selected %q, want nautilus", got)
	}
}

func TestComponentsOfferedPerEdition(t *testing.T) {
	plans := map[string]editionCompositorPlan{
		"arch":    sourceEditionChoices("Arch"),
		"cachyos": sourceEditionChoices("CachyOS"),
	}
	for _, edition := range []string{"fedora", "void"} {
		data, err := os.ReadFile(filepath.Join("..", edition, "packages", "translations.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		plans[edition], err = parseEditionCompositors(data)
		if err != nil {
			t.Fatal(err)
		}
	}
	for name, edition := range plans {
		t.Run(name, func(t *testing.T) {
			p := defaultPlanForEdition(&facts{}, edition)
			items := buildItems(&facts{}, p, edition)
			byLabel := map[string]planItem{}
			for _, item := range items {
				byLabel[item.label] = item
			}
			for _, label := range []string{"Ryotunes", "Ryoport", "RyoManager", "Rashin"} {
				if _, ok := byLabel[label]; !ok {
					t.Errorf("%s row is missing", label)
				}
			}
			gaming, hasGaming := edition.choices.GamingComponent()
			if name == "fedora" {
				if hasGaming {
					t.Fatal("Fedora plan exposes gaming")
				}
			} else {
				if !hasGaming {
					t.Fatal("gaming row is missing")
				}
				item, ok := byLabel[gaming.Component.Label]
				if !ok || item.on == nil {
					t.Fatalf("%s gaming toggle is missing", gaming.Component.Label)
				}
				if *item.on || p.componentOn(gaming.Component.ID) {
					t.Fatal("gaming must default off")
				}
			}
			if byLabel["Rashin"].locked {
				t.Errorf("Rashin is locked on %s", name)
			}
		})
	}
}

func TestDropsDeselectedComponentsAndLosingFileManagers(t *testing.T) {
	edition := sourceEditionChoices("Arch")
	p := defaultPlanForEdition(&facts{}, edition)
	p.fileManager = "dolphin"
	p.setComponent("ryotunes", false)
	p.setComponent("ryoport", false)
	p.setComponent("ryomanager", false)
	p.setComponent("rashin", false)
	drops := packageSet(choiceDropPackages(p))
	for _, pkg := range []string{"nautilus", "nautilus-python", "thunar", "ryotunes", "ryoport", "ryomanager", "ryoku-rashin"} {
		if !drops[pkg] {
			t.Errorf("missing drop %s", pkg)
		}
	}
	if drops["dolphin"] {
		t.Fatal("chosen file manager was dropped")
	}
}

func TestDeselectedRyoportDropsPackageWithoutDesktopHide(t *testing.T) {
	p := defaultPlanForEdition(&facts{}, sourceEditionChoices("Arch"))
	p.setComponent("ryoport", false)
	if !packageSet(choiceDropPackages(p))["ryoport"] {
		t.Fatal("deselected Ryoport package was not dropped")
	}
	if hidden := p.choiceEdition().HiddenDesktopIDs(p.componentSelections()); len(hidden) != 0 {
		t.Fatalf("deselected Ryoport emitted desktop hide entries: %v", hidden)
	}
}

func TestPlanOverridesValidateAndApplyComponents(t *testing.T) {
	edition := sourceEditionChoices("Arch")
	overrides := planOverrides{
		browser: "firefox", shell: "fish", fileManager: "dolphin",
		rashin: false, cachyosLayer: true, without: []string{"ryotunes", "ryoport"},
	}
	if err := validateEditionFlags(overrides, edition); err != nil {
		t.Fatal(err)
	}
	p := defaultPlanForEdition(&facts{}, edition)
	applyPlanOverrides(p, overrides)
	if p.fileManager != "dolphin" || p.componentOn("rashin") || p.componentOn("ryotunes") ||
		p.componentOn("ryoport") || !p.componentOn("cachyos-layer") {
		t.Fatalf("overrides were not applied: file manager=%s components=%v", p.fileManager, p.componentSelections())
	}

	if err := validateEditionFlags(planOverrides{
		browser: "firefox", rashin: true, without: []string{"not-a-component"},
	}, edition); err == nil {
		t.Fatal("unknown component was accepted")
	}

	cachyos := sourceEditionChoices("CachyOS")
	gaming := planOverrides{browser: "firefox", rashin: true, gaming: true}
	if err := validateEditionFlags(gaming, cachyos); err != nil {
		t.Fatal(err)
	}
	cachyosPlan := defaultPlanForEdition(&facts{}, cachyos)
	applyPlanOverrides(cachyosPlan, gaming)
	if !cachyosPlan.componentOn("cachyos-gaming") {
		t.Fatal("--gaming did not select the CachyOS gaming component")
	}

	voidData, err := os.ReadFile(filepath.Join("..", "void", "packages", "translations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	voidEdition, err := parseEditionCompositors(voidData)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEditionFlags(gaming, voidEdition); err != nil {
		t.Fatal(err)
	}
	voidPlan := defaultPlanForEdition(&facts{}, voidEdition)
	applyPlanOverrides(voidPlan, gaming)
	if !voidPlan.componentOn("void-gaming") {
		t.Fatal("--gaming did not select the Void gaming component")
	}
	if err := validateEditionFlags(planOverrides{browser: "firefox", rashin: true, gaming: true}, edition); err == nil {
		t.Fatal("--gaming was accepted on Arch instead of --cachyos-layer")
	}
	for name, target := range map[string]editionCompositorPlan{
		"CachyOS": cachyos,
		"Void":    voidEdition,
	} {
		if err := validateEditionFlags(planOverrides{browser: "firefox", rashin: true, cachyosLayer: true}, target); err == nil {
			t.Errorf("--cachyos-layer was accepted on %s", name)
		}
	}
	fedoraData, err := os.ReadFile(filepath.Join("..", "fedora", "packages", "translations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	fedoraEdition, err := parseEditionCompositors(fedoraData)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEditionFlags(planOverrides{browser: "firefox", rashin: true, cachyosLayer: true}, fedoraEdition); err == nil {
		t.Fatal("--cachyos-layer was accepted on Fedora")
	}
	if err := validateEditionFlags(gaming, fedoraEdition); err == nil {
		t.Fatal("--gaming was accepted on Fedora")
	}
}
