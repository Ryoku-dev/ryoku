package host

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

type profileRemoveArgs struct {
	Account     string
	KeepAccount bool
	EraseFiles  bool
	Digest      string
}

func parseProfileRemoveArgs(args []string, requireDigest bool) (profileRemoveArgs, error) {
	var out profileRemoveArgs
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--account":
			if index+1 >= len(args) || out.Account != "" {
				return out, fmt.Errorf(i18n.T("--account needs one value."))
			}
			index++
			out.Account = args[index]
		case "--keep-account":
			if out.KeepAccount {
				return out, fmt.Errorf(i18n.T("--keep-account can be used only once."))
			}
			out.KeepAccount = true
		case "--erase-files":
			if out.EraseFiles {
				return out, fmt.Errorf(i18n.T("--erase-files can be used only once."))
			}
			out.EraseFiles = true
		case "--digest":
			if !requireDigest || index+1 >= len(args) || out.Digest != "" {
				return out, fmt.Errorf(i18n.T("Unexpected --digest option."))
			}
			index++
			out.Digest = args[index]
		default:
			return out, fmt.Errorf(i18n.T("Unknown profile removal option %s."), args[index])
		}
	}
	if !usernamePattern.MatchString(out.Account) || requireDigest && !validSHA256(out.Digest) {
		return out, fmt.Errorf(i18n.T("Profile removal requires a valid --account and, when removing, --digest."))
	}
	return out, nil
}

func (a *App) buildProfileRemoval(parsed profileRemoveArgs) (*profile.RemovalPlan, *profile.SystemReceipt, []profile.SystemReceipt, error) {
	receipt, others, err := receiptForAccount(profileRootPath, parsed.Account)
	if err != nil {
		return nil, nil, nil, err
	}
	caller, err := a.callerUser()
	if err != nil {
		return nil, nil, nil, err
	}
	scratch, err := a.profileResolverScratch(caller)
	if err != nil {
		return nil, nil, nil, err
	}
	defer os.RemoveAll(scratch)
	resolver := profile.NewPacmanResolver(profile.PacmanResolverOptions{ScratchDir: scratch, PacmanConf: a.cfg.PacmanConf, DBPath: filepath.Dir(a.cfg.PacmanSyncDir)})
	installed, err := resolver.Installed(receipt.Packages.Introduced)
	if err != nil {
		return nil, nil, nil, err
	}
	working := *receipt
	working.Packages.Introduced = make([]string, 0, len(receipt.Packages.Introduced))
	for _, name := range receipt.Packages.Introduced {
		if installed[name] {
			working.Packages.Introduced = append(working.Packages.Introduced, name)
		}
	}
	plan, err := profile.BuildRemovalPlan(&working, others, resolver, profile.RemoveOptions{KeepAccount: parsed.KeepAccount, EraseFiles: parsed.EraseFiles}, a.cfg.Now())
	return plan, &working, others, err
}

func receiptForAccount(root, account string) (*profile.SystemReceipt, []profile.SystemReceipt, error) {
	receipts, err := profile.ReadSystemReceipts(root)
	if err != nil {
		return nil, nil, err
	}
	others := make([]profile.SystemReceipt, 0, len(receipts))
	var found *profile.SystemReceipt
	for index := range receipts {
		if receipts[index].Account.Name == account {
			copy := receipts[index]
			found = &copy
			continue
		}
		others = append(others, receipts[index])
	}
	if found == nil {
		return nil, nil, fmt.Errorf(i18n.T("No applied profile account named %s exists."), account)
	}
	return found, others, nil
}

func (a *App) profilePlanRemove(args []string) int {
	parsed, err := parseProfileRemoveArgs(args, false)
	if err != nil {
		return a.profileFail(err.Error())
	}
	plan, _, _, err := a.buildProfileRemoval(parsed)
	if err != nil {
		return a.profileFail(err.Error())
	}
	return a.profileJSON(plan)
}

