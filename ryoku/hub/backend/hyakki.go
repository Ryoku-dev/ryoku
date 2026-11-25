package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	hyakkiPackage = "hyakki"
	hyakkiCommand = "hyakki"
)

type hyakkiStatus struct {
	Installed bool `json:"installed"`
}

var (
	lookPathHyakki = exec.LookPath
	runHyakkiCLI   = func(args ...string) ([]byte, error) {
		return exec.Command(hyakkiCommand, args...).CombinedOutput()
	}
	runHyakkiHost = func(args ...string) ([]byte, error) {
		return exec.Command("ryoku-host", args...).CombinedOutput()
	}
	installHyakkiPackage = func() error {
		return privileged("pkg", "install", hyakkiPackage)
	}
	removeHyakkiPackage = func() error {
		return privileged("pkg", "remove", hyakkiPackage)
	}
	applyHyakkiDesktop = applyDesktop
)

func runHyakki(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hyakki needs status|on|off")
	}
	switch args[0] {
	case "status":
		b, err := json.Marshal(hyakkiStatus{Installed: hyakkiInstalled()})
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case "on":
		return hyakkiOn()
	case "off":
		return hyakkiOff()
	}
	return fmt.Errorf("unknown hyakki command: %s", args[0])
}

func hyakkiInstalled() bool {
	_, err := lookPathHyakki(hyakkiCommand)
	return err == nil
}

