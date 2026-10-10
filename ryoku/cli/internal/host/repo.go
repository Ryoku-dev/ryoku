package host

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ryoku-cli/internal/sys"
)

const (
	PacmanRepoBase = "https://repo.ryoku.dev/stable"
	XBPSRepoBase   = "https://repo.ryoku.dev/stable/void"
	DNFRepoBase    = "https://repo.ryoku.dev/stable/fedora/$releasever"
	ryokuRepoName  = "ryoku"
	xbpsRepoConfig = "20-ryoku.conf"
	dnfRepoConfig  = "/etc/yum.repos.d/ryoku.repo"
)

var ErrRepoAbsent = errors.New("no Ryoku package repository is configured")

var xbpsPackagePattern = regexp.MustCompile(`^(.+)-([^-]+_[0-9]+)$`)

func repoBaseFor(manager PackageManager) string {
	if base := strings.TrimSpace(os.Getenv("RYOKU_RELEASE_BASE")); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	switch manager {
	case XBPS:
		return XBPSRepoBase
	case DNF:
		return DNFRepoBase
	default:
		return PacmanRepoBase
	}
}

type RepoPackage struct {
	Name       string
	Version    string
	Repository string
}

func RepoURLFor(manager PackageManager, channel string) string {
	base, arch := repoBaseFor(manager), "$arch"
	if manager == XBPS || manager == DNF {
		arch = "x86_64"
	}
	switch {
	case channel == sys.ChannelStable:
		return base + "/" + arch
	case channel == sys.ChannelTesting:
		return base + "/channels/testing/" + arch
	case sys.IsReleaseTag(channel):
		return base + "/releases/" + channel + "/" + arch
	case sys.IsUnstableBuild(channel):
		return base + "/channels/testing/builds/" + sys.UnstableBuildDir(channel) + "/" + arch
	default:
		return ""
	}
}
func (a *App) Repo(args []string) int {
	if len(args) == 0 {
		return ExitUsage
	}
	switch args[0] {
	case "channel":
		if len(args) != 1 {
			return ExitUsage
		}
		channel, err := a.RepoChannel()
		if errors.Is(err, ErrRepoAbsent) {
			return ExitAbsent
		}
		if err != nil {
			return a.failf("%v", err)
		}
		fmt.Fprintln(a.cfg.Stdout, channel)
		return ExitOK
	case "set-channel":
		if len(args) != 2 {
			return ExitUsage
		}
		if err := a.RepoSetChannel(args[1]); err != nil {
			return a.failf("%v", err)
		}
		return ExitOK
	case "set-url":
		if len(args) != 2 {
			return ExitUsage
		}
		if err := a.RepoSetURL(args[1]); err != nil {
			return a.failf("%v", err)
		}
		return ExitOK
	case "sync":
		if len(args) > 2 || len(args) == 2 && args[1] != "--force" {
			return ExitUsage
		}
		if err := a.RepoSync(len(args) == 2); err != nil {
			return a.failf("%v", err)
		}
		return ExitOK
	default:
		return ExitUsage
	}
}

func RepoChannelOf(manager PackageManager, raw string) string {
	base := repoBaseFor(manager)
	value := strings.TrimSuffix(strings.TrimSpace(raw), "/")
	hasArch := false
	for _, arch := range []string{"x86_64", "$basearch", "$arch"} {
		if strings.HasSuffix(value, "/"+arch) {
			value = strings.TrimSuffix(value, "/"+arch)
			hasArch = true
			break
		}
	}
	if manager == DNF && !hasArch {
		return ""
	}
	var rest string
	if manager == DNF {
		var ok bool
		rest, ok = trimDNFRepoBase(value, base)
		if !ok {
			return ""
		}
	} else {
		if !strings.HasPrefix(value, base) {
			return ""
		}
		rest = strings.Trim(strings.TrimPrefix(value, base), "/")
	}
	switch {
	case rest == "":
		return sys.ChannelStable
	case rest == "channels/testing":
		return sys.ChannelTesting
	case strings.HasPrefix(rest, "releases/"):
		tag := strings.TrimPrefix(rest, "releases/")
		if sys.IsReleaseTag(tag) {
			return tag
		}
	case strings.HasPrefix(rest, "channels/testing/builds/"):
		return sys.UnstableBuildFromDir(strings.TrimPrefix(rest, "channels/testing/builds/"))
	}
	return ""
}

