package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const fixUsage = `usage: ryoku-rashin fix doctor [finding name]
       ryoku-rashin fix tip <id>
       ryoku-rashin fix app <name> [what happened...]

Opens your agent (the fix agent picked in Rashin, else the chat agent) with the
problem as its first message: right here when run from a terminal, otherwise in
a new terminal window that runs the check and opens the agent there. The agent reads the logs itself, explains the
cause, and asks before it changes anything.`

// fixKinds are the problems Fix with AI knows how to brief.
var fixKinds = map[string]bool{"doctor": true, "tip": true, "app": true}

// parseFixArgs maps the command line onto a fix request.
func parseFixArgs(args []string) (fixRequest, error) {
	var req fixRequest
	if len(args) == 0 {
		return req, errors.New(fixUsage)
	}
	req.Kind = args[0]
	switch req.Kind {
	case "doctor":
		req.Name = strings.Join(args[1:], " ")
	case "tip":
		if len(args) < 2 {
			return req, errors.New(fixUsage)
		}
		req.ID = args[1]
	case "app":
		if len(args) < 2 {
			return req, errors.New(fixUsage)
		}
		req.App = args[1]
		req.Note = strings.Join(args[2:], " ")
	default:
		return req, errors.New(fixUsage)
	}
	return req, nil
}

func cmdFix(args []string) error {
	req, err := parseFixArgs(args)
	if err != nil {
		return err
	}
	if !onTerminal() {
		return openFixCommandInTerminal(args)
	}
	if req.Kind == "doctor" {
		fmt.Println("Running the health check...")
	}
	display, brief, err := buildFix(req)
	if err != nil {
		return err
	}
	launch, err := planFix(display, brief)
	if err != nil {
		return err
	}
	fmt.Printf("%s\nOpening %s...\n", display, launch.Harness)
	return runFixHere(launch)
}

// openFixCommandInTerminal answers a caller with no terminal (the Settings
// button) at once: the health check takes a minute or more, so it runs in the
// opened terminal with its progress and any error on screen, not behind a
// button that shows nothing.
func openFixCommandInTerminal(args []string) error {
	h, _, _, err := pickConfiguredHarness()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	argv := holdOnFailure("Fix with AI", append([]string{self, "fix"}, args...))
	if err := launchTerminal(VaultDir(), "ryoku-fix-", argv); err != nil {
		return err
	}
	fmt.Printf("Opened %s in a terminal.\n", h.Name)
	return nil
}

// onTerminal: both ends are a terminal, so the harness can take it over.
var onTerminal = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		st, err := f.Stat()
		if err != nil || st.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}