func (a *App) profileRemove(args []string) int {
	parsed, err := parseProfileRemoveArgs(args, true)
	if err != nil {
		return a.profileFail(err.Error())
	}
	plan, receipt, others, err := a.buildProfileRemoval(parsed)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if plan.Digest() != parsed.Digest {
		return a.profileFail(i18n.T("plan changed; review it again"))
	}
	accountPresent := false
	if !parsed.KeepAccount {
		users, _, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
		if err != nil {
			return a.profileFail(err.Error())
		}
		var target localUser
		for _, candidate := range humanUsers(users) {
			if candidate.Name == parsed.Account {
				target = candidate
				accountPresent = true
				break
			}
		}
		if accountPresent {
			if receipt.Account.UID != 0 && receipt.Account.UID != target.UID {
				return a.profileFail(i18n.T("The account no longer matches the profile receipt and will not be deleted."))
			}
			loggedIn, err := a.loggedInUsers(humanUsers(users))
			if err != nil {
				return a.profileFail(err.Error())
			}
			if loggedIn[target.Name] {
				return a.profileFail(fmt.Sprintf(i18n.T("Sign out %s before removing the profile account."), target.Name))
			}
		} else if !receipt.Account.Created {
			return a.profileFail(i18n.T("The existing profile account is missing. Use --keep-account to remove only the profile."))
		}
	}
	if err := a.profileProgressTotal("verify", 1, 4, i18n.T("The reviewed removal plan is unchanged.")); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.removeProfilePackages(plan); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgressTotal("packages", 2, 4, i18n.T("Profile-only packages are removed.")); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.removeProfileSystem(receipt, others); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.profileProgressTotal("system", 3, 4, i18n.T("Profile system pieces are removed.")); err != nil {
		return a.profileFail(err.Error())
	}
	if !parsed.KeepAccount && accountPresent {
		flag := "--keep-files"
		if parsed.EraseFiles {
			flag = "--erase-files"
		}
		stdout := a.cfg.Stdout
		a.cfg.Stdout = io.Discard
		code := a.usersDelete([]string{parsed.Account, flag})
		a.cfg.Stdout = stdout
		if code != ExitOK {
			return a.profileFail(i18n.T("Delete the profile account failed."))
		}
	}
	if err := a.profileProgressTotal("account", 4, 4, i18n.T("The profile account removal choice is complete.")); err != nil {
		return a.profileFail(err.Error())
	}
	if err := a.rewriteProfileRefreshHook(others); err != nil {
		return a.profileFail(err.Error())
	}
	if err := pruneManagedHosts(profileHostsPath, hostSuffixesFromReceipts(others)); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Update lab hosts: %v"), err))
	}
	for _, repo := range plan.Repos {
		if repo.ID == "blackarch" && repo.Action == "disable" {
			if err := a.runProfileCommand("/usr/bin/ryoku-pkg-blackarch", "disable"); err != nil {
				return a.profileFail(fmt.Sprintf(i18n.T("Disable BlackArch: %v"), err))
			}
		}
	}
	if receipt.Staged != "" && !stagedClaimed(receipt.Staged, others) {
		if err := os.RemoveAll(receipt.Staged); err != nil {
			return a.profileFail(fmt.Sprintf(i18n.T("Remove the staged profile files: %v"), err))
		}
	}
	path := profile.SystemReceiptPath(profileRootPath, receipt.Profile.ID, receipt.Account.Name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return a.profileFail(fmt.Sprintf(i18n.T("Remove the profile receipt: %v"), err))
	}
	profileStillApplied := false
	for _, other := range others {
		if other.Profile.ID == receipt.Profile.ID {
			profileStillApplied = true
			break
		}
	}
	if !profileStillApplied {
		root := filepath.Join(profileRootPath, receipt.Profile.ID)
		for _, directory := range []string{filepath.Join(root, "accounts"), filepath.Join(root, "payload"), root} {
			if err := removeEmptyDirectory(directory); err != nil {
				return a.profileFail(fmt.Sprintf(i18n.T("Remove the empty profile data directory: %v"), err))
			}
		}
	}
	if err := removeEmptyDirectory(profileRootPath); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Remove the empty profiles data directory: %v"), err))
	}
	return a.profileJSON(map[string]bool{"ok": true})
}

func (a *App) removeProfilePackages(plan *profile.RemovalPlan) error {
	if len(plan.Packages) == 0 {
		return nil
	}
	names := make([]string, 0, len(plan.Packages))
	planned := map[string]bool{}
	for _, pkg := range plan.Packages {
		names = append(names, pkg.Name)
		planned[pkg.Name] = true
	}
	args := append([]string{"-Rsp", "--print-format", "%n"}, names...)
	result := a.query("pacman", args...)
	if result.Code != 0 {
		return fmt.Errorf(i18n.T("Confirm package removal failed."))
	}
	for _, name := range strings.Fields(result.Output) {
		if !planned[name] {
			return fmt.Errorf(i18n.T("Package removal would also remove %s; review the plan again."), name)
		}
	}
	removeArgs := append([]string{"-Rns", "--noconfirm"}, names...)
	result = a.cfg.Runner.Run(Command{Name: "pacman", Args: removeArgs, Stdout: io.Discard, Stderr: io.Discard})
	if result.Code != 0 {
		return fmt.Errorf(i18n.T("Remove profile packages failed."))
	}
	return nil
}

