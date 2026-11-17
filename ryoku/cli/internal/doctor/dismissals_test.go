package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureDoctor(t *testing.T, args ...string) (string, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = old; file.Close() }()
	runErr := Run(args)
	body, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(body), runErr
}

func dismissalTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("RYOKU_CHANNEL", "")
	oldChecks, oldUID := doctorReconcilers, doctorEffectiveUID
	t.Cleanup(func() { doctorReconcilers, doctorEffectiveUID = oldChecks, oldUID })
	doctorEffectiveUID = func() int { return 1000 }
}

func TestDoctorDismissalLifecycle(t *testing.T) {
	dismissalTestHome(t)
	result := warnRes("root filesystem is not btrfs; snapshots and rollbacks unavailable")
	applied := 0
	doctorReconcilers = func() []reconciler {
		return []reconciler{{"snapper configuration", func(check bool) recResult {
			if !check {
				applied++
			}
			return result
		}}}
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := captureDoctor(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("fake HOME ryoku doctor %s:\n%s", strings.Join(args, " "), out)
		return out
	}
	status := func() string {
		t.Helper()
		var fs []findingJSON
		if err := json.Unmarshal([]byte(run("--json")), &fs); err != nil {
			t.Fatal(err)
		}
		if len(fs) != 1 {
			t.Fatalf("findings = %+v", fs)
		}
		return fs[0].Status
	}
	if got := status(); got != "warn" {
		t.Fatal(got)
	}
	run("--dismiss", "snapper configuration")
	before, err := os.ReadFile(dismissalPath())
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dismissalPath()); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions: %v %v", st, err)
	}
	if got := status(); got != "dismissed" {
		t.Fatal(got)
	}
	if listing := run("--check"); !strings.Contains(listing, "(dismissed)") {
		t.Fatal(listing)
	}
	if listing := run("--dismissed", "--json"); !strings.Contains(listing, "snapper configuration") {
		t.Fatal(listing)
	}
	if plain := run(); strings.Contains(plain, "snapper configuration") || applied != 0 {
		t.Fatalf("dismissed check was reported or applied: %q applied=%d", plain, applied)
	}
	after, err := os.ReadFile(dismissalPath())
	if err != nil || string(before) != string(after) {
		t.Fatalf("diagnostic run changed dismissals: %v", err)
	}
	result.detail = "root filesystem is not btrfs;\n snapshots and rollbacks unavailable"
	if got := status(); got != "dismissed" {
		t.Fatalf("whitespace-only change = %s", got)
	}
	result = warnRes("snapshots now fail on a btrfs root")
	if got := status(); got != "warn" {
		t.Fatalf("changed finding = %s", got)
	}
	run("--restore", "snapper configuration")
	if saved, err := loadDismissals(); err != nil || len(saved) != 0 {
		t.Fatalf("restore left state: %v %v", saved, err)
	}
	if got := status(); got != "warn" {
		t.Fatal(got)
	}
}

func TestDoctorDismissalEligibilityAndChanges(t *testing.T) {
	for _, original := range []recStatus{recWarn, recWouldFix} {
		t.Run(original.label(), func(t *testing.T) {
			base := recResult{status: original, detail: "same detail", remedy: "same remedy"}
			saved := dismissals{"check": dismissalFor(base)}
			other := recWarn
			if original == recWarn {
				other = recWouldFix
			}
			for _, changed := range []recResult{
				{status: recFailed, detail: base.detail, remedy: base.remedy},
				{status: recOK, detail: base.detail, remedy: base.remedy},
				{status: other, detail: base.detail, remedy: base.remedy},
				{status: original, detail: "different detail", remedy: base.remedy},
				{status: original, detail: base.detail, remedy: "different remedy"},
			} {
				if saved.matches("check", changed) {
					t.Fatalf("changed result remained dismissed: %+v", changed)
				}
			}
			checks := 0
			result := runWithDismissals(reconciler{"check", func(check bool) recResult {
				if !check {
					t.Fatal("dismissed check attempted repairs")
				}
				checks++
				return base
			}}, false, saved)
			if result.status != recDismissed || checks != 1 {
				t.Fatalf("dismissed result=%+v checks=%d", result, checks)
			}
		})
	}
	dismissalTestHome(t)
	for _, state := range []recStatus{recOK, recNote, recFixed, recFailed} {
		doctorReconcilers = func() []reconciler {
			return []reconciler{{"check", func(bool) recResult { return recResult{status: state} }}}
		}
		if _, err := captureDoctor(t, "--dismiss", "check"); err == nil {
			t.Fatalf("dismissed %s", state.label())
		}
	}
	for _, args := range [][]string{{"--dismiss"}, {"--restore"}, {"--dismiss", ""}, {"--dismiss", "missing"}, {"--restore", "missing"}, {"--dismissed", "--restore", "check"}, {"--dismiss", "check", "--json"}, {"--dismissed", "--report"}} {
		if _, err := captureDoctor(t, args...); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}

func TestDoctorDismissalDoesNotHideErrors(t *testing.T) {
	dismissalTestHome(t)
	base := warnRes("same detail")
	if err := saveDismissals(dismissals{"check": dismissalFor(base)}); err != nil {
		t.Fatal(err)
	}
	doctorReconcilers = func() []reconciler {
		return []reconciler{{"check", func(bool) recResult { return failRes("same detail") }}}
	}
	if fs := runReconcilers(true); len(fs) != 1 || fs[0].res.status != recFailed {
		t.Fatalf("saved warning hid error: %+v", fs)
	}
	if err := os.WriteFile(dismissalPath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fs := runReconcilers(true); len(fs) != 2 || fs[0].res.status != recFailed || fs[1].res.status != recFailed {
		t.Fatalf("unreadable dismissals hid findings: %+v", fs)
	}
	if _, err := captureDoctor(t, "--dismissed"); err == nil {
		t.Fatal("corrupt dismissal state was silently accepted")
	}
}

func TestDoctorDismissedTodoSkipsRepairs(t *testing.T) {
	dismissalTestHome(t)
	repairs := 0
	doctorReconcilers = func() []reconciler {
		return []reconciler{{"optional repair", func(check bool) recResult {
			if check {
				return wouldRes("a host-specific repair")
			}
			repairs++
			return fixedRes("repaired")
		}}}
	}
	for _, args := range [][]string{{"--dismiss", "optional repair"}, {}, {"--restore", "optional repair"}} {
		if _, err := captureDoctor(t, args...); err != nil {
			t.Fatal(err)
		}
		if repairs != 0 {
			t.Fatal("dismissal action applied a repair")
		}
	}
	if _, err := captureDoctor(t); err != nil || repairs != 1 {
		t.Fatalf("restored repair: repairs=%d err=%v", repairs, err)
	}
}
