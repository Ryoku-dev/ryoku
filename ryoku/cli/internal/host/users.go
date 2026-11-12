package host

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
)

var usernamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
var decimalBytesPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

var reservedUsernames = map[string]bool{
	"_apt": true, "backup": true, "bin": true, "daemon": true,
	"games": true, "irc": true, "list": true, "lp": true,
	"mail": true, "man": true, "news": true, "nfsnobody": true,
	"nobody": true, "proxy": true, "root": true, "sync": true,
	"sys": true, "systemd-coredump": true, "uucp": true,
	"www-data": true,
}

type userPermissionSpec struct {
	ID     string
	Label  string
	Groups []string
}

var userPermissionSpecs = []userPermissionSpec{
	{ID: "admin", Label: "Administrator", Groups: []string{"wheel"}},
	{ID: "devices", Label: "Devices", Groups: []string{"input", "video"}},
	{ID: "audio", Label: "Audio", Groups: []string{"audio"}},
	{ID: "virtualization", Label: "Virtualization", Groups: []string{"kvm", "libvirt"}},
	{ID: "containers", Label: "Containers", Groups: []string{"docker"}},
}

type localUser struct {
	Name     string
	FullName string
	UID      int
	GID      int
	Home     string
	Shell    string
}

type localGroup struct {
	Name    string
	GID     int
	Members map[string]bool
}

type usersStorage struct {
	Supported  bool   `json:"supported"`
	Reason     string `json:"reason"`
	UsedBytes  uint64 `json:"usedBytes"`
	LimitBytes uint64 `json:"limitBytes"`
}

type listedUser struct {
	Name        string       `json:"name"`
	FullName    string       `json:"fullName"`
	UID         int          `json:"uid"`
	Role        string       `json:"role"`
	Permissions []string     `json:"permissions"`
	Home        string       `json:"home"`
	LoggedIn    bool         `json:"loggedIn"`
	Storage     usersStorage `json:"storage"`
}

type usersList struct {
	Self  string       `json:"self"`
	Users []listedUser `json:"users"`
}

type usersPermissionCapability struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Groups []string `json:"groups"`
}

type usersStorageCapability struct {
	Subvolumes bool   `json:"subvolumes"`
	Quota      bool   `json:"quota"`
	Reason     string `json:"reason"`
}

type usersCapability struct {
	Supported   bool                        `json:"supported"`
	Reason      string                      `json:"reason"`
	Permissions []usersPermissionCapability `json:"permissions"`
	Storage     usersStorageCapability      `json:"storage"`
}

type usersMutationResult struct {
	OK bool `json:"ok"`
}

func (a *App) Users(args []string) int {
	if len(args) == 0 {
		return a.usersFail(i18n.T("Choose a users action."))
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return a.usersFail(i18n.T("users list does not accept arguments."))
		}
		return a.usersList()
	case "caps":
		if len(args) != 1 {
			return a.usersFail(i18n.T("users caps does not accept arguments."))
		}
		return a.usersCaps()
	case "seed-self":
		if a.cfg.UID == 0 {
			return a.usersFail(i18n.T("seed-self must run as the account being provisioned."))
		}
		return a.usersSeedSelf(args[1:])
	case "add", "delete", "set-role", "set-perms", "set-password", "set-limit", "provision":
		if a.cfg.UID != 0 {
			return a.usersFail(i18n.T("Administrator authentication is required."))
		}
	default:
		return a.usersFail(i18n.T("Unknown users action."))
	}
	switch args[0] {
	case "add":
		return a.usersAdd(args[1:])
	case "delete":
		return a.usersDelete(args[1:])
	case "set-role":
		return a.usersSetRole(args[1:])
	case "set-perms":
		return a.usersSetPermissions(args[1:])
	case "set-password":
		return a.usersSetPassword(args[1:])
	case "set-limit":
		return a.usersSetLimit(args[1:])
	default:
		return a.usersProvision(args[1:])
	}
}

func (a *App) usersList() int {
	users, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return a.usersFail(fmt.Sprintf(i18n.T("Read local accounts: %v"), err))
	}
	humans := humanUsers(users)
	loggedIn, err := a.loggedInUsers(humans)
	if err != nil {
		return a.usersFail(err.Error())
	}
	storageCaps := a.userStorageCapabilities()
	result := usersList{Self: currentUserName(a, users), Users: make([]listedUser, 0, len(humans))}
	for _, user := range humans {
		role := "standard"
		if userInGroup(user, groups["wheel"]) {
			role = "admin"
		}
		result.Users = append(result.Users, listedUser{
			Name:        user.Name,
			FullName:    user.FullName,
			UID:         user.UID,
			Role:        role,
			Permissions: permissionsForUser(user, groups),
			Home:        user.Home,
			LoggedIn:    loggedIn[user.Name],
			Storage:     a.userStorage(user.Home, storageCaps),
		})
	}
	return a.usersJSON(result)
}

