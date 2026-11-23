package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed assets/phonon/setup.py
var phononSetup []byte

func phononReceiptPath() string {
	return filepath.Join(dataHome(), "phonon-2", "hub-install.json")
}

func runPhononSetup(action string) error {
	if !hostUsesSystemd() {
		return fmt.Errorf("the Phonon CPU runtime currently requires a systemd user session")
	}
	stage, err := os.MkdirTemp("", "ryoku-phonon-setup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	script := filepath.Join(stage, "setup.py")
	if err := os.WriteFile(script, phononSetup, 0o600); err != nil {
		return err
	}
	cmd := exec.Command("python3", script, action)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := runCancelableInstall(cmd); err != nil {
		return err
	}
	if action == "install" || action == "adopt" {
		return userService("daemon-reload")
	}
	return nil
}

func removePhonon() error {
	if !fileExists(phononReceiptPath()) {
		if err := runPhononSetup("adopt"); err != nil {
			return err
		}
	}
	if err := runPhononSetup("check-remove"); err != nil {
		return err
	}
	if selectedPreset(readFileString(voxtypeConfigPath())) == phononPresetKey {
		if err := voxtypeSet(`{"preset":"whisper-fast","enabled":false}`); err != nil {
			return err
		}
	} else {
		if err := configurePhononService(false); err != nil {
			return err
		}
		if err := userService("disable", "--now", "phonon-2"); err != nil {
			return err
		}
	}
	if err := runPhononSetup("remove"); err != nil {
		return err
	}
	if err := userService("daemon-reload"); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}
