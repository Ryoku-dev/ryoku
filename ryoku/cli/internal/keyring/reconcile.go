package keyring

import (
	"fmt"
	"os"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// Drift is what a convergence pass found: the safe user-side changes it made
// (or would make in check-only), and the things it must not touch on its own --
// the root PAM stack and password-protected keyrings -- each with the exact
// command that fixes it.
type Drift struct {
	Fixes    []string
	Warnings []string
	Remedy   string
	Mode     string
}

// Reconcile converges the user side of the keyring policy and reports the drift
// a reconciler must warn about but never fix silently. Safe, idempotent, and
// scoped to the user's own files: it records the inferred mode when nothing is
// recorded yet, and repoints the default keyring at login for unlock-on-login.
// The one root edit it makes is dropping duplicate pam_gnome_keyring lines an
// older Ryoku setup added to /etc/pam.d/sddm; any other PAM drift, or a keyring
// that needs a password, surfaces as a warning with the command that resolves
// it. checkOnly reports the intended fixes without applying them.
func Reconcile(checkOnly bool) Drift {
	st := gatherStatus()
	d := Drift{Mode: st.Mode}

	// record the inferred mode so the choice is explicit and stable across
	// updates. Once written, later passes see it as configured and no-op.
	if st.ModeSource == "inferred" {
		if checkOnly {
			d.Fixes = append(d.Fixes, fmt.Sprintf(i18n.T("record the inferred mode (%s) in keyring.json"), st.Mode))
		} else if err := writeConfig(st.Mode); err == nil {
			d.Fixes = append(d.Fixes, fmt.Sprintf(i18n.T("recorded the inferred mode (%s) in keyring.json"), st.Mode))
		} else {
			d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("could not record the keyring mode: %v"), err))
		}
	}

	// unlock-on-login needs the default alias pointing at the login keyring, so
	// apps that ask for the default get the PAM-unlocked store. A safe, user-side
	// file write.
	if st.Mode == ModeUnlockOnLogin && defaultKeyringName() != "login" {
		if checkOnly {
			d.Fixes = append(d.Fixes, i18n.T("point the default keyring at login"))
		} else if _, changed, err := pointDefaultAt("login"); err != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("could not point the default keyring at login: %v"), err))
		} else if changed {
			d.Fixes = append(d.Fixes, i18n.T("pointed the default keyring at login"))
		}
	}

	// PAM drift: the stack is a root file, so doctor only heals duplicates Ryoku
	// itself wrote and otherwise hands over the exact privileged command.
	wantPAM := st.Mode == ModeUnlockOnLogin
	switch {
	case wantPAM && !st.PamWired:
		d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("the SDDM PAM stack does not carry both pam_gnome_keyring lines but mode is %s"), st.Mode))
		d.Remedy = fmt.Sprintf("sudo ryoku keyring apply-pam %s", st.Mode)
	case wantPAM && st.PamDuplicates && pamDuplicatesHealable():
		if checkOnly {
			d.Fixes = append(d.Fixes, i18n.T("drop the duplicate pam_gnome_keyring lines from the SDDM PAM stack"))
		} else if err := healPAMDuplicates(st.Mode); err != nil {
			d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("could not drop the duplicate pam_gnome_keyring lines from the SDDM PAM stack: %v"), err))
			d.Remedy = fmt.Sprintf("sudo ryoku keyring apply-pam %s", st.Mode)
		} else {
			d.Fixes = append(d.Fixes, i18n.T("dropped the duplicate pam_gnome_keyring lines from the SDDM PAM stack"))
		}
	case wantPAM && st.PamDuplicates:
		d.Warnings = append(d.Warnings, i18n.T("the SDDM PAM stack carries duplicate pam_gnome_keyring lines"))
		d.Remedy = fmt.Sprintf("sudo ryoku keyring apply-pam %s", st.Mode)
	case !wantPAM && st.PamPresent:
		d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("the SDDM PAM stack still carries pam_gnome_keyring but mode is %s"), st.Mode))
		d.Remedy = fmt.Sprintf("sudo ryoku keyring apply-pam %s", st.Mode)
	}

	// autologin + unlock-on-login cannot work: there is no login password for
	// PAM to reuse under autologin.
	if st.Autologin && st.Mode == ModeUnlockOnLogin {
		d.Warnings = append(d.Warnings, i18n.T("autologin is configured but mode is unlock-on-login, which has no login password to reuse; switch to never-ask"))
		if d.Remedy == "" {
			d.Remedy = "ryoku keyring set never-ask"
		}
	}

	// never-ask needs the default-pointed keyring blank; an encrypted one needs a
	// password to convert, which doctor never has.
	if st.Mode == ModeNeverAsk {
		for _, k := range st.Keyrings {
			if k.Role == "default" && k.Format == fmtEncrypted {
				d.Warnings = append(d.Warnings, fmt.Sprintf(i18n.T("the %q keyring is still password-protected but mode is never-ask; convert or reset it"), k.Name))
				if d.Remedy == "" {
					d.Remedy = "ryoku keyring set never-ask --convert  (or --reset to start fresh)"
				}
			}
		}
	}

	return d
}

func pamDuplicatesHealable() bool {
	raw, err := os.ReadFile(pamFilePath())
	if err != nil {
		return false
	}
	_, ok := pamHealDuplicates(string(raw))
	return ok
}

func healPAMDuplicates(mode string) error {
	path := pamFilePath()
	if os.Getenv("RYOKU_PAM_FILE") != "" && pamWritable(path) {
		return applyPAMFile(path, mode)
	}
	return sys.Sudo(selfExe(), "keyring", "apply-pam", mode)
}