func (a *App) usersCaps() int {
	capability := a.usersCapability()
	return a.usersJSON(capability)
}

func (a *App) usersCapability() usersCapability {
	capability := usersCapability{Permissions: []usersPermissionCapability{}}
	_, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		capability.Reason = fmt.Sprintf(i18n.T("Local account data is unavailable: %v"), err)
		capability.Storage = a.userStorageCapabilities()
		return capability
	}
	for _, spec := range userPermissionSpecs {
		available := existingPermissionGroups(spec, groups)
		if len(available) == 0 {
			continue
		}
		capability.Permissions = append(capability.Permissions, usersPermissionCapability{
			ID: spec.ID, Label: i18n.T(spec.Label), Groups: available,
		})
	}
	if _, ok := groups["wheel"]; !ok {
		capability.Reason = i18n.T("The administrator group is unavailable.")
		capability.Storage = a.userStorageCapabilities()
		return capability
	}
	for _, command := range []string{"chpasswd", "gpasswd", "useradd", "userdel", "usermod"} {
		if !a.cfg.Runner.LookPath(command) {
			capability.Reason = fmt.Sprintf(i18n.T("Account management is unavailable because %s is missing."), command)
			capability.Storage = a.userStorageCapabilities()
			return capability
		}
	}
	capability.Supported = true
	capability.Storage = a.userStorageCapabilities()
	return capability
}

func (a *App) usersAdd(args []string) int {
	args, noProvision, err := takeNoProvision(args)
	if err != nil {
		return a.usersFail(err.Error())
	}
	name, fullName, role, permissions, limit, err := parseUsersAdd(args)
	if err != nil {
		return a.usersFail(err.Error())
	}
	users, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return a.usersFail(fmt.Sprintf(i18n.T("Read local accounts: %v"), err))
	}
	if err := validateNewUsername(name, users); err != nil {
		return a.usersFail(err.Error())
	}
	if groups[name] != nil {
		return a.usersFail(i18n.T("That username is reserved by an existing system group."))
	}
	if _, ok := groups["wheel"]; !ok && role == "admin" {
		return a.usersFail(i18n.T("The administrator group is unavailable."))
	}
	requested, err := resolveRequestedPermissions(permissions, groups)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if role == "admin" {
		requested["admin"] = true
	} else if requested["admin"] {
		return a.usersFail(i18n.T("A Standard account cannot request the Administrator permission."))
	}
	devices, _ := userPermission("devices")
	if len(existingPermissionGroups(devices, groups)) > 0 {
		requested["devices"] = true
	}
	groupNames, err := groupsForPermissions(requested, groups)
	if err != nil {
		return a.usersFail(err.Error())
	}
	home := filepath.Join(a.cfg.HomeRoot, name)
	if _, err := os.Lstat(home); err == nil {
		return a.usersFail(i18n.T("That account's home folder already exists."))
	} else if !os.IsNotExist(err) {
		return a.usersFail(fmt.Sprintf(i18n.T("Check the new home folder: %v"), err))
	}
	password, err := readUserPassword(a.cfg.Stdin)
	if err != nil {
		return a.usersFail(err.Error())
	}
	storageCaps := a.userStorageCapabilities()
	if limit > 0 && !storageCaps.Quota {
		return a.usersFail(storageCaps.Reason)
	}
	if storageCaps.Subvolumes {
		if err := a.ensureUserQuota(); err != nil {
			return a.usersFail(err.Error())
		}
	}
	shell := newAccountShell(users, currentUserName(a, users))
	useradd := []string{"--user-group", "--shell", shell, "--comment", fullName, "--home-dir", home}
	if storageCaps.Subvolumes {
		useradd = append(useradd, "--no-create-home")
	} else {
		useradd = append(useradd, "--create-home")
	}
	if len(groupNames) > 0 {
		useradd = append(useradd, "--groups", strings.Join(groupNames, ","))
	}
	useradd = append(useradd, name)
	if err := a.usersRun(nil, "useradd", useradd...); err != nil {
		return a.usersFail(fmt.Sprintf(i18n.T("Create %s: %v"), name, err))
	}
	createdSubvolume := false
	if storageCaps.Subvolumes {
		if err := a.usersRun(nil, "btrfs", "subvolume", "create", home); err != nil {
			a.rollbackAddedUser(name, home, false)
			return a.usersFail(fmt.Sprintf(i18n.T("Create the home storage: %v"), err))
		}
		createdSubvolume = true
	}
	chownArgs := []string{name + ":" + name, home}
	if createdSubvolume {
		skeleton := filepath.Clean(a.cfg.SkeletonDir) + string(filepath.Separator) + "."
		if err := a.usersRun(nil, "cp", "--archive", "--", skeleton, home); err != nil {
			a.rollbackAddedUser(name, home, true)
			return a.usersFail(fmt.Sprintf(i18n.T("Populate the home folder: %v"), err))
		}
		chownArgs = []string{"--recursive", name + ":" + name, home}
	}
	if err := a.usersRun(nil, "chown", chownArgs...); err != nil {
		a.rollbackAddedUser(name, home, createdSubvolume)
		return a.usersFail(fmt.Sprintf(i18n.T("Set the home owner: %v"), err))
	}
	if err := a.usersRun(nil, "chmod", "0700", home); err != nil {
		a.rollbackAddedUser(name, home, createdSubvolume)
		return a.usersFail(fmt.Sprintf(i18n.T("Protect the home folder: %v"), err))
	}
	if limit > 0 {
		if err := a.applyUserLimit(home, limit); err != nil {
			a.rollbackAddedUser(name, home, createdSubvolume)
			return a.usersFail(err.Error())
		}
	}
	if err := a.changeUserPassword(name, password); err != nil {
		a.rollbackAddedUser(name, home, createdSubvolume)
		return a.usersFail(err.Error())
	}
	if !noProvision {
		if err := a.provisionAccount(name, ProvisionOptions{Rice: "default"}); err != nil {
			a.rollbackAddedUser(name, home, createdSubvolume)
			return a.usersFail(fmt.Sprintf(i18n.T("Provision %s: %v"), name, err))
		}
	}
	return a.usersOK()
}

