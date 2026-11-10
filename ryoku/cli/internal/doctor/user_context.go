package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
)

var (
	doctorEffectiveUID = os.Geteuid
	doctorLookupUser   = user.Lookup
	doctorCurrentUser  = user.Current
	resolvedDoctorUser *user.User
)

func doctorAccount() (*user.User, error) {
	if resolvedDoctorUser != nil {
		return resolvedDoctorUser, nil
	}
	return doctorCurrentUser()
}

func doctorUser() string {
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return os.Getenv("LOGNAME")
}

// configureDoctorUser keeps read-only sudo runs pointed at the desktop account.
// A repairing doctor must run as that account so files it creates remain owned
// by the user; individual system repairs ask for sudo themselves.
func configureDoctorUser(readOnly bool) error {
	resolvedDoctorUser = nil
	if doctorEffectiveUID() != 0 {
		return nil
	}
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" {
		return errors.New(i18n.T("run `ryoku doctor` from the desktop user account, not a root shell; it asks for sudo when a system repair needs it"))
	}
	account, err := doctorLookupUser(name)
	if err != nil {
		return fmt.Errorf("resolve sudo user %q: %w", name, err)
	}
	if account.HomeDir == "" || account.Uid == "" {
		return fmt.Errorf("sudo user %q has no usable home or uid", name)
	}
	if !readOnly {
		return errors.New(fmt.Sprintf(i18n.T("run `ryoku doctor` without sudo so user repairs stay owned by %s; it asks for sudo when a system repair needs it"), name))
	}
	resolved := *account
	resolvedDoctorUser = &resolved

	rootHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", account.HomeDir)
	_ = os.Setenv("USER", account.Username)
	_ = os.Setenv("LOGNAME", account.Username)
	setDoctorXDGHome("XDG_CONFIG_HOME", rootHome, filepath.Join(account.HomeDir, ".config"))
	setDoctorXDGHome("XDG_CACHE_HOME", rootHome, filepath.Join(account.HomeDir, ".cache"))
	setDoctorXDGHome("XDG_DATA_HOME", rootHome, filepath.Join(account.HomeDir, ".local", "share"))
	setDoctorXDGHome("XDG_STATE_HOME", rootHome, filepath.Join(account.HomeDir, ".local", "state"))
	runtimeDir := filepath.Join("/run/user", account.Uid)
	_ = os.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	_ = os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(runtimeDir, "bus"))
	return nil
}

func setDoctorXDGHome(key, oldHome, fallback string) {
	value := os.Getenv(key)
	if value == "" || pathWithin(value, oldHome) || pathWithin(value, "/root") {
		_ = os.Setenv(key, fallback)
	}
}

func pathWithin(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}
