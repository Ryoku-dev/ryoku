package securitykey

import (
	"fmt"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

const (
	TargetSudo   = "sudo"
	TargetPolkit = "polkit"
	TargetLogin  = "login"
	TargetLock   = "lock"
)

func authFilePath() string {
	if p := os.Getenv("RYOKU_SECURITY_KEY_AUTHFILE"); p != "" {
		return p
	}
	return filepath.Join(sys.ConfigHome(), "Yubico", "u2f_keys")
}

func fakeFIDO() bool {
	v := os.Getenv("RYOKU_FAKE_FIDO")
	return v == "1" || v == "true" || v == "on" || v == "yes"
}

func Run(args []string) error {
	if len(args) == 0 {
		return usageErr()
	}
	switch args[0] {
	case "status":
		return runStatus(args[1:])
	case "enroll":
		return runEnroll(args[1:])
	case "remove":
		return runRemove(args[1:])
	case "set":
		return runSet(args[1:])
	case "apply-pam":
		return runApplyPAM(args[1:])
	case "-h", "--help", "help":
		fmt.Print(usageText())
		return nil
	default:
		return fmt.Errorf(i18n.T("unknown security-key command %q\n\n%s"), args[0], usageText())
	}
}

func usageText() string {
	return i18n.T("Usage: ryoku security-key <command>\n\n  status [--json]                        show security-key status and wiring\n  enroll                                 enroll the inserted FIDO2/U2F key\n  remove <id|all>                        remove one enrolled key (1-based) or all\n  set <sudo|polkit|login> <on|off>       enable or disable pam_u2f for that target\n  set lock <on|off>                      enable or disable the key on the lock screen\n  set mode <either|mfa>                  security key or password vs key + password\n  set touch-required <on|off>            require touching the security key\n  set pin-verification <on|off>          require the authenticator PIN when supported\n  set user-verification <on|off>         require built-in user verification when supported\n  apply-pam <sudo|polkit|login> <on|off> privileged: edit /etc/pam.d for that target\n")
}

func usageErr() error { return fmt.Errorf("%s", usageText()) }
