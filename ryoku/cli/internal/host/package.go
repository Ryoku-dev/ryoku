package host

import (
	"fmt"
	"os"
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
	if manager == Pacman {
		return a.pacman(args)
	}
	return a.xbps(args)
}

func (a *App) pacman(args []string) int {
	verb, rest := args[0], args[1:]
	switch verb {
	case "installed", "available", "remove":
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
	if verb == "local" {
		for _, name := range rest {
			fmt.Fprintln(a.cfg.Stdout, name)
		}
		return ExitOK
	}
	if verb == "install" {
		aur, upgrade := false, false
		for len(rest) > 0 && strings.HasPrefix(rest[0], "--") {
			switch rest[0] {
			case "--aur":
				aur = true
			case "--upgrade":
				upgrade = true
			default:
				return ExitUsage
			}
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return ExitUsage
		}
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
			action := "-S"
			if upgrade {
				action = "-Syu"
			}
			return commandExit(a.run(helper, append([]string{action, "--needed", "--noconfirm"}, rest...)...))
		}
		action := "-S"
		if upgrade {
			action = "-Syu"
		}
		return commandExit(a.run("pacman", a.pacmanTransactionArgs(action, append([]string{"--needed", "--noconfirm"}, rest...)...)...))
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

func (a *App) xbps(args []string) int {
	verb, rest := args[0], args[1:]
	table, err := a.xbpsTable()
	if err != nil {
		return a.failf("%v", err)
	}
	switch verb {
	case "installed", "available", "remove", "local":
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
			case "--upgrade":
				upgrade = true
			default:
				return ExitUsage
			}
			rest = rest[1:]
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
