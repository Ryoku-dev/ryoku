package profile

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func BuildRemovalPlan(r *SystemReceipt, others []SystemReceipt, res Resolver, o RemoveOptions, now time.Time) (*RemovalPlan, error) {
	p := &RemovalPlan{Schema: API, Packages: []RemovalPackage{}, KeptPackages: []KeptPackage{}, Repos: []RepoPlan{}, System: []SystemPlan{}, UserData: []string{}, Checkpoints: []RemovalCheckpoint{}}
	p.Account.Name = r.Account.Name
	p.Account.Home = "/home/" + r.Account.Name
	switch {
	case o.KeepAccount:
		p.Account.Action = "keep"
	case o.EraseFiles:
		p.Account.Action = "delete-erase"
	default:
		p.Account.Action = "delete-archive"
		p.Account.Archive = "/home/.removed/" + r.Account.Name + "-" + now.Format("2006-01-02")
	}
	if accountReceipt, err := ReadAccountReceipt(p.Account.Home); err == nil {
		p.Files.Managed = len(accountReceipt.Files)
		p.Files.SeedOnce = len(accountReceipt.Seeded)
		p.UserData = append([]string{}, accountReceipt.UserData...)
	}
	if r.Staged != "" {
		if manifest, loadErr := LoadManifest(r.Staged); loadErr == nil {
			p.UserData = append([]string{}, manifest.UserData...)
			if managed, seeds, countErr := stagedFileCounts(filepath.Join(r.Staged, "home"), manifest.SeedOnce); countErr == nil {
				p.Files.Managed = managed
				p.Files.SeedOnce = seeds
			}
		}
	}
	claimedPackages := map[string]string{}
	claimedRepos := map[string]bool{}
	claimedSystem := map[string]bool{}
	for _, other := range others {
		if other.Profile.ID == r.Profile.ID && other.Account.Name == r.Account.Name {
			continue
		}
		for _, name := range other.Packages.Introduced {
			claimedPackages[name] = other.Account.Name
		}
		for _, name := range other.Packages.PreExisting {
			claimedPackages[name] = other.Account.Name
		}
		for _, repo := range other.Repos {
			claimedRepos[repo.ID] = true
		}
		for _, file := range other.System {
			claimedSystem[file.Path] = true
		}
		for _, path := range other.Wordlists {
			claimedSystem[path] = true
		}
	}
	required, err := res.Required(r.Packages.Introduced)
	if err != nil {
		return nil, err
	}
	preExisting := map[string]bool{}
	for _, name := range r.Packages.PreExisting {
		preExisting[name] = true
	}
	candidates := map[string]bool{}
	for _, name := range r.Packages.Introduced {
		if claimedPackages[name] == "" && !preExisting[name] {
			candidates[name] = true
		}
	}
	blocked := map[string]string{}
	for {
		drop := map[string]string{}
		for name := range candidates {
			if by := firstOutsideRequirer(required[name], candidates); by != "" {
				drop[name] = by
			}
		}
		if len(drop) == 0 {
			break
		}
		for name, by := range drop {
			delete(candidates, name)
			blocked[name] = by
		}
	}
	var removable []string
	for _, name := range r.Packages.Introduced {
		if account := claimedPackages[name]; account != "" {
			p.KeptPackages = append(p.KeptPackages, KeptPackage{Name: name, Reason: "used by another profile account"})
		} else if preExisting[name] {
			continue
		} else if by := blocked[name]; by != "" {
			p.KeptPackages = append(p.KeptPackages, KeptPackage{Name: name, Reason: "required by " + by})
		} else if candidates[name] {
			removable = append(removable, name)
		}
	}
	if len(removable) > 0 {
		refs, resolveErr := res.Resolve(removable, nil)
		if resolveErr == nil {
			byName := map[string]PackageRef{}
			for _, ref := range refs {
				byName[ref.Name] = ref
			}
			for _, name := range removable {
				ref := byName[name]
				p.Packages = append(p.Packages, RemovalPackage{Name: name, Repo: ref.Repo, Installed: ref.Installed})
			}
		} else {
			for _, name := range removable {
				p.Packages = append(p.Packages, RemovalPackage{Name: name})
			}
		}
	}
	for _, name := range r.Packages.PreExisting {
		p.KeptPackages = append(p.KeptPackages, KeptPackage{Name: name, Reason: "installed before the profile"})
	}
	for _, repo := range r.Repos {
		entry := RepoPlan{ID: repo.ID, Action: "keep", Reason: "repository was not enabled by this profile"}
		if repo.EnabledByProfile && !claimedRepos[repo.ID] {
			entry.Action, entry.Reason = "disable", "no other profile uses it"
		} else if claimedRepos[repo.ID] {
			entry.Reason = "used by another profile account"
		}
		p.Repos = append(p.Repos, entry)
	}
	for _, file := range r.System {
		action := "remove"
		if claimedSystem[file.Path] {
			action = "keep"
		}
		p.System = append(p.System, SystemPlan{Kind: file.Kind, Path: file.Path, Action: action})
	}
	for _, path := range r.Wordlists {
		action := "remove"
		if claimedSystem[path] {
			action = "keep"
		}
		p.System = append(p.System, SystemPlan{Kind: "wordlists", Path: path, Action: action})
	}
	for _, checkpoint := range r.Checkpoints {
		p.Checkpoints = append(p.Checkpoints, RemovalCheckpoint{ID: checkpoint.ID, Action: "delete"})
	}
	sort.Slice(p.Packages, func(i, j int) bool { return p.Packages[i].Name < p.Packages[j].Name })
	sort.Slice(p.KeptPackages, func(i, j int) bool { return p.KeptPackages[i].Name < p.KeptPackages[j].Name })
	p.DigestValue = p.Digest()
	return p, nil
}

