package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRashinDaemonActions(t *testing.T) {
	cases := []struct {
		name                   string
		state                  rashinUnitState
		wantLinger, wantFailed bool
	}{
		{"disabled does nothing", rashinUnitState{enabled: false, linger: false, failed: true}, false, false},
		{"enabled no linger enables boot-start", rashinUnitState{enabled: true, linger: false, failed: false}, true, false},
		{"enabled wedged clears failed", rashinUnitState{enabled: true, linger: true, failed: true}, false, true},
		{"enabled off and wedged does both", rashinUnitState{enabled: true, linger: false, failed: true}, true, true},
		{"enabled and converged does nothing", rashinUnitState{enabled: true, linger: true, failed: false}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotLinger, gotFailed := rashinDaemonActions(c.state)
			if gotLinger != c.wantLinger || gotFailed != c.wantFailed {
				t.Fatalf("rashinDaemonActions(%+v) = (linger=%v, failed=%v), want (linger=%v, failed=%v)",
					c.state, gotLinger, gotFailed, c.wantLinger, c.wantFailed)
			}
		})
	}
}

// rashinOptedOut gates the default-on convergence: a box that ran
// `ryoku-rashin disable` (optedOut in the config) must be left off. A wrong key
// or loose parse would silently re-enable the AI a user turned off.
func TestRashinOptedOut(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg, "ryoku", "rashin.json")
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"absent config is not opted out", "", false},
		{"opted out", `{"enabled":false,"optedOut":true}`, true},
		{"enabled is not opted out", `{"enabled":true}`, false},
		{"absent key is not opted out", `{"enabled":false,"port":3600}`, false},
		{"garbage is not opted out", `not json`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_ = os.Remove(path)
			if c.body != "" {
				if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := rashinOptedOut(); got != c.want {
				t.Fatalf("rashinOptedOut() = %v, want %v", got, c.want)
			}
		})
	}
}

// prowlAgentNeeded is the pure core of reconcileProwlAgent: only an enabled
// rashin box that lacks the binary should be told to install it. Pinned so the
// finding never fires on a box that never opted into rashin.
func TestProwlAgentNeeded(t *testing.T) {
	cases := []struct {
		name                   string
		enabled, present, want bool
	}{
		{"disabled needs nothing", false, false, false},
		{"disabled with prowl needs nothing", false, true, false},
		{"enabled with prowl is fine", true, true, false},
		{"enabled without prowl needs install", true, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := prowlAgentNeeded(c.enabled, c.present); got != c.want {
				t.Fatalf("prowlAgentNeeded(enabled=%v, present=%v) = %v, want %v", c.enabled, c.present, got, c.want)
			}
		})
	}
}

func TestRashinSkillLinksMissing(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("RYOKU_REPO", "") // no dev checkout via sys.ResolveRepo
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	oldPackaged := packagedSkillRoot
	packagedSkillRoot = filepath.Join(t.TempDir(), "absent") // a packaged box ships the tree
	t.Cleanup(func() { packagedSkillRoot = oldPackaged })

	// No skill installed: nothing to wire, so never "missing".
	skills := t.TempDir()
	t.Setenv("RYOKU_RASHIN_SKILLS", skills)
	if rashinSkillLinksMissing() {
		t.Fatal("no skill source: should not report links missing")
	}

	// Install the skill under the override; the links are absent -> missing.
	ryoku := filepath.Join(skills, "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ryoku, "SKILL.md"), []byte("# ryoku\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !rashinSkillLinksMissing() {
		t.Fatal("skill installed but links absent: should report missing")
	}

	// Create the always-created links pointing at the source -> not missing.
	for _, d := range []string{".agents", ".hermes"} {
		linkDir := filepath.Join(h, d, "skills")
		if err := os.MkdirAll(linkDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(ryoku, filepath.Join(linkDir, "ryoku")); err != nil {
			t.Fatal(err)
		}
	}
	if rashinSkillLinksMissing() {
		t.Fatal("links present: should not report missing")
	}
}
