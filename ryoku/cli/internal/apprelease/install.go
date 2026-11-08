package apprelease

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// installRelease downloads the release's package and its checksum, verifies the
// package by sha256 and then by its own pacman metadata (name, version, arch),
// and only then installs it. Every gate must pass before pacman runs; any
// failure aborts with the package still un-installed.
//
// The download and verification happen in one private, user-only temp directory,
// and the exact file that was verified is the exact path handed to pacman -- it
// is never re-fetched or resolved again between the check and the install, so
// there is no window to swap trusted bytes for untrusted ones.
//
// ensure selects the install-when-absent behaviour. With ensure=false (Upgrade)
// a package that vanished mid-download stays removed -- a removal is respected,
// not resurrected. With ensure=true (Ensure) an absent package is installed
// fresh, which is the point of the reconcile/install path. Either way a build
// that is not strictly newer than one already installed is refused: no path
// here ever downgrades.
func (c *Client) installRelease(ctx context.Context, rel relInfo, ensure bool) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	dir, err := os.MkdirTemp("", c.app.Name+"-release-")
	if err != nil {
		return fmt.Errorf("%s upgrade: staging dir: %w", c.app.Name, err)
	}
	// 0700 already from MkdirTemp; the whole tree goes at the end regardless of
	// outcome.
	defer os.RemoveAll(dir)

	pkgPath := filepath.Join(dir, rel.PkgAsset)
	digest, err := c.downloadPackage(ctx, c.assetURL(rel, rel.PkgAsset), pkgPath)
	if err != nil {
		return err
	}

	want, err := c.fetchChecksum(ctx, c.assetURL(rel, rel.ShaAsset), rel.PkgAsset)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(digest, want) != 1 {
		return fmt.Errorf("%s upgrade: sha256 mismatch on %s (got %s, want %s)",
			c.app.Name, rel.PkgAsset, hex.EncodeToString(digest), hex.EncodeToString(want))
	}

	// The bytes are the ones the release vouches for; now confirm they are the
	// package we think they are, read from the package's own metadata rather than
	// its filename.
	meta, err := c.inspect(ctx, pkgPath)
	if err != nil {
		return fmt.Errorf("%s upgrade: %w", c.app.Name, err)
	}
	if meta.Name != c.app.Name {
		return fmt.Errorf("%s upgrade: package identifies as %q, not %q", c.app.Name, meta.Name, c.app.Name)
	}
	if meta.Arch != wantArch {
		return fmt.Errorf("%s upgrade: package architecture %q, want %q", c.app.Name, meta.Arch, wantArch)
	}
	if meta.Version != rel.Version {
		return fmt.Errorf("%s upgrade: package version %q does not match the release %q", c.app.Name, meta.Version, rel.Version)
	}

	// Re-read the installed version immediately before installing, against the
	// version the package actually carries (not just the filename the release
	// advertised). The download took time; the state may have changed under us,
	// and this fresh read -- not the hint captured when the call started -- is
	// what the decision is made on.
	current := c.installedVersion(c.app.Name)
	if current == "" {
		// Absent right before install. For Upgrade that means a removal during
		// download, which stays removed. For Ensure an absent package is exactly
		// what we are here to install, so a fresh install proceeds.
		if !ensure {
			return fmt.Errorf("%s upgrade: %s was removed during download; not reinstalling", c.app.Name, c.app.Name)
		}
	} else {
		// Already installed: only ever move forward, never downgrade, whether we
		// were upgrading or ensuring.
		newer, err := c.isNewer(meta.Version, current)
		if err != nil {
			return fmt.Errorf("%s upgrade: %w", c.app.Name, err)
		}
		if !newer {
			return fmt.Errorf("%s upgrade: refusing to install %s over installed %s (not newer)", c.app.Name, meta.Version, current)
		}
	}

	// Hand pacman the verified bytes across a root-owned boundary; the digest lets
	// the installer re-check the staged copy so nothing can be swapped in after
	// this point.
	if err := c.install(ctx, pkgPath, digest); err != nil {
		return fmt.Errorf("%s upgrade: %w", c.app.Name, err)
	}
	return nil
}

// downloadPackage streams the package to dst while hashing it, and returns the
// sha256 digest of exactly the bytes written. The stream is capped at maxPkgBytes
// so a runaway or wrong URL cannot fill the disk; overrunning the cap is an
// error, not a silent truncation.
func (c *Client) downloadPackage(ctx context.Context, url, dst string) ([]byte, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s upgrade: download %s: GitHub returned %s", c.app.Name, filepath.Base(url), resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%s upgrade: %w", c.app.Name, err)
	}
	defer f.Close()

	h := sha256.New()
	// Read one byte past the ceiling so an over-size body is detected rather than
	// quietly cut to the limit.
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxPkgBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s upgrade: download %s: %w", c.app.Name, filepath.Base(url), err)
	}
	if n > maxPkgBytes {
		return nil, fmt.Errorf("%s upgrade: %s exceeds the %d-byte size limit", c.app.Name, filepath.Base(url), int64(maxPkgBytes))
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("%s upgrade: %w", c.app.Name, err)
	}
	return h.Sum(nil), nil
}

// fetchChecksum downloads the sha256 sidecar and returns the expected 32-byte
// digest. The sidecar is the standard `sha256sum` line, "HEX  filename"; only the
// hex is required, and when a filename column is present it must name the package
// we are verifying, so a sidecar copied from another asset is rejected.
func (c *Client) fetchChecksum(ctx context.Context, url, pkgAsset string) ([]byte, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s upgrade: download %s: GitHub returned %s", c.app.Name, filepath.Base(url), resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxShaBytes))
	if err != nil {
		return nil, fmt.Errorf("%s upgrade: read checksum: %w", c.app.Name, err)
	}

	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return nil, fmt.Errorf("%s upgrade: empty checksum file for %s", c.app.Name, pkgAsset)
	}
	if len(fields) >= 2 {
		// `sha256sum file` records the name it hashed; a '*' marks binary mode.
		named := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(named) != pkgAsset {
			return nil, fmt.Errorf("%s upgrade: checksum names %q, not %q", c.app.Name, filepath.Base(named), pkgAsset)
		}
	}
	sum, err := hex.DecodeString(strings.ToLower(fields[0]))
	if err != nil || len(sum) != sha256.Size {
		return nil, fmt.Errorf("%s upgrade: malformed sha256 %q for %s", c.app.Name, fields[0], pkgAsset)
	}
	return sum, nil
}

// get issues a bounded GET through the redirect-guarded client.
func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ryoku-cli")
	return c.HTTP.Do(req)
}
