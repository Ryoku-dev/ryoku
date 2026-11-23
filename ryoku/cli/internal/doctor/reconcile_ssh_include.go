package doctor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"
	"strings"

	i18n "ryoku-i18n"
)

// Test seams for the two windows in which another process can save the config:
// before the pre-backup read and after the replacement has been staged.
var (
	sshIncludeBeforeReread = func() {}
	sshIncludeBeforeRename = func() {}
)

// reconcileSSHInclude restores the one line that keeps GUI-added hosts visible.
// ryossh writes it when a host is added, but nothing checks it afterwards; once
// a config rewrite drops it, every managed host disappears from `ryossh list`.
func reconcileSSHInclude(checkOnly bool) recResult {
	path := filepath.Join(sys.Home(), ".ssh", "config")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return okRes(i18n.T("no ~/.ssh/config yet (created when a host is added)"))
	}
	if err != nil {
		return warnRes(i18n.T("cannot stat ~/.ssh/config: %v"), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return warnRes(i18n.T("~/.ssh/config is a symlink; Ryoku leaves it alone")).
			withFix(i18n.T("replace the symlink with a regular file to let doctor reconcile it"))
	}
	if !info.Mode().IsRegular() {
		return warnRes(i18n.T("~/.ssh/config is not a regular file; Ryoku leaves it alone"))
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return okRes(i18n.T("no ~/.ssh/config yet (created when a host is added)"))
	}
	if err != nil {
		return warnRes(i18n.T("cannot read ~/.ssh/config: %v"), err)
	}
	next, changed, err := ensureSSHInclude(raw)
	if err != nil {
		return warnRes(i18n.T("cannot parse ~/.ssh/config: %v"), err)
	}
	if !changed {
		return okRes(i18n.T("~/.ssh/config includes config.d/*"))
	}
	if checkOnly {
		return wouldRes(i18n.T("~/.ssh/config is missing the config.d/* include; GUI-added hosts are hidden")).
			withFix(i18n.T("ryoku doctor restores the include in place"))
	}
	// The bytes we are about to back up and replace must be the bytes we read.
	// An editor or `ryossh add` can land in between; refuse to clobber it.
	sshIncludeBeforeReread()
	current, rereadErr := os.ReadFile(path)
	if rereadErr != nil {
		return warnRes(i18n.T("~/.ssh/config changed while reconciling; left it untouched: %v"), rereadErr)
	}
	if !bytes.Equal(current, raw) {
		return warnRes(i18n.T("~/.ssh/config changed while reconciling; left it untouched")).
			withFix(i18n.T("re-run ryoku doctor or ryoku update to retry"))
	}
	// sys.WriteFileAtomic stages the backup in a private temp file and renames
	// it over the destination, so a planted backup temp symlink is not followed.
	if err := sys.WriteFileAtomic(path+".ryoku-bak", string(raw), info.Mode().Perm()); err != nil {
		return failRes(i18n.T("could not back up %s: %v"), path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.ryoku-*")
	if err != nil {
		return failRes(i18n.T("could not create a temp file beside %s: %v"), path, err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return failRes(i18n.T("could not set permissions on %s: %v"), tmpPath, err)
	}
	if _, err := tmp.Write(next); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return failRes(i18n.T("could not write %s: %v"), tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return failRes(i18n.T("could not close %s: %v"), tmpPath, err)
	}
	// Check again after staging: a save that lands while the backup and temp
	// file are being written must not be replaced by our older bytes.
	sshIncludeBeforeRename()
	current, rereadErr = os.ReadFile(path)
	if rereadErr != nil || !bytes.Equal(current, raw) {
		os.Remove(tmpPath)
		return warnRes(i18n.T("~/.ssh/config changed while reconciling; left it untouched")).
			withFix(i18n.T("re-run ryoku doctor or ryoku update to retry"))
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("restored the config.d/* include in ~/.ssh/config"))
}

// ensureSSHInclude appends the include ryossh needs, preserving every existing
// byte. OpenSSH uses the first value it obtains for a keyword: a prepended
// sidecar `User` would beat a user's existing per-host override. Appending the
// include under `Host *` keeps it global (even when the file ends inside a
// host-specific block) without changing any value already in the config.
//
// A commented line or a different include is not a match. Only an include in a
// global scope counts: `Host`/`Match` open conditional blocks that end at the
// next `Host`/`Match`, while `Host *` and `Match all` apply to every host and
// are effectively global.
func ensureSSHInclude(raw []byte) ([]byte, bool, error) {
	text := string(raw)
	global := true
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		key, attached := sshDirectiveKey(fields[0])
		args := sshActiveArgs(fields[1:], attached)
		if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Match") {
			global = sshGlobalBlock(key, args)
			continue
		}
		if !global || !strings.EqualFold(key, "Include") {
			continue
		}
		for _, value := range args {
			if unquoteSSHValue(value) == "config.d/*" {
				return nil, false, nil
			}
		}
	}
	out := append([]byte(nil), raw...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, []byte("Host *\n  Include config.d/*\n")...), true, nil
}

// sshActiveArgs joins a glued `Key=value` argument with the remaining fields
// and drops an inline comment, which OpenSSH starts at any word beginning
// with '#'. Quoted values are kept intact until unquoteSSHValue.
func sshActiveArgs(fields []string, attached string) []string {
	args := fields
	if attached != "" {
		args = append([]string{attached}, args...)
	}
	for i, arg := range args {
		if strings.HasPrefix(arg, "#") {
			return args[:i]
		}
	}
	return args
}

func unquoteSSHValue(value string) string {
	return strings.Trim(value, "\"'")
}

// sshGlobalBlock reports whether a Host/Match block applies to every host.
// `Host *` and `Match all` are the unconditional forms; a host-specific
// `Match User ...` or `Host laptop` block does not make its include global.
func sshGlobalBlock(key string, args []string) bool {
	// `Host *` is the only unconditional Host form. Negated or extra patterns
	// (`Host * !laptop`) make the block conditional and its include non-global.
	if strings.EqualFold(key, "Host") {
		return len(args) == 1 && unquoteSSHValue(args[0]) == "*"
	}
	return len(args) == 1 && strings.EqualFold(unquoteSSHValue(args[0]), "all")
}

// sshDirectiveKey splits the first token of a directive into its key and any
// value glued on with "=". OpenSSH accepts both "Include config.d/*" and
// "Include=config.d/*"; only the latter needs splitting here.
func sshDirectiveKey(token string) (key, attached string) {
	if i := strings.IndexByte(token, '='); i >= 0 {
		return token[:i], token[i+1:]
	}
	return token, ""
}