func takeNoProvision(args []string) ([]string, bool, error) {
	filtered := make([]string, 0, len(args))
	seen := false
	for _, arg := range args {
		if arg != "--no-provision" {
			filtered = append(filtered, arg)
			continue
		}
		if seen {
			return nil, false, fmt.Errorf(i18n.T("--no-provision can be used only once."))
		}
		seen = true
	}
	return filtered, seen, nil
}

func (a *App) usersDelete(args []string) int {
	if len(args) != 2 || args[1] != "--keep-files" && args[1] != "--erase-files" {
		return a.usersFail(i18n.T("Use delete <name> with --keep-files or --erase-files."))
	}
	name := args[0]
	users, groups, target, err := a.requireHumanUser(name)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if currentUserName(a, users) == name {
		return a.usersFail(i18n.T("You cannot delete your own account."))
	}
	if userInGroup(target, groups["wheel"]) && administratorCount(users, groups) <= 1 {
		return a.usersFail(i18n.T("You cannot delete the last Administrator account."))
	}
	loggedIn, err := a.loggedInUsers(humanUsers(users))
	if err != nil {
		return a.usersFail(err.Error())
	}
	if loggedIn[name] {
		return a.usersFail(fmt.Sprintf(i18n.T("Sign out %s before deleting the account."), name))
	}
	if !a.isManagedHome(target) {
		return a.usersFail(i18n.T("This account does not use a managed home folder."))
	}
	if args[1] == "--keep-files" {
		archiveRoot := filepath.Join(a.cfg.HomeRoot, ".removed")
		archive := filepath.Join(archiveRoot, name+"-"+a.cfg.Now().Format("2006-01-02"))
		if _, err := os.Lstat(archive); err == nil {
			return a.usersFail(i18n.T("An archived home with today's name already exists."))
		} else if !os.IsNotExist(err) {
			return a.usersFail(fmt.Sprintf(i18n.T("Check the archived home: %v"), err))
		}
		if err := a.usersRun(nil, "install", "-d", "-m", "0700", archiveRoot); err != nil {
			return a.usersFail(fmt.Sprintf(i18n.T("Prepare the archived homes folder: %v"), err))
		}
		if err := a.usersRun(nil, "mv", "--", target.Home, archive); err != nil {
			return a.usersFail(fmt.Sprintf(i18n.T("Archive the home folder: %v"), err))
		}
		if err := a.usersRun(nil, "userdel", name); err != nil {
			_ = a.usersRun(nil, "mv", "--", archive, target.Home)
			return a.usersFail(fmt.Sprintf(i18n.T("Delete %s: %v"), name, err))
		}
		a.dropSnapshotReader(name)
		return a.usersOK()
	}
	if a.isBtrfsSubvolume(target.Home) {
		if err := a.usersRun(nil, "userdel", name); err != nil {
			return a.usersFail(fmt.Sprintf(i18n.T("Delete %s: %v"), name, err))
		}
		a.dropSnapshotReader(name)
		if err := a.usersRun(nil, "btrfs", "subvolume", "delete", target.Home); err != nil {
			return a.usersFail(fmt.Sprintf(i18n.T("Erase the home storage: %v"), err))
		}
		return a.usersOK()
	}
	if err := a.usersRun(nil, "userdel", "--remove", name); err != nil {
		return a.usersFail(fmt.Sprintf(i18n.T("Delete %s: %v"), name, err))
	}
	a.dropSnapshotReader(name)
	return a.usersOK()
}

