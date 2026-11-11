package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	i18n "ryoku-i18n"
)

// appProgram is the file name of the Rashin app binary.
const appProgram = "rashin-app"

// packagedApp is where the Rashin package installs the app.
const packagedApp = "/usr/bin/rashin-app"

// verifyPeer refuses a connection opened by another user and returns the pid
// of the process that opened it.
func verifyPeer(conn net.Conn) (int32, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("%s", i18n.T("The connection is not a Unix socket."))
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var cred *syscall.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		cred, socketErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if socketErr != nil {
		return 0, socketErr
	}
	if cred == nil || cred.Uid != uint32(os.Getuid()) {
		return 0, fmt.Errorf("%s", i18n.T("The peer uid does not match the daemon uid."))
	}
	return cred.Pid, nil
}

// appPeer reports whether a process runs the Rashin app: the rashin-app
// binary beside program, the daemon's own executable, or the packaged one.
// An empty program allows only the packaged app.
func appPeer(program string) func(pid int32) bool {
	allowed := appPaths(program)
	return func(pid int32) bool {
		exe, err := processExe(pid)
		return err == nil && allowed[exe]
	}
}

// appPaths are the paths the Rashin app may run from, each also by the path
// its symlinks resolve to, since the kernel names a process's binary resolved.
func appPaths(program string) map[string]bool {
	candidates := []string{packagedApp}
	if program != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(program), appProgram))
	}
	allowed := make(map[string]bool, 2*len(candidates))
	for _, path := range candidates {
		allowed[path] = true
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			allowed[resolved] = true
		}
	}
	return allowed
}

// processExe is the binary a process runs. A binary replaced on disk since the
// process started, as by an upgrade, still counts as the path it ran from.
func processExe(pid int32) (string, error) {
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(int(pid)), "exe"))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(exe, " (deleted)"), nil
}
