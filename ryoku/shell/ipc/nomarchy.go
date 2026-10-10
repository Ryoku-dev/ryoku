package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

var nomarchySystemSource = "/usr/share/ryoku/nomarchy"

func nomarchyPath() string {
	return filepath.Join(dataHome(), "ryoku", "nomarchy")
}

func nomarchySource() (string, error) {
	candidates := []string{
		os.Getenv("RYOKU_NOMARCHY_SOURCE"),
		filepath.Join(dataHome(), "ryoku", "nomarchy-package"),
		nomarchySystemSource,
	}
	for _, root := range candidates {
		if root == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, "nomarchy-activate")); err == nil && !info.IsDir() {
			return root, nil
		}
	}
	return "", fmt.Errorf("Nomarchy assets are not installed")
}

func nomarchyEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if len(entry) >= len("OMARCHY_PATH=") && entry[:len("OMARCHY_PATH=")] == "OMARCHY_PATH=" {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "OMARCHY_PATH="+nomarchyPath())
}

func setNomarchyActive(active bool) error {
	root, err := nomarchySource()
	if err != nil {
		if active {
			return err
		}
		return nil
	}
	name := "nomarchy-deactivate"
	if active {
		name = "nomarchy-activate"
	}
	cmd := exec.Command(filepath.Join(root, name))
	cmd.Env = nomarchyEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) != 0 {
			err = fmt.Errorf("%s: %s", err, out)
		}
		if active {
			return err
		}
		log.Printf("ryoku-shell: Nomarchy deactivation: %v", err)
	}
	return nil
}
