package apprelease

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ryoku-cli/internal/sys"
)

// Bounds on every network operation. They are generous enough for a real
// release on a slow link yet finite, so neither a doctor check nor an update can
// hang on a stalled connection or a runaway body.
const (
	metadataTimeout = 10 * time.Second // the release JSON lookup
	downloadTimeout = 5 * time.Minute  // the package + checksum download
	maxMetaBytes    = 2 << 20          // release JSON is small; cap the decode
	maxShaBytes     = 4 << 10          // a checksum sidecar is one short line
	maxPkgBytes     = 512 << 20        // a hard ceiling on the package stream
	maxCacheBytes   = 4 << 10          // the on-disk cache is a few short fields
)

// Client resolves and installs one app's releases. Its exported transport
// fields and injected system dependencies keep the full path testable.
type Client struct {
	app          App
	Repo         string       // owner/name on GitHub
	APIBase      string       // GitHub API root
	DownloadBase string       // release-asset download host (github.com)
	HTTP         *http.Client // bounds redirects to trusted hosts

	now      func() time.Time
	cacheDir string
	checkTTL time.Duration

	// System dependencies, injected so the install path is testable without
	// pacman, sudo, or a real package on disk.
	installedVersion func(pkg string) string                                     // pacman -Q
	inspect          func(ctx context.Context, path string) (pkgMeta, error)     // pacman -Qip
	install          func(ctx context.Context, path string, digest []byte) error // root-safe pacman -U
	vercmp           func(a, b string) (int, error)                              // vercmp
}

// pkgMeta is the identity a package file claims, read from its own pacman
// metadata (not its filename). Upgrade checks every field against what it
// expected before it lets pacman install the file.
type pkgMeta struct {
	Name    string
	Version string
	Arch    string
}

// defaultClient wires a Client to the real system.
func defaultClient(app App) *Client {
	return &Client{
		app:              app,
		Repo:             app.Repo,
		APIBase:          apiBase(),
		DownloadBase:     "https://github.com",
		HTTP:             defaultHTTPClient(),
		now:              time.Now,
		cacheDir:         filepath.Join(sys.StateDir(), app.Name+"-release"),
		checkTTL:         checkCacheTTL,
		installedVersion: defaultInstalledVersion,
		inspect:          defaultInspect,
		install: func(ctx context.Context, path string, digest []byte) error {
			return defaultInstall(ctx, app.Name, path, digest)
		},
		vercmp: defaultVercmp,
	}
}

// apiBase is the GitHub API root. RYOKU_GITHUB_API points it at a stub, the same
// override the CLI's commit lookups already honour; it affects only metadata
// discovery, and the download still targets DownloadBase.
func apiBase() string {
	if s := strings.TrimSpace(os.Getenv("RYOKU_GITHUB_API")); s != "" {
		return s
	}
	return "https://api.github.com"
}

// defaultHTTPClient follows redirects only within GitHub's own hosts, so a
// tampered redirect cannot walk a download off to another origin. No overall
// Timeout is set: per-request context deadlines bound each call (a short one for
// metadata, a longer one for the package), which a single client Timeout could
// not express.
func defaultHTTPClient() *http.Client {
	return &http.Client{CheckRedirect: trustedRedirect(isTrustedGitHubHost)}
}

// trustedGitHubHosts is the concrete, closed set of hosts a release download
// may traverse.
var trustedGitHubHosts = map[string]bool{
	"github.com":                           true,
	"api.github.com":                       true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// isTrustedGitHubHost reports whether host is one of the concrete GitHub hosts a
// release download legitimately traverses.
func isTrustedGitHubHost(host string) bool {
	return trustedGitHubHosts[strings.ToLower(host)]
}

// trustedRedirect refuses any redirect that is not HTTPS to an allowed host, and
// caps the redirect chain. The initial request is never checked here (only
// redirects are), so a test server reached directly is unaffected while a hop to
// an untrusted origin is rejected.
func trustedRedirect(allow func(host string) bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing non-https redirect to %s", req.URL.Redacted())
		}
		if !allow(req.URL.Hostname()) {
			return fmt.Errorf("refusing redirect to untrusted host %q", req.URL.Hostname())
		}
		return nil
	}
}

// defaultInstalledVersion returns a package's installed version or an empty
// string when it is absent.
func defaultInstalledVersion(pkg string) string {
	out, err := exec.Command("pacman", "-Q", pkg).Output()
	if err != nil {
		return ""
	}
	f := strings.Fields(strings.TrimSpace(string(out)))
	if len(f) == 2 {
		return f[1]
	}
	return ""
}

