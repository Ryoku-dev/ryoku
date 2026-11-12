// ryoku = the user-facing CLI for the distro. one front door to updates,
// rollback, and the shell. It is a thin dispatcher: each command is owned by
// an internal package, orchestrating pacman / yay / snapper / materialize
// rather than reimplementing them.
//
//	ryoku update            snapshot -> the Ryoku packages (or the channel) -> deploy -> reload
//	ryoku update --system   the same, plus the distribution's own upgrade (pacman -Syu, AUR, Flatpak)
//	ryoku update --wait     wait for a channel build or source check to finish
//	ryoku update --force    update despite an in-progress build or source check
//	ryoku update -v         the same, streaming the raw output instead of the curated console
//	ryoku update --gui      the same, in the background, driven from Ryoku Settings
//	ryoku rollback          list releases + snapshots; --to <tag> moves back to a release; [id] guides a snapshot restore
//	ryoku snapshots         list snapper snapshots
//	ryoku boot-menu         Fedora's GRUB snapshot menu: sync; restore (from a look boot); restored
//	ryoku status            version, commits behind the channel, snapshot count
//	ryoku materialize       lay the base configs into ~/.config (override-safe)
//	ryoku reset [path]      drop a user_edits override, back to the Ryoku default
//	ryoku vault            inspect, refresh, wire, or migrate the shared machine map
//	ryoku reload            restart the shell + reload Hyprland
//	ryoku deploy            DEV ONLY: build + materialize from a checkout
//	ryoku recovery          last resort: reset to main + redeploy (overwrites configs)
//	ryoku doctor            run convergent reconcilers (also runs inside update)
//	ryoku verify            report box-vs-release-package-set drift (read-only)
//	ryoku debug             print a shareable diagnostic bundle for bug reports
//
// The concerns live in their own folders: internal/updater (update, status,
// rollback, channel, run-state, materialize, version), internal/doctor (the
// convergent reconcilers), internal/vault (the shared machine map and harness
// pointers), and internal/sys (shared low-level primitives).
package main

import (
	"errors"
	"fmt"
	"os"

	"ryoku-cli/internal/doctor"
	"ryoku-cli/internal/importer"
	"ryoku-cli/internal/keyboard"
	"ryoku-cli/internal/keyring"
	"ryoku-cli/internal/profile"
	"ryoku-cli/internal/securitykey"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"
	"ryoku-cli/internal/vault"

	i18n "ryoku-i18n"
)

func main() {
	scrubQuickshellCrashEnv()
	i18n.Use("") // reads /usr/share/ryoku/i18n on an installed Ryoku
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "update":
		err = updater.Update(os.Args[2:])
	case "materialize":
		err = updater.MaterializeCommand()
	case "reset":
		err = updater.Reset(os.Args[2:])
	case "owner":
		err = updater.Owner(os.Args[2:])
	case "vault":
		err = vault.Run(os.Args[2:])
	case "rollback":
		err = updater.Rollback(os.Args[2:])
	case "boot-guard":
		err = updater.BootGuard(os.Args[2:])
	case "boot-menu":
		err = updater.BootMenu(os.Args[2:])
	case "snapshots":
		err = updater.Snapshots()
	case "status":
		err = updater.Status(os.Args[2:])
	case "channel":
		err = updater.Channel(os.Args[2:])
	case "profile":
		os.Exit(profile.UserMain(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	case "version", "--version", "-v":
		err = updater.Version(os.Args[2:])
	case "reload":
		err = sys.Run("ryoku-shell", "reload")
	case "wm":
		cmdWm(os.Args[2:])
	case "deploy":
		err = updater.Deploy(os.Args[2:])
	case "recovery":
		err = cmdRecovery(os.Args[2:])
	case "track":
		err = cmdTrack(os.Args[2:])
	case "plugin":
		err = cmdPlugin(os.Args[2:])
	case "doctor":
		err = doctor.Run(os.Args[2:])
	case "verify":
		err = doctor.Verify(os.Args[2:])
	case "debug":
		err = doctor.Debug(os.Args[2:])
	case "keyring":
		err = keyring.Run(os.Args[2:])
	case "security-key":
		err = securitykey.Run(os.Args[2:])
	case "keyboard":
		err = keyboard.Run(os.Args[2:])
	case "import":
		err = importer.Run(os.Args[2:])
	case "-h", "--help", "help", "":
		usage()
	default:
		die("unknown command: %s", os.Args[1])
	}
	if err != nil {
		// an update failure the console already showed in full
		var shown interface{ Reported() bool }
		if errors.As(err, &shown) && shown.Reported() {
			os.Exit(1)
		}
		die("%v", err)
	}
}

func usage() {
	fmt.Print(i18n.T("Usage: ryoku <command>\n\n  update         update the Ryoku packages (or channel commits), redeploy, reload\n  update --system  the same, plus your distribution's package upgrade\n  update --wait wait for a channel build or source check to finish\n  update --force  update despite an in-progress build or source check\n  update -v      the same, streaming the full raw output instead of the summary\n  update --gui   run the update in the background, driven from Ryoku Settings\n  track <chan>   packages: stable|unstable|v<tag>; add --source to build from a checkout\n  rollback       list releases and snapshots; --to <tag> moves back to a release; [id] guides a snapshot restore\n  status         show channel, installed/latest version, pending changes, snapshot count\n  channel        show the effective update channel; --json for machine-readable output\n  profile        status|scope|reset|check|first-run|session-stop|lay ...\n  version        <version> [<commit>]; --pretty -> <version> · <commit>, --branch -> channel\n  owner          <path>... [--json] | --map: classify config ownership\n  vault          status|refresh|wire|unwire|path|migrate|repo-index ...\n  materialize    (internal) compose immutable base + user_edits into ~/.config\n  doctor         converge drift (--check reports only; --explain shows every reconciler)\n  wm             list|status|caps|config|act ... (provider-backed window-manager seam)\n  host           internal host abstraction used by scripts and Ryoku Settings\n  bind            set|reset <action> <chord>; list [--json]\n  plugin          new|validate|add|remove|share ...\n  bar             add|remove|move|show|hide|list|settings|position|form ...\n  help           show this help\n"))
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ryoku: "+format+"\n", a...)
	os.Exit(1)
}
