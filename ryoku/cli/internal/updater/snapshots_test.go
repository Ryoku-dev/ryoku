package updater

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// snapshotCount must tell three states apart: a real count, a genuinely empty
// store, and a read it could not perform at all. The last one used to render as
// a bare "0" -- indistinguishable from an empty store, and the opposite meaning
// (issue: "snapshots: 0 when snapshots exist"). It also tries snapper
// unprivileged before a cached-credential sudo, so a machine that granted
// ALLOW_USERS never needs sudo at all.
func TestSnapshotCountDistinguishesUnavailableFromEmpty(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
	header := "number,type,date,description,cleanup\n"
	base := "0,single,,current,\n" // dropped by parseSnapshotRows
	two := "12,pre,2026-08-20 14:03:11,ryoku-update,number\n" +
		"13,post,2026-08-20 14:05:22,ryoku-update,number\n"

	cases := []struct {
		name               string
		snapOut, sudoOut   string
		snapCode, sudoCode int
		wantN              int
		wantKnown          bool
	}{
		{"unprivileged read, two snapshots", header + base + two, "", 0, 1, 2, true},
		{"unprivileged read, empty store", header + base, "", 0, 1, 0, true},
		{"no access, cold sudo -> unavailable not zero", "", "", 1, 1, 0, false},
		{"unprivileged denied, sudo cred cached", "", header + base + two, 1, 0, 2, true},
		// snapper prints "No permissions." to stderr and STILL EXITS 0, so an empty
		// stdout with a zero exit must not read as an empty store.
		{"snapper denies but exits 0 -> unavailable not zero", "", "", 0, 1, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bin := t.TempDir()
			writeExec(t, filepath.Join(bin, "snapper"),
				"#!/bin/sh\nprintf '%s' '"+c.snapOut+"'\nexit "+strconv.Itoa(c.snapCode)+"\n")
			writeExec(t, filepath.Join(bin, "sudo"),
				"#!/bin/sh\nprintf '%s' '"+c.sudoOut+"'\nexit "+strconv.Itoa(c.sudoCode)+"\n")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

			n, known := snapshotCount()
			if n != c.wantN || known != c.wantKnown {
				t.Fatalf("snapshotCount() = (%d, %v), want (%d, %v)", n, known, c.wantN, c.wantKnown)
			}
		})
	}
}
func TestUnsupportedHostNeverRunsSnapshotCommands(t *testing.T) {
	repo, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_REPO", repo)
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	bin := t.TempDir()
	marker := filepath.Join(bin, "called")
	body := "#!/bin/sh\nprintf called >> " + marker + "\n"
	writeExec(t, filepath.Join(bin, "snapper"), body)
	writeExec(t, filepath.Join(bin, "sudo"), body)
	writeExec(t, filepath.Join(bin, "findmnt"), body)
	writeExec(t, filepath.Join(bin, "pacman"), body)
	writeExec(t, filepath.Join(bin, "systemctl"), body)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, reason := snapshotCapability()
	oldStderr := os.Stderr
	stderrRead, stderrWrite, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	os.Stderr = stderrWrite
	got := snapperPre("test update")
	_ = stderrWrite.Close()
	os.Stderr = oldStderr
	warning, readErr := io.ReadAll(stderrRead)
	_ = stderrRead.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got != "" {
		t.Fatalf("snapperPre = %q, want empty", got)
	}
	if string(warning) != "warning: "+reason+"\n" {
		t.Fatalf("snapperPre warning = %q, want reason once", warning)
	}
	snapperPost("42", "test update")
	offerSnapperHelpers()
	if n, known := snapshotCount(); n != 0 || known {
		t.Fatalf("snapshotCount = (%d, %v), want (0, false)", n, known)
	}
	if err := Snapshots(); err == nil || err.Error() != reason {
		t.Fatalf("Snapshots error = %v, want %q", err, reason)
	}
	if err := restoreGuide("42"); err == nil || err.Error() != reason {
		t.Fatalf("restoreGuide error = %v, want %q", err, reason)
	}
	if err := Rollback(nil); err == nil || err.Error() != reason {
		t.Fatalf("Rollback error = %v, want %q", err, reason)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unsupported snapshot path invoked a command (stat err %v)", err)
	}
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestParseSnapshotRows(t *testing.T) {
	out := "number,type,date,description,cleanup\n" +
		"0,single,,current,\n" + // base row, dropped
		"12,pre,2026-08-20 14:03:11,ryoku-update (from a1b2c3d),number\n" +
		"13,post,2026-08-20 14:05:22,ryoku-update,number\n"
	got := parseSnapshotRows(out)
	want := []snapshotRow{
		{number: "12", kind: "pre", date: "2026-08-20 14:03:11", description: "ryoku-update (from a1b2c3d)", cleanup: "number"},
		{number: "13", kind: "post", date: "2026-08-20 14:05:22", description: "ryoku-update", cleanup: "number"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSnapshotRows =\n%#v\nwant\n%#v", got, want)
	}
}

func TestParseSnapshotRowsQuotedDescription(t *testing.T) {
	// snapper quotes a description that contains a comma; the CSV parse must
	// keep it one field, not split it into a bogus extra column.
	out := "number,type,date,description,cleanup\n5,single,2026-01-01 00:00:00,\"hand, made\",\n"
	got := parseSnapshotRows(out)
	if len(got) != 1 || got[0].description != "hand, made" {
		t.Fatalf("quoted description not parsed: %#v", got)
	}
}
