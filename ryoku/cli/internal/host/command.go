package host

import "fmt"

const Usage = `Usage: ryoku-host <verb> [arguments]
       ryoku-host --help

Host detection:
  init
  pkgmgr
  snapshots
  capabilities
  repo channel
  repo set-channel <stable|testing|TAG>
  repo sync

Services:
  svc [--user|--system] <start|stop|restart|try-restart|reload|is-active|is-enabled|enable|disable|kill> [--now] <name>...
  svc [--user|--system] reset-failed <name>...
  svc [--user|--system] daemon-reload
  svc env [--all | NAME | NAME=VALUE ...]

Process helpers:
  inhibit <systemd-inhibit arguments...>
  transient start <name> [--scope] [--slice S] [--prop KEY=VALUE]... [--env KEY=VALUE]... -- <command> [args...]
  transient stop <name>
  transient is-active <name>
  session start [extra-portal-unit...]
  session ensure --system
  session ensure --user
  session check
  session fix-wrappers [--backup-dir DIR]

Packages (names use the Arch/Ryoku catalogue):
  pkg installed <name>...
  pkg available [--aur] <name>...
  pkg install [--upgrade] [--aur] [--overwrite GLOB]... <name>...
  pkg install-file <path>...
  pkg remove <name>...
  pkg explicit <name>...
  pkg version <name>
  pkg owner <path>
  pkg count [--foreign]
  pkg local <name>...
  pkg why <name>
  pkg advice <name>...

System settings:
  power <poweroff|reboot|suspend|hibernate>
  time zone
  time zones
  time set-zone <Area/City>
  keymap set <layout> [variant] [options]

Exit codes: 0 success/true, 1 failure, 2 usage, 3 false, 4 absent, 5 not provided.
`

func (a *App) Execute(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.cfg.Stderr, Usage)
		return ExitUsage
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(a.cfg.Stdout, Usage)
		return ExitOK
	}
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return ExitUsage
		}
		value, err := a.Init()
		if err != nil {
			return a.failf("%v", err)
		}
		fmt.Fprintln(a.cfg.Stdout, value)
		return ExitOK
	case "pkgmgr":
		if len(args) != 1 {
			return ExitUsage
		}
		value, err := a.PackageManager()
		if err != nil {
			return a.failf("%v", err)
		}
		fmt.Fprintln(a.cfg.Stdout, value)
		return ExitOK
	case "snapshots":
		if len(args) != 1 {
			return ExitUsage
		}
		supported, reason := a.Snapshots()
		if supported {
			return ExitOK
		}
		fmt.Fprintln(a.cfg.Stdout, reason)
		return ExitNotProvided
	case "capabilities":
		if len(args) != 1 {
			return ExitUsage
		}
		return a.Capabilities()
	case "repo":
		return a.Repo(args[1:])
	case "svc":
		return a.Service(args[1:])
	case "inhibit":
		return a.Inhibit(args[1:])
	case "transient":
		return a.Transient(args[1:])
	case "session":
		if len(args) >= 2 {
			switch args[1] {
			case "ensure":
				if len(args) != 3 {
					return ExitUsage
				}
				return a.SessionEnsure(args[2])
			case "check":
				if len(args) != 2 {
					return ExitUsage
				}
				return a.SessionCheck()
			case "fix-wrappers":
				return a.SessionFixWrappers(args[2:])
			}
		}
		return a.Session(args[1:])
	case "pkg":
		return a.Package(args[1:])
	case "power":
		return a.Power(args[1:])
	case "time":
		return a.Time(args[1:])
	case "keymap":
		return a.Keymap(args[1:])
	default:
		fmt.Fprint(a.cfg.Stderr, Usage)
		return ExitUsage
	}
}
