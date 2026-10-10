package sys

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
)

// Package channels. A packaged box takes its Ryoku set from one [ryoku] repo
// directory, and which directory is the channel:
//
//	stable    x86_64/                  the pointer every installed box has; a
//	                                   byte copy of the newest frozen release
//	testing   channels/testing/x86_64/ rebuilt on every push to unstable-dev
//	v<tag>    releases/<tag>/x86_64/   one frozen release, never rewritten
//
// All of them live under RepoBase, the one bucket mount the repo domain
// serves (repo.ryoku.dev/stable/<key> is bucket object <key>; the "stable"
// segment is the mount, not the channel). publish-repo.yml writes them. The
// channel a box is on is nothing but the Server line of its [ryoku] stanza,
// so there is no second state to drift from it.
const (
	RepoBase     = "https://repo.ryoku.dev/stable"
	VoidRepoBase = RepoBase + "/void"
)

const (
	ChannelStable  = "stable"
	ChannelTesting = "testing"
	// ChannelUnstable is the user-facing name of the testing channel. The
	// command line and every message the CLI prints use it; ChannelTesting
	// stays the internal key -- the repo path, the [ryoku] Server, and the
	// channel-intent value on disk.
	ChannelUnstable = "unstable"
)

// PacmanConf is where the [ryoku] stanza lives; a var so tests point it at a
// fixture.
var PacmanConf = "/etc/pacman.conf"

// ReleaseFile is the pacman-owned marker ryoku-desktop ships naming the
// release a box runs; a var for tests.
var ReleaseFile = "/etc/ryoku-release"

// the shape stable-release.yml tags (bin/ryoku-release-bump): a core version
// with an optional alpha/beta/rc counter. a testing build's name
// (v0.56.0-beta.19.dev.363+g4d1cf63) is deliberately not one: nothing frozen
// stands behind it, so it can be neither tracked nor gone back to.
var releaseTagRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-(alpha|beta|rc)\.[0-9]+)?$`)

// IsReleaseTag reports whether s names a frozen release (v0.55.7-beta.19,
// v1.0.0), the shape stable-release.yml tags.
func IsReleaseTag(s string) bool { return releaseTagRe.MatchString(s) }

// releaseTagFieldsRe splits a frozen release tag into its comparable parts.
var releaseTagFieldsRe = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)(?:-(alpha|beta|rc)\.([0-9]+))?$`)

// releaseTagOrder is a parsed release tag. A final release carries no
// prerelease, so preRank 3 ("released") ranks above any alpha/beta/rc of the
// same core version.
type releaseTagOrder struct {
	major, minor, patch, preRank, preNum int
	ok                                   bool
}

func parseReleaseTag(tag string) releaseTagOrder {
	m := releaseTagFieldsRe.FindStringSubmatch(strings.TrimSpace(tag))
	if m == nil {
		return releaseTagOrder{}
	}
	o := releaseTagOrder{ok: true, preRank: 3}
	o.major, _ = strconv.Atoi(m[1])
	o.minor, _ = strconv.Atoi(m[2])
	o.patch, _ = strconv.Atoi(m[3])
	switch m[4] {
	case "alpha":
		o.preRank = 0
	case "beta":
		o.preRank = 1
	case "rc":
		o.preRank = 2
	}
	if m[5] != "" {
		o.preNum, _ = strconv.Atoi(m[5])
	}
	return o
}

// CompareReleaseTags orders two frozen release tags (v0.63.1-beta.19), returning
// -1, 0, 1 for a<b, a==b, a>b. Core version dominates; a tag with no prerelease
// counter outranks one that has it (v1.0.0 > v1.0.0-rc.1); among prereleases
// alpha<beta<rc, then the counter numerically. A tag that does not parse sorts
// oldest, so an unknown never reads as ahead of a real release.
func CompareReleaseTags(a, b string) int {
	pa, pb := parseReleaseTag(a), parseReleaseTag(b)
	switch {
	case !pa.ok && !pb.ok:
		return 0
	case !pa.ok:
		return -1
	case !pb.ok:
		return 1
	}
	for _, d := range []int{pa.major - pb.major, pa.minor - pb.minor, pa.patch - pb.patch, pa.preRank - pb.preRank, pa.preNum - pb.preNum} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	return 0
}

// ChannelServer is the [ryoku] Server line for a channel or release tag, or ""
// for a name that is neither.
func ChannelServer(channel string) string {
	switch {
	case channel == ChannelStable:
		return RepoBase + "/$arch"
	case channel == ChannelTesting:
		return RepoBase + "/channels/testing/$arch"
	case IsReleaseTag(channel):
		return RepoBase + "/releases/" + channel + "/$arch"
	}
	return ""
}

