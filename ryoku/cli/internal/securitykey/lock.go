package securitykey

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
)

// The lock screen runs its own PAM conversations as the user, from files it
// owns, so the key half lives under the user's config instead of /etc/pam.d.
// The lock arms a key conversation only while this file exists.
const lockPAMService = "ryoku-lock-key"

func lockPAMPath() string {
	return filepath.Join(actorConfigHome(), "qylock", "pam", lockPAMService)
}

func lockEnabled() bool {
	_, err := os.Stat(lockPAMPath())
	return err == nil
}

// A key-only conversation runs beside the lock's password one, so it can only
// ever be an alternative to the password, never a second factor.
func lockUnsupportedReason(p policy) string {
	if p.Mode == ModeMFA {
		return i18n.T("not supported in security key + password mode")
	}
	return ""
}

// An enrolled credential carries its own flags (pamu2fcfg -N writes +pin), and
// pam_u2f enforces them whatever the PAM line says.
func lockAsksPIN(p policy, creds []string) bool {
	if p.PinVerification {
		return true
	}
	for _, c := range creds {
		fields := strings.Split(c, ",")
		if strings.Contains(fields[len(fields)-1], "+pin") {
			return true
		}
	}
	return false
}

func lockPAMText() string {
	return "#%PAM-1.0\n" +
		"# Written by `ryoku security-key set lock on`; regenerated when the policy changes.\n" +
		pamLine() + "\n" +
		"auth        required    pam_deny.so\n" +
		"account     required    pam_unix.so\n" +
		"password    required    pam_deny.so\n" +
		"session     required    pam_unix.so\n"
}

func writeLockPAM() error {
	path := lockPAMPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, []byte(lockPAMText()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func setLock(on bool) error {
	if !on {
		if err := os.Remove(lockPAMPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if reason := lockUnsupportedReason(readPolicy()); reason != "" {
		return fmt.Errorf(i18n.T("cannot enable the lock screen: %s"), reason)
	}
	if err := requireEnrollment(TargetLock); err != nil {
		return err
	}
	return writeLockPAM()
}

// rewriteLock keeps an enabled lock in step with the policy. A policy the lock
// cannot honour turns it off rather than leaving a stack that would ignore it.
func rewriteLock() error {
	if !lockEnabled() {
		return nil
	}
	if reason := lockUnsupportedReason(readPolicy()); reason != "" {
		if err := setLock(false); err != nil {
			return err
		}
		fmt.Printf(i18n.T("note: security key turned off for the lock screen (%s)\n"), reason)
		return nil
	}
	return writeLockPAM()
}
