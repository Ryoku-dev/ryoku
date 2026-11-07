// ryoku-shell = single control plane for the Ryoku desktop shell. as
// `ryoku-shell daemon` it supervises the Quickshell components, brings up the
// clipboard and wallpaper helpers, and owns one Unix socket. as
// `ryoku-shell <command>` it forwards that command to the daemon over the
// socket; Hyprland keybinds use this client form.
package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const sockName = "ryoku-shell.sock"

func sockPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, sockName)
}

func main() {
	// Before anything else: strip Quickshell's inherited crash-recovery handles
	// so every qs we spawn honours its -c config instead of relaunching ours.
	scrubQuickshellCrashEnv()
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if args[0] == "daemon" {
		if err := runDaemon(); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-shell:", err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "theme" && len(args) == 2 && args[1] == "catalog" {
		// The colour-scheme catalog is static (compiled in) and larger than the
		// daemon's 8192-byte socket reply, so print it here without a round trip;
		// `theme <name>` still forwards to the daemon to apply.
		fmt.Println(themeCatalogJSON())
		return
	}
	if args[0] == "matugen-preview" && len(args) == 2 {
		// Generate (never apply) the palette an image would produce, for the
		// ryowalls live preview. Runs standalone -- no daemon round trip -- so the
		// same generator serves apply and preview from one binary.
		if err := matugenPreview(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-shell:", err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "icons" {
		// The icon index (every name reachable through the theme chain) is far
		// larger than the daemon's socket reply and needs no running daemon, so
		// build and print it here, like `theme catalog`. `icons <name>...` prints
		// one resolved path per line for spot checks.
		os.Exit(runIcons(args[1:]))
	}
	if args[0] == "no-display-entries" {
		// Quickshell's application model intentionally omits NoDisplay entries,
		// but running helper windows still need their metadata and icons.
		os.Exit(runNoDisplayEntries())
	}
	if args[0] == "browser-host" {
		// WebExtension native-messaging host: the browser launches this with the
		// manifest path as argv, then talks over stdin/stdout. Standalone, no
		// daemon socket; it reads colors.json directly.
		os.Exit(runBrowserHost())
	}
	if args[0] == "folders" {
		// Login autostart runs this before the daemon is up, so it builds the
		// folder-icon theme in this process instead of forwarding.
		os.Exit(runFolders(args[1:]))
	}
	if args[0] == "ipc" {
		os.Exit(runIPC(args[1:]))
	}
	if err := sendCommand(strings.Join(args, " ")); err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-shell:", err)
		os.Exit(1)
	}
}

// roundTrip sends one command line to the daemon and returns its raw reply.
func roundTrip(line string) (string, error) {
	conn, err := net.DialTimeout("unix", sockPath(), 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("%w at %s (is `ryoku-shell daemon` running?)", errDaemonUnreachable, sockPath())
	}
	defer conn.Close()
	// Generous: a reload waits for the surface to come back and report, and a
	// client that times out first turns a real failure into a silent success.
	_ = conn.SetDeadline(time.Now().Add(45 * time.Second))
	if _, err := fmt.Fprintln(conn, line); err != nil {
		return "", err
	}
	// The daemon writes one reply and closes, so read to EOF rather than a
	// fixed buffer.
	data, readErr := io.ReadAll(conn)
	if len(data) == 0 && readErr != nil {
		return "", fmt.Errorf("no reply from the daemon: %v", readErr)
	}
	return string(data), nil
}

var errDaemonUnreachable = errors.New("daemon not reachable")

// sendCommand: forward one command line to the daemon and print any reply that
// isn't a bare "ok".
func sendCommand(line string) error {
	data, err := roundTrip(line)
	if errors.Is(err, errDaemonUnreachable) {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "powerprofiles" {
			return directPowerProfilesCLI(fields[1:])
		}
	}
	if err != nil {
		return err
	}
	resp := strings.TrimSpace(data)
	if strings.HasPrefix(resp, "err ") {
		return fmt.Errorf("%s", strings.TrimPrefix(resp, "err "))
	}
	if resp != "" && resp != "ok" {
		fmt.Println(resp)
	}
	return nil
}

// runIPC is `ryoku-shell ipc call|get`: the daemon runs it against the running
// shell and the value lands on stdout. Every failure exits 1 with `err ...`.
func runIPC(words []string) int {
	if !(len(words) >= 3 && words[0] == "call") && !(len(words) == 3 && words[0] == "get") {
		fmt.Fprintln(os.Stderr, "usage: ryoku-shell ipc call <target> <function> [args...]")
		fmt.Fprintln(os.Stderr, "       ryoku-shell ipc get <target> <property>")
		return 2
	}
	data, err := roundTrip(encodeIPCRequest(words))
	if err != nil {
		fmt.Fprintln(os.Stderr, "err ipc:", err)
		return 1
	}
	resp := strings.TrimRight(data, "\n")
	value, ok := strings.CutPrefix(resp, "ok")
	if !ok || (value != "" && value[0] != '\n') {
		if !strings.HasPrefix(resp, "err ") {
			resp = "err ipc: unexpected reply: " + resp
		}
		fmt.Fprintln(os.Stderr, resp)
		return 1
	}
	if value = strings.TrimPrefix(value, "\n"); value != "" {
		fmt.Println(value)
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  ryoku-shell daemon")
	fmt.Fprintln(os.Stderr, "  ryoku-shell <launcher|clipboard|settings|cheatsheet|tools>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell overview")
	fmt.Fprintln(os.Stderr, "  ryoku-shell plugin <id>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell <visualizer|visualizer-overlay>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell lock")
	fmt.Fprintln(os.Stderr, "  ryoku-shell suspend")
	fmt.Fprintln(os.Stderr, "  ryoku-shell theme [<scheme>|catalog]")
	fmt.Fprintln(os.Stderr, "  ryoku-shell gtk apply <light|dark>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell voice")
	fmt.Fprintln(os.Stderr, "  ryoku-shell dock <show|hide|edge|autohide|pin|unpin|list>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell barstyle <id>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell shells <sync|commit>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell powerprofiles <state|set PROFILE>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell ipc call <target> <function> [args...]")
	fmt.Fprintln(os.Stderr, "  ryoku-shell ipc get <target> <property>")
	fmt.Fprintln(os.Stderr, "  ryoku-shell folders [accent]")
	fmt.Fprintln(os.Stderr, "  ryoku-shell <reload|status|ping|quit>")
}
