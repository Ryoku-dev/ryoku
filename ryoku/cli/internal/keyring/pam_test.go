package keyring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archSDDM is the stock Arch /etc/pam.d/sddm, the byte-for-byte baseline the
// wiring must preserve around its inserts.
const archSDDM = `#%PAM-1.0

auth        include     system-login
-auth       optional    pam_kwallet5.so

account     include     system-login

password    include     system-login

session     optional    pam_keyinit.so          force revoke
session     include     system-login
-session    optional    pam_kwallet5.so         auto_start
`

func TestPamHasKeyringAcceptsPlainAndDashTypes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		kind    string
	}{
		{name: "plain auth", content: "auth optional pam_gnome_keyring.so", kind: "auth"},
		{name: "dash auth", content: "-auth optional pam_gnome_keyring.so", kind: "auth"},
		{name: "plain session", content: "session optional pam_gnome_keyring.so auto_start", kind: "session"},
		{name: "dash session", content: "-session optional pam_gnome_keyring.so auto_start", kind: "session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !pamHasKeyring(tc.content, tc.kind) {
				t.Fatalf("did not recognize %q as %s wiring", tc.content, tc.kind)
			}
		})
	}
}

func TestApplyPAMTextInsert(t *testing.T) {
	out, missing := applyPAMText(archSDDM, true)
	if len(missing) != 0 {
		t.Fatalf("stock stack has both anchors; got missing %v", missing)
	}
	if !pamHasKeyring(out, "auth") || !pamHasKeyring(out, "session") {
		t.Fatalf("both keyring lines must be present after insert:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "auth") && strings.Contains(l, "system-login") {
			if lines[i+1] != pamAuthLine {
				t.Fatalf("auth keyring line must sit right after the auth include, got %q", lines[i+1])
			}
		}
		if strings.HasPrefix(l, "session") && strings.Contains(l, "system-login") {
			if lines[i+1] != pamSessionLine {
				t.Fatalf("session keyring line must sit right after the session include, got %q", lines[i+1])
			}
		}
	}
	// the untouched lines survive verbatim.
	if !strings.Contains(out, "-auth       optional    pam_kwallet5.so") ||
		!strings.Contains(out, "session     optional    pam_keyinit.so          force revoke") {
		t.Fatalf("insert disturbed unrelated lines:\n%s", out)
	}
}

func TestApplyPAMTextIdempotent(t *testing.T) {
	once, _ := applyPAMText(archSDDM, true)
	twice, _ := applyPAMText(once, true)
	if once != twice {
		t.Fatalf("second insert changed the file (duplicate lines):\n%s", twice)
	}
	if strings.Count(twice, "pam_gnome_keyring.so") != 2 {
		t.Fatalf("want exactly two keyring lines, got %d", strings.Count(twice, "pam_gnome_keyring.so"))
	}
}

func TestApplyPAMTextStrip(t *testing.T) {
	wired, _ := applyPAMText(archSDDM, true)
	stripped, _ := applyPAMText(wired, false)
	if pamPresent(stripped) {
		t.Fatalf("strip left keyring lines behind:\n%s", stripped)
	}
	if stripped != archSDDM {
		t.Fatalf("strip did not restore the stock stack byte-for-byte:\n%q", stripped)
	}
	// strip is idempotent on an already-clean stack.
	again, _ := applyPAMText(stripped, false)
	if again != stripped {
		t.Fatalf("strip is not idempotent")
	}
}

func TestApplyPAMTextMissingAnchor(t *testing.T) {
	noAnchors := "#%PAM-1.0\n\naccount     include     system-login\n"
	out, missing := applyPAMText(noAnchors, true)
	if out != noAnchors {
		t.Fatalf("with no anchors the content must be untouched:\n%s", out)
	}
	if len(missing) != 2 {
		t.Fatalf("both anchors should be reported missing, got %v", missing)
	}
}

func TestApplyPAMFileWritesOnlyOnChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sddm")
	if err := os.WriteFile(p, []byte(archSDDM), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyPAMFile(p, ModeUnlockOnLogin); err != nil {
		t.Fatalf("apply: %v", err)
	}
	info, _ := os.Stat(p)
	mtime := info.ModTime()
	// a second converge is a no-op and must not rewrite the file.
	if err := applyPAMFile(p, ModeUnlockOnLogin); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	info2, _ := os.Stat(p)
	if !info2.ModTime().Equal(mtime) {
		t.Fatalf("idempotent apply rewrote the file")
	}
	got, _ := os.ReadFile(p)
	if !pamHasKeyring(string(got), "auth") || !pamHasKeyring(string(got), "session") {
		t.Fatalf("file not wired:\n%s", got)
	}
}

func TestApplyPAMFileMissingAnchorErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sddm")
	os.WriteFile(p, []byte("#%PAM-1.0\naccount include system-login\n"), 0o644)
	if err := applyPAMFile(p, ModeUnlockOnLogin); err == nil {
		t.Fatal("want an error when the insert anchor is missing")
	}
}

func TestRunApplyPAMHonorsFixtureEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sddm")
	os.WriteFile(p, []byte(archSDDM), 0o644)
	t.Setenv("RYOKU_PAM_FILE", p)
	if err := runApplyPAM([]string{ModeUnlockOnLogin}); err != nil {
		t.Fatalf("apply-pam: %v", err)
	}
	if err := runApplyPAM([]string{ModeAsk}); err != nil {
		t.Fatalf("apply-pam strip: %v", err)
	}
	got, _ := os.ReadFile(p)
	if pamPresent(string(got)) {
		t.Fatalf("ask should have stripped the lines:\n%s", got)
	}
}
