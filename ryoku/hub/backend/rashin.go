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
	rashinPackage = "ryoku-rashin"
	rashinCommand = "rashin"
)

type rashinStatus struct {
	Installed bool `json:"installed"`
}

var (
	lookPathRashin = exec.LookPath
	runRashinCLI   = func(args ...string) ([]byte, error) {
		return exec.Command(rashinCommand, args...).CombinedOutput()
	}
	installRashinPackage = func() error {
		return privileged("pkg", "install", rashinPackage)
	}
	removeRashinPackage = func() error {
		return privileged("pkg", "remove", rashinPackage)
	}
	applyRashinDesktop = applyDesktop
)

func runRashin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("rashin needs status|on|off")
	}
	switch args[0] {
	case "status":
		b, err := json.Marshal(rashinStatus{Installed: rashinInstalled()})
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case "on":
		return rashinOn()
	case "off":
		return rashinOff()
	}
	return fmt.Errorf("unknown rashin command: %s", args[0])
}

func rashinInstalled() bool {
	_, err := lookPathRashin(rashinCommand)
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

// rashinOn installs Rashin when absent, records it and turns it on. A failed
// step undoes the earlier ones so the switch never lands half on, and the error
// names whatever could not be undone.
func rashinOn() error {
	installedHere, wasOn := false, false
	if rashinInstalled() {
		wasOn = rashinEnabled()
	} else {
		if err := installRashinPackage(); err != nil {
			return err
		}
		installedHere = true
	}
	ledger, err := snapshotFiles(provisionedPath())
	if err != nil {
		return rashinOnRollback(err, installedHere, false, false, nil)
	}
	if !rashinInstalled() {
		return rashinOnRollback(fmt.Errorf("%s was installed, but the rashin command is unavailable", rashinPackage), installedHere, false, false, ledger)
	}
	if err := recordProvisioned(rashinPackage); err != nil {
		return rashinOnRollback(err, installedHere, false, false, ledger)
	}
	if out, err := runRashinCLI("on"); err != nil {
		return rashinOnRollback(fmt.Errorf("rashin could not turn on: %s", commandError(out, err)), installedHere, !wasOn, false, ledger)
	}
	if err := applyRashinDesktop(); err != nil {
		return rashinOnRollback(err, installedHere, !wasOn, true, ledger)
	}
	return nil
}

// rashinOnRollback undoes a failed rashinOn in reverse: `rashin off` when this
// run turned it on (off also clears a half-finished on), the ledger, the package
// this run installed, then the desktop that may already have seen it.
func rashinOnRollback(cause error, installedHere, turnedOn, desktopTried bool, ledger fileSnapshot) error {
	var left []string
	if turnedOn {
		if out, err := runRashinCLI("off"); err != nil {
			left = append(left, "Rashin is still on ("+commandError(out, err)+")")
		}
	}
	if err := ledger.restore(); err != nil {
		left = append(left, fmt.Sprintf("%s stays listed in the provisioned ledger (%v)", rashinPackage, err))
	}
	if installedHere {
		if err := removeRashinPackage(); err != nil {
			left = append(left, fmt.Sprintf("%s is still installed (%v)", rashinPackage, err))
		}
	}
	if desktopTried {
		if err := applyRashinDesktop(); err != nil {
			left = append(left, fmt.Sprintf("the desktop may still show Rashin (%v)", err))
		}
	}
	if len(left) == 0 {
		return fmt.Errorf("%w; Rashin was left as it was", cause)
	}
	return fmt.Errorf("%w; left behind: %s", cause, strings.Join(left, "; "))
}

// rashinOff stops Rashin and removes the package. When the removal is refused
// Rashin is turned back on if it was on, so a dismissed prompt changes nothing.
func rashinOff() error {
	if rashinInstalled() {
		wasOn := rashinEnabled()
		out, stopErr := runRashinCLI("off")
		if err := removeRashinPackage(); err != nil {
			switch {
			case stopErr != nil:
				return fmt.Errorf("%w; %s is still installed and may be partly stopped (%s)", err, rashinPackage, commandError(out, stopErr))
			case !wasOn:
				return fmt.Errorf("%w; %s is still installed and off, as before", err, rashinPackage)
			}
			if out, onErr := runRashinCLI("on"); onErr != nil {
				return fmt.Errorf("%w; %s is still installed but now off: it could not be turned back on (%s)", err, rashinPackage, commandError(out, onErr))
			}
			return fmt.Errorf("%w; %s is still installed and on, as before", err, rashinPackage)
		}
		if err := applyRashinDesktop(); err != nil {
			return fmt.Errorf("%s was removed, but the desktop could not be updated and may still show Rashin: %w", rashinPackage, err)
		}
		if stopErr != nil {
			return fmt.Errorf("%s was removed, but rashin could not stop cleanly: %s", rashinPackage, commandError(out, stopErr))
		}
		return nil
	}
	return applyRashinDesktop()
}

// rashinEnabled reads Rashin's own switch. An unreadable one counts as off, so
// a rollback never turns on what it cannot show was on.
func rashinEnabled() bool {
	out, err := runRashinCLI("status", "--json")
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