// snapper refuses every command, cleanup included, while ALLOW_USERS names an
// account that no longer exists
func (a *App) dropSnapshotReader(name string) {
	info, err := os.Stat(a.cfg.SnapperRootConfig)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	data, err := os.ReadFile(a.cfg.SnapperRootConfig)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "ALLOW_USERS=")
		if !ok {
			continue
		}
		readers := strings.Fields(strings.Trim(value, `"`))
		kept := make([]string, 0, len(readers))
		for _, reader := range readers {
			if reader != name {
				kept = append(kept, reader)
			}
		}
		if len(kept) != len(readers) {
			lines[i] = `ALLOW_USERS="` + strings.Join(kept, " ") + `"`
			changed = true
		}
	}
	if changed {
		_ = replaceFile(a.cfg.SnapperRootConfig, strings.Join(lines, "\n"), info.Mode().Perm())
	}
}

func (a *App) usersSetRole(args []string) int {
	if len(args) != 2 || args[1] != "admin" && args[1] != "standard" {
		return a.usersFail(i18n.T("Use set-role <name> admin or set-role <name> standard."))
	}
	users, groups, target, err := a.requireHumanUser(args[0])
	if err != nil {
		return a.usersFail(err.Error())
	}
	wheel := groups["wheel"]
	if wheel == nil {
		return a.usersFail(i18n.T("The administrator group is unavailable."))
	}
	isAdmin := userInGroup(target, wheel)
	if args[1] == "admin" {
		if !isAdmin {
			if err := a.usersRun(nil, "usermod", "--append", "--groups", "wheel", target.Name); err != nil {
				return a.usersFail(fmt.Sprintf(i18n.T("Make %s an Administrator: %v"), target.Name, err))
			}
		}
		return a.usersOK()
	}
	if isAdmin && administratorCount(users, groups) <= 1 {
		return a.usersFail(i18n.T("You cannot demote the last Administrator account."))
	}
	if isAdmin {
		if err := a.usersRun(nil, "gpasswd", "--delete", target.Name, "wheel"); err != nil {
			return a.usersFail(fmt.Sprintf(i18n.T("Make %s a Standard account: %v"), target.Name, err))
		}
	}
	return a.usersOK()
}

func (a *App) usersSetPermissions(args []string) int {
	if len(args) != 2 {
		return a.usersFail(i18n.T("Use set-perms <name> <id,id,...>."))
	}
	ids, err := parsePermissionList(args[1])
	if err != nil {
		return a.usersFail(err.Error())
	}
	users, groups, target, err := a.requireHumanUser(args[0])
	if err != nil {
		return a.usersFail(err.Error())
	}
	desired, err := resolveRequestedPermissions(ids, groups)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if userInGroup(target, groups["wheel"]) && !desired["admin"] && administratorCount(users, groups) <= 1 {
		return a.usersFail(i18n.T("You cannot remove Administrator access from the last Administrator account."))
	}
	for _, spec := range userPermissionSpecs {
		available := existingPermissionGroups(spec, groups)
		for _, groupName := range available {
			member := userInGroup(target, groups[groupName])
			if desired[spec.ID] && !member {
				if err := a.usersRun(nil, "usermod", "--append", "--groups", groupName, target.Name); err != nil {
					return a.usersFail(fmt.Sprintf(i18n.T("Add %s permission: %v"), spec.Label, err))
				}
			}
			if !desired[spec.ID] && member {
				if err := a.usersRun(nil, "gpasswd", "--delete", target.Name, groupName); err != nil {
					return a.usersFail(fmt.Sprintf(i18n.T("Remove %s permission: %v"), spec.Label, err))
				}
			}
		}
	}
	return a.usersOK()
}

func (a *App) usersSetPassword(args []string) int {
	if len(args) != 1 {
		return a.usersFail(i18n.T("Use set-password <name> and provide the password on standard input."))
	}
	_, _, target, err := a.requireHumanUser(args[0])
	if err != nil {
		return a.usersFail(err.Error())
	}
	password, err := readUserPassword(a.cfg.Stdin)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if err := a.changeUserPassword(target.Name, password); err != nil {
		return a.usersFail(err.Error())
	}
	return a.usersOK()
}

