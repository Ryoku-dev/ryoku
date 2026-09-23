// ryoku-hub is the Go backend for the Ryoku Settings GUI. The Quickshell front end
// (qs -c hub) shells out to it the same way the rest of the desktop talks to
// ryoku-shell: a subcommand prints data on stdout or mutates persisted state.
//
//	ryoku-hub keybinds            print the keybind legend as JSON
//	ryoku-hub config get <key>    print a stored config value
//	ryoku-hub config set <k> <v>  persist a config value (TOML)
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "keybinds":
		b, err := json.Marshal(keybinds())
		if err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		fmt.Println()
	case "apps":
		if err := printApps(); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
		fmt.Println()
	case "config":
		if err := runConfig(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "shell":
		if err := runShellPref(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "desktop":
		if err := runDesktop(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "wm":
		if err := runWm(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "outputs":
		if err := runOutputs(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "lock":
		if err := runLock(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "gpu":
		if err := runGpu(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "cpu":
		if err := runCpu(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "lighting":
		if err := runLighting(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "voxtype":
		if err := runVoxtype(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "rice":
		if err := runRice(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "fastfetch":
		if err := runFastfetch(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "profile":
		if err := runProfile(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "matugen":
		if err := runMatugenCmd(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "reload-cover":
		if err := runReloadCover(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "import":
		if err := runImport(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "palette-bridge":
		if err := runPaletteBridge(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "share":
		if err := runShare(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	case "clipboard":
		if err := runClipboard(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-hub:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config needs get|set")
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("config get needs a key")
		}
		v, ok := configGet(args[1])
		if !ok {
			return fmt.Errorf("unknown config key: %s", args[1])
		}
		fmt.Println(v)
		return nil
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("config set needs a key and value")
		}
		return configSet(args[1], args[2])
	default:
		return fmt.Errorf("config needs get|set")
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  ryoku-hub keybinds")
	fmt.Fprintln(os.Stderr, "  ryoku-hub config get <key>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub config set <key> <value>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub shell get|set <fish|bash|zsh>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop get|defaults|cursors|layouts")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop variants <layout>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop save|preview <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop restore")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop set-rebind <default> <chosen>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop plugins list")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop plugins rebuild [--all|--stale] [--checkout <dir>] [<id>...]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop plugins add [--inspect] <git-url> [<plugin>...]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub desktop plugins remove <id>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub wm list")
	fmt.Fprintln(os.Stderr, "  ryoku-hub wm preview <name>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub outputs [list]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub outputs apply <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub outputs profiles")
	fmt.Fprintln(os.Stderr, "  ryoku-hub outputs save <name> <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub outputs load|rm <name>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lock list")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lock set <slug>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub gpu caps|mode")
	fmt.Fprintln(os.Stderr, "  ryoku-hub gpu tune caps|get")
	fmt.Fprintln(os.Stderr, "  ryoku-hub gpu tune set <gpu> <id> <value>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub gpu tune reset [<gpu>]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub cpu caps [<profile>]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub cpu active")
	fmt.Fprintln(os.Stderr, "  ryoku-hub cpu set <scope> <id> <value>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lighting state|scan|enable|disable|apply")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lighting set <device> <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lighting accent [#RRGGBB]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub lighting save|release <device>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub voxtype get|ensure")
	fmt.Fprintln(os.Stderr, "  ryoku-hub voxtype set <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub voxtype download|rmmodel <key>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub palette-bridge status|install|service|integration|doctor [<source>]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub share status")
	fmt.Fprintln(os.Stderr, "  ryoku-hub rice list|preflight|capture|apply|restore|save|fork|delete|import|publish|setwall|files|export")
	fmt.Fprintln(os.Stderr, "  ryoku-hub fastfetch get|preview <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub fastfetch save <json>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub fastfetch import-logo <path>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub reload-cover import <path>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub reload-cover prune [<managed-path>]")
	fmt.Fprintln(os.Stderr, "  ryoku-hub clipboard stats|prune")
	fmt.Fprintln(os.Stderr, "  ryoku-hub import scan <path|url>")
	fmt.Fprintln(os.Stderr, "  ryoku-hub import apply <decisions.json|->")
	fmt.Fprintln(os.Stderr, "  ryoku-hub import undo [<ts>]")
}
