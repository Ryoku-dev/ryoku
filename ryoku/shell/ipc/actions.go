package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// shellDir (RYOKU_SHELL_DIR): run components straight out of a repo checkout
// (qs -p <dir>/quickshell/<name>) instead of an installed config under
// ~/.config. empty in a deployed setup.
var shellDir = os.Getenv("RYOKU_SHELL_DIR")

var frameBarMenuIDs = map[string]bool{
	"quick-settings": true,
	"theme":          true,
	"wallpaper":      true,
	"weather":        true,
}

func menuID(cmd string) (string, bool) {
	fields := strings.Fields(cmd)
	if len(fields) != 2 || fields[0] != "menu" {
		return "", false
	}
	id := fields[1]
	// Strip any "#page" suffix before checking the catalog; the full id
	// (including the suffix) is returned so QML receives the deep-link page.
	baseID := id
	if h := strings.IndexByte(id, '#'); h >= 0 {
		baseID = id[:h]
	}
	if !frameBarMenuIDs[baseID] {
		return "", false
	}
	return id, true
}

// qsSelect: qs config selector for a component. by repo path in dev, by config
// name when deployed.
func qsSelect(name string) []string {
	if shellDir != "" {
		return []string{"-p", filepath.Join(shellDir, "quickshell", name)}
	}
	return []string{"-c", name}
}

// ipcCall: invoke a Quickshell IpcHandler function. component may have just
// been started, so it retries briefly until the instance answers.
func ipcCall(config, target, fn, arg string) string {
	argv := append(qsSelect(config), "ipc", "call", target, fn)
	if arg != "" {
		argv = append(argv, arg)
	}
	var out []byte
	var err error
	for i := 0; i < 10; i++ {
		out, err = exec.Command("qs", argv...).CombinedOutput()
		if err == nil {
			return "ok"
		}
		time.Sleep(150 * time.Millisecond)
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return "err qs ipc " + config + "/" + fn + ": " + msg
}

// ipcCallN = ipcCall for IpcHandler functions that take more than one arg
// (e.g. pluginPopout(mon, id)). empty trailing args still go positionally.
func ipcCallN(config, target, fn string, args ...string) string {
	argv := append(qsSelect(config), "ipc", "call", target, fn)
	argv = append(argv, args...)
	var out []byte
	var err error
	for i := 0; i < 10; i++ {
		out, err = exec.Command("qs", argv...).CombinedOutput()
		if err == nil {
			return "ok"
		}
		time.Sleep(150 * time.Millisecond)
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return "err qs ipc " + config + "/" + fn + ": " + msg
}

// ryoku: the pill was consolidated into the single shell instance, which serves
// no command socket (its shell.qml omits the SocketServer fast path). The pill
// socket helpers below and pillIpc are therefore dead in production; they are
// retained (and still covered by pillipc_test.go) for the retire phase rather
// than deleted here, to keep this cutover minimal and reversible.
// pillSockPath: the pill's command socket. The pill (a persistent Quickshell
// component) serves it; the daemon writes a surface command here to skip the
// `qs ipc call` subprocess on the keybind hot path.
func pillSockPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "ryoku-pill.sock")
}

