package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"
	"strings"
	"testing"
)

// writeRyoportManagedFile creates the file ryossh writes when a host is added.
// Doctor must only repair the include when this managed file exists.
func writeRyoportManagedFile(t *testing.T) {
	t.Helper()
	managed := filepath.Join(sys.Home(), ".ssh", "config.d", "ryoport")
	if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte("Host port-1\n  HostName 100.1.2.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSSHInclude(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		changed bool
	}{
		{name: "adds the include", in: "Host laptop\n  HostName 100.1.2.3\n", changed: true},
		{name: "empty file gets the include", in: "", changed: true},
		{name: "keeps an existing include", in: "Include config.d/*\n\nHost laptop\n"},
		{name: "equals-form include counts", in: "Include=config.d/*\nHost laptop\n"},
		{name: "indented include counts", in: "  Include   config.d/*\nHost laptop\n"},
		{name: "commented include does not count", in: "# Include config.d/*\nHost laptop\n", changed: true},
		{name: "other include does not count", in: "Include ~/.ssh/other\nHost laptop\n", changed: true},
		{name: "include after an inline comment does not count", in: "Include other # config.d/*\nHost laptop\n", changed: true},
		{name: "quoted global include counts", in: "Include \"config.d/*\"\nHost laptop\n"},
		{name: "include inside Host block does not count", in: "Host laptop\n  Include config.d/*\n", changed: true},
		{name: "include inside Match block does not count", in: "Match User nobody\n  Include config.d/*\n", changed: true},
		{name: "quoted include inside Host block does not count", in: "Host laptop\n  Include \"config.d/*\"\n", changed: true},
		{name: "host-specific block keeps its include hidden", in: "Include ~/.ssh/other\nHost laptop\n  Include config.d/*\n", changed: true},
		{name: "Host * include after a host-specific block counts", in: "Host laptop\n  User alice\nHost *\n  Include config.d/*\n"},
		{name: "Match all include after a host-specific block counts", in: "Host laptop\n  User alice\nMatch all\n  Include config.d/*\n"},
		{name: "equals-form Host and Include count", in: "Host=*\n  Include=config.d/*\n"},
		{name: "quoted equals-form include counts", in: "Include=\"config.d/*\"\n"},
		{name: "include with a trailing inline comment counts", in: "Include config.d/* # keep managed hosts\n"},
		{name: "Match User * does not make an include global", in: "Match User *\n  Include config.d/*\n", changed: true},
		{name: "negated Host * pattern is not global", in: "Host * !laptop\n  Include config.d/*\n", changed: true},
		{name: "unindented include after Host stays in that block", in: "Host laptop\nInclude config.d/*\n", changed: true},
		{name: "lowercase Host block stops the scan", in: "host laptop\n  include config.d/*\n", changed: true},
		{name: "HostName is not a Host block", in: "HostName laptop\nInclude config.d/*\n"},
		{name: "MatchAddress is not a Match block", in: "MatchAddress 10.0.0.1\nInclude config.d/*\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, changed, err := ensureSSHInclude([]byte(tt.in))
			if err != nil {
				t.Fatalf("ensureSSHInclude: %v", err)
			}
			if changed != tt.changed {
				t.Fatalf("changed = %v, want %v", changed, tt.changed)
			}
			if !changed {
				if out != nil {
					t.Fatalf("no-op returned output: %q", out)
				}
				return
			}
			want := tt.in
			if len(want) > 0 && !strings.HasSuffix(want, "\n") {
				want += "\n"
			}
			want += "Host *\n  Include config.d/*\n"
			if string(out) != want {
				t.Fatalf("appended include did not preserve the original config and its precedence:\ngot  %q\nwant %q", out, want)
			}
		})
	}
}

func TestReconcileSSHInclude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)

	if got := reconcileSSHInclude(true); got.status != recOK {
		t.Fatalf("missing config: got %+v, want recOK", got)
	}
	original := "Host laptop\n  HostName 100.1.2.3\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := reconcileSSHInclude(true); got.status != recWouldFix {
		t.Fatalf("checkOnly: got %+v, want recWouldFix", got)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != original {
		t.Fatalf("checkOnly changed config: %q, %v", raw, err)
	}
	if got := reconcileSSHInclude(false); got.status != recFixed {
		t.Fatalf("fix: got %+v, want recFixed", got)
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(fixed) != original+"Host *\n  Include config.d/*\n" {
		t.Fatalf("fixed config = %q", fixed)
	}
	if _, err := os.Stat(path + ".ryoku-bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if got := reconcileSSHInclude(false); got.status != recOK {
		t.Fatalf("second run: got %+v, want recOK", got)
	}
}

