package host

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

func (a *App) Package(args []string) int {
	if len(args) == 0 {
		return ExitUsage
	}
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	switch args[0] {
	case "advice":
		if len(args) < 2 {
			return ExitUsage
		}
		fmt.Fprintln(a.cfg.Stdout, a.InstallAdvice(args[1:]...))
		return ExitOK
	case "why":
		if len(args) != 2 {
			return ExitUsage
		}
		switch manager {
		case Pacman:
			return ExitFalse
		case DNF:
			return a.dnfPackageWhy(args[1])
		default:
			return a.xbpsPackageWhy(args[1])
		}
	}
	switch manager {
	case Pacman:
		return a.pacman(args)
	case DNF:
		return a.dnf(args)
	default:
		return a.xbps(args)
	}
}

func (a *App) xbpsPackageWhy(name string) int {
	table, err := a.xbpsTable()
	if err != nil {
		return a.failf("%v", err)
	}
	return a.packageWhy(table, name, "Not packaged for Void.")
}

func (a *App) packageWhy(table packageTable, name, fallback string) int {
	mapping, ok := table[name]
	if !ok || mapping.Special != "-" {
		return ExitFalse
	}
	note := mapping.Note
	if note == "" {
		note = fallback
	}
	fmt.Fprintln(a.cfg.Stdout, note)
	return ExitOK
}

// Orphans lists packages that were installed as dependencies and are no longer
// required by another installed package.
func (a *App) Orphans() ([]string, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return nil, err
	}
	if manager == DNF {
		return a.dnfOrphans()
	}
	name, args := "pacman", []string{"-Qdtq"}
	if manager == XBPS {
		name, args = "xbps-query", []string{"-O"}
	}
	result := a.query(name, args...)
	output := strings.TrimSpace(result.Output)
	if result.Code != 0 {
		if result.Code == 1 && output == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: exit %d: %s", strings.Join(append([]string{name}, args...), " "), result.Code, output)
	}
	if output == "" {
		return nil, nil
	}
	lines := strings.Split(output, "\n")
	orphans := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			orphans = append(orphans, line)
		}
	}
	return orphans, nil
}
func (a *App) PackageOwner(path string) (string, error) {
	manager, err := a.PackageManager()
	if err != nil {
		return "", err
	}
	name, args := "pacman", []string{"-Qoq", path}
	if manager == DNF {
		name, args = "rpm", []string{"-qf", "--qf", "%{NAME}\n", path}
	} else if manager == XBPS {
		name, args = "xbps-query", []string{"-o", path}
	}
	result := a.query(name, args...)
	if result.Code != 0 {
		return "", fmt.Errorf("%s: exit %d", strings.Join(append([]string{name}, args...), " "), result.Code)
	}
	value := strings.TrimSpace(result.Output)
	if manager == XBPS {
		if owner, _, ok := strings.Cut(value, ":"); ok {
			value = strings.TrimSpace(owner)
		}
		if match := xbpsPackagePattern.FindStringSubmatch(value); len(match) == 3 {
			value = match[1]
		}
	}
	return value, nil
}

