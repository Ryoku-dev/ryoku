package securitykey

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	i18n "ryoku-i18n"
	"strconv"
	"strings"
	"syscall"
)

// The lock screen runs its own PAM conversations as the user, from files it
// owns, so the key half lives under the user's config instead of /etc/pam.d.
// The lock arms a key conversation only while this file exists.
const lockPAMService = "ryoku-lock-key"

func lockPAMPath() string {
	return filepath.Join(actorConfigHome(), "qylock", "pam", lockPAMService)
}

func lockEnabled() bool {
	return validateLockPAM() == nil
}

func lockPAMExists() bool {
	_, err := os.Lstat(lockPAMPath())
	return err == nil
}

func lockOwnerUID() int {
	for _, key := range []string{"SUDO_UID", "PKEXEC_UID"} {
		if uid, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && uid >= 0 {
			return uid
		}
	}
	return os.Geteuid()
}

func validateLockPath(path string, wantDir bool, exactMode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symbolic link", path)
	}
	if wantDir != info.IsDir() || (!wantDir && !info.Mode().IsRegular()) {
		return fmt.Errorf("%s has the wrong file type", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != lockOwnerUID() {
		return fmt.Errorf("%s is not owned by the lock user", path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s is writable by another user", path)
	}
	if exactMode != 0 && info.Mode().Perm() != exactMode {
		return fmt.Errorf("%s has mode %04o, want %04o", path, info.Mode().Perm(), exactMode)
	}
	return nil
}

func lockPAMDir(create bool) (string, error) {
	configHome := filepath.Clean(actorConfigHome())
	if !filepath.IsAbs(configHome) {
		return "", fmt.Errorf("security-key config home must be absolute")
	}
	if create {
		created := false
		if err := os.Mkdir(configHome, 0o700); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return "", err
			}
		} else {
			created = true
		}
		if created {
			if err := os.Chown(configHome, lockOwnerUID(), -1); err != nil {
				return "", err
			}
		}
	}
	if err := validateLockPath(configHome, true, 0); err != nil {
		return "", err
	}
	dir := configHome
	for _, name := range []string{"qylock", "pam"} {
		dir = filepath.Join(dir, name)
		if create {
			created := false
			if err := os.Mkdir(dir, 0o700); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return "", err
				}
			} else {
				created = true
			}
			if created {
				if err := os.Chown(dir, lockOwnerUID(), -1); err != nil {
					return "", err
				}
			}
		}
		if err := validateLockPath(dir, true, 0); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func validateLockPAM() error {
	if reason := lockUnsupportedReason(readPolicy()); reason != "" {
		return errors.New(reason)
	}
	if _, err := lockPAMDir(false); err != nil {
		return err
	}
	path := lockPAMPath()
	if err := validateLockPath(path, false, 0o600); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, pathInfo) {
		return fmt.Errorf("%s changed while it was checked", path)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, []byte(lockPAMText())) {
		return fmt.Errorf("%s does not contain Ryoku's lock PAM policy", path)
	}
	return nil
}

func runCheckLock(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: ryoku security-key check-lock")
	}
	return validateLockPAM()
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

func writeLockPAM() (retErr error) {
	dir, err := lockPAMDir(true)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+lockPAMService+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		if retErr != nil {
			os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Chown(lockOwnerUID(), -1); err != nil {
		return err
	}
	if _, err := tmp.WriteString(lockPAMText()); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, lockPAMPath()); err != nil {
		return err
	}
	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}

func setLock(on bool) error {
	if !on {
		if !lockPAMExists() {
			return nil
		}
		if _, err := lockPAMDir(false); err != nil {
			return err
		}
		return os.Remove(lockPAMPath())
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
	if !lockPAMExists() {
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
