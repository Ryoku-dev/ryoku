package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChannelServerRoundTrips(t *testing.T) {
	for _, ch := range []string{"stable", "testing", "v0.55.7-beta.19", "v1.0.0", "v1.2.3-rc.1"} {
		srv := ChannelServer(ch)
		if srv == "" {
			t.Fatalf("%s: no server", ch)
		}
		if got := ChannelOfServer(srv); got != ch {
			t.Fatalf("%s -> %s -> %q", ch, srv, got)
		}
	}
	if ChannelServer("main") != "" || ChannelServer("v1") != "" || ChannelServer("releases/v1.0.0") != "" ||
		ChannelServer("v0.56.0-beta.19.dev.363+g4d1cf63") != "" {
		t.Fatal("non-channels must not map to a server")
	}
}

func TestVoidChannelURLRoundTrips(t *testing.T) {
	for _, channel := range []string{"stable", "testing", "v0.55.7-beta.19", "v1.0.0"} {
		repository := VoidChannelURL(channel)
		if repository == "" {
			t.Fatalf("%s: no Void repository", channel)
		}
		if got := ChannelOfVoidURL(repository); got != channel {
			t.Fatalf("%s -> %s -> %q", channel, repository, got)
		}
	}
	if VoidChannelURL("main") != "" || ChannelOfVoidURL("https://mirror.example/void") != "" {
		t.Fatal("unpublished Void channels must not map to the Ryoku repository")
	}
}

