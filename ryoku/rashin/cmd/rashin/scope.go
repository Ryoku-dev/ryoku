package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/sandbox"
)

func init() {
	register(&Command{Name: "scope", Group: "tools", Usage: "-- <command> [args...]", Summary: i18n.T("Run a command kept from abstract sockets and from signalling what runs outside it. Rashin's sandboxes run their programs through it."), Run: runScope})
}

// scopeExec replaces this process; a test stands in for it.
var scopeExec = syscall.Exec

// runScope runs the command after "--" in place of rashin, scoped where the
// kernel allows. It returns only when nothing ran.
func runScope(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		return usageError("scope -- <command> [args...]")
	}
	path, err := exec.LookPath(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf("rashin scope: %s is not a command here: %v", args[1], err))
		return 127
	}
	err = sandbox.Exec(path, args[1:], os.Environ(), scopeExec)
	fmt.Fprintln(os.Stderr, i18n.Tf("rashin scope: %s did not run: %v", args[1], err))
	return 126
}
