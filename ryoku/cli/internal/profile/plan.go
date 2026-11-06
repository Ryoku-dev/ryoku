package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
)

func BuildPlan(m *Manifest, src Source, acct AccountTarget, sel Selection, host HostFacts, prev *SystemReceipt, res Resolver) (*Plan, error) {
	p := &Plan{Schema: API, Repos: []RepoPlan{}, Packages: []PackageRef{}, Remove: []RemovalPackage{}, System: []SystemPlan{}, Env: []EnvVar{}, Bleed: []BleedEntry{}, Warnings: []string{}, Blockers: []string{}, manifest: m, source: src}
	p.Profile = PlanProfile{ID: m.ID, Version: m.Version, Name: m.Name, ManifestSHA256: src.ManifestSHA256, Source: src.Dir}
	homeRoot := host.HomeRoot
	if homeRoot == "" {
		homeRoot = "/home"
	}
	home := acct.Home
	if home == "" {
		home = filepath.Join(homeRoot, acct.Name)
	}
	p.Account = PlanAccount{Name: acct.Name, FullName: acct.FullName, Role: m.Account.Role, Permissions: append([]string{}, m.Account.Permissions...), Groups: append([]string{GroupName(m.ID)}, m.Account.Groups...), Shell: m.Account.Shell, Exists: acct.Exists, Home: home, ManagedHome: host.ManagedHomes}
	if p.Account.FullName == "" {
		p.Account.FullName = m.Account.FullName
	}
	if host.PackageManager != "pacman" {
		p.Blockers = append(p.Blockers, i18n.T("Profiles need Arch or CachyOS."))
	}
	if acct.Exists && acct.ProfileID == "" {
		p.Blockers = append(p.Blockers, fmt.Sprintf(i18n.T("Account %s already exists and is not a profile account."), acct.Name))
	}
	if acct.Exists && acct.ProfileID != "" && acct.ProfileID != m.ID {
		p.Blockers = append(p.Blockers, fmt.Sprintf(i18n.T("Account %s belongs to profile %s."), acct.Name, acct.ProfileID))
	}
	if !contains(host.Shells, m.Account.Shell) {
		p.Blockers = append(p.Blockers, fmt.Sprintf(i18n.T("Shell %s is not listed in /etc/shells."), m.Account.Shell))
	}
	if !host.ManagedHomes {
		p.Warnings = append(p.Warnings, i18n.T("Home checkpoints are unavailable on this filesystem."))
	}
	for _, capability := range m.Requires.Capabilities {
		if !host.Capabilities[capability] {
			p.Warnings = append(p.Warnings, fmt.Sprintf(i18n.T("The active window manager does not support %s."), capability))
		}
	}
	for _, repo := range m.Repos {
		action := "enable"
		if repo == "blackarch" && host.BlackArchEnabled {
			action = "already"
		}
		p.Repos = append(p.Repos, RepoPlan{ID: repo, Action: action, Detail: "signed repository database"})
	}
	comps, extras, targetErr := m.Targets(sel)
	if targetErr != nil {
		p.Blockers = append(p.Blockers, targetErr.Error())
	}
	componentByName := map[string]Component{}
	var names []string
	for _, c := range comps {
		names = append(names, c.Name)
		componentByName[c.Name] = c
	}
	names = append(names, extras...)
	if len(extras) > 0 {
		p.Warnings = append(p.Warnings, i18n.T("Extra packages may add apps to every account; their launchers are listed after install."))
	}
	packagePlanReady := false
	if host.PackageManager == "pacman" && targetErr == nil && res != nil {
		packages, err := res.Resolve(names, m.Repos)
		if err != nil {
			p.Blockers = append(p.Blockers, resolverBlocker(err))
		} else {
			for i := range packages {
				if c, ok := componentByName[packages[i].Name]; ok {
					packages[i].Component, packages[i].Group = c.Name, c.Group
				}
				if !packages[i].PreExisting {
					p.Totals.Download += packages[i].Download
					p.Totals.Installed += packages[i].Installed
					if packages[i].Target {
						p.Totals.Targets++
					} else {
						p.Totals.Dependencies++
					}
				}
			}
			p.Packages = packages
			packagePlanReady = true
		}
	}
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	if host.PackageManager == "pacman" && prev != nil && res != nil {
		claimed := map[string]bool{}
		if receipts, err := ReadSystemReceipts(SystemRoot); err == nil {
			for _, receipt := range receipts {
				if receipt.Profile.ID == prev.Profile.ID && receipt.Account.Name == prev.Account.Name {
					continue
				}
				for _, name := range receipt.Packages.Introduced {
					claimed[name] = true
				}
				for _, name := range receipt.Packages.PreExisting {
					claimed[name] = true
				}
			}
		}
		var candidates []string
		for _, name := range prev.Packages.Introduced {
			if !selected[name] && !claimed[name] {
				candidates = append(candidates, name)
			}
		}
		if len(candidates) > 0 {
			required, err := res.Required(candidates)
			if err != nil {
				p.Blockers = append(p.Blockers, resolverBlocker(err))
			} else {
				for _, name := range candidates {
					if len(required[name]) == 0 {
						p.Remove = append(p.Remove, RemovalPackage{Name: name})
					}
				}
			}
		}
	}
	if packagePlanReady {
		componentNames := make([]string, 0, len(comps))
		for _, component := range comps {
			componentNames = append(componentNames, component.Name)
		}
		installed, err := res.Installed(componentNames)
		if err != nil {
			p.Blockers = append(p.Blockers, resolverBlocker(err))
		} else {
			for _, component := range comps {
				if installed[component.Name] {
					continue
				}
				for _, id := range component.DesktopEntries {
					p.Bleed = append(p.Bleed, BleedEntry{DesktopID: id, Name: component.Name, Package: component.Name})
				}
			}
		}
	}
	selectedTools := map[string]bool{}
	for _, component := range comps {
		selectedTools[filepath.Base(component.Detect)] = true
	}
	capabilityCount := 0
	for _, cap := range m.Capabilities {
		if !selectedTools[cap.Tool] {
			continue
		}
		p.System = append(p.System, SystemPlan{Kind: "capability", Path: filepath.Join(ProfileBinDir(m.ID), cap.Tool), Detail: cap.Caps})
		capabilityCount++
	}
	p.System = append(p.System, SystemPlan{Kind: "group", Path: GroupName(m.ID), Detail: "profile account access"})
	if capabilityCount > 0 || len(m.Wordlists.Links)+len(m.Wordlists.Extract) > 0 {
		p.System = append(p.System, SystemPlan{Kind: "pacman-hook", Path: "/etc/pacman.d/hooks/profile-refresh.hook"})
	}
	for _, link := range m.Wordlists.Links {
		p.System = append(p.System, SystemPlan{Kind: "wordlists", Path: filepath.Join(WordlistsRoot, filepath.FromSlash(link.Name)), Detail: link.Target})
	}
	for _, extract := range m.Wordlists.Extract {
		p.System = append(p.System, SystemPlan{Kind: "wordlists", Path: filepath.Join(WordlistsRoot, filepath.FromSlash(extract.Name)), Detail: extract.Archive})
	}
	if len(m.Hosts.Suffixes) > 0 {
		p.System = append(p.System, SystemPlan{Kind: "hosts", Path: "/etc/hosts", Detail: strings.Join(m.Hosts.Suffixes, ", ")})
	}
	p.System = append(p.System, SystemPlan{Kind: "polkit", Path: "dev.ryoku.profiles.manage"})
	p.Files = FilesPlan{Managed: managedPayloadCount(src.Files, m.SeedOnce), SeedOnce: len(m.SeedOnce), Home: home}
	p.Env = DesktopEnv(m, Vars{Home: home, User: acct.Name, ID: m.ID})
	warnStaleSyncDBs(&p.Warnings)
	sort.Slice(p.Bleed, func(i, j int) bool { return p.Bleed[i].DesktopID < p.Bleed[j].DesktopID })
	sort.Strings(p.Warnings)
	sort.Strings(p.Blockers)
	p.Digest = p.Pinned().Digest()
	return p, nil
}