// TestEnsureSSHIncludePreservesPrecedence guards the OpenSSH first-value-wins
// rule: a sidecar User must not jump ahead of a user's own per-host override.
func TestEnsureSSHIncludePreservesPrecedence(t *testing.T) {
	original := "Host laptop\n  User alice\nHost desk\n  User bob\n"
	out, changed, err := ensureSSHInclude([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("config without an include was left unchanged")
	}
	if got, want := string(out), original+"Host *\n  Include config.d/*\n"; got != want {
		t.Fatalf("fixed config = %q, want %q", got, want)
	}
	if strings.Index(string(out), "Include config.d/*") < strings.Index(string(out), "User alice") {
		t.Fatalf("injected include precedes the existing per-host override: %q", out)
	}
	// A file that does not end in a newline still gets a syntactically separate block.
	out, _, err = ensureSSHInclude([]byte("Host laptop\n  User alice"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "Host laptop\n  User alice\nHost *\n  Include config.d/*\n"; got != want {
		t.Fatalf("no-final-newline config = %q, want %q", got, want)
	}
}

func TestReconcileSSHIncludeConcurrentEditAborts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)
	original := "Host laptop\n  HostName 100.1.2.3\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	const concurrent = "Host laptop\n  User alice\n" // an editor wrote this mid-check
	oldHook := sshIncludeBeforeReread
	t.Cleanup(func() { sshIncludeBeforeReread = oldHook })
	sshIncludeBeforeReread = func() {
		if err := os.WriteFile(path, []byte(concurrent), 0o600); err != nil {
			t.Fatalf("concurrent edit: %v", err)
		}
	}

	got := reconcileSSHInclude(false)
	if got.status != recWarn {
		t.Fatalf("status = %s, want warn (%+v)", got.status.label(), got)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != concurrent {
		t.Fatalf("concurrent edit was overwritten: %q, %v", raw, err)
	}
	if _, err := os.Stat(path + ".ryoku-bak"); err == nil {
		t.Fatal("backup created despite the concurrent edit")
	}
}

func TestReconcileSSHIncludeLateEditAborts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)
	original := "Host laptop\n  HostName 100.1.2.3\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	// The save lands after the backup and staged replacement are written.
	oldHook := sshIncludeBeforeRename
	t.Cleanup(func() { sshIncludeBeforeRename = oldHook })
	sshIncludeBeforeRename = func() {
		if err := os.WriteFile(path, []byte("Host laptop\n  User alice\n"), 0o600); err != nil {
			t.Fatalf("late edit: %v", err)
		}
	}

	if got := reconcileSSHInclude(false); got.status != recWarn {
		t.Fatalf("status = %s, want warn (%+v)", got.status.label(), got)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "Host laptop\n  User alice\n" {
		t.Fatalf("late save was overwritten: %q, %v", raw, err)
	}
	if raw, err := os.ReadFile(path + ".ryoku-bak"); err != nil || string(raw) != original {
		t.Fatalf("backup = %q, want the original bytes: %v", raw, err)
	}
}

func TestReconcileSSHIncludeBackupTempSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)
	original := "Host laptop\n  HostName 100.1.2.3\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(sshDir, "victim")
	const victimContents = "do not clobber me\n"
	if err := os.WriteFile(victim, []byte(victimContents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path+".ryoku-bak.ryoku-tmp"); err != nil {
		t.Fatal(err)
	}

	if got := reconcileSSHInclude(false); got.status != recFixed {
		t.Fatalf("status = %s, want fixed (%+v)", got.status.label(), got)
	}
	if raw, err := os.ReadFile(victim); err != nil || string(raw) != victimContents {
		t.Fatalf("planted backup temp symlink clobbered %s: %q, %v", victim, raw, err)
	}
	if raw, err := os.ReadFile(path + ".ryoku-bak"); err != nil || string(raw) != original {
		t.Fatalf("backup = %q, %v", raw, err)
	}
}

func TestReconcileSSHIncludeTempSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)
	original := "Host laptop\n  HostName 100.1.2.3\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(sshDir, "victim")
	const victimContents = "do not clobber me\n"
	if err := os.WriteFile(victim, []byte(victimContents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path+".ryoku-tmp"); err != nil {
		t.Fatal(err)
	}

	if got := reconcileSSHInclude(false); got.status != recFixed {
		t.Fatalf("status = %s, want fixed (%+v)", got.status.label(), got)
	}
	if raw, err := os.ReadFile(victim); err != nil || string(raw) != victimContents {
		t.Fatalf("planted temp symlink clobbered %s: %q, %v", victim, raw, err)
	}
	if info, err := os.Lstat(path + ".ryoku-tmp"); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("planted temp symlink was touched: %v", err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != original+"Host *\n  Include config.d/*\n" {
		t.Fatalf("fixed config = %q, %v", raw, err)
	}
}

func TestReconcileSSHIncludePreservesMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sshDir := filepath.Join(sys.Home(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "config")
	writeRyoportManagedFile(t)
	if err := os.WriteFile(path, []byte("Host laptop\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if got := reconcileSSHInclude(false); got.status != recFixed {
		t.Fatalf("status = %s, want fixed (%+v)", got.status.label(), got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("config mode = %04o, want 0640", got)
	}
}

func TestReconcileSSHIncludeSymlink(t *testing.T) {
	for _, tt := range []struct {
		name         string
		createTarget bool
	}{
		{name: "dangling symlink"},
		{name: "symlink to a regular file", createTarget: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			sshDir := filepath.Join(sys.Home(), ".ssh")
			if err := os.MkdirAll(sshDir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(sshDir, "config")
			writeRyoportManagedFile(t)
			if tt.createTarget {
				if err := os.WriteFile(filepath.Join(sshDir, "real-config"), []byte("Host laptop\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink("real-config", path); err != nil {
				t.Fatal(err)
			}

			for _, checkOnly := range []bool{true, false} {
				got := reconcileSSHInclude(checkOnly)
				if got.status != recWarn {
					t.Fatalf("checkOnly=%v: status = %s, want warn (%+v)", checkOnly, got.status.label(), got)
				}
				if got.remedy == "" {
					t.Fatalf("checkOnly=%v: symlink finding has no remedy", checkOnly)
				}
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("doctor replaced the symlink")
			}
		})
	}
}

// TestReconcileSSHIncludeManagedFileGate pins the boundary from PR #947: a
// hand-written config from someone who never used ryossh stays untouched.
func TestReconcileSSHIncludeManagedFileGate(t *testing.T) {
	const handWritten = "Host laptop\n  HostName 100.1.2.3\n"
	const withInclude = handWritten + "Host *\n  Include config.d/*\n"
	for _, tt := range []struct {
		name         string
		managed      bool
		emptyConfigD bool
		config       string // empty means there is no ~/.ssh/config
		checkOnly    bool
		want         recStatus
		wantConfig   string
		wantBackup   bool
	}{
		{name: "no managed file, no include, check", config: handWritten, checkOnly: true, want: recOK, wantConfig: handWritten},
		{name: "no managed file, no include, fix", config: handWritten, want: recOK, wantConfig: handWritten},
		{name: "no managed file, no config", checkOnly: true, want: recOK},
		{name: "config.d without managed file", emptyConfigD: true, config: handWritten, want: recOK, wantConfig: handWritten},
		{name: "managed file, no include, check", managed: true, config: handWritten, checkOnly: true, want: recWouldFix, wantConfig: handWritten},
		{name: "managed file, no include, fix", managed: true, config: handWritten, want: recFixed, wantConfig: withInclude, wantBackup: true},
		{name: "managed file, include present", managed: true, config: withInclude, checkOnly: true, want: recOK, wantConfig: withInclude},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			sshDir := filepath.Join(sys.Home(), ".ssh")
			if err := os.MkdirAll(sshDir, 0o700); err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.managed:
				writeRyoportManagedFile(t)
			case tt.emptyConfigD:
				if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(sshDir, "config")
			if tt.config != "" {
				if err := os.WriteFile(path, []byte(tt.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if got := reconcileSSHInclude(tt.checkOnly); got.status != tt.want {
				t.Fatalf("status = %s, want %s (%+v)", got.status.label(), tt.want.label(), got)
			}
			if tt.config == "" {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("doctor created ~/.ssh/config: %v", err)
				}
			} else if raw, err := os.ReadFile(path); err != nil || string(raw) != tt.wantConfig {
				t.Fatalf("config = %q, want %q (%v)", raw, tt.wantConfig, err)
			}
			_, backupErr := os.Lstat(path + ".ryoku-bak")
			if tt.wantBackup && backupErr != nil {
				t.Fatalf("backup missing: %v", backupErr)
			}
			if !tt.wantBackup && !errors.Is(backupErr, os.ErrNotExist) {
				t.Fatalf("doctor created a backup: %v", backupErr)
			}
		})
	}
}
