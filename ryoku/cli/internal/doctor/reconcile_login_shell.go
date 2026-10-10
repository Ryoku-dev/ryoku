package doctor

// ---- reconciler: the login shell, one source of truth ------------------------
//
// Two places name the login shell. /etc/passwd holds the account shell usermod
// sets, the one a fresh login reads. The neutral store's desktop.env SHELL is a
// derived copy the compositor exports into every process it spawns, so a
// terminal and fastfetch report it. A deliberate change writes both together,
// but once they drift nothing notices: the account can say fish while the
// session still exports zsh, and everything reading $SHELL runs the wrong one.
//
// passwd is the source; the store override only mirrors it. This aligns the
// derived side back onto the account shell and refreshes the running session so
// the fix holds without a logout. It never touches the account shell, and it
// leaves an absent override absent: with no override the session already
// inherits the account shell, which is not drift.

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// accountLoginShell is the shell field of the current user's /etc/passwd row:
// the real login shell, the value usermod writes. Empty when the user or the
// file cannot be read. A var so a test drives it without a real passwd entry.
var accountLoginShell = func() string {
	me, err := user.Current()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 7 && parts[0] == me.Username {
			return parts[6]
		}
	}
	return ""
}

// syncSessionShellEnv publishes SHELL through the host's session environment
// seam, which updates the systemd user manager on Arch and Turnstile's envdir on
// runit. A var so the test never touches the live session managers.
var syncSessionShellEnv = func(path string) {
	_ = host.Default().Service([]string{"env", "SHELL=" + path})
}

// storeShellOverride reads desktop.env's SHELL value and whether such a pair
// exists at all. Absence is not drift: with no override the session inherits the
// account shell, so the two states are told apart by the bool, not by "".
func storeShellOverride(raw string) (string, bool) {
	var o struct {
		Desktop struct {
			Env []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"env"`
		} `json:"desktop"`
	}
	if json.Unmarshal([]byte(raw), &o) != nil {
		return "", false
	}
	for _, e := range o.Desktop.Env {
		if e.Key == "SHELL" {
			return e.Value, true
		}
	}
	return "", false
}

// setStoreShellOverride rewrites the existing SHELL pair in desktop.env to path,
// leaving every other pair and store key intact. It only ever updates a pair
// that is already there; an absent override is deliberately never created.
func setStoreShellOverride(raw, path string) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	desktop, _ := doc["desktop"].(map[string]any)
	env, _ := desktop["env"].([]any)
	for _, item := range env {
		if e, _ := item.(map[string]any); e != nil && e["key"] == "SHELL" {
			e["value"] = path
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// shellName is the bare name a user recognises (fish, zsh) out of a shell path.
func shellName(path string) string {
	if path == "" {
		return path
	}
	return filepath.Base(path)
}

func reconcileLoginShell(checkOnly bool) recResult {
	account := accountLoginShell()
	if account == "" {
		return okRes(i18n.T("could not read the account login shell to check it against the session"))
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	raw := readFileSafe(store)
	override, present := storeShellOverride(raw)
	if !present {
		return okRes(i18n.T("no session shell override; the session inherits the %q account login shell"), shellName(account))
	}
	if override == account {
		return okRes(i18n.T("login shell is %q on both the account and the session override"), shellName(account))
	}
	if checkOnly {
		return wouldRes(i18n.T("the account login shell is %q but the session still exports %q, so anything reading $SHELL runs the wrong one"), shellName(account), shellName(override)).
			withFix(i18n.T("ryoku doctor points the session override back at %q"), shellName(account))
	}
	fixed, err := setStoreShellOverride(raw, account)
	if err != nil {
		return failRes(i18n.T("could not align the session shell override to %q: %v"), shellName(account), err).withFix("ryoku doctor")
	}
	if err := writeStore(store, []byte(fixed)); err != nil {
		return failRes(i18n.T("could not save the session shell override: %v"), err).withFix("ryoku doctor")
	}
	syncSessionShellEnv(account)
	return fixedRes(i18n.T("the session exported %q but the account login shell is %q; pointed the override at %q and refreshed the running session"), shellName(override), shellName(account), shellName(account))
}
