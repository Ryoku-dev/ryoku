package securitykey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lockFixture(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	bin := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("USER", "nero")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(bin, "pamu2fcfg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func readLockPAM(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(lockPAMPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSetLockNeedsEnrollment(t *testing.T) {
	lockFixture(t)
	if err := runSet([]string{"lock", "on"}); err == nil {
		t.Fatal("lock enabled without an enrolled key")
	}
	if lockEnabled() {
		t.Fatal("lock PAM file written without an enrolled key")
	}
}

func TestSetLockWritesUserPAMFile(t *testing.T) {
	cfg := lockFixture(t)
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatalf("set lock on: %v", err)
	}
	if want := filepath.Join(cfg, "qylock", "pam", "ryoku-lock-key"); lockPAMPath() != want {
		t.Fatalf("lock PAM path %q, want %q", lockPAMPath(), want)
	}
	text := readLockPAM(t)
	if !strings.Contains(text, "auth sufficient pam_u2f.so") || !strings.Contains(text, "auth        required    pam_deny.so") {
		t.Fatalf("lock PAM stack is not key-or-deny: %q", text)
	}
	st := gatherStatus()
	if !st.Lock || !st.LockSupported || st.LockReason != "" {
		t.Fatalf("status does not report the lock: %#v", st)
	}
	if err := runSet([]string{"lock", "off"}); err != nil {
		t.Fatalf("set lock off: %v", err)
	}
	if lockEnabled() {
		t.Fatal("lock PAM file left behind after off")
	}
}

func TestPolicyChangeRewritesLock(t *testing.T) {
	lockFixture(t)
	t.Setenv("RYOKU_SECURITY_KEY_PAM_ROOT", t.TempDir())
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"pin-verification", "on"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readLockPAM(t), "pinverification=1") {
		t.Fatalf("lock PAM not rewritten with the PIN policy: %q", readLockPAM(t))
	}
	if got := lockLine(gatherStatus()); !strings.HasPrefix(got, "yes (PIN, then touch;") {
		t.Fatalf("lockscreen line %q", got)
	}
}

func TestMFAModeTurnsLockOffAndRefusesIt(t *testing.T) {
	lockFixture(t)
	t.Setenv("RYOKU_SECURITY_KEY_PAM_ROOT", t.TempDir())
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"mode", "mfa"}); err != nil {
		t.Fatal(err)
	}
	if lockEnabled() {
		t.Fatal("mfa mode left a key-only lock stack in place")
	}
	if err := runSet([]string{"lock", "on"}); err == nil {
		t.Fatal("lock enabled in mfa mode")
	}
	st := gatherStatus()
	if st.LockSupported || st.LockReason == "" {
		t.Fatalf("status does not explain the unavailable lock: %#v", st)
	}
}

func TestLockReportsPINFromCredentialFlags(t *testing.T) {
	lockFixture(t)
	if err := writeAuthFile(authFile{creds: []string{"handle,key,es256,+presence+pin"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	st := gatherStatus()
	if st.PinVerification || !st.LockPIN {
		t.Fatalf("a +pin credential must make the lock report a PIN: %#v", st)
	}
}
