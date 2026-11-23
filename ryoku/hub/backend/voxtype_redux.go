package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const reduxPresetKey = "parakeet-redux"

func reduxModelPath() string {
	return filepath.Join(dataHome(), "voxtype", "models", "redux-packed.gguf")
}
func reduxAdapterPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "bin", "voxtype-redux")
}

func reduxDependencyDropinPath() string {
	return filepath.Join(configHome(), "systemd", "user", "voxtype.service.d", "ryoku-redux.conf")
}

// reduxInstalled means the Hub's installer published every Redux file and its
// receipt; a hand-made install without the receipt is not managed here.
func reduxInstalled() bool {
	return fileExists(reduxReceiptPath()) && fileExists(reduxModelPath()) && fileExists(reduxAdapterPath()) &&
		fileExists(filepath.Join(configHome(), "systemd", "user", "voxtype-redux.service"))
}

// Keep the resident worker dependency only while Redux is selected.
func configureReduxService(selected, enabled bool) error {
	if !hostUsesSystemd() {
		if selected {
			return fmt.Errorf("the installed Redux adapter requires systemd")
		}
		return nil
	}
	path := reduxDependencyDropinPath()
	if selected {
		if err := atomicWrite(path, []byte("[Unit]\nRequires=voxtype-redux.service\nAfter=voxtype-redux.service\n"), 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := userService("daemon-reload"); err != nil {
		return err
	}
	if selected && enabled {
		if err := userService("enable", "--now", "voxtype-redux"); err != nil {
			return err
		}
	}
	return nil
}
