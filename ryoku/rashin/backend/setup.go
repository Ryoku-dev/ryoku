package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// setup.go is the one-click actuator behind the Hub's "Set up Hermes agent"
// button. It runs interactively inside a floating terminal and streams its
// phase into $XDG_RUNTIME_DIR/ryoku-rashin/setup.json, which the Hub page
// watches. Two hard rules: an existing Hermes is never reinstalled or
// re-onboarded, and Hermes config is only ever touched through hermes's own
// interfaces.

const hermesInstallURL = "https://hermes-agent.nousresearch.com/install.sh"

type setupPhase struct {
	Phase  string `json:"phase"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

func reportPhase(phase, detail string, ok bool) {
	rt := RuntimeDir()
	if os.MkdirAll(rt, 0o700) != nil {
		return
	}
	b, err := json.Marshal(setupPhase{Phase: phase, Detail: detail, OK: ok})
	if err != nil {
		return
	}
	tmp := filepath.Join(rt, "setup.json.tmp")
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, filepath.Join(rt, "setup.json"))
	}
	if ok {
		fmt.Printf("== %s: %s\n", phase, detail)
	} else {
		fmt.Fprintf(os.Stderr, "!! %s: %s\n", phase, detail)
	}
}

func setupDryRun() bool { return os.Getenv("RYOKU_RASHIN_SETUP_DRYRUN") == "1" }

// runInteractive inherits the terminal so installers and onboarding can prompt.
func runInteractive(name string, args ...string) error {
	if setupDryRun() {
		fmt.Printf("dry-run: %s %v\n", name, args)
		return nil
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdSetup() error {
	if err := RunSetup(); err != nil {
		reportPhase("error", err.Error(), false)
		fmt.Fprintln(os.Stderr, "\nsetup failed; press enter to close")
		fmt.Scanln()
		return err
	}
	return nil
}

func RunSetup() error {
	// Preflight: the installer downloads and builds the agent, so it needs curl,
	// a Python toolchain (uv or python3), a reachable network, and ~2 GiB free in
	// $HOME. Checking all four here turns a mid-install failure (the cryptic
	// "uv lock missing" a broken or offline install throws) into one clear line.
	reportPhase("preflight", "checking tools, connectivity, and disk space", true)
	if !haveCmd("curl") {
		return errors.New("curl is required for the Hermes installer (sudo pacman -S curl)")
	}
	if !haveCmd("uv") && !haveCmd("python3") {
		return errors.New("Hermes needs uv or python3 to install (sudo pacman -S uv), then re-run setup")
	}
	if !setupOnline() {
		return errors.New("Hermes setup needs an internet connection: it downloads the agent and its Python dependencies from GitHub and PyPI")
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(home(), &st); err == nil {
		free := st.Bavail * uint64(st.Bsize)
		if free < 2<<30 {
			return fmt.Errorf("less than 2 GiB free in %s", home())
		}
	}

	// Install: skipped entirely when Hermes already exists (never clobber).
	if _, ok := FindHermes(); ok {
		reportPhase("install", "existing Hermes detected, leaving it untouched", true)
	} else {
		reportPhase("install", "running the official Hermes installer", true)
		if err := installHermes(); err != nil {
			return fmt.Errorf("hermes installer: %w", err)
		}
	}
	hermesBin, ok := FindHermes()
	if !ok && !setupDryRun() {
		return errors.New("hermes not found after install; open a new terminal and re-run setup")
	}

	// Onboard: only when hermes has no config yet, and through hermes itself.
	if HermesStatus().Configured {
		reportPhase("onboard", "Hermes already configured, keeping your provider and model", true)
	} else {
		reportPhase("onboard", "running hermes setup (pick your provider and model)", true)
		if hermesBin != "" {
			if err := runInteractive(hermesBin, "setup"); err != nil {
				return fmt.Errorf("hermes setup: %w", err)
			}
		}
	}

	// Wire: vault first, then the memory pointer and every detected agent.
	// Runs after onboarding so nothing hermes writes can clobber it.
	reportPhase("wire", "building the vault and wiring agents", true)
	if err := EnsureVault(); err != nil {
		return err
	}
	if err := Reindex(); err != nil {
		reportPhase("wire", "index incomplete: "+err.Error(), true)
	}
	if !setupDryRun() {
		if err := WireHermesMemory(); err != nil {
			return fmt.Errorf("wire hermes memory: %w", err)
		}
	}
	n := WireAll()
	reportPhase("wire", fmt.Sprintf("wired hermes and %d coding agents", n), true)

	// Enable: gate autostart on and bring the daemon up now, at boot via
	// lingering when the desktop allows it (cmdEnable degrades to login-start
	// if enable-linger is refused), so the dashboard is up without a login.
	reportPhase("enable", "starting the rashin daemon", true)
	if setupDryRun() {
		reportPhase("done", "dry run complete", true)
		return nil
	}
	if err := cmdEnable(true); err != nil {
		return err
	}
	// Verify the chat backend: a working hermes CLI does not guarantee the ACP
	// adapter's deps are present, and the dashboard, sidebar, and terminal chat
	// all drive `hermes acp`. Checking here surfaces a "settings say OK but the
	// chat will not start" gap at setup instead of as a dead chat later.
	if hermesBin != "" {
		reportPhase("verify", "checking the chat backend (hermes acp)", true)
		if err := exec.Command(hermesBin, "acp", "--check").Run(); err != nil {
			reportPhase("verify", "chat backend not ready; run 'hermes acp --check' for details", false)
		} else {
			reportPhase("verify", "chat backend ready", true)
		}
	}
	cfg := LoadConfig()
	url := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	reportPhase("done", "dashboard at "+url, true)
	fmt.Printf("\nRashin is ready: %s\npress enter to close\n", url)
	fmt.Scanln()
	return nil
}

// installHermes downloads the official installer to a temp file and runs it
// with only the options that build of the script actually advertises. Not
// `curl | bash`: piping leaves the installer's stdin on the pipe, so its
// optional-package steps block on a hidden /dev/tty y/n + sudo prompt.
func installHermes() error {
	f, err := os.CreateTemp("", "hermes-install-*.sh")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_ = f.Close()
	if err := runInteractive("curl", "-fsSL", hermesInstallURL, "-o", name); err != nil {
		return err
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return runInteractive("bash", append([]string{name}, hermesInstallerFlags(string(body))...)...)
}

// hermesInstallerFlags picks the installer options to pass. The official script
// has grown and dropped options across releases, and it exits 1 on any flag it
// does not know: a hardcoded --skip-computer-use broke every setup the day
// upstream removed it (#279). So --non-interactive (the one option Ryoku cannot
// live without, it is what keeps the installer off a hidden tty prompt) is
// always passed, and each optional flag only when the downloaded script names
// it. A flag the script no longer supports is simply not a thing to skip.
func hermesInstallerFlags(script string) []string {
	args := []string{"--non-interactive"}
	for _, opt := range []string{"--skip-browser", "--skip-computer-use"} {
		if strings.Contains(script, opt) {
			args = append(args, opt)
		}
	}
	return args
}

// haveCmd reports whether a command is resolvable on PATH.
func haveCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// setupOnline probes the hosts the Hermes installer must reach, so an offline
// box fails preflight with one clear message instead of a cryptic uv or git
// error partway through the download. Any one host answering counts as online.
func setupOnline() bool {
	if setupDryRun() {
		return true
	}
	for _, host := range []string{"hermes-agent.nousresearch.com:443", "github.com:443", "pypi.org:443"} {
		if c, err := net.DialTimeout("tcp", host, 4*time.Second); err == nil {
			_ = c.Close()
			return true
		}
	}
	return false
}
