package host

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

var profileShellsPath = "/etc/shells"

type profilePlanArgs struct {
	Source    string
	Account   string
	Selection profile.Selection
	Digest    string
}

type profileStatusAccount struct {
	Account     string               `json:"account"`
	UID         int                  `json:"uid"`
	Profile     string               `json:"profile"`
	Version     string               `json:"version"`
	AppliedAt   string               `json:"appliedAt"`
	Repos       []string             `json:"repos"`
	Packages    []string             `json:"packages"`
	Bleed       []string             `json:"bleed"`
	Checkpoints []profile.Checkpoint `json:"checkpoints"`
	LoggedIn    bool                 `json:"loggedIn"`
}

func parseProfilePlanArgs(args []string, requireDigest bool) (profilePlanArgs, error) {
	var out profilePlanArgs
	seenSource, seenAccount, seenDigest := false, false, false
	for index := 0; index < len(args); {
		flag := args[index]
		if index+1 >= len(args) {
			return out, fmt.Errorf(i18n.T("%s needs a value."), flag)
		}
		value := args[index+1]
		switch flag {
		case "--source":
			if seenSource {
				return out, fmt.Errorf(i18n.T("--source can be used only once."))
			}
			seenSource, out.Source = true, value
		case "--account":
			if seenAccount {
				return out, fmt.Errorf(i18n.T("--account can be used only once."))
			}
			seenAccount, out.Account = true, value
		case "--component":
			out.Selection.Components = append(out.Selection.Components, value)
		case "--no-component":
			out.Selection.Without = append(out.Selection.Without, value)
		case "--extra":
			out.Selection.Extras = append(out.Selection.Extras, value)
		case "--digest":
			if !requireDigest || seenDigest {
				return out, fmt.Errorf(i18n.T("Unexpected --digest option."))
			}
			seenDigest, out.Digest = true, value
		default:
			return out, fmt.Errorf(i18n.T("Unknown profile option %s."), flag)
		}
		index += 2
	}
	if !seenSource || !seenAccount || requireDigest && !seenDigest {
		return out, fmt.Errorf(i18n.T("Profile planning requires --source and --account; applying also requires --digest."))
	}
	if !usernamePattern.MatchString(out.Account) {
		return out, fmt.Errorf(i18n.T("Invalid username."))
	}
	if requireDigest && !validSHA256(out.Digest) {
		return out, fmt.Errorf(i18n.T("The plan digest is invalid."))
	}
	return out, nil
}

func (a *App) profileAPI() int {
	return a.profileJSON(map[string]int{"api": profile.API})
}

func (a *App) profileStatus(args []string) int {
	account := ""
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "--account" || !usernamePattern.MatchString(args[1]) {
			return a.profileFail(i18n.T("Use profile status [--account <name>]."))
		}
		account = args[1]
	}
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read applied profiles: %v"), err))
	}
	users, _, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read local accounts: %v"), err))
	}
	loggedIn, err := a.loggedInUsers(humanUsers(users))
	if err != nil {
		return a.profileFail(err.Error())
	}
	accounts := make([]profileStatusAccount, 0, len(receipts))
	for _, receipt := range receipts {
		if account != "" && receipt.Account.Name != account {
			continue
		}
		repos := make([]string, 0, len(receipt.Repos))
		for _, claim := range receipt.Repos {
			repos = append(repos, claim.ID)
		}
		packages := []string{}
		manifest, manifestErr := profile.LoadManifest(receipt.Staged)
		if manifestErr == nil {
			components, extras, targetErr := manifest.Targets(receipt.Selection)
			if targetErr != nil {
				return a.profileFail(targetErr.Error())
			}
			packages = append(packages, extras...)
			for _, component := range components {
				packages = append(packages, component.Name)
			}
		} else {
			packages = append(packages, receipt.Packages.Introduced...)
			packages = append(packages, receipt.Packages.PreExisting...)
		}
		sort.Strings(repos)
		packages = uniqueSorted(packages)
		accounts = append(accounts, profileStatusAccount{
			Account: receipt.Account.Name, UID: receipt.Account.UID,
			Profile: receipt.Profile.ID, Version: receipt.Profile.Version,
			AppliedAt: receipt.AppliedAt.Format("2006-01-02T15:04:05Z07:00"),
			Repos:     repos, Packages: packages, Bleed: append([]string(nil), receipt.Bleed...),
			Checkpoints: append([]profile.Checkpoint(nil), receipt.Checkpoints...),
			LoggedIn:    loggedIn[receipt.Account.Name],
		})
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Account < accounts[j].Account })
	return a.profileJSON(struct {
		API      int                    `json:"api"`
		Accounts []profileStatusAccount `json:"accounts"`
	}{API: profile.API, Accounts: accounts})
}