func (a *App) pacman(args []string) int {
	verb, rest := args[0], args[1:]
	switch verb {
	case "installed", "available", "remove", "explicit":
		if len(rest) == 0 {
			return ExitUsage
		}
	case "version":
		if len(rest) != 1 {
			return ExitUsage
		}
	case "owner":
		if len(rest) != 1 {
			return ExitUsage
		}
	case "count":
		if len(rest) > 1 || len(rest) == 1 && rest[0] != "--foreign" {
			return ExitUsage
		}
	case "local":
		if len(rest) == 0 {
			return ExitUsage
		}
	case "install-file":
		if len(rest) == 0 {
			return ExitUsage
		}
	case "install":
	default:
		return ExitUsage
	}
	if verb == "available" && rest[0] == "--aur" {
		if len(rest) == 1 {
			return ExitUsage
		}
		return ExitOK
	}
	if verb == "local" {
		for _, name := range rest {
			fmt.Fprintln(a.cfg.Stdout, name)
		}
		return ExitOK
	}
	if verb == "install" {
		aur, upgrade := false, false
		var overwrite []string
		for len(rest) > 0 && strings.HasPrefix(rest[0], "--") {
			switch rest[0] {
			case "--aur":
				aur = true
				rest = rest[1:]
			case "--upgrade":
				upgrade = true
				rest = rest[1:]
			case "--overwrite":
				if len(rest) < 2 {
					return ExitUsage
				}
				overwrite = append(overwrite, "--overwrite", rest[1])
				rest = rest[2:]
			default:
				return ExitUsage
			}
		}
		if len(rest) == 0 {
			return ExitUsage
		}
		action := "-S"
		if upgrade {
			action = "-Syu"
		}
		installArgs := append([]string{action, "--needed", "--noconfirm"}, overwrite...)
		installArgs = append(installArgs, rest...)
		if aur {
			helper := ""
			if a.cfg.Runner.LookPath("yay") {
				helper = "yay"
			} else if a.cfg.Runner.LookPath("paru") {
				helper = "paru"
			}
			if helper == "" {
				return ExitAbsent
			}
			// makepkg refuses to run as root, and the doctor's manifest step
			// arrives here through sudo. Build as the invoking user, who owns
			// the helper's cache; the helper escalates just the pacman step.
			if a.cfg.UID == 0 {
				if run := a.dropForAUR(helper, installArgs); len(run) > 0 {
					return commandExit(a.run(run[0], run[1:]...))
				}
			}
			return commandExit(a.run(helper, installArgs...))
		}
		return commandExit(a.run("pacman", a.pacmanTransactionArgs(installArgs[0], installArgs[1:]...)...))
	}
	var command []string
	switch verb {
	case "installed":
		command = append([]string{"-Q"}, rest...)
	case "available":
		command = a.pacmanTransactionArgs("-Si", rest...)
	case "install-file":
		command = append([]string{"-U", "--noconfirm"}, rest...)
	case "remove":
		command = append([]string{"-R", "--noconfirm"}, rest...)
	case "explicit":
		command = append([]string{"-D", "--asexplicit", "--quiet"}, rest...)
	case "version":
		command = []string{"-Q", rest[0]}
	case "owner":
		command = []string{"-Qoq", rest[0]}
	case "count":
		flag := "-Qq"
		if len(rest) == 1 {
			flag = "-Qmq"
		}
		result := a.query("pacman", flag)
		if result.Code != 0 {
			return ExitFailure
		}
		fmt.Fprintln(a.cfg.Stdout, countLines(result.Output))
		return ExitOK
	}
	if verb == "installed" || verb == "available" || verb == "version" {
		var result Result
		if verb == "version" {
			result = a.query("pacman", command...)
		} else {
			result = a.probe("pacman", command...)
		}
		if result.Code != 0 {
			return ExitFalse
		}
		if verb == "version" {
			value := strings.TrimSpace(result.Output)
			value = strings.TrimPrefix(value, rest[0]+" ")
			fmt.Fprintln(a.cfg.Stdout, value)
		}
		return ExitOK
	}
	result := a.run("pacman", command...)
	if result.Code == 0 {
		return ExitOK
	}
	if verb == "owner" {
		return ExitFalse
	}
	return ExitFailure
}

func (a *App) pacmanTransactionArgs(action string, rest ...string) []string {
	args := []string{}
	if path := a.getenv("RYOKU_PACMAN_CONF"); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			args = append(args, "--config", path)
		}
	}
	args = append(args, action)
	return append(args, rest...)
}

// lookupUser resolves an account for its home directory; a var so tests can
// substitute an account without touching the host passwd.
var lookupUser = user.Lookup

// dropForAUR rebuilds the helper command to run as the invoking user when
// ryoku-host was called through sudo: makepkg refuses to run as root, so a
// root-side AUR build always fails. The helper escalates the final pacman
// transaction itself. Returns nil when no invoking user can be attributed.
func (a *App) dropForAUR(helper string, args []string) []string {
	name := a.getenv("SUDO_USER")
	if name == "" || name == "root" {
		return nil
	}
	u, err := lookupUser(name)
	if err != nil {
		return nil
	}
	run := []string{"runuser", "-u", name, "--", "env",
		"HOME=" + u.HomeDir, "USER=" + name, "LOGNAME=" + name, helper}
	return append(run, args...)
}

