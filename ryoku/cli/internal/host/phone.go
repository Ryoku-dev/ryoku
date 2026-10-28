package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var phoneBasePackages = []string{
	"kdeconnect",
	"scrcpy",
	"android-tools",
	"python-gobject",
	"python-dbus",
	"avahi",
	"python-pillow",
	"v4l2loopback-dkms",
}

const (
	phoneModulesLoad = "v4l2loopback\n"
	phoneModprobe    = "options v4l2loopback devices=1 video_nr=10 card_label=\"Ryoku Phone\" exclusive_caps=1\n"
)

func (a *App) phoneLegacyConfigPaths() []string {
	return []string{
		filepath.Join(filepath.Dir(a.cfg.PhoneModulesLoadPath), "ryoku-phone.conf"),
		filepath.Join(filepath.Dir(a.cfg.PhoneModprobePath), "ryoku-phone.conf"),
	}
}

func (a *App) removePhoneLegacyConfig() error {
	for _, path := range a.phoneLegacyConfigPaths() {
		if path == a.cfg.PhoneModulesLoadPath || path == a.cfg.PhoneModprobePath {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

type phoneStatus struct {
	Manager    PackageManager `json:"manager"`
	Installed  bool           `json:"installed"`
	Configured bool           `json:"configured"`
	Supported  bool           `json:"supported"`
	Reason     string         `json:"reason,omitempty"`
}

type phonePlan struct {
	Manager  PackageManager    `json:"manager"`
	Action   string            `json:"action"`
	Packages []string          `json:"packages"`
	Commands [][]string        `json:"commands"`
	Writes   map[string]string `json:"writes,omitempty"`
	Removes  []string          `json:"removes,omitempty"`
}

func (a *App) Phone(args []string) int {
	if len(args) != 1 {
		return ExitUsage
	}
	switch args[0] {
	case "status":
		return a.phoneStatus()
	case "enable":
		return a.phoneEnable()
	case "disable":
		return a.phoneDisable()
	case "plan-enable", "plan-disable":
		return a.phonePlan(strings.TrimPrefix(args[0], "plan-"))
	default:
		return ExitUsage
	}
}

func (a *App) phonePackages(manager PackageManager) []string {
	packages := append([]string{}, phoneBasePackages...)
	if manager == Pacman {
		modules, err := os.ReadDir(a.cfg.KernelModulesDir)
		if err == nil {
			for _, module := range modules {
				pkgbase, err := os.ReadFile(filepath.Join(a.cfg.KernelModulesDir, module.Name(), "pkgbase"))
				if err != nil {
					continue
				}
				kernel := strings.TrimSpace(string(pkgbase))
				if strings.HasPrefix(kernel, "linux") && !strings.ContainsAny(kernel, " \t\r\n/") {
					packages = append(packages, kernel+"-headers")
				}
			}
		}
	} else {
		packages = append(packages, "linux-headers")
	}
	seen := make(map[string]bool, len(packages))
	out := make([]string, 0, len(packages))
	for _, name := range packages {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func (a *App) phonePackageInstalled(manager PackageManager, name string) bool {
	native, err := a.phoneNativePackages(manager, []string{name})
	if err != nil || len(native) == 0 {
		return false
	}
	for _, packageName := range native {
		var result Result
		switch manager {
		case DNF:
			result = a.probe("rpm", "-q", "--whatprovides", "--quiet", packageName)
		case XBPS:
			result = a.probe("xbps-query", "-e", packageName)
		default:
			result = a.probe("pacman", "-Q", "--quiet", packageName)
		}
		if result.Code != 0 {
			return false
		}
	}
	return true
}

func (a *App) phoneScrcpyCompatible() (bool, string) {
	result := a.query("scrcpy", "--version")
	if result.Code != 0 {
		return false, ""
	}
	for _, field := range strings.Fields(result.Output) {
		field = strings.TrimLeft(field, "vV")
		var major, minor int
		if _, err := fmt.Sscanf(field, "%d.%d", &major, &minor); err == nil {
			return major > 2 || major == 2 && minor >= 2, field
		}
	}
	return false, strings.TrimSpace(result.Output)
}

func (a *App) phoneStatus() int {
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	packages := a.phonePackages(manager)
	installed := true
	for _, name := range packages {
		if !a.phonePackageInstalled(manager, name) {
			installed = false
			break
		}
		if name == "scrcpy" {
			if compatible, _ := a.phoneScrcpyCompatible(); !compatible {
				installed = false
				break
			}
		}
	}
	_, modulesErr := os.Stat(a.cfg.PhoneModulesLoadPath)
	_, modprobeErr := os.Stat(a.cfg.PhoneModprobePath)
	status := phoneStatus{
		Manager: manager, Installed: installed,
		Configured: modulesErr == nil && modprobeErr == nil,
		Supported:  true,
	}
	body, err := json.Marshal(status)
	if err != nil {
		return a.failf("encode phone status: %v", err)
	}
	fmt.Fprintln(a.cfg.Stdout, string(body))
	return ExitOK
}

func (a *App) phoneEnable() int {
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	packages := a.phonePackages(manager)
	missing := make([]string, 0, len(packages))
	installPackages := make([]string, 0, len(packages))
	for _, name := range packages {
		present := a.phonePackageInstalled(manager, name)
		needsInstall := !present
		if name == "scrcpy" && present {
			compatible, _ := a.phoneScrcpyCompatible()
			needsInstall = !compatible
		}
		if !needsInstall {
			continue
		}
		if !present {
			missing = append(missing, name)
		}
		installPackages = append(installPackages, name)
	}
	if manager == DNF {
		if err := a.writePhoneCoprFiles(); err != nil {
			return a.failf("configure phone repositories: %v", err)
		}
	}
	if err := a.writePhoneState(missing); err != nil {
		if manager == DNF {
			for _, path := range a.phoneCoprPaths() {
				_ = os.Remove(path)
			}
		}
		return a.failf("record phone packages: %v", err)
	}
	rollback := func() { _ = a.phoneDisable() }
	if len(installPackages) > 0 {
		if code := a.Package(append([]string{"install"}, installPackages...)); code != ExitOK {
			rollback()
			return code
		}
	}
	if compatible, version := a.phoneScrcpyCompatible(); !compatible {
		rollback()
		if version == "" {
			return a.failf("scrcpy 2.2 or newer is required")
		}
		return a.failf("scrcpy 2.2 or newer is required; found %s", version)
	}
	if err := a.removePhoneLegacyConfig(); err != nil {
		rollback()
		return a.failf("remove legacy phone module config: %v", err)
	}
	if err := writePhoneFile(a.cfg.PhoneModulesLoadPath, phoneModulesLoad); err != nil {
		rollback()
		return a.failf("write %s: %v", a.cfg.PhoneModulesLoadPath, err)
	}
	if err := writePhoneFile(a.cfg.PhoneModprobePath, phoneModprobe); err != nil {
		rollback()
		return a.failf("write %s: %v", a.cfg.PhoneModprobePath, err)
	}
	if result := a.run("modprobe", "v4l2loopback"); result.Code != 0 {
		rollback()
		return a.failf("load v4l2loopback: exit %d", result.Code)
	}
	return ExitOK
}

func (a *App) phoneDisable() int {
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	configPaths := []string{a.cfg.PhoneModulesLoadPath, a.cfg.PhoneModprobePath}
	configPaths = append(configPaths, a.phoneLegacyConfigPaths()...)
	for _, path := range configPaths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return a.failf("remove %s: %v", path, err)
		}
	}
	_ = a.run("modprobe", "-r", "v4l2loopback")
	packages, err := a.readPhoneState()
	if err != nil {
		return a.failf("read phone package record: %v", err)
	}
	if len(packages) > 0 {
		if code := a.Package(append([]string{"remove"}, packages...)); code != ExitOK {
			return code
		}
	}
	if err := os.Remove(a.cfg.PhoneStatePath); err != nil && !os.IsNotExist(err) {
		return a.failf("remove phone package record: %v", err)
	}
	if manager == DNF {
		for _, path := range a.phoneCoprPaths() {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return a.failf("remove %s: %v", path, err)
			}
		}
	}
	return ExitOK
}

func (a *App) phonePlan(action string) int {
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	packages := a.phonePackages(manager)
	if action == "disable" {
		if recorded, readErr := a.readPhoneState(); readErr == nil {
			packages = recorded
		}
	}
	native, err := a.phoneNativePackages(manager, packages)
	if err != nil {
		return a.failf("resolve phone package plan: %v", err)
	}
	plan := phonePlan{Manager: manager, Action: action, Packages: native}
	if action == "enable" {
		plan.Writes = map[string]string{a.cfg.PhoneModulesLoadPath: phoneModulesLoad, a.cfg.PhoneModprobePath: phoneModprobe}
		plan.Removes = a.phoneLegacyConfigPaths()
		if manager == DNF {
			for _, path := range a.phoneCoprPaths() {
				plan.Commands = append(plan.Commands, []string{"install", "-Dm644", "<generated>", path})
			}
		}
		if len(native) > 0 {
			plan.Commands = append(plan.Commands, phoneInstallCommand(manager, a.dnfCommand(), native))
		}
		plan.Commands = append(plan.Commands, []string{"modprobe", "v4l2loopback"})
	} else {
		plan.Removes = []string{a.cfg.PhoneModulesLoadPath, a.cfg.PhoneModprobePath, a.cfg.PhoneStatePath}
		plan.Removes = append(plan.Removes, a.phoneLegacyConfigPaths()...)
		if manager == DNF {
			plan.Removes = append(plan.Removes, a.phoneCoprPaths()...)
		}
		plan.Commands = append(plan.Commands, []string{"modprobe", "-r", "v4l2loopback"})
		if len(native) > 0 {
			plan.Commands = append(plan.Commands, phoneRemoveCommand(manager, a.dnfCommand(), native))
		}
	}
	body, err := json.Marshal(plan)
	if err != nil {
		return a.failf("encode phone plan: %v", err)
	}
	fmt.Fprintln(a.cfg.Stdout, string(body))
	return ExitOK
}

func (a *App) phoneNativePackages(manager PackageManager, packages []string) ([]string, error) {
	if manager == Pacman {
		return packages, nil
	}
	var table packageTable
	var err error
	if manager == DNF {
		table, err = a.fedoraTable()
	} else {
		table, err = a.xbpsTable()
	}
	if err != nil {
		return nil, err
	}
	native, unavailable := translatePackages(table, packages)
	if len(unavailable) > 0 {
		return nil, fmt.Errorf("unavailable: %s", strings.Join(unavailable, ", "))
	}
	return native, nil
}

func phoneInstallCommand(manager PackageManager, dnf string, packages []string) []string {
	switch manager {
	case DNF:
		return append([]string{dnf, "install", "-y", "--setopt=install_weak_deps=False"}, packages...)
	case XBPS:
		return append([]string{"xbps-install", "-Sy"}, packages...)
	default:
		return append([]string{"pacman", "-S", "--needed", "--noconfirm"}, packages...)
	}
}

func phoneRemoveCommand(manager PackageManager, dnf string, packages []string) []string {
	switch manager {
	case DNF:
		return append([]string{dnf, "remove", "-y", "--setopt=clean_requirements_on_remove=False"}, packages...)
	case XBPS:
		return append([]string{"xbps-remove", "-y"}, packages...)
	default:
		return append([]string{"pacman", "-R", "--noconfirm"}, packages...)
	}
}

func writePhoneFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ryoku-phone-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.WriteString(body); err == nil {
		err = tmp.Chmod(0o644)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (a *App) writePhoneState(packages []string) error {
	prior, err := a.readPhoneState()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range append(prior, packages...) {
		seen[name] = true
	}
	all := make([]string, 0, len(seen))
	for name := range seen {
		all = append(all, name)
	}
	sort.Strings(all)
	body := ""
	if len(all) > 0 {
		body = strings.Join(all, "\n") + "\n"
	}
	return writePhoneFile(a.cfg.PhoneStatePath, body)
}

func (a *App) readPhoneState() ([]string, error) {
	body, err := os.ReadFile(a.cfg.PhoneStatePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, name := range strings.Fields(string(body)) {
		if strings.ContainsAny(name, "/\\\x00") {
			return nil, fmt.Errorf("invalid package name %q", name)
		}
		packages = append(packages, name)
	}
	return packages, nil
}

func (a *App) phoneCoprPaths() []string {
	return []string{
		filepath.Join(a.cfg.PhoneCoprDir, "ryoku-copr-boobaa-scrcpy.repo"),
		filepath.Join(a.cfg.PhoneCoprDir, "ryoku-copr-kuya-carlo-v4l2loopback.repo"),
	}
}

func (a *App) writePhoneCoprFiles() error {
	repos := []struct{ owner, project, packages string }{
		{"boobaa", "scrcpy", "scrcpy"},
		{"kuya-carlo", "v4l2loopback", "v4l2loopback v4l2loopback-dkms"},
	}
	for _, repo := range repos {
		path := filepath.Join(a.cfg.PhoneCoprDir, "ryoku-copr-"+repo.owner+"-"+repo.project+".repo")
		body := fmt.Sprintf("[ryoku-copr-%s-%s]\nname=Ryoku dependency: %s/%s\nbaseurl=https://download.copr.fedorainfracloud.org/results/%s/%s/fedora-$releasever-$basearch/\nenabled=1\ngpgcheck=1\nrepo_gpgcheck=0\ngpgkey=https://download.copr.fedorainfracloud.org/results/%s/%s/pubkey.gpg\nincludepkgs=%s\nskip_if_unavailable=1\n", repo.owner, repo.project, repo.owner, repo.project, repo.owner, repo.project, repo.owner, repo.project, repo.packages)
		if err := writePhoneFile(path, body); err != nil {
			return err
		}
	}
	return nil
}
