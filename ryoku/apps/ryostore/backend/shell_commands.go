package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// runShellsSyncCommand is `ryostore internal shells-sync [--prune] [--json]`.
func runShellsSyncCommand(w io.Writer, args []string) error {
	flags, err := parseShellFlags("shells-sync", args, "--prune", "--json")
	if err != nil {
		return err
	}
	result, err := runShellsSync(flags["--prune"])
	if flags["--json"] {
		if encodeErr := json.NewEncoder(w).Encode(result); encodeErr != nil && err == nil {
			err = encodeErr
		}
	} else {
		for _, problem := range result.Problems {
			fmt.Fprintln(w, problem)
		}
	}
	return err
}

// runShellsRequireCommand is
// `ryostore internal shells-require [--install] [--fallback] [--json]`. The
// JSON is printed even when an install failed, so callers see what changed.
func runShellsRequireCommand(w io.Writer, args []string) error {
	flags, err := parseShellFlags("shells-require", args, "--install", "--fallback", "--json")
	if err != nil {
		return err
	}
	result, err := runShellsRequire(flags["--install"], flags["--fallback"])
	if flags["--json"] {
		if encodeErr := json.NewEncoder(w).Encode(result); encodeErr != nil && err == nil {
			err = encodeErr
		}
		return err
	}
	for _, requirement := range result.Missing {
		state := "not offered"
		if requirement.Available {
			state = "available as " + requirement.Category + "/" + requirement.ID
		} else if requirement.ID != "" {
			state = requirement.Category + "/" + requirement.ID + " cannot be installed here"
		}
		fmt.Fprintf(w, "%s %s: %s\n", requirement.Selection, requirement.Value, state)
	}
	return err
}

func parseShellFlags(command string, args []string, allowed ...string) (map[string]bool, error) {
	flags := map[string]bool{}
	for _, arg := range args {
		known := false
		for _, flag := range allowed {
			if arg == flag {
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("internal %s: unknown flag %q; expected %s", command, arg, strings.Join(allowed, ", "))
		}
		flags[arg] = true
	}
	return flags, nil
}
