package host

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	i18n "ryoku-i18n"
)

var (
	xbpsPackageVersion = regexp.MustCompile(`-[0-9][^-]*(?:_[0-9]+)?$`)
	xbpsKernelPackage  = regexp.MustCompile(`^linux[0-9]+\.[0-9]+$`)
)

func (a *App) Initramfs(args []string) int {
	if len(args) != 1 || args[0] != "rebuild" {
		return ExitUsage
	}
	if a.cfg.UID != 0 {
		return commandExit(a.run("pkexec", "ryoku-host", "initramfs", "rebuild"))
	}
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	switch manager {
	case Pacman:
		// the bare mkinitcpio can be limine-mkinitcpio-hook's /usr/local/bin
		// wrapper, which builds nothing on a UKI box and then asks [Y/n]
		if a.cfg.Runner.LookPath("limine-mkinitcpio") {
			return commandExit(a.run("limine-mkinitcpio"))
		}
		if !a.cfg.Runner.LookPath("mkinitcpio") {
			return a.initramfsMissing("mkinitcpio")
		}
		return commandExit(a.run("/usr/bin/mkinitcpio", "-P"))
	case DNF:
		if !a.cfg.Runner.LookPath("dracut") {
			return a.initramfsMissing("dracut")
		}
		return commandExit(a.run("dracut", "--regenerate-all", "--force"))
	case XBPS:
		if !a.cfg.Runner.LookPath("xbps-query") || !a.cfg.Runner.LookPath("xbps-reconfigure") {
			return a.initramfsMissing("xbps-reconfigure")
		}
		result := a.query("xbps-query", "-l")
		if result.Code != ExitOK {
			return commandExit(result)
		}
		kernels := xbpsKernelPackages(result.Output)
		if len(kernels) == 0 {
			fmt.Fprintln(a.cfg.Stderr, i18n.T("no installed Void kernel series was found for the initramfs rebuild"))
			return ExitAbsent
		}
		for _, kernel := range kernels {
			if result := a.run("xbps-reconfigure", "-f", kernel); result.Code != ExitOK {
				return commandExit(result)
			}
		}
		return ExitOK
	default:
		return ExitNotProvided
	}
}

func (a *App) initramfsMissing(tool string) int {
	fmt.Fprintf(a.cfg.Stderr, i18n.T("%s is not installed")+"\n", tool)
	return ExitAbsent
}

func xbpsKernelPackages(output string) []string {
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ii" {
			continue
		}
		name := xbpsPackageVersion.ReplaceAllString(fields[1], "")
		if xbpsKernelPackage.MatchString(name) {
			seen[name] = true
		}
	}
	packages := make([]string, 0, len(seen))
	for name := range seen {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	return packages
}
