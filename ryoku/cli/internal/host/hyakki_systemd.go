package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type legacyHyakkiLink struct {
	path   string
	target string
}

type legacyHyakkiMask struct {
	path   string
	backup string
	info   os.FileInfo
}

func (a *App) legacyHyakkiUserUnitDir() string {
	config := a.getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(config) {
		config = filepath.Join(a.home(), ".config")
	}
	return filepath.Join(config, "systemd", "user")
}

func (a *App) legacyHyakkiLinks(name string) ([]legacyHyakkiLink, error) {
	var links []legacyHyakkiLink
	for _, root := range []string{a.legacyHyakkiUserUnitDir(), filepath.Join(a.runtimeDir(), "systemd", "user")} {
		for _, suffix := range []string{"*.wants", "*.requires"} {
			paths, err := filepath.Glob(filepath.Join(root, suffix, name))
			if err != nil {
				return nil, err
			}
			for _, path := range paths {
				target, err := os.Readlink(path)
				if err != nil {
					return nil, fmt.Errorf("snapshot legacy activation %s: %w", path, err)
				}
				links = append(links, legacyHyakkiLink{path, target})
			}
		}
	}
	return links, nil
}

func restoreLegacyHyakkiLinks(links []legacyHyakkiLink) error {
	for _, link := range links {
		if target, err := os.Readlink(link.path); err == nil && target == link.target {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(link.path), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(link.target, link.path); err != nil {
			return fmt.Errorf("restore legacy activation %s: %w", link.path, err)
		}
	}
	return nil
}

// A user disable cannot override global enablement. Mask only this user's
// predecessor, preserving its definition and exact local links for rollback.
func (a *App) maskInheritedHyakkiUnit(service *legacyHyakkiService) (*legacyHyakkiMask, error) {
	enabled, _, err := a.legacyHyakkiEnabled(service.name)
	if err != nil || !enabled {
		return nil, err
	}
	global := a.query("systemctl", "--global", "is-enabled", service.name)
	state := strings.TrimSpace(global.Output)
	if global.Code != 0 || state != "enabled" && state != "enabled-runtime" {
		return nil, fmt.Errorf("legacy %s was re-enabled outside migration; its owner must stop activation normally", service.name)
	}
	service.inherited = true
	root := a.legacyHyakkiUserUnitDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	dir, err := legacyHyakkiOwnedDirectory(root, a.cfg.UID)
	if err != nil {
		return nil, err
	}
	dir.Close()
	mask := &legacyHyakkiMask{path: filepath.Join(root, service.name)}
	if info, err := os.Lstat(mask.path); err == nil {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(stat.Uid) != a.cfg.UID || !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return nil, fmt.Errorf("unsafe legacy user unit %s", mask.path)
		}
		backup, err := os.MkdirTemp(root, ".hyakki-retired-units-")
		if err != nil {
			return nil, err
		}
		mask.backup = filepath.Join(backup, service.name)
		if err := os.Rename(mask.path, mask.backup); err != nil {
			_ = os.Remove(backup)
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.Symlink("/dev/null", mask.path); err != nil {
		return mask, err
	}
	mask.info, err = os.Lstat(mask.path)
	if err != nil {
		return mask, err
	}
	if code := a.Service([]string{"--user", "daemon-reload"}); code != ExitOK {
		return mask, fmt.Errorf("reload per-user legacy mask: service exited %d", code)
	}
	return mask, nil
}

func (mask *legacyHyakkiMask) restore() error {
	if info, err := os.Lstat(mask.path); err == nil {
		if mask.info == nil || !os.SameFile(info, mask.info) {
			return fmt.Errorf("legacy mask changed outside migration: %s; definition retained at %s", mask.path, mask.backup)
		}
		if err := os.Remove(mask.path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if mask.backup != "" {
		if err := os.Rename(mask.backup, mask.path); err != nil {
			return err
		}
		return os.Remove(filepath.Dir(mask.backup))
	}
	return nil
}
