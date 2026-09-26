package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// fakeLocker lays a stub lock.sh and stable launcher where lockSession expects
// them. The script body decides whether and when the qylock "secure" marker
// appears.
func fakeLocker(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "quickshell-lockscreen")
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lock.sh"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := "#!/bin/sh\nexec \"$HOME/.local/share/quickshell-lockscreen/lock.sh\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-qylock-lock"), []byte(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func setLockSeams(t *testing.T, running func() bool, starter func(int) error) {
	t.Helper()
	t.Setenv("XDG_SESSION_ID", "test")
	oldRunning, oldProof, oldStarter, oldPoll := lockProcessRunning, lockProofValid, lockStarter, lockPoll
	lockProcessRunning = running
	lockProofValid = func() bool {
		if !running() {
			return false
		}
		_, err := os.Stat(lockMarker())
		return err == nil
	}
	lockStarter, lockPoll = starter, time.Millisecond
	t.Cleanup(func() {
		lockProcessRunning, lockProofValid, lockStarter, lockPoll = oldRunning, oldProof, oldStarter, oldPoll
	})
}

func TestLockerProcessLookupIgnoresAnotherUsersLock(t *testing.T) {
	bin := t.TempDir()
	argsPath := filepath.Join(bin, "args")
	script := `#!/bin/sh
printf '%s\n' "$*" >"$PGREP_ARGS"
if [ "${1:-}" = "-u" ]; then
  [ "${2:-}" = "$FOREIGN_UID" ]
  exit
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "pgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PGREP_ARGS", argsPath)
	t.Setenv("FOREIGN_UID", strconv.Itoa(os.Getuid()+1))

	if exec.Command("pgrep", "-f", lockClientPattern).Run() != nil {
		t.Fatal("foreign-user fixture did not produce a global pgrep match")
	}
	if currentUserProcessRunning(lockClientPattern) {
		t.Fatal("another user's qylock was treated as this session's locker")
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "-u " + strconv.Itoa(os.Getuid()) + " -f " + lockClientPattern + "\n"
	if string(args) != want {
		t.Fatalf("pgrep args = %q, want %q", args, want)
	}
}

func TestLockerProcessLookupFindsRetainedStageClient(t *testing.T) {
	if _, err := exec.LookPath("pgrep"); err != nil {
		t.Skip("pgrep unavailable")
	}
	cmd := exec.Command(
		"bash", "-c",
		"exec -a 'quickshell -p /tmp/qylock-next/lockscreen/lock_shell.qml' sleep 10",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	deadline := time.Now().Add(time.Second)
	for !currentUserProcessRunning(lockClientPattern) {
		if time.Now().After(deadline) {
			t.Fatal("retained-stage qylock client was not detected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUnlockInvalidatesSecureMarkerBeforeLogin1(t *testing.T) {
	unlock, err := filepath.Abs(filepath.Join("..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "unlock.sh"))
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	bin := t.TempDir()
	client := t.TempDir()
	unlockBody, err := os.ReadFile(unlock)
	if err != nil {
		t.Fatal(err)
	}
	unlock = filepath.Join(client, "unlock.sh")
	if err := os.WriteFile(unlock, unlockBody, 0o755); err != nil {
		t.Fatal(err)
	}
	token := "0123456789abcdef"
	marker := filepath.Join(run, "qylock.test.locked")
	expected := filepath.Join(run, "qylock.test.expected")
	observed := filepath.Join(run, "loginctl-called")
	if err := os.WriteFile(marker, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loginctl := `#!/bin/sh
[ "${1:-}" = "unlock-session" ]
[ "${2:-}" = "$XDG_SESSION_ID" ]
if [ -e "$XDG_RUNTIME_DIR/qylock.$XDG_SESSION_ID.locked" ]; then
  exit 42
fi
: >"$UNLOCK_OBSERVED"
`
	prepare := `#!/bin/sh
[ "${1:-}" = "$XDG_SESSION_ID" ]
[ -e "$XDG_RUNTIME_DIR/qylock.$XDG_SESSION_ID.locked" ]
rm -f "$XDG_RUNTIME_DIR/qylock.$XDG_SESSION_ID.locked"
: >"$UNLOCK_PREPARED"
`
	if err := os.WriteFile(filepath.Join(client, "ryoku-qylock-unlock-prepare"), []byte(prepare), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "loginctl"), []byte(loginctl), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(unlock)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"XDG_RUNTIME_DIR="+run,
		"XDG_SESSION_ID=test",
		"QYLOCK_PROOF_TOKEN="+token,
		"UNLOCK_OBSERVED="+observed,
		"UNLOCK_PREPARED="+filepath.Join(run, "unlock-prepared"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unlock helper: %v: %s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("secure marker remained after unlock")
	}
	if _, err := os.Stat(observed); err != nil {
		t.Fatal("login1 was not called after the marker became invalid")
	}
	if _, err := os.Stat(filepath.Join(run, "unlock-prepared")); err != nil {
		t.Fatal("sleep block was not restored before marker invalidation")
	}
}

func TestLockProofRequiresSelectedSessionAndGeneration(t *testing.T) {
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)
	t.Setenv("XDG_SESSION_ID", "session-a")
	token := "aaaaaaaaaaaaaaaa"
	if err := os.WriteFile(lockMarker(), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockExpected(), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	start := func(session, processToken string) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(
			"bash", "-c",
			"exec -a 'quickshell -p /tmp/quickshell-lockscreen/test/lock_shell.qml' sleep 30",
		)
		cmd.Env = append(os.Environ(),
			"XDG_SESSION_ID="+session,
			"QYLOCK_PROOF_TOKEN="+processToken,
		)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		return cmd
	}
	stop := func(cmd *exec.Cmd) {
		t.Helper()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	foreign := start("session-b", token)
	time.Sleep(20 * time.Millisecond)
	if currentLockProofValid() {
		t.Fatal("another login1 session's qylock authorized this session")
	}
	stop(foreign)

	stale := start("session-a", "bbbbbbbbbbbbbbbb")
	time.Sleep(20 * time.Millisecond)
	if currentLockProofValid() {
		t.Fatal("a replacement qylock accepted the previous generation's proof")
	}
	stop(stale)

	current := start("session-a", token)
	time.Sleep(20 * time.Millisecond)
	if !currentLockProofValid() {
		t.Fatal("matching session and qylock generation did not validate")
	}
	proof, err := filepath.Abs(filepath.Join(
		"..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "proof.sh",
	))
	if err != nil {
		t.Fatal(err)
	}
	status := exec.Command(proof, "status", "session-a")
	status.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+run)
	if out, err := status.CombinedOutput(); err != nil {
		t.Fatalf("proof status rejected matching live generation: %v: %s", err, out)
	}
	stop(current)
}

func TestInvalidProofCleanupCannotDeleteRecoveredClientProof(t *testing.T) {
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)
	t.Setenv("XDG_SESSION_ID", "session-a")
	oldToken := "aaaaaaaaaaaaaaaa"
	newToken := "bbbbbbbbbbbbbbbb"
	if err := os.WriteFile(lockMarker(), []byte(oldToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockExpected(), []byte(oldToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	guard, err := os.OpenFile(lockProofGuard(), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	cleared := make(chan error, 1)
	go func() { cleared <- clearInvalidLockProof() }()

	cmd := exec.Command(
		"bash", "-c",
		"exec -a 'quickshell -p /tmp/qylock-next/lockscreen/lock_shell.qml' sleep 30",
	)
	cmd.Env = append(os.Environ(),
		"XDG_SESSION_ID=session-a",
		"QYLOCK_PROOF_TOKEN="+newToken,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	if err := os.WriteFile(lockExpected(), []byte(newToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockMarker(), []byte(newToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := <-cleared; err != nil {
		t.Fatal(err)
	}
	if got := readLockProof(lockMarker()); got != newToken {
		t.Fatalf("recovered proof marker = %q, want %q", got, newToken)
	}
}

func TestLockProofRejectsDelayedPublisherFromPreviousGeneration(t *testing.T) {
	proof, err := filepath.Abs(filepath.Join(
		"..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "proof.sh",
	))
	if err != nil {
		t.Fatal(err)
	}
	run := t.TempDir()
	invoke := func(action, token string) error {
		cmd := exec.Command(proof, action, token, "test")
		cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+run)
		return cmd.Run()
	}
	oldToken := "1111111111111111"
	newToken := "2222222222222222"
	if err := invoke("begin", oldToken); err != nil {
		t.Fatal(err)
	}
	if err := invoke("begin", newToken); err != nil {
		t.Fatal(err)
	}
	if err := invoke("publish", oldToken); err == nil {
		t.Fatal("delayed old lock generation overwrote replacement proof")
	}
	if err := invoke("publish", newToken); err != nil {
		t.Fatal(err)
	}
	if got := readLockProof(filepath.Join(run, "qylock.test.locked")); got != newToken {
		t.Fatalf("published token = %q, want replacement generation", got)
	}
}

// A lock request must wait for the compositor-secure marker rather than return
// when the launcher merely starts.
func TestLockSessionWaitsForMarker(t *testing.T) {
	home := t.TempDir()
	run := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", run)
	old := lockWait
	lockWait = 2 * time.Second
	defer func() { lockWait = old }()

	spawned := filepath.Join(run, "spawned")
	setLockSeams(t, func() bool {
		_, err := os.Stat(spawned)
		return err == nil
	}, spawnLocker)
	fakeLocker(t, home, ": > \""+spawned+"\"\nsleep 0.15\numask 077\n: > \"$XDG_RUNTIME_DIR/qylock.test.locked\"\nsleep 0.5\n")

	start := time.Now()
	if got := lockSession(); got != "ok" {
		t.Fatalf("lockSession = %q, want ok", got)
	}
	elapsed := time.Since(start)
	if _, err := os.Stat(filepath.Join(run, "qylock.test.locked")); err != nil {
		t.Fatal("lockSession returned without the compositor-confirmed marker")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("returned in %v: did not wait for the lock to be confirmed", elapsed)
	}
	if elapsed >= lockWait {
		t.Fatalf("took %v: marker did not short-circuit the timeout", elapsed)
	}
}

// A marker left by a killed locker must not fake "locked": lockSession clears
// it before spawning, then reports an honest error if the replacement never
// becomes compositor-secure.
func TestLockSessionClearsStaleMarkerAndReportsTimeout(t *testing.T) {
	home := t.TempDir()
	run := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", run)
	old := lockWait
	lockWait = 80 * time.Millisecond
	defer func() { lockWait = old }()

	marker := filepath.Join(run, "qylock.test.locked")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	spawned := filepath.Join(run, "spawned")
	setLockSeams(t, func() bool {
		_, err := os.Stat(spawned)
		return err == nil
	}, spawnLocker)
	fakeLocker(t, home, ": > \""+spawned+"\"\nsleep 0.2\n")

	start := time.Now()
	if got := lockSession(); !strings.HasPrefix(got, "err lock: ") {
		t.Fatalf("lockSession = %q, want an honest secure-confirmation error", got)
	}
	if time.Since(start) < lockWait {
		t.Fatal("a locker that never confirms returned before the deadline")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("stale marker survived: a dead locker's marker faked the locked state")
	}
	waitFor(t, spawned)
}

// waitFor polls briefly for a file the fake locker writes asynchronously.
func waitFor(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared: lock.sh was not spawned", path)
}

func TestLockWrapperRecoversWithBoundedCrashBackoff(t *testing.T) {
	lockScript, err := filepath.Abs(filepath.Join(
		"..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "lock.sh",
	))
	if err != nil {
		t.Fatal(err)
	}

	run := func(t *testing.T, lifetime string, failures int, timeout time.Duration, wantStarts int, wantTimeout bool) {
		t.Helper()
		bin := t.TempDir()
		runtimeDir := t.TempDir()
		home := t.TempDir()
		count := filepath.Join(runtimeDir, "count")
		quickshell := `#!/bin/sh
printf 'x\n' >>"$COUNT"
n=$(wc -l <"$COUNT")
if [ "$n" -gt "$LOCKER_FAILURES" ]; then
    exit 0
fi
sleep "$LOCKER_LIFETIME"
exit 17
`
		if err := os.WriteFile(filepath.Join(bin, "quickshell"), []byte(quickshell), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"killall", "ryoku"} {
			if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, lockScript)
		cmd.Env = append(os.Environ(),
			"PATH="+bin+":/usr/bin",
			"HOME="+home,
			"XDG_RUNTIME_DIR="+runtimeDir,
			"XDG_SESSION_ID=test",
			"XDG_SESSION_TYPE=wayland",
			"COUNT="+count,
			"LOCKER_LIFETIME="+lifetime,
			"LOCKER_FAILURES="+strconv.Itoa(failures),
		)
		output, runErr := cmd.CombinedOutput()
		if wantTimeout {
			if ctx.Err() == nil {
				t.Fatalf("persistent crash loop exited instead of backing off: %v: %s", runErr, output)
			}
		} else {
			if ctx.Err() != nil {
				t.Fatalf("lock wrapper never recovered: %s", output)
			}
			if runErr != nil {
				t.Fatalf("clean unlock failed: %v: %s", runErr, output)
			}
		}
		body, err := os.ReadFile(count)
		if err != nil {
			t.Fatal(err)
		}
		if starts := strings.Count(string(body), "x\n"); starts != wantStarts {
			t.Fatalf("qylock starts = %d, want %d: %s", starts, wantStarts, output)
		}
	}

	t.Run("three 1.1 second failures recover after backoff", func(t *testing.T) {
		run(t, "1.1", 3, 7*time.Second, 4, false)
	})
	t.Run("persistent failures cannot spin", func(t *testing.T) {
		run(t, "0", 100, 700*time.Millisecond, 3, true)
	})
	t.Run("authentication exit stays unlocked", func(t *testing.T) {
		run(t, "0", 0, time.Second, 1, false)
	})
}

func TestEnsureSessionLockedSerializesConcurrentCallers(t *testing.T) {
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)

	var alive atomic.Bool
	var starts atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	setLockSeams(t, alive.Load, func(int) error {
		starts.Add(1)
		alive.Store(true)
		started <- struct{}{}
		go func() {
			<-release
			_ = os.WriteFile(lockMarker(), nil, 0o600)
		}()
		return nil
	})

	results := make(chan error, 2)
	deadline := time.Now().Add(time.Second)
	go func() { results <- ensureSessionLocked(deadline) }()
	<-started
	go func() { results <- ensureSessionLocked(deadline) }()

	select {
	case err := <-results:
		t.Fatalf("lock request returned before secure confirmation: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)

	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("ensureSessionLocked: %v", err)
		}
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("started %d lockers, want exactly one", got)
	}
}

func TestEnsureSessionLockedRejectsMarkerFromExitedClient(t *testing.T) {
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)
	setLockSeams(t, func() bool { return false }, func(int) error {
		return os.WriteFile(lockMarker(), nil, 0o600)
	})

	err := ensureSessionLocked(time.Now().Add(100 * time.Millisecond))
	if err == nil || !strings.Contains(err.Error(), "did not confirm compositor security") {
		t.Fatalf("stale handoff error = %v", err)
	}
	if _, statErr := os.Stat(lockMarker()); !os.IsNotExist(statErr) {
		t.Fatalf("marker from exited client survived: %v", statErr)
	}
}

func TestEnsureSessionLockedAcceptsSecureLockAfterCallerWaited(t *testing.T) {
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)
	t.Setenv("XDG_SESSION_ID", "test")
	if err := os.WriteFile(lockMarker(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	setLockSeams(t, func() bool { return true }, func(int) error {
		t.Fatal("already-secure lock started another locker")
		return nil
	})

	if err := ensureSessionLocked(time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("already-secure lock rejected after mutex wait: %v", err)
	}
}

func TestLockLauncherFlockLeavesActiveMarkerUntouched(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "lock.sh"))
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	run := t.TempDir()
	home := t.TempDir()
	starts := filepath.Join(run, "starts")
	ready := filepath.Join(run, "ready")
	release := filepath.Join(run, "release")
	for name, body := range map[string]string{
		"quickshell": "#!/bin/sh\nprintf 'x\\n' >> \"$QYLOCK_TEST_STARTS\"\n: > \"$QYLOCK_TEST_READY\"\nwhile [ ! -e \"$QYLOCK_TEST_RELEASE\" ]; do sleep 0.01; done\n",
		"killall":    "#!/bin/sh\nexit 0\n",
		"ryoku":      "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", run)
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("XDG_SESSION_ID", "test")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("QYLOCK_TEST_STARTS", starts)
	t.Setenv("QYLOCK_TEST_READY", ready)
	t.Setenv("QYLOCK_TEST_RELEASE", release)

	first := exec.Command(script)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		if first.Process != nil {
			_ = first.Process.Kill()
		}
	})
	waitFor(t, ready)

	marker := filepath.Join(run, "qylock.test.locked")
	token, err := os.ReadFile(filepath.Join(run, "qylock.test.expected"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, token, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, script).CombinedOutput(); err != nil {
		t.Fatalf("duplicate launcher did not exit cleanly: %v: %s", err, out)
	}
	b, err := os.ReadFile(starts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(b), "x"); got != 1 {
		t.Fatalf("duplicate launcher started quickshell %d times, want one", got)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("duplicate launcher removed the active locker's secure marker")
	}

	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("active launcher exit: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("active launcher left the marker after exit: %v", err)
	}
}

func TestLockLauncherWinnerClearsStaleMarkerBeforeQuickshell(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "lockscreen", "qylock", "quickshell-lockscreen", "lock.sh"))
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	run := t.TempDir()
	home := t.TempDir()
	observed := filepath.Join(run, "marker-cleared")
	for name, body := range map[string]string{
		"quickshell": `#!/bin/sh
if [ ! -e "$XDG_RUNTIME_DIR/qylock.test.locked" ]; then
  : > "$QYLOCK_TEST_OBSERVED"
fi
`,
		"killall": "#!/bin/sh\nexit 0\n",
		"ryoku":   "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(run, "qylock.test.locked")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_RUNTIME_DIR="+run,
		"XDG_SESSION_TYPE=wayland",
		"XDG_SESSION_ID=test",
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"QYLOCK_TEST_OBSERVED="+observed,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("winning launcher: %v: %s", err, out)
	}
	if _, err := os.Stat(observed); err != nil {
		t.Fatal("winning launcher exposed a stale secure marker to quickshell")
	}
}