func privileged(args ...string) error {
	cmd := exec.Command("pkexec", append([]string{"ryoku-host"}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
			return fmt.Errorf("the password prompt was dismissed")
		}
		return fmt.Errorf("ryoku-host %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// hyakkiOn installs Hyakki when absent, records it and turns it on. A failed
// step undoes the earlier ones so the switch never lands half on, and the error
// names whatever could not be undone.
func hyakkiOn() error {
	installedHere := false
	if !hyakkiInstalled() {
		if err := installHyakkiPackage(); err != nil {
			return err
		}
		installedHere = true
	}
	ledger, err := snapshotFiles(provisionedPath())
	if err != nil {
		return hyakkiOnRollback(err, installedHere, false, false, nil)
	}
	if !hyakkiInstalled() {
		return hyakkiOnRollback(fmt.Errorf("%s was installed, but the hyakki command is unavailable", hyakkiPackage), installedHere, false, false, ledger)
	}
	if out, err := runHyakkiHost("hyakki", "migrate"); err != nil {
		return hyakkiOnRollback(fmt.Errorf("hyakki migration failed: %s", commandError(out, err)), installedHere, false, false, ledger)
	}
	wasOn := hyakkiEnabled()
	if err := recordProvisioned(hyakkiPackage); err != nil {
		return hyakkiOnRollback(err, installedHere, false, false, ledger)
	}
	if out, err := switchHyakki("on"); err != nil {
		return hyakkiOnRollback(fmt.Errorf("hyakki could not turn on: %s", commandError(out, err)), installedHere, !wasOn, false, ledger)
	}
	if err := applyHyakkiDesktop(); err != nil {
		return hyakkiOnRollback(err, installedHere, !wasOn, true, ledger)
	}
	return nil
}

// hyakkiOnRollback undoes a failed hyakkiOn in reverse: `hyakki off` when this
// run turned it on (off also clears a half-finished on), the ledger, the package
// this run installed, then the desktop that may already have seen it.
func hyakkiOnRollback(cause error, installedHere, turnedOn, desktopTried bool, ledger fileSnapshot) error {
	var left []string
	if turnedOn {
		if out, err := switchHyakki("off"); err != nil {
			left = append(left, "Hyakki did not stop cleanly ("+commandError(out, err)+")")
		}
	}
	if err := ledger.restore(); err != nil {
		left = append(left, fmt.Sprintf("%s stays listed in the provisioned ledger (%v)", hyakkiPackage, err))
	}
	if installedHere {
		if err := removeHyakkiPackage(); err != nil {
			left = append(left, fmt.Sprintf("%s is still installed (%v)", hyakkiPackage, err))
		}
	}
	if desktopTried {
		if err := applyHyakkiDesktop(); err != nil {
			left = append(left, fmt.Sprintf("the desktop may still show Hyakki (%v)", err))
		}
	}
	if len(left) == 0 {
		return fmt.Errorf("%w; Hyakki was left as it was", cause)
	}
	return fmt.Errorf("%w; left behind: %s", cause, strings.Join(left, "; "))
}

// hyakkiOff stops Hyakki and removes the package. When the removal is refused
// Hyakki is turned back on if it was on, so a dismissed prompt changes nothing.
func hyakkiOff() error {
	if hyakkiInstalled() {
		if out, err := runHyakkiHost("hyakki", "migrate"); err != nil {
			return fmt.Errorf("hyakki migration failed: %s", commandError(out, err))
		}
		wasOn := hyakkiEnabled()
		out, stopErr := switchHyakki("off")
		if err := removeHyakkiPackage(); err != nil {
			switch {
			case stopErr != nil:
				return fmt.Errorf("%w; %s is still installed and may be partly stopped (%s)", err, hyakkiPackage, commandError(out, stopErr))
			case !wasOn:
				return fmt.Errorf("%w; %s is still installed and off, as before", err, hyakkiPackage)
			}
			if out, onErr := switchHyakki("on"); onErr != nil {
				return fmt.Errorf("%w; %s is still installed but now off: it could not be turned back on (%s)", err, hyakkiPackage, commandError(out, onErr))
			}
			return fmt.Errorf("%w; %s is still installed and on, as before", err, hyakkiPackage)
		}
		if err := applyHyakkiDesktop(); err != nil {
			return fmt.Errorf("%s was removed, but the desktop could not be updated and may still show Hyakki: %w", hyakkiPackage, err)
		}
		if stopErr != nil {
			return fmt.Errorf("%s was removed, but hyakki could not stop cleanly: %s", hyakkiPackage, commandError(out, stopErr))
		}
		return nil
	}
	return applyHyakkiDesktop()
}

// The native CLI manages systemd itself; Ryoku owns runit's service directory.
func switchHyakki(verb string) ([]byte, error) {
	out, err := runHyakkiCLI(verb)
	if err != nil {
		return out, err
	}
	init, err := runHyakkiHost("init")
	if err != nil {
		return out, fmt.Errorf("identify Hyakki's supervisor: %s", commandError(init, err))
	}
	switch strings.TrimSpace(string(init)) {
	case "systemd":
		return out, nil
	case "runit":
		units, action := []string{"hyakki-usage"}, "enable"
		if verb == "off" {
			units, action = []string{"hyakki-usage", "oni-routines", "oni"}, "disable"
		}
		for _, unit := range units {
			reply, err := runHyakkiHost("svc", "--user", action, "--now", unit)
			if err != nil {
				return out, fmt.Errorf("%s %s: %s", action, unit, commandError(reply, err))
			}
		}
		return out, nil
	default:
		return out, fmt.Errorf("unsupported Hyakki supervisor: %s", strings.TrimSpace(string(init)))
	}
}

// hyakkiEnabled reads Hyakki's own switch. An unreadable one counts as off, so
// a rollback never turns on what it cannot show was on.
func hyakkiEnabled() bool {
	out, err := runHyakkiCLI("status", "--json")
	if err != nil {
		return false
	}
	var status struct {
		Enabled bool `json:"enabled"`
	}
	return json.Unmarshal(out, &status) == nil && status.Enabled
}

func commandError(out []byte, err error) string {
	message := strings.TrimSpace(string(out))
	if message != "" {
		return message
	}
	return err.Error()
}

func provisionedPath() string {
	return filepath.Join(stateHome(), "ryoku", "provisioned")
}

func recordProvisioned(pkg string) error {
	path := provisionedPath()
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == pkg {
				f.Close()
				return nil
			}
		}
		f.Close()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(pkg + "\n")
	return err
}
