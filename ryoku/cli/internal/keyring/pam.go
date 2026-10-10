package keyring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// The pam_gnome_keyring lines that make unlock-on-login work: the auth module
// captures the login password and unlocks (or creates) the login keyring, the
// session module keeps the daemon started for the session. They go into the
// SDDM stack right after the shared system-login include, the same placement
// Arch's own display managers use.
const (
	pamAuthLine    = "auth optional pam_gnome_keyring.so"
	pamSessionLine = "session optional pam_gnome_keyring.so auto_start"
)

// pamFilePath is the SDDM PAM stack, or the $RYOKU_PAM_FILE fixture override so
// tests and the sandbox never touch the real /etc/pam.d/sddm.
func pamFilePath() string {
	if p := os.Getenv("RYOKU_PAM_FILE"); p != "" {
		return p
	}
	return "/etc/pam.d/sddm"
}

// pamHasKeyring reports whether the stack already carries a pam_gnome_keyring
// module of the given kind ("auth" or "session"), regardless of its exact form,
// so an insert never duplicates and status can read the true state.
func pamHasKeyring(content, kind string) bool {
	for _, l := range strings.Split(content, "\n") {
		f := strings.Fields(l)
		if len(f) > 0 &&
			strings.TrimPrefix(f[0], "-") == kind &&
			strings.Contains(l, "pam_gnome_keyring.so") {
			return true
		}
	}
	return false
}

// pamPresent reports whether the stack wires pam_gnome_keyring at all.
func pamPresent(content string) bool {
	return pamHasKeyring(content, "auth") || pamHasKeyring(content, "session")
}

// insertAfterInclude puts line immediately after the `<kind> include
// system-login` anchor. Returns the new content and whether the anchor existed;
// a missing anchor leaves the content untouched so the caller can report it.
func insertAfterInclude(content, kind, line string) (string, bool) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 3 && f[0] == kind && f[1] == "include" && f[2] == "system-login" {
			out := append([]string{}, lines[:i+1]...)
			out = append(out, line)
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n"), true
		}
	}
	return content, false
}

// stripKeyring removes every pam_gnome_keyring line, preserving the rest of the
// stack byte-for-byte.
func stripKeyring(content string) string {
	lines := strings.Split(content, "\n")
	out := lines[:0]
	for _, l := range lines {
		if strings.Contains(l, "pam_gnome_keyring.so") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// applyPAMText converges the stack for want (unlock-on-login wants the lines
// present, the other modes want them stripped). Idempotent. missing reports any
// insert anchor that was absent so unlock-on-login can warn instead of silently
// half-wiring.
func applyPAMText(content string, want bool) (out string, missing []string) {
	if !want {
		return stripKeyring(content), nil
	}
	out = content
	if !pamHasKeyring(out, "auth") {
		var ok bool
		if out, ok = insertAfterInclude(out, "auth", pamAuthLine); !ok {
			missing = append(missing, "auth include system-login")
		}
	}
	if !pamHasKeyring(out, "session") {
		var ok bool
		if out, ok = insertAfterInclude(out, "session", pamSessionLine); !ok {
			missing = append(missing, "session include system-login")
		}
	}
	return out, missing
}

// applyPAMFile reads the PAM file at path, converges it for mode, and writes it
// back only when the content actually changes. Refuses when the file is not
// writable (the privileged half runs as root; the test path checks first).
func applyPAMFile(path, mode string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf(i18n.T("read %s: %w"), path, err)
	}
	want := mode == ModeUnlockOnLogin
	out, missing := applyPAMText(string(raw), want)
	if len(missing) > 0 {
		return fmt.Errorf(i18n.T("%s has no %s anchor to wire pam_gnome_keyring after"), path, strings.Join(missing, " / "))
	}
	if out == string(raw) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	// tmp+rename in the same directory: a crash mid-write must never leave a
	// truncated PAM stack behind, that locks every login out of the machine.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ryoku-pam-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// pamWritable reports whether the current process can write path in place,
// deciding the test path (direct apply) from the escalation path (pkexec).
func pamWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func runApplyPAM(args []string) error {
	if len(args) != 1 || !validMode(args[0]) {
		return fmt.Errorf(i18n.T("usage: ryoku keyring apply-pam <unlock-on-login|never-ask|ask>"))
	}
	path := pamFilePath()
	if !sys.Exists(path) {
		return fmt.Errorf(i18n.T("%s not present; nothing to wire"), path)
	}
	if !pamWritable(path) {
		return fmt.Errorf(i18n.T("%s is not writable (run via pkexec/root)"), path)
	}
	return applyPAMFile(path, args[0])
}