// pillSocketCall writes one command line to the pill socket and reports whether
// the pill acknowledged with "ok". A miss (socket down, pill restarting, an
// unknown command) returns false so the caller falls back to the qs client.
func pillSocketCall(line string) bool {
	conn, err := net.DialTimeout("unix", pillSockPath(), 200*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintln(conn, line); err != nil {
		return false
	}
	buf := make([]byte, 16)
	n, _ := conn.Read(buf)
	return strings.TrimSpace(string(buf[:n])) == "ok"
}

// pillIpc invokes a pill IpcHandler function, preferring the command socket and
// falling back to the qs client when it is unreachable. Empty args drop out of
// the socket line; the qs fallback keeps the same positional argv.
func pillIpc(fn string, args ...string) string {
	line := fn
	for _, a := range args {
		if a != "" {
			line += " " + a
		}
	}
	if pillSocketCall(line) {
		return "ok"
	}
	return ipcCallN("pill", "pill", fn, args...)
}

// shellIpc invokes a "shell" IpcHandler function through the qs client. Unlike
// the pill, the single consolidated shell serves no command socket, so there is
// no socket fast path to prefer: every call goes straight to the qs client. A
// package var so tests can capture the emitted call in place of a live
// Quickshell.
var shellIpc = func(fn string, args ...string) string {
	return ipcCallN("shell", "shell", fn, args...)
}

// Lock proof is scoped to the selected login1 session and one qylock
// generation. A stale process, marker, or delayed callback from another
// compositor instance can never authorize this session's suspend.
func validSessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') &&
			r != '_' && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func lockSessionID() string {
	id := os.Getenv("XDG_SESSION_ID")
	if !validSessionID(id) {
		return ""
	}
	return id
}

func lockProofPath(suffix string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "qylock."+lockSessionID()+"."+suffix)
}

func lockMarker() string     { return lockProofPath("locked") }
func lockExpected() string   { return lockProofPath("expected") }
func lockProofGuard() string { return lockProofPath("proof.lock") }

