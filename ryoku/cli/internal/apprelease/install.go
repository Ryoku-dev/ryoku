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

// HeldError reports release files that the app's legacy owner package still
// ships. Installing over them would leave both packages claiming one file, and
// the next upgrade of the older package would put its copies back.
type HeldError struct {
	App   string
	Owner string
	Paths []string
}

func (e *HeldError) Error() string {
	return fmt.Sprintf("%s still ships %s", e.Owner, strings.Join(e.Paths, ", "))
}

// installRelease installs a verified release, labelling its errors as an
// install when the app was absent and as an upgrade otherwise.
func (c *Client) installRelease(ctx context.Context, rel relInfo, installed string, ensure bool) error {
	op := "upgrade"
	if installed == "" {
		op = "install"
	}
	if err := c.verifyAndInstall(ctx, rel, ensure); err != nil {
		return fmt.Errorf("%s %s: %w", c.app.Name, op, err)
	}
	return nil
}

// verifyAndInstall downloads the release's package and its checksum, verifies
// the package by sha256 and then by its own pacman metadata (name, version,
// arch), checks that no other package owns a file it carries, and only then
// installs it. Every gate must pass before pacman runs; any failure aborts with
// the package still un-installed.
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
func (c *Client) verifyAndInstall(ctx context.Context, rel relInfo, ensure bool) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	dir, err := os.MkdirTemp("", c.app.Name+"-release-")
	if err != nil {
		return fmt.Errorf("staging dir: %w", err)
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
		return fmt.Errorf("sha256 mismatch on %s (got %s, want %s)",
			rel.PkgAsset, hex.EncodeToString(digest), hex.EncodeToString(want))
	}

	// The bytes are the ones the release vouches for; now confirm they are the
	// package we think they are, read from the package's own metadata rather than
	// its filename.
	meta, err := c.inspect(ctx, pkgPath)
	if err != nil {
		return err
	}
	if meta.Name != c.app.Name {
		return fmt.Errorf("package identifies as %q, not %q", meta.Name, c.app.Name)
	}
	if meta.Arch != wantArch {
		return fmt.Errorf("package architecture %q, want %q", meta.Arch, wantArch)
	}
	if meta.Version != rel.Version {
		return fmt.Errorf("package version %q does not match the release %q", meta.Version, rel.Version)
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
			return fmt.Errorf("%s was removed during download; not reinstalling", c.app.Name)
		}
	} else {
		// Already installed: only ever move forward, never downgrade, whether we
		// were upgrading or ensuring.
		newer, err := c.isNewer(meta.Version, current)
		if err != nil {
			return err
		}
		if !newer {
			return fmt.Errorf("refusing to install %s over installed %s (not newer)", meta.Version, current)
		}
	}

	if err := c.checkFileOwners(ctx, pkgPath); err != nil {
		return err
	}

	// Hand pacman the verified bytes across a root-owned boundary; the digest lets
	// the installer re-check the staged copy so nothing can be swapped in after
	// this point.
	return c.install(ctx, pkgPath, digest)
}

// checkFileOwners refuses a package whose files another installed package
// owns. pacman would refuse them too; checking first keeps the sudo prompt away
// and tells a legacy owner (the app's files shipped inside an older package)
// apart from a real conflict.
func (c *Client) checkFileOwners(ctx context.Context, pkgPath string) error {
	files, err := c.listFiles(ctx, pkgPath)
	if err != nil {
		return err
	}
	var held, foreign []string
	for _, f := range files {
		if strings.HasSuffix(f, "/") {
			continue
		}
		for _, owner := range c.fileOwners(f) {
			switch {
			case owner == c.app.Name:
			case c.app.LegacyOwner != "" && owner == c.app.LegacyOwner:
				held = append(held, f)
			default:
				foreign = append(foreign, f+" ("+owner+")")
			}
		}
	}
	if len(foreign) > 0 {
		return fmt.Errorf("files owned by other packages: %s", strings.Join(foreign, ", "))
	}
	if len(held) > 0 {
		return &HeldError{App: c.app.Name, Owner: c.app.LegacyOwner, Paths: held}
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
		return nil, fmt.Errorf("download %s: GitHub returned %s", filepath.Base(url), resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	// Read one byte past the ceiling so an over-size body is detected rather than
	// quietly cut to the limit.
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxPkgBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", filepath.Base(url), err)
	}
	if n > maxPkgBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte size limit", filepath.Base(url), int64(maxPkgBytes))
	}
	if err := f.Sync(); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("download %s: GitHub returned %s", filepath.Base(url), resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxShaBytes))
	if err != nil {
		return nil, fmt.Errorf("read checksum: %w", err)
	}

	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty checksum file for %s", pkgAsset)
	}
	if len(fields) >= 2 {
		// `sha256sum file` records the name it hashed; a '*' marks binary mode.
		named := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(named) != pkgAsset {
			return nil, fmt.Errorf("checksum names %q, not %q", filepath.Base(named), pkgAsset)
		}
	}
	sum, err := hex.DecodeString(strings.ToLower(fields[0]))
	if err != nil || len(sum) != sha256.Size {
		return nil, fmt.Errorf("malformed sha256 %q for %s", fields[0], pkgAsset)
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