// VoidChannelURL is the signed XBPS repository URL for a channel or frozen
// release. Unlike pacman's Server line, XBPS repositories do not carry an
// architecture placeholder.
func VoidChannelURL(channel string) string {
	switch {
	case channel == ChannelStable:
		return VoidRepoBase + "/x86_64"
	case channel == ChannelTesting:
		return VoidRepoBase + "/channels/testing/x86_64"
	case IsReleaseTag(channel):
		return VoidRepoBase + "/releases/" + channel + "/x86_64"
	}
	return ""
}

// ChannelOfVoidURL maps a published XBPS repository URL to its channel.
func ChannelOfVoidURL(repository string) string {
	value := strings.TrimSuffix(strings.TrimSpace(repository), "/")
	value = strings.TrimSuffix(strings.TrimSuffix(value, "/"), "x86_64")
	if !strings.HasPrefix(value, VoidRepoBase) {
		return ""
	}
	rest := strings.Trim(strings.TrimPrefix(value, VoidRepoBase), "/")
	switch {
	case rest == "":
		return ChannelStable
	case rest == "channels/testing":
		return ChannelTesting
	case strings.HasPrefix(rest, "releases/"):
		if tag := strings.TrimPrefix(rest, "releases/"); IsReleaseTag(tag) {
			return tag
		}
	}
	return ""
}

// ChannelOfServer maps a Server line back to its channel name: stable,
// testing, a release tag, or "" for a mirror Ryoku does not publish (a local
// build-repo.sh out/ tree, a private mirror), which is left alone everywhere.
func ChannelOfServer(server string) string {
	s := strings.TrimSpace(server)
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), "$arch")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), "x86_64")
	s = strings.TrimSuffix(s, "/")
	if !strings.HasPrefix(s, RepoBase) {
		return ""
	}
	rest := strings.Trim(strings.TrimPrefix(s, RepoBase), "/")
	switch {
	case rest == "":
		return ChannelStable
	case rest == "channels/testing":
		return ChannelTesting
	case strings.HasPrefix(rest, "releases/"):
		if tag := strings.TrimPrefix(rest, "releases/"); IsReleaseTag(tag) {
			return tag
		}
	}
	return ""
}

// ChannelURL is the browsable base of a channel's x86_64 directory (Server with
// $arch resolved), used to read release.json.
func ChannelURL(channel string) string {
	return strings.Replace(ChannelServer(channel), "$arch", "x86_64", 1)
}

// DisplayChannel is the user-facing name of a package channel: the testing
// channel reads as "unstable", while its internal key stays "testing". Every
// other name (stable, a release tag) is shown as itself. Callers that print a
// PackagedChannel or ReadChannelIntent value to the user route it through here.
func DisplayChannel(channel string) string {
	if channel == ChannelTesting {
		return ChannelUnstable
	}
	return channel
}

// TrackName is the `ryoku track` argument a user runs to follow a channel or a
// source branch, so a hint the CLI prints always names a command that still
// works: the testing channel and the unstable-dev branch are both reached with
// "unstable", the main branch with "stable". A package channel otherwise maps
// through DisplayChannel.
func TrackName(channel string) string {
	switch channel {
	case "unstable-dev":
		return ChannelUnstable
	case "main":
		return ChannelStable
	}
	return DisplayChannel(channel)
}

// RyokuServer returns the Server line of the [ryoku] stanza in pacman.conf, or
// "" when the stanza is absent.
func RyokuServer() string {
	b, err := os.ReadFile(PacmanConf)
	if err != nil {
		return ""
	}
	in := false
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = line == "[ryoku]"
			continue
		}
		if in && strings.HasPrefix(line, "Server") {
			if i := strings.Index(line, "="); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

// PackagedChannel is the channel a packaged box follows: the channel its
// [ryoku] Server names, "" when there is no stanza or it points at a mirror
// Ryoku does not publish.
func PackagedChannel() string { return ChannelOfServer(RyokuServer()) }

// SetPackagedChannel rewrites the [ryoku] Server line to channel (stable,
// testing, or a release tag). It needs a stanza to rewrite; the doctor adds a
// missing one.
func SetPackagedChannel(channel string) error {
	server := ChannelServer(channel)
	if server == "" {
		return fmt.Errorf(i18n.T("unknown channel %q (stable, unstable, or a release tag like v0.55.7-beta.19)"), channel)
	}
	return SetRyokuServer(server)
}

// SetRyokuServer rewrites the [ryoku] Server line to the exact server string and
// drops the cached sync db so the next refresh pulls a matched pair. Unlike
// SetPackagedChannel it takes the raw line, so a caller can put a previously
// recorded Server back byte-for-byte after a failed channel move (the
// transactional pin in the updater). Written through WriteRootFile, so the file
// is replaced whole.
func SetRyokuServer(server string) error {
	b, err := os.ReadFile(PacmanConf)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	in, done := false, false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			in = line == "[ryoku]"
			continue
		}
		if in && strings.HasPrefix(line, "Server") && strings.Contains(line, "=") {
			lines[i] = "Server = " + server
			done = true
		}
	}
	if !done {
		return fmt.Errorf(i18n.T("no [ryoku] repo in %s; run `ryoku doctor` to add it"), PacmanConf)
	}
	if err := WriteRootFile(PacmanConf, strings.Join(lines, "\n"), "0644"); err != nil {
		return err
	}
	// the cached sync db describes the server the box just left; pacman only
	// refetches a db it thinks is newer, so a stale copy against a frozen
	// (older) release fails its signature check until it is gone.
	return DropRyokuSyncDB()
}