func firstOutsideRequirer(requirers []string, candidates map[string]bool) string {
	outside := make([]string, 0, len(requirers))
	for _, name := range requirers {
		if !candidates[name] {
			outside = append(outside, name)
		}
	}
	sort.Strings(outside)
	if len(outside) == 0 {
		return ""
	}
	return outside[0]
}

func stagedFileCounts(root string, seedOnce []string) (managed, seeds int, err error) {
	seedSet := stringSet(seedOnce)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if seedSet[filepath.ToSlash(rel)] {
			seeds++
		} else {
			managed++
		}
		return nil
	})
	return managed, seeds, err
}

func (p *RemovalPlan) Digest() string {
	copy := *p
	copy.DigestValue = ""
	copy.Packages = append([]RemovalPackage{}, copy.Packages...)
	copy.KeptPackages = append([]KeptPackage{}, copy.KeptPackages...)
	copy.Repos = append([]RepoPlan{}, copy.Repos...)
	copy.System = append([]SystemPlan{}, copy.System...)
	copy.UserData = append([]string{}, copy.UserData...)
	copy.Checkpoints = append([]RemovalCheckpoint{}, copy.Checkpoints...)
	sort.Slice(copy.Packages, func(i, j int) bool { return copy.Packages[i].Name < copy.Packages[j].Name })
	sort.Slice(copy.KeptPackages, func(i, j int) bool { return copy.KeptPackages[i].Name < copy.KeptPackages[j].Name })
	sort.Slice(copy.Repos, func(i, j int) bool { return copy.Repos[i].ID < copy.Repos[j].ID })
	sort.Slice(copy.System, func(i, j int) bool { return copy.System[i].Path < copy.System[j].Path })
	sort.Strings(copy.UserData)
	sort.Slice(copy.Checkpoints, func(i, j int) bool { return copy.Checkpoints[i].ID < copy.Checkpoints[j].ID })
	body, err := json.Marshal(copy)
	if err != nil {
		return ""
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil {
		return ""
	}
	delete(value, "digest")
	return digestJSON(value)
}