func (a *App) usersSetLimit(args []string) int {
	if len(args) != 2 {
		return a.usersFail(i18n.T("Use set-limit <name> <bytes|0>."))
	}
	limit, err := parseBytes(args[1])
	if err != nil {
		return a.usersFail(err.Error())
	}
	_, _, target, err := a.requireHumanUser(args[0])
	if err != nil {
		return a.usersFail(err.Error())
	}
	if !a.isManagedHome(target) {
		return a.usersFail(i18n.T("This account does not use a managed home folder."))
	}
	caps := a.userStorageCapabilities()
	if !caps.Quota {
		return a.usersFail(caps.Reason)
	}
	if !a.isBtrfsSubvolume(target.Home) {
		return a.usersFail(i18n.T("This account's home folder is not its own Btrfs subvolume, so it cannot have a storage limit."))
	}
	if err := a.ensureUserQuota(); err != nil {
		return a.usersFail(err.Error())
	}
	if err := a.applyUserLimit(target.Home, limit); err != nil {
		return a.usersFail(err.Error())
	}
	return a.usersOK()
}

func parseUsersAdd(args []string) (string, string, string, []string, uint64, error) {
	if len(args) < 5 {
		return "", "", "", nil, 0, fmt.Errorf(i18n.T("Use add <name> --full-name <name> --role admin|standard [--perm id]... [--limit bytes]."))
	}
	name := args[0]
	fullName, role := "", ""
	var permissions []string
	var limit uint64
	seenFullName, seenRole, seenLimit := false, false, false
	for index := 1; index < len(args); {
		flag := args[index]
		if index+1 >= len(args) {
			return "", "", "", nil, 0, fmt.Errorf(i18n.T("%s needs a value."), flag)
		}
		value := args[index+1]
		switch flag {
		case "--full-name":
			if seenFullName {
				return "", "", "", nil, 0, fmt.Errorf(i18n.T("--full-name can be used only once."))
			}
			seenFullName, fullName = true, value
		case "--role":
			if seenRole {
				return "", "", "", nil, 0, fmt.Errorf(i18n.T("--role can be used only once."))
			}
			seenRole, role = true, value
		case "--perm":
			permissions = append(permissions, value)
		case "--limit":
			if seenLimit {
				return "", "", "", nil, 0, fmt.Errorf(i18n.T("--limit can be used only once."))
			}
			parsed, err := parseBytes(value)
			if err != nil {
				return "", "", "", nil, 0, err
			}
			seenLimit, limit = true, parsed
		default:
			return "", "", "", nil, 0, fmt.Errorf(i18n.T("Unknown add option %s."), flag)
		}
		index += 2
	}
	if !seenFullName || !seenRole || role != "admin" && role != "standard" {
		return "", "", "", nil, 0, fmt.Errorf(i18n.T("Add requires --full-name and --role admin|standard."))
	}
	if strings.ContainsAny(fullName, ",:\r\n\x00") {
		return "", "", "", nil, 0, fmt.Errorf(i18n.T("The full name contains an unsupported character."))
	}
	return name, fullName, role, permissions, limit, nil
}

func parsePermissionList(value string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	parts := strings.Split(value, ",")
	seen := map[string]bool{}
	for _, part := range parts {
		if part == "" || seen[part] {
			return nil, fmt.Errorf(i18n.T("Permissions must be a comma-separated list without empty or repeated values."))
		}
		seen[part] = true
	}
	return parts, nil
}

func parseBytes(value string) (uint64, error) {
	if !decimalBytesPattern.MatchString(value) {
		return 0, fmt.Errorf(i18n.T("Storage limits must be whole bytes, or 0 for no limit."))
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(i18n.T("Storage limit is too large."))
	}
	return parsed, nil
}

func validateNewUsername(name string, users []localUser) error {
	if !usernamePattern.MatchString(name) {
		return fmt.Errorf(i18n.T("Usernames must start with a lowercase letter or underscore and use at most 32 lowercase letters, numbers, underscores, or hyphens."))
	}
	if reservedUsernames[name] {
		return fmt.Errorf(i18n.T("That username is reserved by the system."))
	}
	for _, user := range users {
		if user.Name == name {
			return fmt.Errorf(i18n.T("A local account named %s already exists."), name)
		}
	}
	return nil
}