func withLockProofGuard(fn func() error) error {
	guard, err := os.OpenFile(lockProofGuard(), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open qylock proof guard: %w", err)
	}
	defer guard.Close()
	if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock qylock proof guard: %w", err)
	}
	defer syscall.Flock(int(guard.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}

// clearCurrentLockProof shares proof.sh's advisory lock. prepareUnlock calls it
// while holding the suspend transaction mutex, so no suspend request can accept
// the authenticated generation between guard restoration and invalidation.
func clearCurrentLockProof() error {
	return withLockProofGuard(func() error {
		if err := os.Remove(lockMarker()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear qylock proof: %w", err)
		}
		return nil
	})
}

// clearInvalidLockProof rechecks marker, expectation and live owner under the
// same flock used by proof.sh. A recovered client can publish a new proof
// between polling iterations without an old mismatch deleting that new marker.
func clearInvalidLockProof() error {
	return withLockProofGuard(func() error {
		token := readLockProof(lockMarker())
		if token != "" && readLockProof(lockExpected()) == token &&
			qylockProcessMatches(lockSessionID(), token) {
			return nil
		}
		if err := os.Remove(lockMarker()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear invalid qylock proof: %w", err)
		}
		return nil
	})
}

func processEnvironmentValue(data []byte, key string) string {
	prefix := []byte(key + "=")
	for _, field := range bytes.Split(data, []byte{0}) {
		if bytes.HasPrefix(field, prefix) {
			return string(field[len(prefix):])
		}
	}
	return ""
}

func qylockProcessMatches(session, token string) bool {
	if session == "" {
		return false
	}
	output, err := exec.Command(
		"pgrep", "-u", strconv.Itoa(os.Getuid()), "-f", lockClientPattern,
	).Output()
	if err != nil {
		return false
	}
	for _, field := range strings.Fields(string(output)) {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 0 {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", field, "environ"))
		if err != nil || processEnvironmentValue(env, "XDG_SESSION_ID") != session {
			continue
		}
		if token == "" || processEnvironmentValue(env, "QYLOCK_PROOF_TOKEN") == token {
			return true
		}
	}
	return false
}

func readLockProof(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

func currentLockProofValid() bool {
	valid := false
	if err := withLockProofGuard(func() error {
		token := readLockProof(lockMarker())
		valid = token != "" &&
			readLockProof(lockExpected()) == token &&
			qylockProcessMatches(lockSessionID(), token)
		return nil
	}); err != nil {
		return false
	}
	return valid
}

// lockWait bounds an ordinary IPC lock request. Suspend requests pass their
// live logind deadline directly to ensureSessionLocked.
var (
	lockWait           = 3 * time.Second
	lockPoll           = 50 * time.Millisecond
	lockRequestMu      sync.Mutex
	lockProcessRunning = func() bool { return qylockProcessMatches(lockSessionID(), "") }
	lockProofValid     = currentLockProofValid
	lockStarter        = spawnLocker
)

// lockClientPattern matches live and retained-stage qylock clients. Proof
// validation also checks the selected session and generation token through
// /proc.
const lockClientPattern = "quickshell.*(quickshell-lockscreen|qylock-next/lockscreen).*/lock_shell[.]qml"

// lockSession locks the screen with qylock, the in-session lock Ryoku ships.
// The shell has no lock surface of its own.
func lockSession() string {
	if err := ensureSessionLocked(time.Now().Add(lockWait)); err != nil {
		return "err lock: " + err.Error()
	}
	return "ok"
}

// ensureSessionLocked serializes the full launch-to-secure handshake. The
// process-local mutex closes the daemon race; lock.sh's flock closes the same
// race against another launcher process.
func ensureSessionLocked(deadline time.Time) error {
	lockRequestMu.Lock()
	defer lockRequestMu.Unlock()

	if lockSessionID() == "" {
		return fmt.Errorf("cannot identify the graphical login1 session")
	}
	marker := lockMarker()
	running := lockProcessRunning()
	if running && lockProofValid() {
		return nil
	}
	if !deadline.After(time.Now()) {
		return fmt.Errorf("secure lock deadline already expired")
	}
	started := false
	if !running {
		// A marker with no live locker is stale and must not make an unlocked
		// session look secure.
		_ = clearInvalidLockProof()
		if err := lockStarter(0); err != nil {
			return fmt.Errorf("start qylock: %w", err)
		}
		started = true
	}

	for {
		if _, err := os.Stat(marker); err == nil {
			if lockProofValid() {
				return nil
			}
			_ = clearInvalidLockProof()
		}
		if !lockProcessRunning() && !started {
			if err := lockStarter(0); err != nil {
				return fmt.Errorf("restart qylock after pending unlock: %w", err)
			}
			started = true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("qylock did not confirm compositor security before the deadline")
		}
		pause := lockPoll
		if remaining < pause {
			pause = remaining
		}
		time.Sleep(pause)
	}
}

// spawnLocker starts the long-lived qylock wrapper. The wrapper owns crash
// recovery so its budget cannot be multiplied by a second daemon-side retry
// loop; this goroutine only reaps it when the lock cycle ends.
func spawnLocker(_ int) error {
	launcher, err := exec.LookPath("ryoku-qylock-lock")
	if err != nil {
		return err
	}
	cmd := exec.Command(launcher)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// voxtypeRecord starts or stops dictation on the running Voxtype daemon (the
// Super+` tap). Voxtype is an optional AUR app (voxtype-bin); absent, this is a
// no-op and the voice surface stays a plain mic meter. `voxtype record` drives
// the user service in place (Voxtype's own hotkey is disabled so the shell owns
// Super+`). verb is "start" or "stop".
func voxtypeRecord(verb string) {
	if _, err := exec.LookPath("voxtype"); err != nil {
		return
	}
	// reap the short-lived record client in the background so it does not linger
	// as a zombie; voxtypeRecord fires once per tap (start on show, stop on hide).
	cmd := exec.Command("voxtype", "record", verb)
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}

// dictationReady reports whether a Super+` tap can actually dictate: Voxtype
// installed and its user service running. When it can't, the pill shows an
// "off" note instead of a listening wave that would capture nothing.
func dictationReady() bool {
	if _, err := exec.LookPath("voxtype"); err != nil {
		return false
	}
	return exec.Command("systemctl", "--user", "is-active", "--quiet", "voxtype.service").Run() == nil
}

func currentUserProcessRunning(pattern string) bool {
	return exec.Command("pgrep", "-u", strconv.Itoa(os.Getuid()), "-f", pattern).Run() == nil
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state")
}
