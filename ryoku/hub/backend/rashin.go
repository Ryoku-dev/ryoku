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

func rashinOn() error {
	if !rashinInstalled() {
		if err := installRashinPackage(); err != nil {
			return err
		}
		if !rashinInstalled() {
			return fmt.Errorf("%s was installed, but the rashin command is unavailable", rashinPackage)
		}
	}
	if err := recordProvisioned(rashinPackage); err != nil {
		return err
	}
	if out, err := runRashinCLI("on"); err != nil {
		return fmt.Errorf("rashin could not turn on: %s", commandError(out, err))
	}
	return applyRashinDesktop()
}

func rashinOff() error {
	var stopErr error
	if rashinInstalled() {
		if out, err := runRashinCLI("off"); err != nil {
			stopErr = fmt.Errorf("rashin could not stop cleanly: %s", commandError(out, err))
		}
		if err := removeRashinPackage(); err != nil {
			return err
		}
	}
	if err := applyRashinDesktop(); err != nil {
		return err
	}
	return stopErr
}

func commandError(out []byte, err error) string {
	message := strings.TrimSpace(string(out))
	if message != "" {
		return message
	}
	return err.Error()
}

func recordProvisioned(pkg string) error {
	path := filepath.Join(stateHome(), "ryoku", "provisioned")
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
