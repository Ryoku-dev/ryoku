package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packageSet(pkgs []string) map[string]bool {
	set := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		set[pkg] = true
	}
	return set
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
					want := candidate == browser
					if got[browserPackage[candidate]] != want {
						t.Errorf("browser package %s present=%v, want %v", browserPackage[candidate], got[browserPackage[candidate]], want)
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
}

func TestZenForcesAURAndAddsItsPackage(t *testing.T) {
	p := &plan{browser: "firefox", shell: "fish", aur: false}
	applyPlanChoices(p, "zen", "")
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
	next, _ := m.onKey("space")
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
