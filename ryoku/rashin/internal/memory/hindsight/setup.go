package hindsight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// setupParts counts the steps setup reports.
const setupParts = 4

// Progress hears how far setup has come: a step in words, its number and
// the number of steps.
type Progress func(step string, part, parts int)

// Setup installs a local Hindsight: its own Python and environment through
// uv, the packages in the shipped lock with their hashes checked, a key and
// database password made and kept in the vault, then one first start that
// fetches the models and makes the database. Nothing runs when it returns.
func (p *Process) Setup(ctx context.Context, progress Progress) error {
	layout := p.cfg.Layout
	uv, err := exec.LookPath("uv")
	if err != nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hindsight needs uv. Install the uv package, then set it up again."))
	}
	p.Stop()
	if err := os.Remove(layout.installed()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, dir := range []string{layout.Data, layout.Cache, layout.State} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}

	progress(i18n.T("Making a Python environment"), 1, setupParts)
	if err := os.RemoveAll(layout.venv()); err != nil {
		return err
	}
	if err := runUV(ctx, uv, layout, "venv", "--python", pythonVersion, "--python-preference", "only-managed", layout.venv()); err != nil {
		return err
	}

	progress(i18n.T("Installing Hindsight"), 2, setupParts)
	lockFile := filepath.Join(layout.Data, "hindsight.lock")
	if err := os.WriteFile(lockFile, lock, 0o600); err != nil {
		return err
	}
	defer os.Remove(lockFile)
	// --no-deps: the lock already names every package, and the two it leaves
	// out on purpose must not come back as dependencies
	if err := runUV(ctx, uv, layout, "pip", "install", "--python", layout.python(),
		"--require-hashes", "--no-deps", "--only-binary", ":all:", "-r", lockFile); err != nil {
		return err
	}

	progress(i18n.T("Making a key for Hindsight"), 3, setupParts)
	for _, name := range []string{KeyName, DatabaseName} {
		if _, err := p.cfg.Secrets.Secret(name); err == nil {
			continue
		}
		value, err := newSecret()
		if err != nil {
			return err
		}
		if err := p.cfg.Secrets.Keep(name, value); err != nil {
			return err
		}
		// pg0 sets the password only when it makes the database, so a database
		// left from an earlier password could not be opened with the new one
		if name == DatabaseName {
			if err := os.RemoveAll(layout.home()); err != nil {
				return err
			}
		}
	}

	progress(i18n.T("Fetching the models"), 4, setupParts)
	if err := os.WriteFile(layout.installed(), []byte(lockSum()+"\n"), 0o600); err != nil {
		return err
	}
	r := p.ensure(true)
	select {
	case <-r.ready:
	case <-ctx.Done():
		p.Stop()
		_ = os.Remove(layout.installed())
		return ctx.Err()
	}
	p.Stop()
	if r.err != nil {
		_ = os.Remove(layout.installed())
		return r.err
	}
	return nil
}

// runUV runs uv with its cache, its Pythons and its config kept to
// Hindsight's own folders. Its output stays out of Rashin's log; a failure
// says the last lines.
func runUV(ctx context.Context, uv string, layout Layout, args ...string) error {
	cmd := exec.CommandContext(ctx, uv, args...)
	cmd.Env = append(os.Environ(),
		"UV_CACHE_DIR="+layout.uvCache(),
		"UV_PYTHON_INSTALL_DIR="+layout.pythons(),
		"UV_NO_CONFIG=1",
		"UV_NO_PROGRESS=1",
	)
	cmd.Dir = layout.Data
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return fmt.Errorf("%s: %s", i18n.Tf("uv %s failed", args[0]), strings.Join(lines[max(len(lines)-3, 0):], " "))
}