// defaultInspect reads a package file's own claimed identity via `pacman -Qip`.
// LC_ALL=C pins the field labels (Name/Version/Architecture) so the parse does
// not break under a localized pacman.
func defaultInspect(ctx context.Context, path string) (pkgMeta, error) {
	cmd := exec.CommandContext(ctx, "pacman", "-Qip", path)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return pkgMeta{}, fmt.Errorf("pacman -Qip %s: %w", filepath.Base(path), err)
	}
	return parsePkgInfo(string(out)), nil
}

// parsePkgInfo pulls Name, Version and Architecture out of `pacman -Qip` output,
// whose lines are "Label : value".
func parsePkgInfo(out string) pkgMeta {
	var m pkgMeta
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "Name":
			m.Name = v
		case "Version":
			m.Version = v
		case "Architecture":
			m.Arch = v
		}
	}
	return m
}

// defaultInstall installs a verified package through pacman across a root-owned
// staging boundary, so the bytes pacman finally opens cannot be swapped between
// verification and install by any process running as the same unprivileged user.
//
// The caller verified the package (sha256 + pacman metadata) in a user-owned temp
// dir, but a same-user process could still replace that file before `pacman -U`
// opened it (a classic TOCTOU). So this:
//
//  1. creates a fresh root-only directory under /var/tmp (mktemp -d),
//  2. copies the verified file into it as root (install(1), root-owned),
//  3. re-hashes the root-owned copy as root and checks it against the verified
//     digest -- a swap that happened before the copy is caught here, and after
//     the copy the file lives where only root can write,
//  4. runs the ordinary `pacman -U` on that immutable copy.
//
// It removes the root-owned directory on every path, drops the sudo wrapper when
// already root, and never interpolates a shell or a helper script: only fixed
// arguments and the two file paths are passed.
func defaultInstall(ctx context.Context, appName, path string, digest []byte) error {
	root := os.Geteuid() == 0

	out, err := privOut(ctx, root, "mktemp", "-d", "/var/tmp/"+appName+"-release.XXXXXXXXXX")
	if err != nil {
		return fmt.Errorf("root staging dir: %w", err)
	}
	dir := strings.TrimSpace(out)
	if dir == "" {
		return errors.New("root staging dir: mktemp returned no path")
	}
	// Clean up on every path, with a fresh context so a cancelled ctx cannot leak
	// the root-owned tree.
	defer func() { _ = privRun(context.Background(), root, "rm", "-rf", "--", dir) }()

	staged := filepath.Join(dir, filepath.Base(path))
	if err := privRun(ctx, root, "install", "-m", "0644", "--", path, staged); err != nil {
		return fmt.Errorf("stage package as root: %w", err)
	}

	sumOut, err := privOut(ctx, root, "sha256sum", "--", staged)
	if err != nil {
		return fmt.Errorf("re-hash staged package: %w", err)
	}
	if err := verifyStagedDigest(sumOut, digest); err != nil {
		return err
	}

	return privRun(ctx, root, "pacman", "-U", "--noconfirm", staged)
}

// verifyStagedDigest parses `sha256sum` output and constant-time compares its
// digest to want. A mismatch means the staged bytes are not the verified bytes
// (a swap between verification and staging), so the install is refused.
func verifyStagedDigest(sha256sumOutput string, want []byte) error {
	fields := strings.Fields(sha256sumOutput)
	if len(fields) == 0 {
		return errors.New("staged package: empty sha256sum output")
	}
	got, err := hex.DecodeString(strings.ToLower(fields[0]))
	if err != nil || len(got) != sha256.Size {
		return fmt.Errorf("staged package: malformed sha256 %q", fields[0])
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return errors.New("staged package: hash does not match the verified download")
	}
	return nil
}

// privCommand wraps a command in sudo unless the process is already root.
func privCommand(ctx context.Context, root bool, name string, args ...string) *exec.Cmd {
	if root {
		return exec.CommandContext(ctx, name, args...)
	}
	return exec.CommandContext(ctx, "sudo", append([]string{name}, args...)...)
}

// privRun runs a privileged command with the terminal wired through, so pacman
// streams and any sudo password prompt reach the user.
func privRun(ctx context.Context, root bool, name string, args ...string) error {
	cmd := privCommand(ctx, root, name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// privOut runs a privileged command and captures stdout, leaving stdin and
// stderr on the terminal so a sudo prompt still works.
func privOut(ctx context.Context, root bool, name string, args ...string) (string, error) {
	cmd := privCommand(ctx, root, name, args...)
	cmd.Stdin, cmd.Stderr = os.Stdin, os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

// defaultVercmp shells out to pacman's vercmp, the authority on Arch version
// ordering (epoch, pkgver, pkgrel). It prints -1, 0 or 1 for a<b, a==b, a>b.
func defaultVercmp(a, b string) (int, error) {
	out, err := exec.Command("vercmp", a, b).Output()
	if err != nil {
		return 0, fmt.Errorf("vercmp %s %s: %w", a, b, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("vercmp returned %q: %w", strings.TrimSpace(string(out)), err)
	}
	return n, nil
}