func trimDNFRepoBase(value, base string) (string, bool) {
	before, after, variable := strings.Cut(base, "$releasever")
	if !variable {
		if value != base && !strings.HasPrefix(value, base+"/") {
			return "", false
		}
		return strings.Trim(strings.TrimPrefix(value, base), "/"), true
	}
	if !strings.HasPrefix(value, before) {
		return "", false
	}
	remainder := strings.TrimPrefix(value, before)
	release, remainder, found := strings.Cut(remainder, "/")
	if after != "" {
		return "", false
	}
	if release != "$releasever" && !decimalRelease(release) {
		return "", false
	}
	if !found {
		remainder = ""
	}
	return strings.Trim(remainder, "/"), true
}

func decimalRelease(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
func ExpandRepoURL(raw string) string {
	path := strings.TrimSpace(os.Getenv("RYOKU_OS_RELEASE"))
	if path == "" {
		path = "/etc/os-release"
	}
	release := osReleaseVersion(path)
	if release != "" {
		raw = strings.ReplaceAll(raw, "$releasever", release)
	}
	raw = strings.ReplaceAll(raw, "$basearch", "x86_64")
	return strings.ReplaceAll(raw, "$arch", "x86_64")
}

func osReleaseVersion(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "VERSION_ID=") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "VERSION_ID="))
			return strings.Trim(value, "\"'")
		}
	}
	return ""
}

func (a *App) xbpsConfigDir() string {
	if path := a.getenv("RYOKU_XBPS_CONFIG_DIR"); path != "" {
		return path
	}
	return a.cfg.XBPSConfigDir
}

func (a *App) xbpsShippedConfig() string {
	if path := a.getenv("RYOKU_XBPS_SHIPPED_CONFIG"); path != "" {
		return path
	}
	return a.cfg.XBPSShippedConfig
}
func (a *App) dnfRepoPath() string {
	if path := a.getenv("RYOKU_DNF_REPO_CONFIG"); path != "" {
		return path
	}
	return dnfRepoConfig
}

func (a *App) RepoURL() (string, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return "", err
	}
	if manager == DNF {
		body, err := os.ReadFile(a.dnfRepoPath())
		if err != nil {
			return "", ErrRepoAbsent
		}
		if value := dnfRepoURL(string(body)); value != "" {
			return value, nil
		}
		return "", ErrRepoAbsent
	}
	if manager == Pacman {
		path := a.cfg.PacmanConf
		if override := a.getenv("RYOKU_PACMAN_CONF"); override != "" {
			path = override
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", ErrRepoAbsent
		}
		if value := pacmanRepoURL(string(body)); value != "" {
			return value, nil
		}
		return "", ErrRepoAbsent
	}
	path := filepath.Join(a.xbpsConfigDir(), xbpsRepoConfig)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = a.xbpsShippedConfig()
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", ErrRepoAbsent
	}
	if value := xbpsRepoURL(string(body)); value != "" {
		return value, nil
	}
	return "", ErrRepoAbsent
}

func pacmanRepoURL(body string) string {
	inRepo := false
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inRepo = line == "[ryoku]"
			continue
		}
		if inRepo && strings.HasPrefix(line, "Server") {
			if _, value, ok := strings.Cut(line, "="); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}
