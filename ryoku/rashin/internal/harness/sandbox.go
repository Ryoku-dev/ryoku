package harness

import (
	"os"
	"path/filepath"
	"slices"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/sandbox"
)

// cacheDir is the cache under HOME a sandboxed harness may write, where its
// package managers, language servers and downloads keep what they fetch.
const cacheDir = ".cache"

// stateOf is the state a harness writes under its HOME: its definition's, or
// for a custom instance the definition whose command it runs, by base name.
func stateOf(id string, command []string) []string {
	for _, def := range definitions {
		if def.id == id {
			return def.state
		}
	}
	if len(command) == 0 {
		return nil
	}
	base := filepath.Base(command[0])
	for _, def := range definitions {
		for _, candidate := range def.commands {
			if candidate[0] == base {
				return def.state
			}
		}
	}
	return nil
}

// sandboxFor completes a team session's sandbox for one harness: its state
// that exists and its cache, which is created when missing, both under the
// HOME it runs with and resolved as the daemon's paths are. An instance's own
// HOME wins over the user's.
func sandboxFor(base sandbox.Spec, state []string, environment map[string]string) (sandbox.Spec, error) {
	home := environment["HOME"]
	if home == "" {
		home = paths.Home()
	}
	cache := filepath.Join(home, cacheDir)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return sandbox.Spec{}, err
	}
	spec := base
	spec.Writable = append(slices.Clone(base.Writable), paths.Resolved(cache))
	for _, relative := range state {
		path := filepath.Join(home, relative)
		if _, err := os.Stat(path); err == nil {
			spec.Writable = append(spec.Writable, paths.Resolved(path))
		}
	}
	return spec, nil
}

// withSandbox runs the agent inside spec and keeps Rashin from reading,
// writing or running anything on its behalf. The sandbox is tried at every
// start, so an agent whose sandbox cannot run does not start.
func withSandbox(spec sandbox.Spec) runnerOption {
	return func(r *runner) {
		r.sandbox = func(argv []string) ([]string, error) {
			if err := sandbox.Available(); err != nil {
				return nil, err
			}
			return sandbox.Command(spec, argv)
		}
	}
}
