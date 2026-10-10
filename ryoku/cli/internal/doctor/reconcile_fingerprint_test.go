package doctor

import "testing"

// The plan is the whole decision: it must stay silent where there is no reader
// (so a readerless desktop never builds an AUR package), report the module as
// present when it is, ask before installing under checkOnly, and only signal an
// install when a reader is present, the module is missing, and it is not a
// check-only run.
func TestPlanFingerprintModule(t *testing.T) {
	cases := []struct {
		name        string
		reader      bool
		moduleOK    bool
		checkOnly   bool
		wantStatus  recStatus
		wantInstall bool
	}{
		{"no reader is a clean no-op", false, false, false, recOK, false},
		{"no reader even in check-only", false, false, true, recOK, false},
		{"reader with the module present is ok", true, true, false, recOK, false},
		{"reader, module missing, check-only reports would-fix", true, false, true, recWouldFix, false},
		{"reader, module missing, apply signals install", true, false, false, recOK, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, install := planFingerprintModule(c.reader, c.moduleOK, c.checkOnly)
			if install != c.wantInstall {
				t.Fatalf("install = %v, want %v", install, c.wantInstall)
			}
			// When the plan defers to the caller for the install, the result is
			// the zero value and its status is not meaningful; only assert status
			// on the branches the plan resolves itself.
			if !install && res.status != c.wantStatus {
				t.Fatalf("status = %v, want %v", res.status, c.wantStatus)
			}
		})
	}
}

func TestReconcileFingerprintNotesUnavailableProvider(t *testing.T) {
	oldReader, oldPath := fingerprintReaderPresent, fingerprintModulePath
	oldAvailable, oldAdvice, oldInstall := fingerprintProviderAvailable, fingerprintInstallAdvice, fingerprintInstall
	fingerprintReaderPresent = func() bool { return true }
	fingerprintModulePath = t.TempDir() + "/missing.so"
	fingerprintProviderAvailable = func() bool { return false }
	fingerprintInstallAdvice = func() string { return "pam-fprint-grosshack is not packaged for this system" }
	fingerprintInstall = func() int {
		t.Fatal("unavailable provider was installed")
		return -1
	}
	t.Cleanup(func() {
		fingerprintReaderPresent, fingerprintModulePath = oldReader, oldPath
		fingerprintProviderAvailable, fingerprintInstallAdvice, fingerprintInstall = oldAvailable, oldAdvice, oldInstall
	})

	got := reconcileFingerprintModule(true)
	if got.status != recNote || got.remedy != "pam-fprint-grosshack is not packaged for this system" {
		t.Fatalf("result = %#v, want unavailable note with host advice", got)
	}
}