func (a *App) xbps(args []string) int {
	verb, rest := args[0], args[1:]
	table, err := a.xbpsTable()
	if err != nil {
		return a.failf("%v", err)
	}
	switch verb {
	case "installed", "available", "remove", "local", "explicit":
		if len(rest) == 0 {
			return ExitUsage
		}
	case "version", "owner":
		if len(rest) != 1 {
			return ExitUsage
		}
	case "count":
		if len(rest) > 1 || len(rest) == 1 && rest[0] != "--foreign" {
			return ExitUsage
		}
	case "install-file":
		if len(rest) == 0 {
			return ExitUsage
		}
		return ExitNotProvided
	case "install":
	default:
		return ExitUsage
	}
	if verb == "count" {
		if len(rest) == 1 {
			fmt.Fprintln(a.cfg.Stdout, 0)
			return ExitOK
		}
		result := a.query("xbps-query", "-l")
		if result.Code != 0 {
			return ExitFailure
		}
		fmt.Fprintln(a.cfg.Stdout, countLines(result.Output))
		return ExitOK
	}
	if verb == "owner" {
		result := a.run("xbps-query", "-o", rest[0])
		if result.Code == 0 {
			return ExitOK
		}
		return ExitFalse
	}
	if verb == "available" && rest[0] == "--aur" {
		rest = rest[1:]
		if len(rest) == 0 {
			return ExitUsage
		}
	}
	if verb == "local" {
		for _, arch := range rest {
			mapping := translatePackage(table, arch)
			fmt.Fprintln(a.cfg.Stdout, strings.Join(mapping.Names, " "))
		}
		return ExitOK
	}
	upgrade := false
	if verb == "install" {
		for len(rest) > 0 && strings.HasPrefix(rest[0], "--") {
			switch rest[0] {
			case "--aur":
				rest = rest[1:]
			case "--upgrade":
				upgrade = true
				rest = rest[1:]
			case "--overwrite":
				if len(rest) < 2 {
					return ExitUsage
				}
				rest = rest[2:]
			default:
				return ExitUsage
			}
		}
		if len(rest) == 0 {
			return ExitUsage
		}
	}
	translated, unavailable := translatePackages(table, rest)
	if len(unavailable) > 0 {
		if verb == "installed" || verb == "version" {
			return ExitFalse
		}
		if verb == "available" {
			return ExitNotProvided
		}
		fmt.Fprintf(a.cfg.Stderr, "ryoku-host: packages unavailable on this host: %s\n", strings.Join(unavailable, " "))
		return ExitNotProvided
	}
	switch verb {
	case "installed":
		for _, name := range translated {
			if a.probe("xbps-query", name).Code != 0 {
				return ExitFalse
			}
		}
		return ExitOK
	case "available":
		for _, name := range translated {
			if a.probe("xbps-query", "-R", name).Code != 0 {
				return ExitFalse
			}
		}
		return ExitOK
	case "version":
		result := a.query("xbps-query", "-p", "pkgver", translated[0])
		if result.Code != 0 {
			return ExitFalse
		}
		value := strings.TrimSpace(result.Output)
		value = strings.TrimPrefix(value, translated[0]+"-")
		fmt.Fprintln(a.cfg.Stdout, value)
		return ExitOK
	case "install":
		if upgrade {
			return commandExit(a.run("xbps-install", append([]string{"-Syu"}, translated...)...))
		}
		missing := make([]string, 0, len(translated))
		for _, name := range translated {
			if a.probe("xbps-query", name).Code != 0 {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			return ExitOK
		}
		return commandExit(a.run("xbps-install", append([]string{"-Sy"}, missing...)...))
	case "remove":
		installed := make([]string, 0, len(translated))
		for _, name := range translated {
			if a.probe("xbps-query", name).Code == 0 {
				installed = append(installed, name)
			}
		}
		if len(installed) == 0 {
			return ExitOK
		}
		return commandExit(a.run("xbps-remove", append([]string{"-y"}, installed...)...))
	case "explicit":
		return commandExit(a.run("xbps-pkgdb", append([]string{"-m", "manual"}, translated...)...))
	}
	return ExitUsage
}

func translatePackages(table packageTable, names []string) ([]string, []string) {
	var translated, unavailable []string
	seen := map[string]bool{}
	for _, arch := range names {
		mapping := translatePackage(table, arch)
		if mapping.Special != "" {
			unavailable = append(unavailable, arch)
			continue
		}
		for _, name := range mapping.Names {
			if !seen[name] {
				seen[name] = true
				translated = append(translated, name)
			}
		}
	}
	return translated, unavailable
}

func countLines(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return len(strings.Split(value, "\n"))
}
