package doctor

import (
	"os"
	"path/filepath"
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

func TestReconcileKeyringWarnsOnDuplicatePamLines(t *testing.T) {
	kySetup(t, stockSDDM+
		"auth optional pam_gnome_keyring.so\n"+
		"-auth optional pam_gnome_keyring.so\n"+
		"session optional pam_gnome_keyring.so auto_start\n"+
		"-session optional pam_gnome_keyring.so auto_start\n")
	writeKeyringConfig(t, "unlock-on-login")
	r := reconcileKeyring(false)
	if r.status != recWarn {
		t.Fatalf("duplicate PAM wiring must warn, got %s (%s)", r.status.label(), r.detail)
	}
	if r.remedy != "sudo ryoku keyring apply-pam unlock-on-login" {
		t.Fatalf("warning must carry the deduplication command, got %q", r.remedy)
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
