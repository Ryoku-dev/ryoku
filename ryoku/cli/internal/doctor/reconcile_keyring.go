package doctor

import (
	"strings"

	"ryoku-cli/internal/keyring"

	i18n "ryoku-i18n"
)

// reconcileKeyring watches the GNOME keyring unlock policy for drift. It lets
// the keyring package converge the user side (record the inferred mode, point
// the default keyring at login for unlock-on-login) and drop duplicate
// pam_gnome_keyring lines an older Ryoku setup wrote into the SDDM stack. Any
// other PAM drift or a password-protected keyring only warns, with the exact
// command that resolves it. Idempotent.
func reconcileKeyring(checkOnly bool) recResult {
	d := keyring.Reconcile(checkOnly)
	switch {
	case len(d.Warnings) > 0:
		detail := strings.Join(d.Warnings, "; ")
		if len(d.Fixes) > 0 {
			detail += i18n.Tf(" (converged: %s)", strings.Join(d.Fixes, "; "))
		}
		r := warnRes("%s", detail)
		if d.Remedy != "" {
			r = r.withFix("%s", d.Remedy)
		}
		return r
	case len(d.Fixes) > 0:
		if checkOnly {
			return wouldRes("%s", strings.Join(d.Fixes, "; "))
		}
		return fixedRes("%s", strings.Join(d.Fixes, "; "))
	default:
		return okRes(i18n.T("keyring mode %s is consistent with the system wiring"), d.Mode)
	}
}
