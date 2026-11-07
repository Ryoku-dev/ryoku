package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

// runtimeIPCReadyFile is the ready file of the supervised item runtime that
// currently replaces the resident shell, or "" when none runs.
var runtimeIPCReadyFile atomic.Value

// runtimeIPCPid reads the runtime's qs pid from its ready file. It reports
// false while the file is absent, unreadable or empty, or the pid is gone.
func runtimeIPCPid(readyFile string) (int, bool) {
	if readyFile == "" {
		return 0, false
	}
	body, err := os.ReadFile(readyFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return 0, false
	}
	return pid, true
}

// ipcArgv builds the qs argv for an IpcHandler call on a component. Shell calls
// go to a ready item runtime by pid, since it does not answer to -c shell.
func ipcArgv(config, target, fn string, args ...string) []string {
	var argv []string
	if config == "shell" {
		ready, _ := runtimeIPCReadyFile.Load().(string)
		if pid, ok := runtimeIPCPid(ready); ok {
			argv = []string{"ipc", "--pid", strconv.Itoa(pid), "call", target, fn}
		}
	}
	if argv == nil {
		argv = append(qsSelect(config), "ipc", "call", target, fn)
	}
	return append(argv, args...)
}
