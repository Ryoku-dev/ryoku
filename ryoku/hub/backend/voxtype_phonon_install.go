package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed assets/phonon/setup.py
var phononSetup []byte

//go:embed assets/phonon/phonon-2.service
var phononUnitTemplate []byte

func phononReceiptPath() string {
	return filepath.Join(dataHome(), "phonon-2", "hub-install.json")
}

func runPhononSetup(action string) error {
	manager := hostInitName()
	if manager != "systemd" && manager != "runit" {
		return fmt.Errorf("the Phonon CPU runtime requires a systemd or runit user session")
	}
	if manager == "runit" {
		if err := exec.Command("ryoku-host", "session", "ensure", "--user").Run(); err != nil {
			return fmt.Errorf("provision the Phonon-2 runit service: %w", err)
		}
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
	if manager == "systemd" {
		if err := os.WriteFile(filepath.Join(stage, "phonon-2.service"), phononUnitTemplate, 0o600); err != nil {
			return err
		}
	}
	cmd := exec.Command("python3", script, action)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "RYOKU_PHONON_SERVICE_MANAGER=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "RYOKU_PHONON_SERVICE_MANAGER="+manager)
	if err := runCancelableInstall(cmd); err != nil {
		return err
	}
	if manager == "systemd" && (action == "install" || action == "adopt") {
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