func dnfRepoURL(body string) string {
	inRepo := false
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inRepo = strings.EqualFold(line, "[ryoku]")
			continue
		}
		if !inRepo {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "baseurl") {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func xbpsRepoURL(body string) string {
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && strings.TrimSpace(key) == "repository" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (a *App) RepoChannel() (string, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return "", err
	}
	url, err := a.RepoURL()
	if err != nil {
		return "", err
	}
	channel := RepoChannelOf(manager, url)
	if channel == "" {
		return "", ErrRepoAbsent
	}
	return channel, nil
}

func (a *App) RepoSetChannel(channel string) error {
	manager, err := a.PackageManager()
	if err != nil {
		return err
	}
	url := RepoURLFor(manager, channel)
	if url == "" {
		return fmt.Errorf("unknown Ryoku package channel %q", channel)
	}
	if manager == XBPS {
		override := strings.TrimSpace(a.getenv("RYOKU_XBPS_REPO"))
		if override != "" {
			return a.RepoSetURL(override)
		}
		if channel == "stable" && strings.TrimSpace(os.Getenv("RYOKU_RELEASE_BASE")) == "" {
			path := filepath.Join(a.xbpsConfigDir(), xbpsRepoConfig)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
	}
	return a.RepoSetURL(url)
}

func (a *App) RepoSetURL(url string) error {
	manager, err := a.PackageManager()
	if err != nil {
		return err
	}
	if manager == DNF {
		path := a.dnfRepoPath()
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrRepoAbsent
			}
			return err
		}
		rewritten, ok := rewriteDNFRepoURL(string(body), strings.TrimSpace(url))
		if !ok {
			return ErrRepoAbsent
		}
		return replaceFile(path, rewritten, 0o644)
	}
	if manager == XBPS {
		path := filepath.Join(a.xbpsConfigDir(), xbpsRepoConfig)
		return replaceFile(path, "repository="+strings.TrimSpace(url)+"\n", 0o644)
	}
	path := a.cfg.PacmanConf
	if override := a.getenv("RYOKU_PACMAN_CONF"); override != "" {
		path = override
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	inRepo, changed := false, false
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			inRepo = line == "[ryoku]"
			continue
		}
		if inRepo && strings.HasPrefix(line, "Server") && strings.Contains(line, "=") {
			lines[index] = "Server = " + strings.TrimSpace(url)
			changed = true
		}
	}
	if !changed {
		return ErrRepoAbsent
	}
	if err := replaceFile(path, strings.Join(lines, "\n"), 0o644); err != nil {
		return err
	}
	for _, name := range []string{"ryoku.db", "ryoku.db.sig", "ryoku.files", "ryoku.files.sig"} {
		if err := os.Remove(filepath.Join(a.cfg.PacmanSyncDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
func rewriteDNFRepoURL(body, url string) (string, bool) {
	lines := strings.Split(body, "\n")
	inRepo, changed := false, false
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			inRepo = strings.EqualFold(line, "[ryoku]")
			continue
		}
		if !inRepo {
			continue
		}
		equal := strings.IndexByte(raw, '=')
		if equal < 0 || !strings.EqualFold(strings.TrimSpace(raw[:equal]), "baseurl") {
			continue
		}
		value := raw[equal+1:]
		space := value[:len(value)-len(strings.TrimLeft(value, " \t"))]
		lines[index] = raw[:equal+1] + space + url
		changed = true
	}
	return strings.Join(lines, "\n"), changed
}

func replaceFile(path, body string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ryoku-repo-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (a *App) RepoSync(force bool) error {
	manager, err := a.PackageManager()
	if err != nil {
		return err
	}
	name, args := "pacman", []string{"-Sy", "--noconfirm"}
	if manager == XBPS {
		name, args = "xbps-install", []string{"-S"}
	} else if manager == DNF {
		name, args = a.dnfCommand(), []string{"makecache", "-y", "--refresh", "--repo=" + ryokuRepoName}
	} else if force {
		args[0] = "-Syy"
	}
	result := a.run(name, args...)
	if result.Code != 0 {
		return fmt.Errorf("%s: exit %d", strings.Join(append([]string{name}, args...), " "), result.Code)
	}
	return nil
}

func (a *App) RepoPackages() ([]RepoPackage, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return nil, err
	}
	if manager == DNF {
		command := a.dnfCommand()
		args := []string{"repoquery", "-y"}
		if command == "dnf5" {
			args = append(args, "--available")
		}
		args = append(args, "--repo="+ryokuRepoName, "--qf", dnfRepoQueryFormat)
		result := a.query(command, args...)
		if result.Code != 0 {
			return nil, commandError(command, args, result)
		}
		packages := parseDNFRepoPackages(result.Output)
		if len(packages) == 0 {
			return nil, fmt.Errorf("the Ryoku DNF repository package list is empty")
		}
		return packages, nil
	}
	if manager == Pacman {
		result := a.query("pacman", "-Sl", ryokuRepoName)
		if result.Code != 0 {
			return nil, commandError("pacman", []string{"-Sl", ryokuRepoName}, result)
		}
		var packages []RepoPackage
		for _, line := range strings.Split(result.Output, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == ryokuRepoName {
				packages = append(packages, RepoPackage{Name: fields[1], Version: fields[2], Repository: ryokuRepoName})
			}
		}
		if len(packages) == 0 {
			return nil, fmt.Errorf("the [ryoku] repository package list is empty")
		}
		return packages, nil
	}
	url, err := a.RepoURL()
	if err != nil {
		return nil, err
	}
	result := a.query("xbps-query", "-i", "-M", "--repository", url, "-Rs", "")
	if result.Code != 0 {
		return nil, commandError("xbps-query", []string{"-i", "-M", "--repository", url, "-Rs", ""}, result)
	}
	packages := parseXBPSRepoPackages(result.Output, url)
	if len(packages) == 0 {
		return nil, fmt.Errorf("the Ryoku XBPS repository package list is empty")
	}
	return packages, nil
}

func parseXBPSRepoPackages(output, repository string) []RepoPackage {
	seen := map[string]bool{}
	var packages []RepoPackage
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		match := xbpsPackagePattern.FindStringSubmatch(fields[1])
		if len(match) != 3 || seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		packages = append(packages, RepoPackage{Name: match[1], Version: match[2], Repository: repository})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return packages
}
func parseDNFRepoPackages(output string) []RepoPackage {
	byName := map[string]RepoPackage{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			continue
		}
		candidate := RepoPackage{Name: fields[0], Version: normalizeRPMEVR(fields[1]), Repository: ryokuRepoName}
		current, exists := byName[candidate.Name]
		if !exists || compareRPMEVR(current.Version, candidate.Version) < 0 {
			byName[candidate.Name] = candidate
		}
	}
	packages := make([]RepoPackage, 0, len(byName))
	for _, pkg := range byName {
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return packages
}

func (a *App) RepoAvailableVersion(name string) (string, error) {
	packages, err := a.RepoPackages()
	if err != nil {
		return "", err
	}
	for _, pkg := range packages {
		if pkg.Name == name {
			return pkg.Version, nil
		}
	}
	return "", fmt.Errorf("%s is not served by the Ryoku repository", name)
}

func (a *App) RepoInstalledSet(allowDowngrade bool, excluded map[string]bool) ([]string, int, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return nil, 0, err
	}
	packages, err := a.RepoPackages()
	if err != nil {
		return nil, 0, err
	}
	var targets []string
	skipped := 0
	for _, pkg := range packages {
		if excluded[pkg.Name] {
			continue
		}
		installed, ok := a.installedVersion(manager, pkg.Name)
		if !ok {
			continue
		}
		compared := a.compareVersions(manager, installed, pkg.Version)
		if manager == DNF {
			if !allowDowngrade {
				if compared > 0 {
					skipped++
					continue
				}
			}
			targets = append(targets, pkg.Name+"-"+pkg.Version)
			continue
		}
		if !allowDowngrade && compared > 0 {
			skipped++
			continue
		}
		target := pkg.Name
		if manager == Pacman {
			target = ryokuRepoName + "/" + target
		}
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets, skipped, nil
}

func (a *App) PackageVersion(name string) (string, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return "", err
	}
	version, ok := a.installedVersion(manager, name)
	if !ok {
		return "", ErrRepoAbsent
	}
	return version, nil
}