func resolveRequestedPermissions(ids []string, groups map[string]*localGroup) (map[string]bool, error) {
	requested := make(map[string]bool, len(ids))
	for _, id := range ids {
		spec, ok := userPermission(id)
		if !ok {
			return nil, fmt.Errorf(i18n.T("Unknown permission %s."), id)
		}
		if requested[id] {
			return nil, fmt.Errorf(i18n.T("Permission %s was provided more than once."), id)
		}
		if len(existingPermissionGroups(spec, groups)) == 0 {
			return nil, fmt.Errorf(i18n.T("Permission %s is unavailable on this system."), id)
		}
		requested[id] = true
	}
	return requested, nil
}

func userPermission(id string) (userPermissionSpec, bool) {
	for _, spec := range userPermissionSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return userPermissionSpec{}, false
}

func groupsForPermissions(requested map[string]bool, groups map[string]*localGroup) ([]string, error) {
	var names []string
	for _, spec := range userPermissionSpecs {
		if !requested[spec.ID] {
			continue
		}
		available := existingPermissionGroups(spec, groups)
		if len(available) == 0 {
			return nil, fmt.Errorf(i18n.T("Permission %s is unavailable on this system."), spec.ID)
		}
		names = append(names, available...)
	}
	sort.Strings(names)
	return names, nil
}

func existingPermissionGroups(spec userPermissionSpec, groups map[string]*localGroup) []string {
	available := make([]string, 0, len(spec.Groups))
	for _, name := range spec.Groups {
		if groups[name] != nil {
			available = append(available, name)
		}
	}
	return available
}

func permissionsForUser(user localUser, groups map[string]*localGroup) []string {
	permissions := []string{}
	for _, spec := range userPermissionSpecs {
		available := existingPermissionGroups(spec, groups)
		if len(available) == 0 {
			continue
		}
		hasPermission := true
		for _, name := range available {
			if !userInGroup(user, groups[name]) {
				hasPermission = false
				break
			}
		}
		if hasPermission {
			permissions = append(permissions, spec.ID)
		}
	}
	return permissions
}

func loadAccountDB(passwdPath, groupPath string) ([]localUser, map[string]*localGroup, error) {
	passwdFile, err := os.Open(passwdPath)
	if err != nil {
		return nil, nil, err
	}
	defer passwdFile.Close()
	users := []localUser{}
	scanner := bufio.NewScanner(passwdFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != 7 {
			return nil, nil, fmt.Errorf("invalid passwd entry")
		}
		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid uid for %s", fields[0])
		}
		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid gid for %s", fields[0])
		}
		fullName, _, _ := strings.Cut(fields[4], ",")
		users = append(users, localUser{
			Name:     fields[0],
			FullName: fullName,
			UID:      uid,
			GID:      gid,
			Home:     fields[5],
			Shell:    fields[6],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	groupFile, err := os.Open(groupPath)
	if err != nil {
		return nil, nil, err
	}
	defer groupFile.Close()
	groups := map[string]*localGroup{}
	scanner = bufio.NewScanner(groupFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			return nil, nil, fmt.Errorf("invalid group entry")
		}
		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid gid for %s", fields[0])
		}
		members := map[string]bool{}
		for _, member := range strings.Split(fields[3], ",") {
			if member != "" {
				members[member] = true
			}
		}
		groups[fields[0]] = &localGroup{Name: fields[0], GID: gid, Members: members}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return users, groups, nil
}

func humanUsers(users []localUser) []localUser {
	humans := make([]localUser, 0, len(users))
	for _, user := range users {
		if user.UID < 1000 || user.UID == 65534 || !filepath.IsAbs(user.Home) || !realLoginShell(user.Shell) {
			continue
		}
		humans = append(humans, user)
	}
	sort.Slice(humans, func(left, right int) bool { return humans[left].Name < humans[right].Name })
	return humans
}

func realLoginShell(shell string) bool {
	if !filepath.IsAbs(shell) {
		return false
	}
	base := filepath.Base(shell)
	return base != "false" && base != "nologin"
}

// the installer gives every account it creates the same login shell, so an
// account added later follows the administrator adding it
func newAccountShell(users []localUser, caller string) string {
	for _, user := range users {
		if user.Name != caller || !realLoginShell(user.Shell) {
			continue
		}
		if info, err := os.Stat(user.Shell); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return user.Shell
		}
	}
	return "/bin/bash"
}

func userInGroup(user localUser, group *localGroup) bool {
	return group != nil && (user.GID == group.GID || group.Members[user.Name])
}

func administratorCount(users []localUser, groups map[string]*localGroup) int {
	count := 0
	for _, user := range humanUsers(users) {
		if userInGroup(user, groups["wheel"]) {
			count++
		}
	}
	return count
}