func (a *App) buildProfilePlan(parsed profilePlanArgs) (*profile.Plan, *profile.Manifest, profile.Source, *profile.SystemReceipt, error) {
	source, err := a.verifiedProfileSource(parsed.Source)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	manifest, err := profile.LoadManifest(source.Dir)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	if filepath.Base(source.Dir) != manifest.ID {
		return nil, nil, profile.Source{}, nil, fmt.Errorf(i18n.T("The profile source and manifest identity do not match."))
	}
	users, _, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	target := profile.AccountTarget{Name: parsed.Account, FullName: manifest.Account.FullName, Home: filepath.Join(a.cfg.HomeRoot, parsed.Account)}
	existingManaged := true
	for _, user := range humanUsers(users) {
		if user.Name == parsed.Account {
			target.Exists, target.UID, target.FullName, target.Home = true, user.UID, user.FullName, user.Home
			existingManaged = a.isManagedHome(user)
			break
		}
	}
	if !target.Exists {
		if err := validateNewUsername(parsed.Account, users); err != nil {
			return nil, nil, profile.Source{}, nil, err
		}
	}
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	var previous *profile.SystemReceipt
	for index := range receipts {
		if receipts[index].Account.Name != parsed.Account {
			continue
		}
		if target.Exists && receipts[index].Account.UID != 0 && receipts[index].Account.UID != target.UID {
			break
		}
		target.ProfileID = receipts[index].Profile.ID
		if receipts[index].Profile.ID == manifest.ID {
			copy := receipts[index]
			previous = &copy
		}
		break
	}
	host, err := a.profileHostFacts()
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	caller, err := a.callerUser()
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	scratch, err := a.profileResolverScratch(caller)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	defer os.RemoveAll(scratch)
	resolver := profile.NewPacmanResolver(profile.PacmanResolverOptions{ScratchDir: scratch, PacmanConf: a.cfg.PacmanConf, DBPath: filepath.Dir(a.cfg.PacmanSyncDir)})
	plan, err := profile.BuildPlan(manifest, source, target, parsed.Selection, host, previous, resolver)
	if err != nil {
		return nil, nil, profile.Source{}, nil, err
	}
	if target.Exists && !existingManaged {
		plan.Account.ManagedHome = false
		plan.Blockers = append(plan.Blockers, i18n.T("The existing account does not use its managed home path."))
		sort.Strings(plan.Blockers)
	}
	if previous != nil && len(plan.Remove) > 0 {
		claimed := map[string]bool{}
		for _, receipt := range receipts {
			if receipt.Account.Name == parsed.Account {
				continue
			}
			for _, name := range receipt.Packages.Introduced {
				claimed[name] = true
			}
			for _, name := range receipt.Packages.PreExisting {
				claimed[name] = true
			}
		}
		kept := plan.Remove[:0]
		for _, pkg := range plan.Remove {
			if !claimed[pkg.Name] {
				kept = append(kept, pkg)
			}
		}
		plan.Remove = kept
		plan.Digest = plan.Pinned().Digest()
	}
	return plan, manifest, source, previous, nil
}

func (a *App) profilePlan(args []string) int {
	parsed, err := parseProfilePlanArgs(args, false)
	if err != nil {
		return a.profileFail(err.Error())
	}
	plan, _, _, _, err := a.buildProfilePlan(parsed)
	if err != nil {
		return a.profileFail(err.Error())
	}
	return a.profileJSON(plan)
}

func (a *App) profileHostFacts() (profile.HostFacts, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return profile.HostFacts{}, err
	}
	shells, err := readLoginShells(profileShellsPath)
	if err != nil {
		return profile.HostFacts{}, fmt.Errorf(i18n.T("Read login shells: %v"), err)
	}
	facts := profile.HostFacts{
		PackageManager: string(manager), HomeRoot: a.cfg.HomeRoot,
		ManagedHomes: a.userStorageCapabilities().Subvolumes,
		Capabilities: map[string]bool{}, Shells: shells,
	}
	status := a.query("/usr/bin/ryoku-pkg-blackarch", "status", "--json")
	if status.Code == 0 {
		var value struct {
			Enabled bool `json:"enabled"`
		}
		if json.Unmarshal([]byte(status.Output), &value) == nil {
			facts.BlackArchEnabled = value.Enabled
		}
	}
	caps := a.query("ryoku", "wm", "caps")
	if caps.Code == 0 {
		var value struct {
			Supports []string `json:"supports"`
		}
		if json.Unmarshal([]byte(caps.Output), &value) == nil {
			for _, capability := range value.Supports {
				facts.Capabilities[capability] = true
			}
		}
	}
	return facts, nil
}

func readLoginShells(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var shells []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if filepath.IsAbs(value) && !strings.HasPrefix(value, "#") {
			shells = append(shells, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return shells, nil
}

func (a *App) profileResolverScratch(caller localUser) (string, error) {
	if a.cfg.UID == 0 {
		return os.MkdirTemp("/var/tmp", fmt.Sprintf("profile-plan-%d-", caller.UID))
	}
	base := filepath.Join(caller.Home, ".cache", "ryoku", "profile-plan")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "run-")
}
