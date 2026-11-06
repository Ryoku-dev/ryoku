package host

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

var checkpointIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z$`)

func (a *App) profileCheckpoint(args []string) int {
	if len(args) == 0 {
		return a.profileFail(i18n.T("Choose a checkpoint action."))
	}
	action := args[0]
	account, label, id, err := parseCheckpointArgs(action, args[1:])
	if err != nil {
		return a.profileFail(err.Error())
	}
	receipt, _, err := receiptForAccount(profileRootPath, account)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if action == "list" {
		checkpoints := append([]profile.Checkpoint(nil), receipt.Checkpoints...)
		sort.Slice(checkpoints, func(i, j int) bool { return checkpoints[i].CreatedAt.Before(checkpoints[j].CreatedAt) })
		return a.profileJSON(struct {
			Checkpoints []profile.Checkpoint `json:"checkpoints"`
		}{Checkpoints: checkpoints})
	}
	_, _, user, err := a.requireHumanUser(account)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if receipt.Account.UID != 0 && receipt.Account.UID != user.UID {
		return a.profileFail(i18n.T("The profile receipt belongs to an earlier account with this name."))
	}
	if !a.userStorageCapabilities().Subvolumes || !a.isBtrfsSubvolume(user.Home) {
		return a.profileFail(i18n.T("Checkpoints require a managed Btrfs home."))
	}
	switch action {
	case "save":
		return a.profileCheckpointSave(receipt, user, label)
	case "revert":
		return a.profileCheckpointRevert(receipt, user, id)
	case "delete":
		return a.profileCheckpointDelete(receipt, user, id)
	default:
		return a.profileFail(i18n.T("Unknown checkpoint action."))
	}
}

func parseCheckpointArgs(action string, args []string) (account, label, id string, err error) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--account":
			if index+1 >= len(args) || account != "" {
				return "", "", "", fmt.Errorf(i18n.T("--account needs one value."))
			}
			index++
			account = args[index]
		case "--label":
			if action != "save" || index+1 >= len(args) || label != "" {
				return "", "", "", fmt.Errorf(i18n.T("Unexpected --label option."))
			}
			index++
			label = args[index]
		case "--id":
			if action != "revert" && action != "delete" || index+1 >= len(args) || id != "" {
				return "", "", "", fmt.Errorf(i18n.T("Unexpected --id option."))
			}
			index++
			id = args[index]
		default:
			return "", "", "", fmt.Errorf(i18n.T("Unknown checkpoint option %s."), args[index])
		}
	}
	if !usernamePattern.MatchString(account) || (action == "revert" || action == "delete") && !checkpointIDPattern.MatchString(id) {
		return "", "", "", fmt.Errorf(i18n.T("Checkpoint action arguments are invalid."))
	}
	if strings.ContainsAny(label, "\r\n\x00") {
		return "", "", "", fmt.Errorf(i18n.T("A checkpoint label must be one line."))
	}
	return account, label, id, nil
}

func (a *App) profileCheckpointSave(receipt *profile.SystemReceipt, user localUser, label string) int {
	created := a.cfg.Now().UTC()
	checkpoint := profile.Checkpoint{ID: created.Format("20060102T150405Z"), Label: label, CreatedAt: created}
	for _, existing := range receipt.Checkpoints {
		if existing.ID == checkpoint.ID {
			return a.profileFail(i18n.T("A checkpoint was already saved this second."))
		}
	}
	root := filepath.Join(a.cfg.HomeRoot, ".checkpoints", user.Name)
	if err := a.usersRun(nil, "install", "-d", "-m", "0700", root); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Prepare checkpoints: %v"), err))
	}
	path := filepath.Join(root, checkpoint.ID)
	if err := a.usersRun(nil, "btrfs", "subvolume", "snapshot", "-r", user.Home, path); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Save the checkpoint: %v"), err))
	}
	receipt.Checkpoints = append(receipt.Checkpoints, checkpoint)
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		_ = a.usersRun(nil, "btrfs", "subvolume", "delete", path)
		return a.profileFail(err.Error())
	}
	return a.profileJSON(struct {
		OK         bool               `json:"ok"`
		Checkpoint profile.Checkpoint `json:"checkpoint"`
	}{OK: true, Checkpoint: checkpoint})
}

func (a *App) profileCheckpointRevert(receipt *profile.SystemReceipt, user localUser, id string) int {
	if !receiptHasCheckpoint(receipt, id) {
		return a.profileFail(i18n.T("No checkpoint with that id exists."))
	}
	loggedIn, err := a.loggedInUsers([]localUser{user})
	if err != nil {
		return a.profileFail(err.Error())
	}
	if loggedIn[user.Name] {
		return a.profileFail(fmt.Sprintf(i18n.T("Sign out %s before reverting a checkpoint."), user.Name))
	}
	manifest, err := profile.LoadManifest(receipt.Staged)
	if err != nil {
		return a.profileFail(err.Error())
	}
	storage := a.userStorage(user.Home, a.userStorageCapabilities())
	checkpoint := filepath.Join(a.cfg.HomeRoot, ".checkpoints", user.Name, id)
	aside := filepath.Join(a.cfg.HomeRoot, ".checkpoints", user.Name, ".revert-"+id)
	if _, err := os.Lstat(aside); err == nil {
		return a.profileFail(i18n.T("A previous checkpoint revert needs repair before trying again."))
	}
	if err := a.usersRun(nil, "mv", "--", user.Home, aside); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Move the current home aside: %v"), err))
	}
	if err := a.usersRun(nil, "btrfs", "subvolume", "snapshot", checkpoint, user.Home); err != nil {
		_ = a.usersRun(nil, "mv", "--", aside, user.Home)
		return a.profileFail(fmt.Sprintf(i18n.T("Restore the checkpoint: %v"), err))
	}
	for _, relative := range manifest.UserData {
		oldPath := filepath.Join(aside, filepath.FromSlash(relative))
		if _, err := os.Lstat(oldPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return a.profileFail(a.rollbackCheckpointHome(user.Home, aside, err).Error())
		}
		newPath := filepath.Join(user.Home, filepath.FromSlash(relative))
		if err := ensureCheckpointParents(user, newPath); err != nil {
			return a.profileFail(a.rollbackCheckpointHome(user.Home, aside, err).Error())
		}
		if err := a.usersRun(nil, "/usr/bin/rm", "-rf", "--", newPath); err != nil {
			return a.profileFail(a.rollbackCheckpointHome(user.Home, aside, err).Error())
		}
		if err := a.usersRun(nil, "/usr/bin/cp", "-a", "--no-dereference", "--reflink=auto", "--", oldPath, newPath); err != nil {
			return a.profileFail(a.rollbackCheckpointHome(user.Home, aside, fmt.Errorf(i18n.T("Restore current user data: %v"), err)).Error())
		}
	}
	if err := a.usersRun(nil, "btrfs", "subvolume", "delete", aside); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Delete the replaced home: %v"), err))
	}
	if storage.LimitBytes > 0 {
		if err := a.ensureUserQuota(); err != nil {
			return a.profileFail(err.Error())
		}
		if err := a.applyUserLimit(user.Home, storage.LimitBytes); err != nil {
			return a.profileFail(err.Error())
		}
	}
	return a.profileJSON(map[string]bool{"ok": true})
}

func ensureCheckpointParents(user localUser, destination string) error {
	home := filepath.Clean(user.Home)
	parent := filepath.Clean(filepath.Dir(destination))
	relative, err := filepath.Rel(home, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf(i18n.T("A profile user-data path escapes the account home."))
	}
	current := home
	components := []string{}
	if relative != "." {
		components = strings.Split(relative, string(filepath.Separator))
	}
	for index := -1; index < len(components); index++ {
		if index >= 0 {
			current = filepath.Join(current, components[index])
		}
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && index >= 0 {
			if err := os.Mkdir(current, 0o755); err != nil {
				return err
			}
			if err := os.Chown(current, user.UID, user.GID); err != nil {
				return err
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf(i18n.T("Profile user-data parent %s is not a real directory."), current)
		}
	}
	return nil
}

func (a *App) rollbackCheckpointHome(home, aside string, cause error) error {
	if err := a.usersRun(nil, "btrfs", "subvolume", "delete", home); err != nil {
		return fmt.Errorf(i18n.T("%v; rolling back the home failed: %v"), cause, err)
	}
	if err := a.usersRun(nil, "mv", "--", aside, home); err != nil {
		return fmt.Errorf(i18n.T("%v; restoring the previous home failed: %v"), cause, err)
	}
	return cause
}

func (a *App) profileCheckpointDelete(receipt *profile.SystemReceipt, user localUser, id string) int {
	if !receiptHasCheckpoint(receipt, id) {
		return a.profileFail(i18n.T("No checkpoint with that id exists."))
	}
	path := filepath.Join(a.cfg.HomeRoot, ".checkpoints", user.Name, id)
	if _, err := os.Lstat(path); err == nil {
		if err := a.usersRun(nil, "btrfs", "subvolume", "delete", path); err != nil {
			return a.profileFail(fmt.Sprintf(i18n.T("Delete the checkpoint: %v"), err))
		}
	} else if !os.IsNotExist(err) {
		return a.profileFail(err.Error())
	}
	kept := receipt.Checkpoints[:0]
	for _, checkpoint := range receipt.Checkpoints {
		if checkpoint.ID != id {
			kept = append(kept, checkpoint)
		}
	}
	receipt.Checkpoints = kept
	if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
		return a.profileFail(err.Error())
	}
	return a.profileJSON(map[string]bool{"ok": true})
}

func receiptHasCheckpoint(receipt *profile.SystemReceipt, id string) bool {
	for _, checkpoint := range receipt.Checkpoints {
		if checkpoint.ID == id {
			return true
		}
	}
	return false
}