// DropRyokuSyncDB removes the cached [ryoku] sync db, its signature, and the
// files db. pacman refetches a db only when it thinks the server's is newer, so
// a cached db whose bytes no longer match its ryoku.db.sig wedges every
// transaction on "invalid or corrupted database (PGP signature)"; dropping it
// lets the next -Sy pull a matched pair. Callers refresh afterwards.
func DropRyokuSyncDB() error {
	names := []string{"ryoku.db", "ryoku.db.sig", "ryoku.files", "ryoku.files.sig"}
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(PacmanSyncDir, n)
	}
	// Remove directly when we own the sync dir (a test fixture under a temp dir);
	// the real root-owned /var/lib/pacman/sync falls back to sudo.
	if writableDir(PacmanSyncDir) {
		for _, p := range paths {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	return Sudo(append([]string{"rm", "-f"}, paths...)...)
}

// PacmanSyncDir holds pacman's cached sync dbs; a var so tests point it at a
// fixture and DropRyokuSyncDB never removes the real one.
var PacmanSyncDir = "/var/lib/pacman/sync"

// WriteRootFile replaces path whole with contents at the given mode. A target
// whose directory we can write (a user-owned file, a test fixture under a temp
// dir) is written in place; only a root-owned path like /etc falls back to
// sudo install. The in-place path keeps writers hermetic under a temp
// PacmanConf and never shells out to sudo in a unit test.
func WriteRootFile(path, contents, mode string) error {
	if writableDir(filepath.Dir(path)) {
		return writeFileAtomic(path, contents, fileMode(mode))
	}
	tmp, err := os.CreateTemp("", "ryoku-root-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return Run("sudo", "install", "-D", "-m", mode, "-o", "root", "-g", "root", tmp.Name(), path)
}

// writableDir reports whether the current process can create a file in dir (a
// user-owned config dir or a test temp dir); a root-owned dir like /etc returns
// false so the caller escalates.
func writableDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".ryoku-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

// writeFileAtomic writes contents to path via a temp file in the same directory
// and an atomic rename, so a reader never sees a half-written file.
func writeFileAtomic(path, contents string, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ryoku-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// fileMode parses an octal mode string ("0644") for the in-place writer,
// defaulting to 0644 for anything unparseable.
func fileMode(mode string) os.FileMode {
	if m, err := strconv.ParseUint(mode, 8, 32); err == nil {
		return os.FileMode(m)
	}
	return 0o644
}

// Release is /etc/ryoku-release: the named state a packaged box runs, written
// by the ryoku-desktop package from what build-repo.sh was told at publish.
type Release struct {
	Release string // v0.55.7-beta.19 on stable, v0.55.9.dev.412+gabc1234 on testing, local-* for a hand build
	Name    string // the line's name (CODENAME at the build, see release/names.md)
	Channel string // stable, testing, local
	Version string // the pacman pkgver
	Commit  string
	Date    string
}

// ReadRelease parses ReleaseFile; every field is "" when the file is absent
// (a box installed before releases were named; the next update carries it).
func ReadRelease() Release {
	var r Release
	b, err := os.ReadFile(ReleaseFile)
	if err != nil {
		return r
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch strings.TrimSpace(k) {
		case "RELEASE":
			r.Release = v
		case "NAME":
			r.Name = v
		case "CHANNEL":
			r.Channel = v
		case "VERSION":
			r.Version = v
		case "COMMIT":
			r.Commit = v
		case "DATE":
			r.Date = v
		}
	}
	return r
}

// ChannelIntentFile records the channel the user deliberately chose with `ryoku
// track` or by picking the stable or unstable shell installer (root-owned). It
// is the only signal that separates a pin the user asked for from one a failed
// boot-guard revert or a hand-edit left behind, so nothing else writes it. A
// var so tests point it at a fixture.
var ChannelIntentFile = "/var/lib/ryoku/channel-intent"

// ReadChannelIntent returns the channel Track recorded, or "" when none is on
// disk (a box that never tracked; the caller defaults to stable).
func ReadChannelIntent() string {
	b, err := os.ReadFile(ChannelIntentFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// WriteChannelIntent records channel as the deliberate choice. Root-owned like
// the boot-guard marker beside it, written through WriteRootFile.
func WriteChannelIntent(channel string) error {
	return WriteRootFile(ChannelIntentFile, strings.TrimSpace(channel)+"\n", "0644")
}