// The testing channel is shown as "unstable" everywhere the CLI prints a channel
// to the user; stable and a release tag are shown as themselves. TrackName maps
// a channel or a source branch to a `ryoku track` argument that still works, so
// a printed hint never names the retired main/unstable-dev.
func TestDisplayChannelAndTrackName(t *testing.T) {
	display := map[string]string{
		"testing":         "unstable",
		"stable":          "stable",
		"v0.55.7-beta.19": "v0.55.7-beta.19",
		"unstable":        "unstable",
	}
	for in, want := range display {
		if got := DisplayChannel(in); got != want {
			t.Errorf("DisplayChannel(%q) = %q, want %q", in, got, want)
		}
	}
	track := map[string]string{
		"testing":         "unstable",
		"unstable-dev":    "unstable",
		"main":            "stable",
		"stable":          "stable",
		"v0.55.7-beta.19": "v0.55.7-beta.19",
	}
	for in, want := range track {
		if got := TrackName(in); got != want {
			t.Errorf("TrackName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChannelOfServerAcceptsWhatBoxesCarry(t *testing.T) {
	cases := map[string]string{
		"https://repo.ryoku.dev/stable/$arch":                            "stable",
		"https://repo.ryoku.dev/stable/x86_64":                           "stable",
		"https://repo.ryoku.dev/stable/x86_64/":                          "stable",
		"https://repo.ryoku.dev/stable/channels/testing/$arch":           "testing",
		"https://repo.ryoku.dev/stable/releases/v0.55.7-beta.19/$arch":   "v0.55.7-beta.19",
		"https://repo.ryoku.dev/stable/releases/v0.55.7-beta.19/x86_64/": "v0.55.7-beta.19",
		"file:///home/x/ryoku-arch/release/repo/out/$arch":               "",
		"https://mirror.example.org/ryoku/$arch":                         "",
		"https://repo.ryoku.dev/stable/releases/not-a-tag/$arch":         "",
	}
	for in, want := range cases {
		if got := ChannelOfServer(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestRyokuServerReadsTheStanza(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "pacman.conf")
	os.WriteFile(conf, []byte("[options]\nHoldPkg = pacman\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n\n[ryoku]\nSigLevel = Required\nServer = https://repo.ryoku.dev/stable/$arch\n\n[extra]\nServer = https://example/$arch\n"), 0o644)
	old := PacmanConf
	PacmanConf = conf
	defer func() { PacmanConf = old }()
	if got := RyokuServer(); got != "https://repo.ryoku.dev/stable/$arch" {
		t.Fatalf("server = %q", got)
	}
	if got := PackagedChannel(); got != "stable" {
		t.Fatalf("channel = %q", got)
	}
}

func TestReadReleaseParsesTheMarker(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ryoku-release")
	os.WriteFile(f, []byte("RELEASE=v0.55.7-beta.19\nNAME=Onogoro\nCHANNEL=stable\nVERSION=0.55.7.r3280.g097f522\nCOMMIT=097f522b5\nDATE=2026-09-03T20:00:00Z\n"), 0o644)
	old := ReleaseFile
	ReleaseFile = f
	defer func() { ReleaseFile = old }()
	r := ReadRelease()
	if r.Release != "v0.55.7-beta.19" || r.Name != "Onogoro" || r.Channel != "stable" || r.Version != "0.55.7.r3280.g097f522" || r.Commit != "097f522b5" {
		t.Fatalf("release = %+v", r)
	}
	ReleaseFile = filepath.Join(dir, "missing")
	if r := ReadRelease(); r.Release != "" {
		t.Fatalf("missing file must read empty, got %+v", r)
	}
}

// CompareReleaseTags is the ordering the doctor and `ryoku status` use to spot a
// [ryoku] pin older than the installed release (#291), so its ordering is
// load-bearing: core version dominates, a final release outranks its prereleases,
// and alpha<beta<rc then the counter.
func TestCompareReleaseTags(t *testing.T) {
	older := []struct{ a, b string }{
		{"v0.63.1-beta.19", "v0.75.3-beta.20"}, // the issue's exact pair
		{"v0.75.3-beta.19", "v0.75.3-beta.20"}, // same core, lower counter
		{"v0.75.3-alpha.9", "v0.75.3-beta.1"},  // alpha before beta
		{"v0.75.3-rc.2", "v0.75.3"},            // prerelease before the final release
		{"v0.9.9", "v0.10.0"},                  // numeric, not lexical, minor
	}
	for _, c := range older {
		if got := CompareReleaseTags(c.a, c.b); got != -1 {
			t.Errorf("CompareReleaseTags(%q,%q) = %d, want -1", c.a, c.b, got)
		}
		if got := CompareReleaseTags(c.b, c.a); got != 1 {
			t.Errorf("CompareReleaseTags(%q,%q) = %d, want 1", c.b, c.a, got)
		}
	}
	if got := CompareReleaseTags("v0.75.3-beta.20", "v0.75.3-beta.20"); got != 0 {
		t.Errorf("equal tags = %d, want 0", got)
	}
	// an unparseable tag sorts oldest, so it never reads as ahead of a real one.
	if got := CompareReleaseTags("testing", "v0.1.0"); got != -1 {
		t.Errorf("unparseable vs real = %d, want -1", got)
	}
}

// SetRyokuServer puts an exact Server line back, the restore path a failed
// channel move relies on to undo its pin (#291).
func TestSetRyokuServerRestoresExactLine(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "pacman.conf")
	os.WriteFile(conf, []byte("[options]\n\n[ryoku]\nSigLevel = Required\nServer = "+ChannelServer("stable")+"\n"), 0o644)
	oldConf, oldSync := PacmanConf, PacmanSyncDir
	PacmanConf, PacmanSyncDir = conf, t.TempDir()
	defer func() { PacmanConf, PacmanSyncDir = oldConf, oldSync }()

	prev := RyokuServer()
	if err := SetPackagedChannel("v0.63.1-beta.19"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if RyokuServer() == prev {
		t.Fatal("pin did not change the Server")
	}
	if err := SetRyokuServer(prev); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := RyokuServer(); got != prev {
		t.Fatalf("restored Server = %q, want %q", got, prev)
	}
	if got := PackagedChannel(); got != "stable" {
		t.Fatalf("restored channel = %q, want stable", got)
	}
}
