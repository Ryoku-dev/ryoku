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
const phononEndpoint = "http://127.0.0.1:8010"

func phononUnitPath() string {
	return filepath.Join(configHome(), "systemd", "user", "phonon-2.service")
}

func phononModelPath() string {
	return filepath.Join(os.Getenv("HOME"), ".cache", "fermion", "speech", "FermionResearch__Phonon-2", "model_phonon2_c4c_int6", "model.fermion")
}

func phononInstalled() bool {
	return fileExists(filepath.Join(dataHome(), "phonon-2", "venv", "bin", "phonon")) &&
		fileExists(phononModelPath()) && fileExists(phononUnitPath())
}

func configurePhononService(selected bool) error {
	if !hostUsesSystemd() {
		if selected {
			return fmt.Errorf("the local Phonon server requires systemd")
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
func startPhononService() error {
	wasEnabled := userService("is-enabled", "phonon-2") == nil
	wasActive := userService("is-active", "phonon-2") == nil
	ready := false
	defer func() {
		if !ready {
			if !wasEnabled {
				_ = userService("disable", "phonon-2")
			}
			if !wasActive {
				_ = userService("stop", "phonon-2")
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
	return fmt.Errorf("Phonon-2 did not become ready at %s; check phonon-2.service", phononEndpoint)
}