func (a *App) removeProfileSystem(receipt *profile.SystemReceipt, others []profile.SystemReceipt) error {
	claimedFiles := map[string]bool{}
	claimedWords := map[string]bool{}
	claimedGroups := map[string]bool{}
	for _, other := range others {
		for _, file := range other.System {
			claimedFiles[file.Path] = true
		}
		for _, path := range other.Wordlists {
			claimedWords[path] = true
		}
		for _, group := range other.GroupsCreated {
			claimedGroups[group] = true
		}
	}
	remove := make([]profile.SystemFile, 0)
	for _, file := range receipt.System {
		if !claimedFiles[file.Path] {
			remove = append(remove, file)
		}
	}
	if err := removeRecordedSystemFiles(remove); err != nil {
		return err
	}
	for _, path := range receipt.Wordlists {
		if !claimedWords[path] {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, checkpoint := range receipt.Checkpoints {
		path := filepath.Join(a.cfg.HomeRoot, ".checkpoints", receipt.Account.Name, checkpoint.ID)
		if a.isBtrfsSubvolume(path) {
			if err := a.usersRun(nil, "btrfs", "subvolume", "delete", path); err != nil {
				return err
			}
		}
	}
	checkpointRoot := filepath.Join(a.cfg.HomeRoot, ".checkpoints")
	if err := removeEmptyDirectory(filepath.Join(checkpointRoot, receipt.Account.Name)); err != nil {
		return err
	}
	if err := removeEmptyDirectory(checkpointRoot); err != nil {
		return err
	}
	_, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return err
	}
	for _, group := range receipt.GroupsCreated {
		if !claimedGroups[group] && groups[group] != nil {
			if err := a.usersRun(nil, "groupdel", group); err != nil {
				return err
			}
		}
	}
	return nil
}

func stagedClaimed(path string, receipts []profile.SystemReceipt) bool {
	for _, receipt := range receipts {
		if receipt.Staged == path {
			return true
		}
	}
	return false
}

func (a *App) profileReset(args []string) int {
	account, dryRun, err := parseAccountDryRun(args)
	if err != nil {
		return a.profileFail(err.Error())
	}
	_, _, user, err := a.requireHumanUser(account)
	if err != nil {
		return a.profileFail(err.Error())
	}
	receipt, _, err := receiptForAccount(profileRootPath, account)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if receipt.Account.UID != 0 && receipt.Account.UID != user.UID {
		return a.profileFail(i18n.T("The profile receipt belongs to an earlier account with this name."))
	}
	manifest, err := profile.LoadManifest(receipt.Staged)
	if err != nil {
		return a.profileFail(err.Error())
	}
	command := []string{"profile", "reset", "--json"}
	if dryRun {
		command = append(command, "--dry-run")
	}
	result := a.runAsProfileUser(user, manifest.Account.Shell, command...)
	if result.Code != 0 {
		return a.profileFail(i18n.T("Reset the profile account failed."))
	}
	fmt.Fprint(a.cfg.Stdout, result.Output)
	return ExitOK
}

func parseAccountDryRun(args []string) (string, bool, error) {
	account, dryRun := "", false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--account":
			if index+1 >= len(args) || account != "" {
				return "", false, fmt.Errorf(i18n.T("--account needs one value."))
			}
			index++
			account = args[index]
		case "--dry-run":
			dryRun = true
		default:
			return "", false, fmt.Errorf(i18n.T("Unknown reset option %s."), args[index])
		}
	}
	if !usernamePattern.MatchString(account) {
		return "", false, fmt.Errorf(i18n.T("Reset requires a valid --account."))
	}
	return account, dryRun, nil
}

func (a *App) runAsProfileUser(user localUser, shell string, command ...string) Result {
	env := []string{
		"HOME=" + user.Home, "USER=" + user.Name, "LOGNAME=" + user.Name, "SHELL=" + shell,
		"XDG_CONFIG_HOME=" + filepath.Join(user.Home, ".config"),
		"XDG_STATE_HOME=" + filepath.Join(user.Home, ".local", "state"),
		"XDG_DATA_HOME=" + filepath.Join(user.Home, ".local", "share"),
		"XDG_CACHE_HOME=" + filepath.Join(user.Home, ".cache"), "PATH=/usr/local/bin:/usr/bin:/bin",
	}
	args := []string{"-u", user.Name, "--", "env", "-i"}
	args = append(args, env...)
	args = append(args, "/usr/bin/ryoku")
	args = append(args, command...)
	return a.cfg.Runner.Run(Command{Name: "runuser", Args: args})
}
