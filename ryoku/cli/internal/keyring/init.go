package keyring

import (
	"fmt"

	i18n "ryoku-i18n"
)

// runInit is the first-login keyring default, run by each compositor's
// autostart (`ryoku keyring init`) every login. Its job is that no app ever
// prompts for a keyring password out of the box: it records the mode for a user
// who has never chosen one and seeds the blank, passwordless default keyring
// that never-ask needs. Idempotent and user-side only (it never edits the root
// PAM stack).
//
//   - Already configured -> no-op, so a user who picked a mode in Ryoku Settings
//     keeps it.
//   - PAM already wired (unlock-on-login) -> record that and touch nothing.
//   - Otherwise never-ask: seed a blank default keyring so nothing prompts. This
//     is non-destructive -- a pre-existing password-protected keyring is left
//     intact and only its policy is recorded (the user converts or resets it
//     explicitly), and a transient "secret service not up yet" failure is left
//     unrecorded so the next login retries instead of locking in a half-done
//     state.
func runInit(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf(i18n.T("usage: ryoku keyring init"))
	}
	if mode, ok := readConfig(); ok {
		fmt.Printf(i18n.T("keyring: already configured (%s); leaving it\n"), mode)
		return nil
	}

	st := gatherStatus()
	if st.Mode != ModeNeverAsk {
		// inferred unlock-on-login (PAM wired): the user chose a secured keyring;
		// record it and leave the files alone.
		if err := writeConfig(st.Mode); err != nil {
			return fmt.Errorf(i18n.T("record keyring mode: %w"), err)
		}
		fmt.Printf(i18n.T("keyring: defaulted to %s\n"), st.Mode)
		return nil
	}

	name := defaultKeyringName()
	switch probeFormat(keyringFile(name)) {
	case fmtEncrypted:
		// a secured keyring already exists: never destroy it silently. Record the
		// policy so status/doctor stop inferring and point at the fix.
		if err := writeConfig(ModeNeverAsk); err != nil {
			return fmt.Errorf(i18n.T("record keyring mode: %w"), err)
		}
		fmt.Printf(i18n.T("keyring: the %q keyring is password-protected; leaving it intact\n"), name)
		fmt.Println(i18n.T("keyring: run 'ryoku keyring set never-ask --reset' (or use Ryoku Settings) to blank it and stop the prompts"))
		return nil
	default:
		// absent or already blank: make sure a blank passwordless keyring is the
		// default, so no app prompts. Only record once that actually took, so a
		// daemon-not-ready race just retries next login.
		if err := setNeverAsk(setOpts{mode: ModeNeverAsk}); err != nil {
			fmt.Printf(i18n.T("keyring: secret service not ready yet (%v); will set up on next login\n"), err)
			return nil
		}
		if err := writeConfig(ModeNeverAsk); err != nil {
			return fmt.Errorf(i18n.T("record keyring mode: %w"), err)
		}
		fmt.Println(i18n.T("keyring: defaulted to never-ask (blank keyring, no prompts)"))
		return nil
	}
}
