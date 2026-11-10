package host

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	i18n "ryoku-i18n"
)

const defaultShareRoot = "/usr/share/ryoku"

var provisionRicePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var provisionDesktopIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*\.desktop$`)

type ProvisionOptions struct {
	Rice        string
	Rashin      bool
	HideApps    []string
	SkipWelcome bool
}

type provisionSource struct {
	source      string
	destination string
	directory   bool
}

func (a *App) usersProvision(args []string) int {
	name, options, err := parseProvisionArgs(args, true)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if err := a.provisionAccount(name, options); err != nil {
		return a.usersFail(err.Error())
	}
	return a.usersOK()
}

func (a *App) usersSeedSelf(args []string) int {
	_, options, err := parseProvisionArgs(args, false)
	if err != nil {
		return a.usersFail(err.Error())
	}
	if err := a.seedSelf(options); err != nil {
		return a.usersFail(err.Error())
	}
	return a.usersOK()
}

func parseProvisionArgs(args []string, named bool) (string, ProvisionOptions, error) {
	options := ProvisionOptions{Rice: "default"}
	name := ""
	if named {
		if len(args) == 0 {
			return "", options, fmt.Errorf(i18n.T("Use provision <name> [--rice <slug>] [--rashin] [--hide-app <id>]... [--skip-welcome]."))
		}
		name, args = args[0], args[1:]
	}
	seenRice, seenRashin, seenSkip := false, false, false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--rice":
			if seenRice {
				return "", options, fmt.Errorf(i18n.T("--rice can be used only once."))
			}
			if index+1 >= len(args) {
				return "", options, fmt.Errorf(i18n.T("--rice needs a value."))
			}
			index++
			options.Rice = args[index]
			seenRice = true
		case "--rashin":
			if seenRashin {
				return "", options, fmt.Errorf(i18n.T("--rashin can be used only once."))
			}
			options.Rashin, seenRashin = true, true
		case "--hide-app":
			if index+1 >= len(args) {
				return "", options, fmt.Errorf(i18n.T("--hide-app needs a value."))
			}
			index++
			options.HideApps = append(options.HideApps, args[index])
		case "--skip-welcome":
			if seenSkip {
				return "", options, fmt.Errorf(i18n.T("--skip-welcome can be used only once."))
			}
			options.SkipWelcome, seenSkip = true, true
		default:
			return "", options, fmt.Errorf(i18n.T("Unknown provision option %s."), args[index])
		}
	}
	if !provisionRicePattern.MatchString(options.Rice) {
		return "", options, fmt.Errorf(i18n.T("The rice name is invalid."))
	}
	seenApps := map[string]bool{}
	for _, id := range options.HideApps {
		if !provisionDesktopIDPattern.MatchString(id) {
			return "", options, fmt.Errorf(i18n.T("Invalid desktop entry %s."), id)
		}
		if seenApps[id] {
			return "", options, fmt.Errorf(i18n.T("Desktop entry %s was requested more than once."), id)
		}
		seenApps[id] = true
	}
	return name, options, nil
}

func provisionOptionArgs(options ProvisionOptions) []string {
	args := []string{"--rice", options.Rice}
	if options.Rashin {
		args = append(args, "--rashin")
	}
	for _, id := range options.HideApps {
		args = append(args, "--hide-app", id)
	}
	if options.SkipWelcome {
		args = append(args, "--skip-welcome")
	}
	return args
}

func (a *App) provisionAccount(name string, options ProvisionOptions) error {
	if a.cfg.UID != 0 {
		return fmt.Errorf(i18n.T("Administrator authentication is required."))
	}
	if options.Rice == "" {
		options.Rice = "default"
	}
	if _, _, err := parseProvisionArgs(append([]string{name}, provisionOptionArgs(options)...), true); err != nil {
		return err
	}
	_, _, account, err := a.requireHumanUser(name)
	if err != nil {
		return err
	}
	if !a.isManagedHome(account) {
		return fmt.Errorf(i18n.T("This account does not use a managed home folder."))
	}
	environment := []string{
		"-i",
		"HOME=" + account.Home,
		"USER=" + account.Name,
		"LOGNAME=" + account.Name,
		"SHELL=" + account.Shell,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	for _, key := range []string{"LANG", "LC_ALL", "RYOKU_HOST_INIT", "RYOKU_SHARE_ROOT", "RYOKU_DROP_PACKAGES"} {
		if value := a.getenv(key); value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	command := append([]string{"-u", account.Name, "--", "/usr/bin/env"}, environment...)
	command = append(command, "/usr/bin/ryoku-host", "users", "seed-self")
	command = append(command, provisionOptionArgs(options)...)
	result := a.cfg.Runner.Run(Command{Name: "runuser", Args: command, Stdout: io.Discard, Stderr: a.cfg.Stderr})
	if result.Code != 0 {
		return fmt.Errorf(i18n.T("Provisioning %s exited with status %d."), name, result.Code)
	}
	return nil
}

func (a *App) seedSelf(options ProvisionOptions) error {
	home := filepath.Clean(a.getenv("HOME"))
	if !filepath.IsAbs(home) || home == string(filepath.Separator) {
		return fmt.Errorf(i18n.T("HOME must name the account's home folder."))
	}
	for _, location := range [...]struct {
		key      string
		relative string
	}{
		{"XDG_CONFIG_HOME", ".config"},
		{"XDG_DATA_HOME", ".local/share"},
		{"XDG_STATE_HOME", ".local/state"},
		{"XDG_CACHE_HOME", ".cache"},
	} {
		key, relative := location.key, location.relative
		value := a.getenv(key)
		if value != "" && filepath.Clean(value) != filepath.Join(home, relative) {
			return fmt.Errorf(i18n.T("%s must stay inside the account's home folder."), key)
		}
	}
	shareRoot := filepath.Clean(a.getenv("RYOKU_SHARE_ROOT"))
	if shareRoot == "." || shareRoot == "" {
		shareRoot = defaultShareRoot
	}
	if !filepath.IsAbs(shareRoot) {
		return fmt.Errorf(i18n.T("RYOKU_SHARE_ROOT must be an absolute path."))
	}
	sources := []provisionSource{
		{filepath.Join(shareRoot, "brand"), ".local/share/ryoku/assets/brand", true},
		{filepath.Join(shareRoot, "ryodecors"), "Pictures/ryodecors", true},
		{filepath.Join(shareRoot, "npmrc"), ".npmrc", false},
	}
	applicationsRoot := filepath.Join(filepath.Dir(shareRoot), "applications")
	for _, source := range sources {
		if err := preflightProvisionSource(home, source); err != nil {
			return err
		}
	}
	for _, relative := range []string{
		".config", ".config/user-dirs.dirs", ".local", ".local/share", ".local/share/applications",
		".local/share/quickshell-lockscreen", ".local/share/qylock", ".local/state", ".local/state/ryoku",
		"Desktop", "Documents", "Downloads", "Music", "Pictures", "Public", "Templates", "Videos", "Videos/Recordings",
	} {
		if _, err := checkedHomePath(home, relative); err != nil {
			return err
		}
	}
	if strings.HasSuffix(a.getenv("SHELL"), "/zsh") {
		if _, err := checkedHomePath(home, ".zshrc"); err != nil {
			return err
		}
	}
	ids := append([]string(nil), options.HideApps...)
	if regularFile(filepath.Join(applicationsRoot, "nvim.desktop")) && regularFile(filepath.Join(applicationsRoot, "ryoku-nvim.desktop")) {
		ids = append(ids, "nvim.desktop")
	}
	for _, id := range ids {
		if !regularFile(filepath.Join(applicationsRoot, id)) {
			return fmt.Errorf(i18n.T("Desktop entry %s is not installed."), id)
		}
		if _, err := checkedHomePath(home, filepath.Join(".local/share/applications", id)); err != nil {
			return err
		}
	}
	if result := a.cfg.Runner.Run(Command{Name: "/usr/bin/ryoku", Args: []string{"materialize"}, Stdout: io.Discard, Stderr: a.cfg.Stderr}); result.Code != 0 {
		return fmt.Errorf(i18n.T("Materialize the desktop configuration: ryoku exited with status %d."), result.Code)
	}
	for _, source := range sources {
		if err := installProvisionSource(home, source); err != nil {
			return err
		}
	}
	if a.cfg.Runner.LookPath("xdg-user-dirs-update") {
		if result := a.cfg.Runner.Run(Command{Name: "xdg-user-dirs-update", Stdout: io.Discard, Stderr: a.cfg.Stderr}); result.Code != 0 {
			return fmt.Errorf(i18n.T("Initialize the account folders: xdg-user-dirs-update exited with status %d."), result.Code)
		}
	}
	if err := seedXDGDirectories(home); err != nil {
		return err
	}
	if strings.HasSuffix(a.getenv("SHELL"), "/zsh") {
		if err := installFileIfAbsent(filepath.Join(shareRoot, "zsh", "zshrc"), home, ".zshrc"); err != nil {
			return err
		}
	}
	state := ".local/state/ryoku"
	if err := writeHomeFile(home, filepath.Join(state, "default-rice-pending"), []byte(options.Rice+"\n"), 0o644); err != nil {
		return err
	}
	if options.Rashin {
		if err := writeHomeFile(home, filepath.Join(state, "rashin-on-pending"), []byte("pending\n"), 0o644); err != nil {
			return err
		}
	} else if err := removeHomeFile(home, filepath.Join(state, "rashin-on-pending")); err != nil {
		return err
	}
	if options.SkipWelcome {
		version, err := a.welcomeVersion()
		if err != nil {
			return err
		}
		if err := writeHomeFile(home, filepath.Join(state, "welcome-seen"), []byte(version), 0o644); err != nil {
			return err
		}
	}
	if err := seedLineLedger(home, filepath.Join(state, "provisioned"), splitList(a.getenv("RYOKU_DROP_PACKAGES"))); err != nil {
		return err
	}
	if err := seedLauncherOverrides(home, applicationsRoot, ids); err != nil {
		return err
	}
	qylockInstaller := filepath.Join(shareRoot, "lockscreen", "install-qylock")
	if regularFile(qylockInstaller) {
		result := a.cfg.Runner.Run(Command{
			Name: qylockInstaller,
			Env: []string{
				"RYOKU_QYLOCK_BUNDLE=" + filepath.Join(shareRoot, "lockscreen", "qylock"),
				"RYOKU_QYLOCK_MODE=live",
				"RYOKU_QYLOCK_USER_ONLY=1",
			},
			Stdout: io.Discard,
			Stderr: a.cfg.Stderr,
		})
		if result.Code != 0 {
			return fmt.Errorf(i18n.T("Install the account lock screen: installer exited with status %d."), result.Code)
		}
	}
	if a.getenv("RYOKU_HOST_INIT") == "runit" {
		for _, args := range [][]string{{"session", "ensure", "--user"}, {"svc", "reconcile", "--user"}} {
			if result := a.cfg.Runner.Run(Command{Name: "/usr/bin/ryoku-host", Args: args, Stdout: io.Discard, Stderr: a.cfg.Stderr}); result.Code != 0 {
				return fmt.Errorf(i18n.T("Provision account services: ryoku-host exited with status %d."), result.Code)
			}
		}
	}
	return nil
}

func (a *App) welcomeVersion() (string, error) {
	result := a.cfg.Runner.Run(Command{Name: "/usr/bin/ryoku-session-intro", Args: []string{"version"}, Stderr: a.cfg.Stderr})
	version := strings.TrimSpace(result.Output)
	if result.Code != 0 || version == "" {
		return "", fmt.Errorf(i18n.T("Read the welcome tour version."))
	}
	for _, char := range version {
		if char < '0' || char > '9' {
			return "", fmt.Errorf(i18n.T("The welcome tour version is invalid."))
		}
	}
	return version, nil
}

func preflightProvisionSource(home string, source provisionSource) error {
	info, err := os.Lstat(source.source)
	if err != nil {
		return fmt.Errorf(i18n.T("Read packaged seed %s: %v"), source.source, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || source.directory != info.IsDir() {
		return fmt.Errorf(i18n.T("Packaged seed %s has an unsupported file type."), source.source)
	}
	if !source.directory {
		_, err = checkedHomePath(home, source.destination)
		return err
	}
	return filepath.WalkDir(source.source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf(i18n.T("Packaged seed %s contains a symbolic link."), source.source)
		}
		relative, err := filepath.Rel(source.source, path)
		if err != nil {
			return err
		}
		_, err = checkedHomePath(home, filepath.Join(source.destination, relative))
		return err
	})
}

func installProvisionSource(home string, source provisionSource) error {
	if !source.directory {
		return installFile(source.source, home, source.destination)
	}
	return filepath.WalkDir(source.source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source.source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(source.destination, relative)
		if entry.IsDir() {
			_, err = ensureHomeDir(home, destination)
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf(i18n.T("Packaged seed %s contains an unsupported file."), source.source)
		}
		return installFile(path, home, destination)
	})
}

func installFile(source, home, relative string) error {
	body, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf(i18n.T("Read packaged seed %s: %v"), source, err)
	}
	return writeHomeFile(home, relative, body, 0o644)
}

func installFileIfAbsent(source, home, relative string) error {
	if !regularFile(source) {
		return fmt.Errorf(i18n.T("Packaged seed %s is missing."), source)
	}
	destination, err := checkedHomePath(home, relative)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return installFile(source, home, relative)
}

func checkedHomePath(home, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf(i18n.T("Provisioning path %s must be relative."), relative)
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(i18n.T("Provisioning path %s leaves the home folder."), relative)
	}
	home = filepath.Clean(home)
	info, err := os.Lstat(home)
	if err != nil {
		return "", fmt.Errorf(i18n.T("Check the home folder: %v"), err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf(i18n.T("The home folder must be a real directory."))
	}
	current := home
	parts := strings.Split(clean, string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf(i18n.T("Refusing symbolic link destination %s."), current)
		}
	}
	return filepath.Join(home, clean), nil
}

func ensureHomeDir(home, relative string) (string, error) {
	if filepath.Clean(relative) == "." {
		info, err := os.Lstat(home)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf(i18n.T("The home folder must be a real directory."))
		}
		return filepath.Clean(home), nil
	}
	path, err := checkedHomePath(home, relative)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(home)
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if statErr != nil {
			return "", statErr
		}
		if !info.IsDir() {
			return "", fmt.Errorf(i18n.T("Provisioning destination %s is not a directory."), current)
		}
	}
	return path, nil
}

func writeHomeFile(home, relative string, body []byte, mode fs.FileMode) error {
	path, err := checkedHomePath(home, relative)
	if err != nil {
		return err
	}
	if _, err := ensureHomeDir(home, filepath.Dir(relative)); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf(i18n.T("Provisioning destination %s is not a regular file."), path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(body); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func removeHomeFile(home, relative string) error {
	path, err := checkedHomePath(home, relative)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf(i18n.T("Provisioning destination %s is not a regular file."), path)
	}
	return os.Remove(path)
}

func seedXDGDirectories(home string) error {
	locations := [...]struct {
		key      string
		fallback string
	}{
		{"XDG_DESKTOP_DIR", "Desktop"},
		{"XDG_DOWNLOAD_DIR", "Downloads"},
		{"XDG_TEMPLATES_DIR", "Templates"},
		{"XDG_PUBLICSHARE_DIR", "Public"},
		{"XDG_DOCUMENTS_DIR", "Documents"},
		{"XDG_MUSIC_DIR", "Music"},
		{"XDG_PICTURES_DIR", "Pictures"},
		{"XDG_VIDEOS_DIR", "Videos"},
	}
	config := `XDG_DESKTOP_DIR="$HOME/Desktop"
