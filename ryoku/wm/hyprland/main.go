package main

import (
	"bufio"
	"fmt"
	"os"
)

// The Hyprland half of the window-manager seam, and the only program allowed to
// run hyprctl, read HYPRLAND_INSTANCE_SIGNATURE or write Hyprland's config.
//
// Shipped by ryoku-desktop-hyprland, so a niri box simply lacks the binary and
// wm.Client reports no provider.
//
//	caps            capability manifest
//	act <id> [args] one neutral action
//	watch           state frames, newline-delimited JSON
//	apply <store>   write the config from the neutral store, report what it
//	                could not honour
//	plugins <verb>  the plugin subsystem

// Buffered because watch writes one object per event; a resync burst would
// otherwise syscall per frame.
var stdout = bufio.NewWriter(os.Stdout)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	defer stdout.Flush()

	var err error
	switch os.Args[1] {
	case "caps":
		err = runCaps()
	case "act":
		err = runAct(os.Args[2:])
	case "watch":
		err = runWatch(os.Args[2:])
	case "apply":
		if len(os.Args) < 3 {
			err = fmt.Errorf("apply: missing store path")
			break
		}
		err = runApply(os.Args[2:])
	case "outputs":
		err = runOutputs(os.Args[2:])
	case "state":
		err = runState()
	case "defaults":
		err = runDefaults()
	case "session":
		err = runSession()
	case "schema":
		err = runSchema()
	case "binds":
		err = runBinds(os.Args[2:])
	case "environment":
		err = runEnvironment(os.Args[2:])
	case "plugins":
		// Separate from act because plugins are a subsystem with list and
		// rebuild semantics; folding them in would make act a passthrough.
		err = runPlugins(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown verb %q", os.Args[1])
	}

	// Flush before reporting, so a verb that printed a partial result and then
	// failed still shows what it managed.
	_ = stdout.Flush()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryoku-wm-hyprland: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ryoku-wm-hyprland: the Hyprland window-manager provider

  caps             capability manifest (JSON)
  act <id> [args]  perform a neutral action
  state            one state snapshot (JSON)
  session          the wayland-session desktop entry
  schema           exclusive settings rows the Hub renders (JSON)
  binds [store]    compositor-exclusive keybinds the cheatsheet lists (JSON)
  watch            stream state frames (newline-delimited JSON)
  apply <store>    write the compositor config from the neutral store
  outputs <file>   apply an output layout from the neutral display store
  environment <pid> export this provider's session handle (NUL-delimited)
  plugins <verb>   list | rebuild [--stale]

Consumers should go through wm.Client rather than exec this directly.
`)
}