func resolverBlocker(err error) string {
	text := strings.TrimSpace(err.Error())
	lower := strings.ToLower(text)
	if strings.Contains(lower, "conflict") || strings.Contains(lower, "unable to satisfy") || strings.Contains(lower, "could not satisfy") || strings.Contains(lower, "not found") {
		return fmt.Sprintf(i18n.T("%s. Run ryoku update first, then review the plan again."), text)
	}
	return text
}

func managedPayloadCount(files []FileHash, seeds []string) int {
	n := 0
	for _, file := range files {
		rel := strings.TrimPrefix(filepath.ToSlash(file.Path), "home/")
		if strings.HasPrefix(filepath.ToSlash(file.Path), "home/") && !contains(seeds, rel) {
			n++
		}
	}
	return n
}

func warnStaleSyncDBs(warnings *[]string) {
	matches, _ := filepath.Glob("/var/lib/pacman/sync/*.db")
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var stale []string
	for _, path := range matches {
		if strings.Contains(filepath.Base(path), "blackarch") {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.ModTime().Before(cutoff) {
			stale = append(stale, strings.TrimSuffix(filepath.Base(path), ".db"))
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		*warnings = append(*warnings, fmt.Sprintf(i18n.T("Package databases older than 7 days: %s. Run ryoku update first."), strings.Join(stale, ", ")))
	}
}
