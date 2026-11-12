package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprintAuthStatusAndSet(t *testing.T) {
	dir := t.TempDir()
	pam := filepath.Join(dir, "pam.d")
	if err := os.MkdirAll(pam, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, target := range fingerprintPAMTargets {
		body := "# " + target + "\nauth required pam_unix.so\n"
		if err := os.WriteFile(filepath.Join(pam, target), []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	module := filepath.Join(dir, "pam_fprintd_grosshack.so")
	if err := os.WriteFile(module, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	app, stdout, _ := testApp(&fakeRunner{}, nil)
	app.cfg.PAMDir = pam
	app.cfg.PAMModulePaths = []string{module}
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Auth([]string{"fingerprint", "set", "sudo", "on"}); code != ExitOK {
		t.Fatalf("enable exit = %d", code)
	}
	body, err := os.ReadFile(filepath.Join(pam, "sudo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), fingerprintPAMLine+"\n") {
		t.Fatalf("enabled PAM = %q", body)
	}
	backup, err := os.ReadFile(filepath.Join(pam, "sudo") + ".ryoku-fp-bak")
	if err != nil || strings.Contains(string(backup), "pam_fprintd_grosshack") {
		t.Fatalf("backup = %q, err = %v", backup, err)
	}

	stdout.Reset()
	if code := app.Auth([]string{"fingerprint", "status"}); code != ExitOK {
		t.Fatalf("status exit = %d", code)
	}
	var status fingerprintStatus
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Supported || !status.Targets["sudo"] || status.Targets["sddm"] {
		t.Fatalf("status = %#v", status)
	}

	stdout.Reset()
	if code := app.Auth([]string{"fingerprint", "set", "sudo", "off"}); code != ExitOK {
		t.Fatalf("disable exit = %d", code)
	}
	body, _ = os.ReadFile(filepath.Join(pam, "sudo"))
	if strings.Contains(string(body), "pam_fprintd_grosshack") {
		t.Fatalf("disabled PAM = %q", body)
	}
}

func TestFingerprintAuthSetEscalatesWholeOperation(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, nil)
	app.cfg.UID, app.cfg.UIDSet = 1000, true
	if code := app.Auth([]string{"fingerprint", "set", "polkit-1", "on"}); code != ExitOK {
		t.Fatalf("set exit = %d", code)
	}
	if got := argv(runner.commands[0]); got != "pkexec ryoku-host auth fingerprint set polkit-1 on" {
		t.Fatalf("escalation = %q", got)
	}
}

func TestFingerprintAuthStatusReportsMissingIntegration(t *testing.T) {
	app, stdout, _ := testApp(&fakeRunner{}, nil)
	app.cfg.PAMDir = t.TempDir()
	app.cfg.PAMModulePaths = []string{filepath.Join(t.TempDir(), "missing.so")}
	if code := app.Auth([]string{"fingerprint", "status"}); code != ExitOK {
		t.Fatalf("status exit = %d", code)
	}
	var status fingerprintStatus
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Supported || status.Reason == "" {
		t.Fatalf("status = %#v", status)
	}
}

// polkit 128 ships its stack only in the vendor directory (/usr/lib/pam.d),
// which PAM reads when /etc has no file of that name. Turning fingerprint on
// seeds /etc from it; turning it off hands the service back to the vendor file.
func TestFingerprintAuthSeedsFromVendorPAM(t *testing.T) {
	dir := t.TempDir()
	etc, vendor := filepath.Join(dir, "etc"), filepath.Join(dir, "vendor")
	for _, d := range []string{etc, vendor} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stack := "#%PAM-1.0\nauth include system-auth\naccount include system-auth\n"
	if err := os.WriteFile(filepath.Join(vendor, "polkit-1"), []byte(stack), 0o644); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(dir, "pam_fprintd_grosshack.so")
	if err := os.WriteFile(module, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	app, stdout, _ := testApp(&fakeRunner{}, nil)
	app.cfg.PAMDir, app.cfg.VendorPAMDir = etc, vendor
	app.cfg.PAMModulePaths = []string{module}
	app.cfg.UID, app.cfg.UIDSet = 0, true
	seeded := filepath.Join(etc, "polkit-1")

	if code := app.Auth([]string{"fingerprint", "set", "polkit-1", "off"}); code != ExitOK {
		t.Fatalf("disable with only the vendor stack: exit = %d", code)
	}
	if _, err := os.Stat(seeded); !os.IsNotExist(err) {
		t.Fatalf("disabling created %s (err = %v)", seeded, err)
	}

	if code := app.Auth([]string{"fingerprint", "set", "polkit-1", "on"}); code != ExitOK {
		t.Fatalf("enable exit = %d", code)
	}
	body, err := os.ReadFile(seeded)
	if err != nil || string(body) != fingerprintPAMLine+"\n"+stack {
		t.Fatalf("seeded PAM = %q, err = %v", body, err)
	}
	if info, err := os.Stat(seeded); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("seeded mode = %v, err = %v", info.Mode().Perm(), err)
	}
	stdout.Reset()
	if code := app.Auth([]string{"fingerprint", "status"}); code != ExitOK {
		t.Fatalf("status exit = %d", code)
	}
	var status fingerprintStatus
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || !status.Targets["polkit-1"] {
		t.Fatalf("status = %#v, err = %v", status, err)
	}

	if code := app.Auth([]string{"fingerprint", "set", "polkit-1", "off"}); code != ExitOK {
		t.Fatalf("disable exit = %d", code)
	}
	if _, err := os.Stat(seeded); !os.IsNotExist(err) {
		t.Fatalf("the /etc copy should be gone once it matches the vendor stack (err = %v)", err)
	}
	if v, _ := os.ReadFile(filepath.Join(vendor, "polkit-1")); string(v) != stack {
		t.Fatalf("vendor stack changed: %q", v)
	}
}

// An /etc copy carrying local edits is kept when fingerprint is turned off.
func TestFingerprintAuthKeepsAnEditedPAMCopy(t *testing.T) {
	dir := t.TempDir()
	etc, vendor := filepath.Join(dir, "etc"), filepath.Join(dir, "vendor")
	for _, d := range []string{etc, vendor} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stack := "#%PAM-1.0\nauth include system-auth\n"
	edited := fingerprintPAMLine + "\n" + stack + "session optional pam_local.so\n"
	if err := os.WriteFile(filepath.Join(vendor, "polkit-1"), []byte(stack), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(etc, "polkit-1"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(dir, "pam_fprintd_grosshack.so")
	if err := os.WriteFile(module, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp(&fakeRunner{}, nil)
	app.cfg.PAMDir, app.cfg.VendorPAMDir = etc, vendor
	app.cfg.PAMModulePaths = []string{module}
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Auth([]string{"fingerprint", "set", "polkit-1", "off"}); code != ExitOK {
		t.Fatalf("disable exit = %d", code)
	}
	body, err := os.ReadFile(filepath.Join(etc, "polkit-1"))
	if err != nil || string(body) != stack+"session optional pam_local.so\n" {
		t.Fatalf("edited copy = %q, err = %v", body, err)
	}
}