XDG_DOWNLOAD_DIR="$HOME/Downloads"
XDG_TEMPLATES_DIR="$HOME/Templates"
XDG_PUBLICSHARE_DIR="$HOME/Public"
XDG_DOCUMENTS_DIR="$HOME/Documents"
XDG_MUSIC_DIR="$HOME/Music"
XDG_PICTURES_DIR="$HOME/Pictures"
XDG_VIDEOS_DIR="$HOME/Videos"
`
	path, err := checkedHomePath(home, ".config/user-dirs.dirs")
	if err != nil {
		return err
	}
	body, readErr := os.ReadFile(path)
	if os.IsNotExist(readErr) {
		body = []byte(config)
		if err := writeHomeFile(home, ".config/user-dirs.dirs", body, 0o644); err != nil {
			return err
		}
	} else if readErr != nil {
		return readErr
	}
	video := "Videos"
	for _, location := range locations {
		relative := xdgHomeRelative(home, body, location.key, location.fallback)
		if _, err := ensureHomeDir(home, relative); err != nil {
			return err
		}
		if location.key == "XDG_VIDEOS_DIR" {
			video = relative
		}
	}
	recordings := recordingHomeRelative(home, filepath.Join(video, "Recordings"))
	_, err = ensureHomeDir(home, recordings)
	return err
}

func xdgHomeRelative(home string, body []byte, key, fallback string) string {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	prefix := key + "="
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"`)
		value = strings.ReplaceAll(value, "$HOME", home)
		if !filepath.IsAbs(value) {
			return fallback
		}
		relative, err := filepath.Rel(home, filepath.Clean(value))
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fallback
		}
		return relative
	}
	return fallback
}

