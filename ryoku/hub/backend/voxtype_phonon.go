package main

import (
	"encoding/json"
	"errors"
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

func phononDependencyDropinPath() string {
	return filepath.Join(configHome(), "systemd", "user", "voxtype.service.d", "ryoku-phonon.conf")
}

func phononRunitServicePath() string {
	return runitUserServicePath("phonon-2")
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
	path := phononDependencyDropinPath()
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
func startPhononService() (returnErr error) {
	if !hostSupportsPhonon() {
		return fmt.Errorf("the local Phonon server requires a systemd or runit user session")
	}
	previous, err := snapshotVoxtypeUserService(hostInitName(), "phonon-2")
	if err != nil {
		return err
	}
	if !previous.tracked {
		return fmt.Errorf("the Phonon-2 user service is missing")
	}
	ready := false
	defer func() {
		if !ready {
			if err := errors.Join(previous.restoreEnabledState(), previous.restoreActiveState(false)); err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("restore the previous Phonon-2 service state: %w", err))
			}
		}
	}()
	if err := userService("enable", "--now", "phonon-2"); err != nil {
		return err
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
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("Phonon-2 did not become ready at %s; check the phonon-2 user service", phononEndpoint)
}
