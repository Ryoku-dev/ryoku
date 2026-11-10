package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kySetup isolates every path the keyring reconciler reads: config, keyrings,
// PAM fixture, and SDDM config root. No real HOME, no /etc, no daemon.
func kySetup(t *testing.T, pamBody string) (data, pam string) {
	t.Helper()
	data = t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("RYOKU_SDDM_CONF_ROOT", t.TempDir())
	os.MkdirAll(filepath.Join(data, "keyrings"), 0o700)
	pam = filepath.Join(t.TempDir(), "sddm")
	if err := os.WriteFile(pam, []byte(pamBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PAM_FILE", pam)
	return data, pam
}

const stockSDDM = "#%PAM-1.0\n\nauth        include     system-login\n\naccount     include     system-login\n\nsession     include     system-login\n"

// A bare box (no config, no PAM lines) infers ask: the reconciler records that
// mode, then is a stable no-op.
func TestReconcileKeyringSeedsInferredAsk(t *testing.T) {
	kySetup(t, stockSDDM)
	if r := reconcileKeyring(true); r.status != recWouldFix {
		t.Fatalf("check-only on a fresh box: status=%s detail=%q, want todo", r.status.label(), r.detail)
	}
	if r := reconcileKeyring(false); r.status != recFixed {
		t.Fatalf("fix on a fresh box: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if r := reconcileKeyring(false); r.status != recOK {
		t.Fatalf("idempotent re-run: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}

// PAM lines present but no recorded mode infers unlock-on-login; the reconciler
// records it and points the default keyring at login, then holds ok.
func TestReconcileKeyringPointsDefaultForUnlockOnLogin(t *testing.T) {
	data, _ := kySetup(t, stockSDDM+"auth optional pam_gnome_keyring.so\nsession optional pam_gnome_keyring.so auto_start\n")
	// a stale pointer at another keyring must be repointed at login.
	os.WriteFile(filepath.Join(data, "keyrings", "default"), []byte("Default_Keyring\n"), 0o600)
	if r := reconcileKeyring(false); r.status != recFixed {
		t.Fatalf("first pass: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	def, _ := os.ReadFile(filepath.Join(data, "keyrings", "default"))
	if string(def) != "login\n" {
		t.Fatalf("default keyring must be pointed at login, got %q", def)
	}
	if r := reconcileKeyring(false); r.status != recOK {
		t.Fatalf("idempotent: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}

// A recorded mode that disagrees with the root PAM stack is a warning with the
// privileged fix command; the reconciler never edits the stack itself.
func TestReconcileKeyringWarnsOnPamDrift(t *testing.T) {
	_, pam := kySetup(t, stockSDDM)
	// record unlock-on-login but leave the stack without the lines.
	writeKeyringConfig(t, "unlock-on-login")
	r := reconcileKeyring(false)
	if r.status != recWarn {
		t.Fatalf("PAM drift must warn, got %s (%s)", r.status.label(), r.detail)
	}
	if r.remedy != "sudo ryoku keyring apply-pam unlock-on-login" {
		t.Fatalf("warning must carry the apply-pam fix, got %q", r.remedy)
	}
	// the reconciler must not have touched the root file.
	got, _ := os.ReadFile(pam)
	if string(got) != stockSDDM {
		t.Fatalf("reconciler edited the PAM stack; it must only warn:\n%s", got)
	}
}

// stock Arch /etc/pam.d/sddm, which already wires the keyring module
const archStockSDDM = `#%PAM-1.0

auth        include     system-login
-auth       optional    pam_gnome_keyring.so
-auth       optional    pam_kwallet5.so

account     include     system-login

password    include     system-login
-password   optional    pam_gnome_keyring.so    use_authtok

session     optional    pam_keyinit.so          force revoke
session     include     system-login
-session    optional    pam_gnome_keyring.so    auto_start
-session    optional    pam_kwallet5.so         auto_start
`

// what the 1.0.6 ISO's sddm setup left behind on top of the stock stack
const setup106SDDM = `#%PAM-1.0

auth        include     system-login
auth optional pam_gnome_keyring.so
-auth       optional    pam_gnome_keyring.so
-auth       optional    pam_kwallet5.so

account     include     system-login

password    include     system-login
-password   optional    pam_gnome_keyring.so    use_authtok

session     optional    pam_keyinit.so          force revoke
session     include     system-login
session optional pam_gnome_keyring.so auto_start
-session    optional    pam_gnome_keyring.so    auto_start
-session    optional    pam_kwallet5.so         auto_start
`

func readPAM(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReconcileKeyringHealsSetupDuplicatesAfterUpgrade(t *testing.T) {
	_, pam := kySetup(t, setup106SDDM)
	writeKeyringConfig(t, "unlock-on-login")
	if r := reconcileKeyring(true); r.status != recWouldFix {
		t.Fatalf("check-only must plan the heal, got %s (%s)", r.status.label(), r.detail)
	}
	if got := readPAM(t, pam); got != setup106SDDM {
		t.Fatalf("check-only rewrote the PAM stack:\n%s", got)
	}
	if r := reconcileKeyring(false); r.status != recFixed {
		t.Fatalf("duplicates Ryoku wrote must heal, got %s (%s)", r.status.label(), r.detail)
	}
	if got := readPAM(t, pam); got != archStockSDDM {
		t.Fatalf("healed stack is not the stock stack:\n%s", got)
	}
	if r := reconcileKeyring(false); r.status != recOK {
		t.Fatalf("healed box must be clean, got %s (%s)", r.status.label(), r.detail)
	}
}

func TestReconcileKeyringKeepsCleanStockStack(t *testing.T) {
	_, pam := kySetup(t, archStockSDDM)
	writeKeyringConfig(t, "unlock-on-login")
	reconcileKeyring(false)
	if r := reconcileKeyring(false); r.status != recOK {
		t.Fatalf("stock stack must be consistent, got %s (%s)", r.status.label(), r.detail)
	}
	if got := readPAM(t, pam); got != archStockSDDM {
		t.Fatalf("stock stack was rewritten:\n%s", got)
	}
}

func TestReconcileKeyringLeavesHandEditedDuplicates(t *testing.T) {
	for name, body := range map[string]string{
		"own arguments": strings.Replace(setup106SDDM,
			"auth optional pam_gnome_keyring.so\n",
			"auth optional pam_gnome_keyring.so try_first_pass\n", 1),
		"unreadable line": setup106SDDM + "auth\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, pam := kySetup(t, body)
			writeKeyringConfig(t, "unlock-on-login")
			r := reconcileKeyring(false)
			if r.status != recWarn {
				t.Fatalf("a stack doctor did not write must only warn, got %s (%s)", r.status.label(), r.detail)
			}
			if r.remedy != "sudo ryoku keyring apply-pam unlock-on-login" {
				t.Fatalf("warning must carry the deduplication command, got %q", r.remedy)
			}
			if got := readPAM(t, pam); got != body {
				t.Fatalf("hand-edited stack was rewritten:\n%s", got)
			}
		})
	}
}

func TestReconcileKeyringWarnsOnPartialPamWiring(t *testing.T) {
	kySetup(t, stockSDDM+"-auth optional pam_gnome_keyring.so\n")
	writeKeyringConfig(t, "unlock-on-login")
	r := reconcileKeyring(false)
	if r.status != recWarn {
		t.Fatalf("partial PAM wiring must warn, got %s (%s)", r.status.label(), r.detail)
	}
}

// autologin + unlock-on-login is a conflict the reconciler flags.
func TestReconcileKeyringWarnsAutologinConflict(t *testing.T) {
	kySetup(t, stockSDDM+"auth optional pam_gnome_keyring.so\nsession optional pam_gnome_keyring.so auto_start\n")
	confd := filepath.Join(os.Getenv("RYOKU_SDDM_CONF_ROOT"), "sddm.conf.d")
	os.MkdirAll(confd, 0o755)
	os.WriteFile(filepath.Join(confd, "autologin.conf"), []byte("[Autologin]\nUser=nero\n"), 0o644)
	writeKeyringConfig(t, "unlock-on-login")
	r := reconcileKeyring(false)
	if r.status != recWarn {
		t.Fatalf("autologin + unlock-on-login must warn, got %s (%s)", r.status.label(), r.detail)
	}
}

func writeKeyringConfig(t *testing.T, mode string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryoku")
	os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "keyring.json"), []byte(`{"mode":"`+mode+`"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
