package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
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

// ipcBase is the qs argv up to and including "ipc" for a component. Shell
// calls go to a ready item runtime by pid, since it does not answer to -c shell.
func ipcBase(config string) []string {
	if config == "shell" {
		ready, _ := runtimeIPCReadyFile.Load().(string)
		if pid, ok := runtimeIPCPid(ready); ok {
			return []string{"ipc", "--pid", strconv.Itoa(pid)}
		}
	}
	return append(qsSelect(config), "ipc")
}

// ipcArgv builds the qs argv for an IpcHandler call on a component.
func ipcArgv(config, target, fn string, args ...string) []string {
	argv := append(ipcBase(config), "call", target, fn)
	return append(argv, args...)
}

// ipcPropArgv builds the qs argv that reads one IpcHandler property.
func ipcPropArgv(config, target, prop string) []string {
	return append(ipcBase(config), "prop", "get", target, prop)
}

var (
	ipcAttemptTimeout = 3 * time.Second
	ipcRetryWindow    = 1500 * time.Millisecond
	ipcRetryEvery     = 150 * time.Millisecond
)

// qsRun runs one qs invocation. A package var so tests inject a runner.
var qsRun = func(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, "qs", argv...)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

type ipcErrKind int

const (
	ipcFailed ipcErrKind = iota
	ipcMissing
	ipcNotRunning
	ipcTimedOut
)

type ipcError struct {
	kind ipcErrKind
	msg  string
}

func (e *ipcError) Error() string { return e.msg }

// qsLine returns the first output line that starts with one of the phrases,
// case-insensitively, or "".
func qsLine(phrases []string, outputs ...[]byte) string {
	for _, out := range outputs {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			lower := strings.ToLower(line)
			for _, p := range phrases {
				if strings.HasPrefix(lower, p) {
					return line
				}
			}
		}
	}
	return ""
}

var (
	qsMissingPhrases    = []string{"target not found", "function not found", "property not found"}
	qsNotRunningPhrases = []string{"no running instances", "not ready"}
)

// classifyQs maps one qs attempt to an error, or nil on success. qs reports a
// missing target or function on its output and still exits 0.
func classifyQs(stdout, stderr []byte, err error, timedOut bool) *ipcError {
	if timedOut {
		return &ipcError{ipcTimedOut, "timed out after " + ipcAttemptTimeout.String()}
	}
	if line := qsLine(qsMissingPhrases, stderr, stdout); line != "" {
		return &ipcError{ipcMissing, line}
	}
	if line := qsLine(qsNotRunningPhrases, stderr, stdout); line != "" {
		return &ipcError{ipcNotRunning, "no shell is running (" + line + ")"}
	}
	if err != nil {
		msg := strings.TrimSpace(string(stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(stdout))
		}
		if msg == "" {
			msg = err.Error()
		}
		return &ipcError{ipcFailed, msg}
	}
	return nil
}

// qsIpcOnce runs one qs ipc attempt, bounded by ipcAttemptTimeout, and returns
// the call's stdout.
func qsIpcOnce(argv []string) (string, *ipcError) {
	ctx, cancel := context.WithTimeout(context.Background(), ipcAttemptTimeout)
	defer cancel()
	stdout, stderr, err := qsRun(ctx, argv)
	timedOut := err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
	if ierr := classifyQs(stdout, stderr, err, timedOut); ierr != nil {
		return "", ierr
	}
	return strings.TrimRight(string(stdout), "\n"), nil
}

// qsIpc is qsIpcOnce that retries a shell which is not running or not ready
// yet, within ipcRetryWindow. Every other failure is final.
func qsIpc(argv []string) (string, error) {
	deadline := time.Now().Add(ipcRetryWindow)
	for {
		value, ierr := qsIpcOnce(argv)
		if ierr == nil {
			return value, nil
		}
		if ierr.kind != ipcNotRunning || !time.Now().Add(ipcRetryEvery).Before(deadline) {
			return "", ierr
		}
		time.Sleep(ipcRetryEvery)
	}
}

// encodeIPCRequest is the control line for `ryoku-shell ipc`. The words ride
// as one JSON array so empty and spaced args survive the line split.
func encodeIPCRequest(words []string) string {
	body, _ := json.Marshal(words)
	return "ipc " + string(body)
}

// ipcRequest serves `ipc ["call",target,fn,args...]` and `ipc ["get",target,prop]`
// against the running shell. The reply is "ok", a newline and the value, or
// "err ...".
func ipcRequest(payload string) string {
	var words []string
	if err := json.Unmarshal([]byte(payload), &words); err != nil {
		return "err ipc: malformed request"
	}
	var argv []string
	switch {
	case len(words) >= 3 && words[0] == "call":
		argv = ipcArgv("shell", words[1], words[2], words[3:]...)
	case len(words) == 3 && words[0] == "get":
		argv = ipcPropArgv("shell", words[1], words[2])
	default:
		return "err ipc: expected call <target> <function> [args...] or get <target> <property>"
	}
	value, err := qsIpc(argv)
	if err != nil {
		return "err ipc " + words[1] + " " + words[2] + ": " + err.Error()
	}
	return "ok\n" + value
}