func currentUserName(a *App, users []localUser) string {
	if value := a.getenv("PKEXEC_UID"); value != "" {
		if uid, err := strconv.Atoi(value); err == nil {
			for _, user := range users {
				if user.UID == uid {
					return user.Name
				}
			}
		}
	}
	for _, key := range []string{"SUDO_USER", "USER"} {
		name := a.getenv(key)
		for _, user := range users {
			if user.Name == name {
				return name
			}
		}
	}
	for _, user := range users {
		if user.UID == a.cfg.UID {
			return user.Name
		}
	}
	return ""
}

func (a *App) requireHumanUser(name string) ([]localUser, map[string]*localGroup, localUser, error) {
	if !usernamePattern.MatchString(name) {
		return nil, nil, localUser{}, fmt.Errorf(i18n.T("Invalid username."))
	}
	users, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return nil, nil, localUser{}, fmt.Errorf(i18n.T("Read local accounts: %v"), err)
	}
	for _, user := range humanUsers(users) {
		if user.Name == name {
			return users, groups, user, nil
		}
	}
	return nil, nil, localUser{}, fmt.Errorf(i18n.T("No managed local account named %s exists."), name)
}

func (a *App) isManagedHome(user localUser) bool {
	return filepath.Clean(user.Home) == filepath.Join(filepath.Clean(a.cfg.HomeRoot), user.Name)
}

func (a *App) loggedInUsers(users []localUser) (map[string]bool, error) {
	loggedIn := map[string]bool{}
	init, err := a.Init()
	if err != nil {
		return nil, err
	}
	if init == Systemd {
		result := a.usersQuery("loginctl", "list-users", "--no-legend", "--no-pager")
		if result.Code != 0 {
			return nil, fmt.Errorf(i18n.T("Check signed-in users: loginctl failed."))
		}
		for _, line := range strings.Split(result.Output, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				loggedIn[fields[1]] = true
			}
		}
		return loggedIn, nil
	}
	if !a.cfg.Runner.LookPath("ps") {
		return nil, fmt.Errorf(i18n.T("Check signed-in users: ps is unavailable."))
	}
	result := a.usersQuery("who")
	if result.Code != 0 {
		return nil, fmt.Errorf(i18n.T("Check signed-in users: who failed."))
	}
	for _, line := range strings.Split(result.Output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			loggedIn[fields[0]] = true
		}
	}
	for _, user := range users {
		if loggedIn[user.Name] {
			continue
		}
		processes := a.usersQuery("ps", "-u", user.Name, "-o", "pid=")
		if processes.Code == 0 && strings.TrimSpace(processes.Output) != "" {
			loggedIn[user.Name] = true
		}
	}
	return loggedIn, nil
}

func (a *App) userStorageCapabilities() usersStorageCapability {
	if !a.cfg.Runner.LookPath("btrfs") {
		return usersStorageCapability{Reason: i18n.T("Btrfs tools are not installed.")}
	}
	result := a.usersQuery("stat", "-f", "-c", "%T", a.cfg.HomeRoot)
	if result.Code != 0 {
		return usersStorageCapability{Reason: i18n.T("Home storage could not be inspected.")}
	}
	if strings.TrimSpace(result.Output) != "btrfs" {
		return usersStorageCapability{Reason: i18n.T("Home storage is not Btrfs.")}
	}
	return usersStorageCapability{Subvolumes: true, Quota: true}
}

func (a *App) userStorage(home string, caps usersStorageCapability) usersStorage {
	if !caps.Subvolumes {
		return usersStorage{Reason: caps.Reason}
	}
	qgroupID := ""
	subvolume := a.usersQuery("btrfs", "subvolume", "show", home)
	if subvolume.Code == 0 {
		qgroupID = btrfsSubvolumeQgroup(subvolume.Output)
	}
	if qgroupID == "" {
		subvolumes := a.usersQuery("btrfs", "subvolume", "list", "-o", a.cfg.HomeRoot)
		if subvolumes.Code == 0 {
			qgroupID = listedBtrfsSubvolumeQgroup(subvolumes.Output, filepath.Base(home))
		}
	}
	if qgroupID == "" {
		return usersStorage{Reason: i18n.T("Storage limits need this home folder to be its own Btrfs subvolume.")}
	}
	storage := usersStorage{Supported: true}
	result := a.usersQuery("btrfs", "qgroup", "show", "--raw", "-re", a.cfg.HomeRoot)
	if result.Code == 0 {
		if used, limit, ok := parseQgroupUsage(result.Output, qgroupID); ok {
			storage.UsedBytes, storage.LimitBytes = used, limit
			return storage
		}
	}
	result = a.usersQuery("btrfs", "filesystem", "du", "--raw", home)
	if result.Code == 0 {
		if used, ok := parseBtrfsDU(result.Output); ok {
			storage.UsedBytes = used
			return storage
		}
	}
	storage.Reason = i18n.T("Home storage usage is unavailable.")
	return storage
}

