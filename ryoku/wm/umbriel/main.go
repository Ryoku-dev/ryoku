package main

import (
	"bufio"
	"fmt"
	"os"
)

// EXPERIMENTAL Umbriel half of the window-manager seam. Speaks Umbriel 0.1.0's
// JSON-lines IPC and emits neutral Ryoku frames.
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
		err = runWatch()
	case "state":
		err = runState()
	case "apply":
		err = runApply(os.Args[2:])
	case "outputs":
		err = runOutputs(os.Args[2:])
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
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown verb %q", os.Args[1])
	}

	_ = stdout.Flush()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryoku-wm-umbriel: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ryoku-wm-umbriel: experimental Umbriel seam provider

  caps             capability manifest (JSON)
  act <id> [args]  perform a neutral action
  state            one state snapshot (JSON)
  watch            stream state frames (newline-delimited JSON)
  apply <store>    writes settings.toml and rebinds.toml from the neutral store
  outputs <file>   writes monitors.toml from the neutral output layout
  defaults         the store leaves this provider owns (JSON)
  session          the wayland-session desktop entry
  schema           this provider's exclusive settings rows (JSON)
  binds [store]    the shipped keybind legend (JSON)
  environment <pid> export UMBRIEL_SOCKET from a verified session process
`)
}
