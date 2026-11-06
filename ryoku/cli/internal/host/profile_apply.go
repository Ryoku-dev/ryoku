package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

type profileProgress struct {
	Stage   string `json:"stage"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

func (a *App) profileApply(args []string) int {
	parsed, err := parseProfilePlanArgs(args, true)
	if err != nil {
		return a.profileFail(err.Error())
	}
	plan, manifest, source, previous, err := a.buildProfilePlan(parsed)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if err := refuseChangedPlan(plan, parsed.Digest); err != nil {
		return a.profileFail(err.Error())
	}
	if len(plan.Blockers) > 0 {
		return a.profileFail(fmt.Sprintf(i18n.T("The profile cannot be applied: %s"), strings.Join(plan.Blockers, "; ")))
	}
	if err := a.profileProgress("verify", 1, i18n.T("The reviewed plan is unchanged.")); err != nil {
		return a.profileFail(err.Error())
	}

	receipt := newSystemReceipt(manifest, plan, parsed.Selection, previous, a.cfg.Now())
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Write the profile receipt: %v"), err))
	}
	if err := a.applyProfileRepos(plan, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgress("repos", 2, i18n.T("Profile repositories are ready.")); err != nil {
		return a.profileFail(err.Error())
	}

	if err := a.applyProfilePackages(plan, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgress("packages", 3, i18n.T("Profile packages are installed.")); err != nil {
		return a.profileFail(err.Error())
	}

	if err := a.ensureProfileGroup(manifest.ID, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	account, created, err := a.applyProfileAccount(manifest, plan, previous, receipt)
	if err != nil {
		return a.profileFail(err.Error())
	}
	receipt.Account.Name, receipt.Account.UID, receipt.Account.Created = account.Name, account.UID, created || receipt.Account.Created
	for _, group := range manifest.Account.Groups {
		receipt.Groups = appendUnique(receipt.Groups, group)
	}
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgress("account", 4, i18n.T("The profile account is ready.")); err != nil {
		return a.profileFail(err.Error())
	}

	if err := a.provisionAccount(account.Name, ProvisionOptions{Rice: manifest.Rice, SkipWelcome: true}); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Provision the profile account: %v"), err))
	}
	if err := a.profileProgress("provision", 5, i18n.T("The desktop is provisioned.")); err != nil {
		return a.profileFail(err.Error())
	}

	if err := a.applyProfileSystem(manifest, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.pruneProfileSystem(manifest, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgress("system", 6, i18n.T("System profile pieces are ready.")); err != nil {
		return a.profileFail(err.Error())
	}

	staged, err := stageProfilePayload(profileRootPath, manifest, source)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Stage the profile files: %v"), err))
	}
	receipt.Staged = staged
	receipt.AppliedAt = a.cfg.Now().UTC()
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if previous != nil && previous.Staged != "" && previous.Staged != staged && !stagedClaimed(previous.Staged, receipts) {
		_ = os.RemoveAll(previous.Staged)
	}
	if err := a.rewriteProfileRefreshHook(receipts); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Write the profile refresh hook: %v"), err))
	}
	if digest, err := hashFile(profileRefreshHookPath); err == nil {
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "pacman-hook", Path: profileRefreshHookPath, SHA256: digest})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return a.profileFail(err.Error())
		}
	}
	if err := a.profileProgress("files", 7, i18n.T("The profile payload is staged.")); err != nil {
		return a.profileFail(err.Error())
	}

	check, err := a.layProfileAccount(account, manifest.Account.Shell, staged, profile.SystemReceiptPath(profileRootPath, manifest.ID, account.Name))
	if err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgress("check", 8, i18n.T("The profile check is complete.")); err != nil {
		return a.profileFail(err.Error())
	}
	return a.profileJSON(struct {
		OK      bool                `json:"ok"`
		Account string              `json:"account"`
		Check   profile.CheckResult `json:"check"`
	}{OK: true, Account: account.Name, Check: check})
}

func refuseChangedPlan(plan *profile.Plan, expected string) error {
	if plan == nil || plan.Digest != expected {
		return fmt.Errorf(i18n.T("plan changed; review it again"))
	}
	return nil
}

func newSystemReceipt(manifest *profile.Manifest, plan *profile.Plan, selection profile.Selection, previous *profile.SystemReceipt, now time.Time) *profile.SystemReceipt {
	receipt := &profile.SystemReceipt{Schema: profile.API, Selection: selection, AppliedAt: now.UTC()}
	receipt.Profile.ID, receipt.Profile.Version, receipt.Profile.ManifestSHA256 = manifest.ID, manifest.Version, plan.Profile.ManifestSHA256
	receipt.Account.Name = plan.Account.Name
	receipt.Bleed = make([]string, 0, len(plan.Bleed))
	for _, entry := range plan.Bleed {
		receipt.Bleed = append(receipt.Bleed, entry.DesktopID)
	}
	if previous != nil {
		receipt.Account = previous.Account
		receipt.Checkpoints = append([]profile.Checkpoint(nil), previous.Checkpoints...)
		receipt.Groups = append([]string(nil), previous.Groups...)
		receipt.GroupsCreated = append([]string(nil), previous.GroupsCreated...)
		receipt.Repos = append([]profile.RepoClaim(nil), previous.Repos...)
		receipt.Packages.Introduced = append([]string(nil), previous.Packages.Introduced...)
		receipt.Packages.PreExisting = append([]string(nil), previous.Packages.PreExisting...)
		receipt.System = append([]profile.SystemFile(nil), previous.System...)
		receipt.Wordlists = append([]string(nil), previous.Wordlists...)
		if previous.Hosts != nil {
			hosts := *previous.Hosts
			receipt.Hosts = &hosts
		}
	}
	return receipt
}

func (a *App) applyProfileRepos(plan *profile.Plan, receipt *profile.SystemReceipt) error {
	for _, repo := range plan.Repos {
		claim := profile.RepoClaim{ID: repo.ID, EnabledByProfile: repo.Action == "enable"}
		for _, prior := range receipt.Repos {
			if prior.ID == repo.ID && prior.EnabledByProfile {
				claim.EnabledByProfile = true
			}
		}
		replaced := false
		for index := range receipt.Repos {
			if receipt.Repos[index].ID == claim.ID {
				receipt.Repos[index] = claim
				replaced = true
				break
			}
		}
		if !replaced {
			receipt.Repos = append(receipt.Repos, claim)
		}
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if repo.ID == "blackarch" && repo.Action == "enable" {
			if err := a.runProfileCommand("/usr/bin/ryoku-pkg-blackarch", "enable"); err != nil {
				return fmt.Errorf(i18n.T("Enable BlackArch: %v"), err)
			}
		}
		if repo.ID == "blackarch" && repo.Action == "enable" {
			if err := a.runProfileCommand("/usr/bin/ryoku-pkg-blackarch", "sync"); err != nil {
				return fmt.Errorf(i18n.T("Sync BlackArch: %v"), err)
			}
		}
	}
	return nil
}

func (a *App) applyProfilePackages(plan *profile.Plan, receipt *profile.SystemReceipt) error {
	if len(plan.Remove) > 0 {
		if err := a.removeProfilePackages(&profile.RemovalPlan{Packages: plan.Remove}); err != nil {
			return err
		}
	}
	targets := []string{}
	retained := map[string]bool{}
	for _, name := range receipt.Packages.Introduced {
		retained[name] = true
	}
	for _, pkg := range plan.Remove {
		delete(retained, pkg.Name)
	}
	introducedSet := map[string]bool{}
	for name := range retained {
		introducedSet[name] = true
	}
	preExisting := []string{}
	for _, pkg := range plan.Packages {
		if pkg.Target {
			targets = append(targets, pkg.Name)
		}
		if introducedSet[pkg.Name] || !pkg.PreExisting {
			introducedSet[pkg.Name] = true
		} else {
			preExisting = append(preExisting, pkg.Name)
		}
	}
	sort.Strings(preExisting)
	retainedNames := make([]string, 0, len(retained))
	for name := range retained {
		retainedNames = append(retainedNames, name)
	}
	sort.Strings(retainedNames)
	receipt.Packages.Introduced = retainedNames
	receipt.Packages.PreExisting = preExisting
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return err
	}
	introduced := make([]string, 0, len(introducedSet))
	for name := range introducedSet {
		introduced = append(introduced, name)
	}
	sort.Strings(introduced)
	receipt.Packages.Introduced = introduced
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return err
	}
	if len(targets) > 0 {
		args := []string{"-S", "--needed", "--noconfirm"}
		args = append(args, targets...)
		result := a.cfg.Runner.Run(Command{Name: "pacman", Args: args, Stdin: nil, Stdout: io.Discard, Stderr: io.Discard})
		if result.Code != 0 {
			return fmt.Errorf(i18n.T("Install profile packages failed. Run ryoku update first, then review and apply the plan again."))
		}
	}
	return nil
}

func (a *App) applyProfileAccount(manifest *profile.Manifest, plan *profile.Plan, previous *profile.SystemReceipt, receipt *profile.SystemReceipt) (localUser, bool, error) {
	created := false
	if !plan.Account.Exists {
		args := []string{plan.Account.Name, "--full-name", manifest.Account.FullName, "--role", "standard", "--no-provision"}
		for _, permission := range manifest.Account.Permissions {
			args = append(args, "--perm", permission)
		}
		var output bytes.Buffer
		stdout := a.cfg.Stdout
		a.cfg.Stdout = &output
		code := a.usersAdd(args)
		a.cfg.Stdout = stdout
		if code != ExitOK {
			var failure struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(output.Bytes(), &failure) == nil && failure.Error != "" {
				return localUser{}, false, fmt.Errorf("%s", failure.Error)
			}
			return localUser{}, false, fmt.Errorf(i18n.T("Create the profile account."))
		}
		created = true
	} else if previous == nil {
		return localUser{}, false, fmt.Errorf(i18n.T("The existing account is not owned by this profile."))
	}
	accountUsers, accountGroups, account, err := a.requireHumanUser(plan.Account.Name)
	if err != nil {
		return localUser{}, created, err
	}
	receipt.Account.Name, receipt.Account.UID, receipt.Account.Created = account.Name, account.UID, created || receipt.Account.Created
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return localUser{}, created, err
	}
	if userInGroup(account, accountGroups["wheel"]) {
		if administratorCount(accountUsers, accountGroups) <= 1 {
			return localUser{}, created, fmt.Errorf(i18n.T("The profile account is the last Administrator account. Add another Administrator before applying it as Standard."))
		}
		if err := a.usersRun(nil, "gpasswd", "--delete", account.Name, "wheel"); err != nil {
			return localUser{}, created, fmt.Errorf(i18n.T("Keep the profile account Standard: %v"), err)
		}
	}
	if account.FullName != manifest.Account.FullName {
		if err := a.usersRun(nil, "usermod", "--comment", manifest.Account.FullName, account.Name); err != nil {
			return localUser{}, created, fmt.Errorf(i18n.T("Set the profile account name: %v"), err)
		}
		account.FullName = manifest.Account.FullName
	}
	groups := append([]string{profile.GroupName(manifest.ID)}, manifest.Account.Groups...)
	for _, permission := range manifest.Account.Permissions {
		if spec, ok := userPermission(permission); ok {
			_, allGroups, loadErr := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
			if loadErr != nil {
				return localUser{}, created, loadErr
			}
			groups = append(groups, existingPermissionGroups(spec, allGroups)...)
		}
	}
	groups = uniqueSorted(groups)
	if len(groups) > 0 {
		if err := a.usersRun(nil, "usermod", "--append", "--groups", strings.Join(groups, ","), account.Name); err != nil {
			return localUser{}, created, fmt.Errorf(i18n.T("Set profile account groups: %v"), err)
		}
	}
	if account.Shell != manifest.Account.Shell {
		if err := a.usersRun(nil, "usermod", "--shell", manifest.Account.Shell, account.Name); err != nil {
			return localUser{}, created, fmt.Errorf(i18n.T("Set the profile account shell: %v"), err)
		}
		account.Shell = manifest.Account.Shell
	}
	return account, created, nil
}

func (a *App) layProfileAccount(account localUser, shell, payload, receiptPath string) (profile.CheckResult, error) {
	env := []string{
		"HOME=" + account.Home,
		"USER=" + account.Name,
		"LOGNAME=" + account.Name,
		"SHELL=" + shell,
		"XDG_CONFIG_HOME=" + filepath.Join(account.Home, ".config"),
		"XDG_STATE_HOME=" + filepath.Join(account.Home, ".local", "state"),
		"XDG_DATA_HOME=" + filepath.Join(account.Home, ".local", "share"),
		"XDG_CACHE_HOME=" + filepath.Join(account.Home, ".cache"),
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	args := []string{"-u", account.Name, "--", "env", "-i"}
	args = append(args, env...)
	args = append(args, "/usr/bin/ryoku", "profile", "lay", "--payload", payload, "--system-receipt", receiptPath, "--json")
	result := a.cfg.Runner.Run(Command{Name: "runuser", Args: args, Stdin: nil})
	if result.Code != 0 {
		return profile.CheckResult{}, fmt.Errorf(i18n.T("Lay out the profile account files failed."))
	}
	var output struct {
		Check profile.CheckResult `json:"check"`
	}
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		return profile.CheckResult{}, fmt.Errorf(i18n.T("Read the profile check result: %v"), err)
	}
	return output.Check, nil
}

func (a *App) profileProgress(stage string, done int, message string) error {
	return a.profileProgressTotal(stage, done, 8, message)
}

func (a *App) profileProgressTotal(stage string, done, total int, message string) error {
	return json.NewEncoder(a.cfg.Stdout).Encode(profileProgress{Stage: stage, Done: done, Total: total, Message: message})
}

func (a *App) runProfileCommand(name string, args ...string) error {
	result := a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: nil, Stdout: io.Discard, Stderr: io.Discard})
	if result.Code != 0 {
		return fmt.Errorf("%s exited with status %d", name, result.Code)
	}
	return nil
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
