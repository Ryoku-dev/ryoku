package securitykey

import (
	"fmt"
	"strings"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

func parseOnOff(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf(i18n.T("expected on or off, got %q"), s)
	}
}

func requireEnrollment(target string) error {
	if !sys.Has("pamu2fcfg") {
		return fmt.Errorf(i18n.T("pamu2fcfg is not installed; install pam-u2f first"))
	}
	a, err := loadAuthFile()
	if err != nil {
		return err
	}
	if len(a.creds) == 0 {
		return fmt.Errorf(i18n.T("enroll a security key before enabling %s"), target)
	}
	return nil
}

func runSet(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf(i18n.T("usage: ryoku security-key set <sudo|polkit|login|lock> <on|off>|mode <either|mfa>|touch-required <on|off>|pin-verification <on|off>|user-verification <on|off>"))
	}
	subject := args[0]
	if subject == "mode" {
		mode, err := parseMode(args[1])
		if err != nil {
			return err
		}
		p := readPolicy()
		p.Mode = mode
		if err := writePolicy(p); err != nil {
			return err
		}
		if err := rewriteEnabledTargets(); err != nil {
			fmt.Printf(i18n.T("note: policy saved; re-apply enabled PAM targets with sudo (%v)\n"), err)
		}
		if err := rewriteLock(); err != nil {
			fmt.Printf(i18n.T("note: policy saved; could not update the lock screen (%v)\n"), err)
		}
		fmt.Printf(i18n.T("security key mode set to %s\n"), mode)
		return nil
	}
	if subject == "touch-required" || subject == "pin-verification" || subject == "user-verification" {
		on, err := parseOnOff(args[1])
		if err != nil {
			return err
		}
		p := readPolicy()
		if subject == "touch-required" {
			p.TouchRequired = on
		} else if subject == "pin-verification" {
			p.PinVerification = on
		} else {
			p.UserVerification = on
		}
		if err := writePolicy(p); err != nil {
			return err
		}
		if err := rewriteEnabledTargets(); err != nil {
			fmt.Printf(i18n.T("note: policy saved; re-apply enabled PAM targets with sudo (%v)\n"), err)
		}
		if err := rewriteLock(); err != nil {
			fmt.Printf(i18n.T("note: policy saved; could not update the lock screen (%v)\n"), err)
		}
		fmt.Printf(i18n.T("security key %s %s\n"), subject, map[bool]string{true: i18n.T("enabled"), false: i18n.T("disabled")}[on])
		return nil
	}
	target := subject
	on, err := parseOnOff(args[1])
	if err != nil {
		return err
	}
	if target == TargetLock {
		if err := setLock(on); err != nil {
			return err
		}
		fmt.Printf(i18n.T("security key %s for %s\n"), map[bool]string{true: i18n.T("enabled"), false: i18n.T("disabled")}[on], target)
		return nil
	}
	if _, err := targetName(target); err != nil {
		return err
	}
	if on {
		if err := requireEnrollment(target); err != nil {
			return err
		}
	}
	if err := applyPAMHalf(target, on); err != nil {
		return fmt.Errorf(i18n.T("wire PAM: %w"), err)
	}
	fmt.Printf(i18n.T("security key %s for %s\n"), map[bool]string{true: i18n.T("enabled"), false: i18n.T("disabled")}[on], target)
	return nil
}