func btrfsSubvolumeQgroup(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "Subvolume" && fields[1] == "ID:" {
			if _, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
				return "0/" + fields[2]
			}
		}
	}
	return ""
}

func listedBtrfsSubvolumeQgroup(output, name string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "ID" || filepath.Base(fields[len(fields)-1]) != name {
			continue
		}
		if _, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			return "0/" + fields[1]
		}
	}
	return ""
}

func parseQgroupUsage(output, qgroupID string) (uint64, uint64, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != qgroupID {
			continue
		}
		used, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		limit := uint64(0)
		if fields[3] != "none" && fields[3] != "-" {
			limit, err = strconv.ParseUint(fields[3], 10, 64)
			if err != nil {
				continue
			}
		}
		return used, limit, true
	}
	return 0, 0, false
}

func parseBtrfsDU(output string) (uint64, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		used, err := strconv.ParseUint(fields[0], 10, 64)
		if err == nil {
			return used, true
		}
	}
	return 0, false
}

func (a *App) ensureUserQuota() error {
	if result := a.usersQuery("btrfs", "qgroup", "show", a.cfg.HomeRoot); result.Code == 0 {
		return nil
	}
	if err := a.usersRun(nil, "btrfs", "quota", "enable", a.cfg.HomeRoot); err != nil {
		return fmt.Errorf(i18n.T("Enable home storage limits: %v"), err)
	}
	return nil
}

func (a *App) applyUserLimit(home string, limit uint64) error {
	value := strconv.FormatUint(limit, 10)
	if limit == 0 {
		value = "none"
	}
	if err := a.usersRun(nil, "btrfs", "qgroup", "limit", value, home); err != nil {
		return fmt.Errorf(i18n.T("Set the home storage limit: %v"), err)
	}
	return nil
}

func (a *App) isBtrfsSubvolume(path string) bool {
	if !a.cfg.Runner.LookPath("btrfs") {
		return false
	}
	return a.usersQuery("btrfs", "subvolume", "show", path).Code == 0
}

func (a *App) changeUserPassword(name, password string) error {
	stdin := strings.NewReader(name + ":" + password + "\n")
	if err := a.usersRun(stdin, "chpasswd"); err != nil {
		return fmt.Errorf(i18n.T("Set the password for %s: %v"), name, err)
	}
	return nil
}

// the caller may keep stdin open after the password line, so stop at the newline
func readUserPassword(input io.Reader) (string, error) {
	data, err := bufio.NewReader(io.LimitReader(input, 4098)).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf(i18n.T("Read the password: %v"), err)
	}
	if len(data) > 4096 {
		return "", fmt.Errorf(i18n.T("The password is too long."))
	}
	value := string(data)
	if strings.HasSuffix(value, "\n") {
		value = strings.TrimSuffix(value, "\n")
		value = strings.TrimSuffix(value, "\r")
	}
	if value == "" {
		return "", fmt.Errorf(i18n.T("The password cannot be empty."))
	}
	if strings.ContainsAny(value, ":\r\n\x00") {
		return "", fmt.Errorf(i18n.T("The password must be one line and cannot contain a colon."))
	}
	return value, nil
}

func (a *App) rollbackAddedUser(name, home string, subvolume bool) {
	if subvolume {
		_ = a.usersRun(nil, "userdel", name)
		_ = a.usersRun(nil, "btrfs", "subvolume", "delete", home)
		return
	}
	_ = a.usersRun(nil, "userdel", "--remove", name)
}

func (a *App) usersQuery(name string, args ...string) Result {
	return a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: a.cfg.Stdin, Stderr: io.Discard})
}

func (a *App) usersRun(stdin io.Reader, name string, args ...string) error {
	result := a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: stdin, Stdout: io.Discard, Stderr: io.Discard})
	if result.Code != 0 {
		return fmt.Errorf("%s exited with status %d", name, result.Code)
	}
	return nil
}

func (a *App) usersOK() int {
	return a.usersJSON(usersMutationResult{OK: true})
}

func (a *App) usersFail(message string) int {
	_ = json.NewEncoder(a.cfg.Stdout).Encode(map[string]string{"error": message})
	return ExitFailure
}

func (a *App) usersJSON(value any) int {
	if err := json.NewEncoder(a.cfg.Stdout).Encode(value); err != nil {
		return a.usersFail(fmt.Sprintf(i18n.T("Encode users response: %v"), err))
	}
	return ExitOK
}
