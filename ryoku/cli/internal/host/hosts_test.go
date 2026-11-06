package host

import (
	"bytes"
	"os"
	"testing"
)

func TestAllowedLabAddress(t *testing.T) {
	t.Parallel()
	accepted := []string{"10.10.11.5", "172.31.255.1", "192.168.4.2", "100.64.0.1", "fd00::1"}
	for _, value := range accepted {
		entries := []labHostEntry{}
		if err := addManagedHost(&entries, value, []string{"box.htb"}, []string{"htb"}); err != nil {
			t.Errorf("addManagedHost(%q): %v", value, err)
		}
	}
	rejected := []string{"8.8.8.8", "172.15.0.1", "100.128.0.1", "fe80::1", "not-an-ip"}
	for _, value := range rejected {
		entries := []labHostEntry{}
		if err := addManagedHost(&entries, value, []string{"box.htb"}, []string{"htb"}); err == nil {
			t.Errorf("addManagedHost(%q) succeeded", value)
		}
	}
}

func TestValidateHostNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"BOX.htb", "box.example", "-box.htb", "box..htb", "box.htb."} {
		if err := validateHostNames([]string{name}, []string{"htb"}); err == nil {
			t.Errorf("validateHostNames(%q) succeeded", name)
		}
	}
	if err := validateHostNames([]string{"box.htb", "web.thm"}, []string{"htb", "thm"}); err != nil {
		t.Fatalf("valid names rejected: %v", err)
	}
}

func TestRewriteManagedHostsPreservesUnmanagedBytesAndModeContent(t *testing.T) {
	t.Parallel()
	original := []byte("127.0.0.1 localhost\n# user comment\n" + hostsBlockStart + "\n10.0.0.2\told.htb\n" + hostsBlockEnd + "\n# final\tline\n")
	entries := []labHostEntry{{IP: "10.10.11.5", Names: []string{"box.htb", "web.htb"}}}
	got, err := rewriteManagedHosts(original, entries)
	if err != nil {
		t.Fatal(err)
	}
	before, _, after, found, err := splitHostsBlock(got)
	if err != nil || !found {
		t.Fatalf("split rewritten block: found=%v err=%v", found, err)
	}
	wantBefore := []byte("127.0.0.1 localhost\n# user comment\n")
	wantAfter := []byte("# final\tline\n")
	if !bytes.Equal(before, wantBefore) || !bytes.Equal(after, wantAfter) {
		t.Fatalf("unmanaged bytes changed: before=%q after=%q", before, after)
	}
	parsed, err := parseManagedHosts(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].IP != "10.10.11.5" || len(parsed[0].Names) != 2 {
		t.Fatalf("unexpected entries: %#v", parsed)
	}
}

func TestRewriteManagedHostsAppendsBlockWithoutChangingExistingData(t *testing.T) {
	t.Parallel()
	original := []byte("127.0.0.1 localhost")
	got, err := rewriteManagedHosts(original, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("127.0.0.1 localhost\n")) {
		t.Fatalf("existing hosts data changed: %q", got)
	}
}

func TestManagedHostsRejectsBrokenMarkers(t *testing.T) {
	t.Parallel()
	data := []byte(hostsBlockStart + "\n10.0.0.2 box.htb\n")
	if _, err := parseManagedHosts(data); err == nil {
		t.Fatal("broken markers accepted")
	}
}

func TestWriteHostsAtomicKeepsMode(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/hosts"
	if err := os.WriteFile(path, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeHostsAtomic(path, []byte("new\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
func TestAddManagedHostMovesExistingName(t *testing.T) {
	t.Parallel()
	entries := []labHostEntry{{IP: "10.0.0.2", Names: []string{"box.htb", "old.htb"}}}
	if err := addManagedHost(&entries, "10.0.0.3", []string{"box.htb"}, []string{"htb"}); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || len(entries[0].Names) != 1 || entries[0].Names[0] != "old.htb" || entries[1].Names[0] != "box.htb" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