func (a *App) installedVersion(manager PackageManager, name string) (string, bool) {
	command, args := "pacman", []string{"-Q", name}
	if manager == XBPS {
		command, args = "xbps-query", []string{"-p", "pkgver", name}
	} else if manager == DNF {
		command, args = "rpm", []string{"-q", "--qf", "%{EPOCHNUM}:%{VERSION}-%{RELEASE}\n", name}
	}
	result := a.query(command, args...)
	if result.Code != 0 {
		return "", false
	}
	value := strings.TrimSpace(result.Output)
	if manager == DNF {
		value = normalizeRPMEVR(value)
	}
	if manager == Pacman {
		value = strings.TrimSpace(strings.TrimPrefix(value, name+" "))
	} else if manager == XBPS {
		value = strings.TrimSpace(strings.TrimPrefix(value, name+"-"))
	}
	return value, value != ""
}

func (a *App) compareVersions(manager PackageManager, installed, available string) int {
	if manager == DNF {
		return compareRPMEVR(installed, available)
	}
	command := "vercmp"
	if manager == XBPS {
		command = "xbps-uhelper"
	}
	args := []string{installed, available}
	if manager == XBPS {
		args = append([]string{"cmpver"}, args...)
	}
	result := a.query(command, args...)
	if result.Code != 0 {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(result.Output))
	if err != nil {
		return 0
	}
	return value
}

func commandError(name string, args []string, result Result) error {
	detail := strings.TrimSpace(result.Output)
	if detail == "" {
		detail = fmt.Sprintf("exit %d", result.Code)
	}
	return fmt.Errorf("%s: %s", strings.Join(append([]string{name}, args...), " "), detail)
}
