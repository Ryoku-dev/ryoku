package host

import "fmt"

const Usage = `Usage: ryoku-host <verb> [arguments]
       ryoku-host --help

Host detection:
  init
  pkgmgr
  snapshots [create <description>]
  capabilities
  repo channel
  repo set-channel <stable|testing|TAG>
  repo sync

Services:
  svc reconcile <--system|--user> [--check]
  svc [--user|--system] <start|stop|restart|try-restart|reload|is-active|is-enabled|enable|disable|kill> [--now] <name>...
  svc [--user|--system] reset-failed <name>...
  svc [--user|--system] failed
  svc env [--all | NAME | NAME=VALUE ...]

Process helpers:
  app [--cwd DIR] [--label DESKTOP_ID] -- ARGV...
  inhibit <systemd-inhibit arguments...>
  transient start <name> [--scope] [--slice S] [--prop KEY=VALUE]... [--env KEY=VALUE]... -- <command> [args...]
  transient stop <name>
  transient is-active <name>
  session start [extra-portal-unit...]
  session ensure --system
  session ensure --user
  session check
  session fix-wrappers [--backup-dir DIR]

Authentication:
  auth fingerprint status
  auth fingerprint set <sudo|sddm|polkit-1> <on|off>

Network:
  network kill <status|on|off>
  network wifi-backend get
  network wifi-backend set <iwd|wpa_supplicant>
Phone:
  phone status
  phone enable|disable
  phone plan-enable|plan-disable

Users:
  users list
  users caps
  users add <name> --full-name <s> --role <admin|standard> [--perm id]... [--limit bytes] [--no-provision]
  users delete <name> <--keep-files|--erase-files>
  users set-role <name> <admin|standard>
  users set-perms <name> <id,id,...>
  users set-password <name>
  users set-limit <name> <bytes|0>
  users provision <name> [--rice <slug>] [--rashin] [--hide-app <id>]... [--skip-welcome]
  users seed-self [--rice <slug>] [--rashin] [--hide-app <id>]... [--skip-welcome]

Packages (names use the Arch/Ryoku catalogue):
  pkg [--overlay TABLE] <verb> ...   TABLE (or RYOKU_HOST_PKG_OVERLAY) maps names
                                     Ryoku's edition table does not list
  pkg installed <name>...
  pkg installed-at <name>
  pkg available [--aur] <name>...
  pkg status <name>...               name<TAB>installed|available|unavailable<TAB>repo|aur<TAB>reason
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
  power <capabilities|profile|idle|charge-limit|aspm> [arguments...]
  time zone
  time zones
  time set-zone <Area/City>
  keymap <set|apply> <layout> [variant] [options]
  initramfs rebuild

Exit codes: 0 success/true, 1 failure, 2 usage, 3 false, 4 absent, 5 not provided, 127 command not found.
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
		return a.Snapshot(args[1:])
	case "capabilities":
		if len(args) != 1 {
			return ExitUsage
		}
		return a.Capabilities()
	case "repo":
		return a.Repo(args[1:])
	case "svc":
		return a.Service(args[1:])
	case "app":
		return a.App(args[1:])
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
	case "initramfs":
		return a.Initramfs(args[1:])
	case "auth":
		return a.Auth(args[1:])
	case "network":
		return a.Network(args[1:])
	case "phone":
		return a.Phone(args[1:])
	case "users":
		return a.Users(args[1:])
	default:
		fmt.Fprint(a.cfg.Stderr, Usage)
		return ExitUsage
	}
}