func recordingHomeRelative(home, fallback string) string {
	body, err := os.ReadFile(filepath.Join(home, ".config", "ryoku", "recording.json"))
	if err != nil {
		return fallback
	}
	var config struct {
		Directory string `json:"directory"`
	}
	if json.Unmarshal(body, &config) != nil {
		return fallback
	}
	value := strings.TrimSpace(config.Directory)
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	if !filepath.IsAbs(value) {
		return fallback
	}
	relative, err := filepath.Rel(home, filepath.Clean(value))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fallback
	}
	return relative
}

func seedLauncherOverrides(home, applicationsRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	unique := map[string]bool{}
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		if unique[id] {
			continue
		}
		unique[id] = true
		ordered = append(ordered, id)
		body, err := os.ReadFile(filepath.Join(applicationsRoot, id))
		if err != nil {
			return err
		}
		override, err := NoDisplayDesktopEntry(body)
		if err != nil {
			return fmt.Errorf(i18n.T("Prepare launcher override %s: %v"), id, err)
		}
		if err := writeHomeFile(home, filepath.Join(".local/share/applications", id), override, 0o644); err != nil {
			return err
		}
	}
	return seedLineLedger(home, ".local/state/ryoku/launcher-overrides", ordered)
}

// NoDisplayDesktopEntry returns a user override that hides the desktop entry.
func NoDisplayDesktopEntry(body []byte) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	groupStart, groupEnd := -1, len(lines)
	for index, line := range lines {
		if line == "[Desktop Entry]" {
			groupStart = index
			continue
		}
		if groupStart >= 0 && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			groupEnd = index
			break
		}
	}
	if groupStart < 0 {
		return nil, fmt.Errorf(i18n.T("desktop entry has no [Desktop Entry] group"))
	}
	output := make([]string, 0, len(lines)+1)
	output = append(output, lines[:groupStart+1]...)
	output = append(output, "NoDisplay=true")
	for index := groupStart + 1; index < groupEnd; index++ {
		if strings.HasPrefix(lines[index], "NoDisplay=") {
			continue
		}
		output = append(output, lines[index])
	}
	output = append(output, lines[groupEnd:]...)
	return []byte(strings.TrimRight(strings.Join(output, "\n"), "\n") + "\n"), nil
}

func seedLineLedger(home, relative string, additions []string) error {
	if len(additions) == 0 {
		return nil
	}
	path, err := checkedHomePath(home, relative)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var lines []string
	if body, err := os.ReadFile(path); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !seen[line] {
				seen[line] = true
				lines = append(lines, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, line := range additions {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	return writeHomeFile(home, relative, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func splitList(value string) []string {
	return strings.Fields(strings.ReplaceAll(value, ",", " "))
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}
