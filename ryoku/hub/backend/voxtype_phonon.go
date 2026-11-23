package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const phononPresetKey = "phonon-2"

var phononEndpoint = "http://127.0.0.1:8010"

func phononUnitPath() string {
	return filepath.Join(configHome(), "systemd", "user", "phonon-2.service")
}

func phononRunitServicePath() string {
	root := os.Getenv("RYOKU_USER_SERVICE_DIR")
	if root == "" {
		root = filepath.Join(os.Getenv("HOME"), ".config", "service")
	}
	return filepath.Join(root, "phonon-2")
}

func phononServiceExists() bool {
	return phononServiceExistsFor(hostInitName())
}

func phononServiceExistsFor(initSystem string) bool {
	switch initSystem {
	case "systemd":
		return fileExists(phononUnitPath())
	case "runit":
		info, err := os.Stat(filepath.Join(phononRunitServicePath(), "run"))
		return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
	default:
		return false
	}
}

func phononModelPath() string {
	return filepath.Join(os.Getenv("HOME"), ".cache", "fermion", "speech", "FermionResearch__Phonon-2", "model_phonon2_c4c_int6", "model.fermion")
}

func phononInstalled() bool {
	return fileExists(filepath.Join(dataHome(), "phonon-2", "venv", "bin", "phonon")) &&
		fileExists(phononModelPath()) && phononServiceExists()
}

func configurePhononService(selected bool) error {
	initSystem := hostInitName()
	switch initSystem {
	case "runit":
		if selected && !phononServiceExistsFor(initSystem) {
			return fmt.Errorf("the Phonon-2 runit service is not provisioned for this user")
		}
		return nil
	case "systemd":
	default:
		if selected {
			return fmt.Errorf("the local Phonon server requires a systemd or runit user session")
		}
		return nil
	}
	path := filepath.Join(configHome(), "systemd", "user", "voxtype.service.d", "ryoku-phonon.conf")
	if selected {
		if err := atomicWrite(path, []byte("[Unit]\nRequires=phonon-2.service\nAfter=phonon-2.service\n"), 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return userService("daemon-reload")
}

// Readiness is checked before committing the new selection or changing the
// previous engine's dependencies. A failed start leaves dictation usable.
func startPhononService() (func(), error) {
	if !hostSupportsPhonon() {
		return nil, fmt.Errorf("the local Phonon server requires a systemd or runit user session")
	}
	wasEnabled := userService("is-enabled", "phonon-2") == nil
	wasActive := userService("is-active", "phonon-2") == nil
	restore := func() {
		isEnabled := userService("is-enabled", "phonon-2") == nil
		if wasEnabled != isEnabled {
			if wasEnabled {
				_ = userService("enable", "phonon-2")
			} else {
				_ = userService("disable", "phonon-2")
			}
		}
		isActive := userService("is-active", "phonon-2") == nil
		if wasActive != isActive {
			if wasActive {
				_ = userService("start", "phonon-2")
			} else {
				_ = userService("stop", "phonon-2")
			}
		}
	}
	ready := false
	defer func() {
		if !ready {
			restore()
		}
	}()
	if err := userService("enable", "--now", "phonon-2"); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(phononEndpoint + "/health")
		if err == nil {
			var health struct {
				Status string `json:"status"`
				Model  string `json:"model"`
			}
			err = json.NewDecoder(response.Body).Decode(&health)
			response.Body.Close()
			if err == nil && response.StatusCode == http.StatusOK && health.Status == "ok" && health.Model == phononPresetKey {
				ready = true
				return restore, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("Phonon-2 did not become ready at %s; check phonon-2.service", phononEndpoint)
}
