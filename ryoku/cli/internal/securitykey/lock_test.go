package securitykey

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	info, err := os.Stat(lockPAMPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock PAM mode = %04o, want 0600", got)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != lockOwnerUID() {
		t.Fatalf("lock PAM owner is not the lock user")
	}
	for _, dir := range []string{filepath.Join(cfg, "qylock"), filepath.Join(cfg, "qylock", "pam")} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o022 != 0 {
			t.Fatalf("lock PAM directory %s is writable by another user: %04o", dir, info.Mode().Perm())
		}
	}
	if err := runCheckLock(nil); err != nil {
		t.Fatalf("check-lock rejected generated policy: %v", err)
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

func TestLockPAMRejectsUnsafeFiles(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{
			name: "empty",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "altered",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("auth sufficient pam_permit.so\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unreadable",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "group writable",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Chmod(path, 0o620); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(filepath.Dir(path), "attacker-policy")
				if err := os.WriteFile(target, []byte(lockPAMText()), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockFixture(t)
			if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
				t.Fatal(err)
			}
			if err := runSet([]string{"lock", "on"}); err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, lockPAMPath())
			if err := runCheckLock(nil); err == nil {
				t.Fatal("unsafe lock PAM file passed validation")
			}
			if lockEnabled() {
				t.Fatal("unsafe lock PAM file reported enabled")
			}
			if gatherStatus().Lock {
				t.Fatal("unsafe lock PAM file reported by status")
			}
		})
	}
}

func TestLockPAMRejectsWritableParent(t *testing.T) {
	cfg := lockFixture(t)
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(cfg, "qylock", "pam"), 0o770); err != nil {
		t.Fatal(err)
	}
	if err := runCheckLock(nil); err == nil {
		t.Fatal("lock PAM under a group-writable directory passed validation")
	}
}

func TestLockPAMRejectsRelativeConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative-config")
	if err := runCheckLock(nil); err == nil {
		t.Fatal("relative config home passed lock PAM validation")
	}
}

func TestMFAPolicyRejectsStaleLockPAM(t *testing.T) {
	lockFixture(t)
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	p := readPolicy()
	p.Mode = ModeMFA
	if err := writePolicy(p); err != nil {
		t.Fatal(err)
	}
	if err := runCheckLock(nil); err == nil {
		t.Fatal("mfa mode accepted a stale key-only lock policy")
	}
	if lockEnabled() {
		t.Fatal("mfa mode reported a stale key-only lock policy enabled")
	}
}

func TestPolicyChangeRepairsAlteredLockPAM(t *testing.T) {
	lockFixture(t)
	t.Setenv("RYOKU_SECURITY_KEY_PAM_ROOT", t.TempDir())
	if err := writeAuthFile(authFile{creds: []string{"cred-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"lock", "on"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPAMPath(), []byte("auth sufficient pam_permit.so\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runSet([]string{"pin-verification", "on"}); err != nil {
		t.Fatal(err)
	}
	if err := runCheckLock(nil); err != nil {
		t.Fatalf("policy update did not repair lock PAM: %v", err)
	}
}
