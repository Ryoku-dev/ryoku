package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// productViews are the durable copies the shell loads a QML product from: one
// directory per receipt, named by a digest of every receipt-owned file. QML
// caches each relative dependency by URL and resolves relative imports without
// inheriting an entry point's query string, so only a content-specific directory
// URL invalidates the complete dependency graph when a product updates.
type productViews struct {
	category string // the receipt category the views copy
	root     string // the state-relative directory that holds them
}

var (
	pluginViews   = productViews{category: "plugins", root: "plugin-views"}
	launcherViews = productViews{category: "launchers", root: "launcher-views"}
	dockViews     = productViews{category: "docks", root: "dock-views"}
	// The overlays every bar style shares: designs.osd, .notification and
	// .sidebar in shell.json.
	osdViews          = productViews{category: "osds", root: "osd-views"}
	notificationViews = productViews{category: "notifications", root: "notification-views"}
	sidebarViews      = productViews{category: "sidebars", root: "sidebar-views"}
)

// path is the state-relative view of one receipt: <root>/<id>/<digest>.
func (v productViews) path(id string, receipt Receipt) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(id))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(receipt.Version))
	for _, file := range receipt.Files {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(file.Destination))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(file.SHA256))
	}
	return filepath.ToSlash(filepath.Join(v.root, id, fmt.Sprintf("%x", digest.Sum(nil))))
}

// ensure leaves a verified view untouched and rebuilds a missing or damaged one
// from the installed product, swapping it in only once the copy verifies.
func (v productViews) ensure(id, view string, receipt Receipt) error {
	destination, _, err := productDestination(v.category, id)
	if err != nil {
		return err
	}
	if view != v.path(id, receipt) {
		return fmt.Errorf("%s/%s changed while rebuilding its view", v.category, id)
	}
	target := filepath.Join(storeStateDir(), filepath.FromSlash(view))
	exists := false
	if _, err := os.Lstat(target); err == nil {
		exists = true
		if validateProductView(target, receipt) == nil {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temporary := target + ".tmp"
	backup := target + ".old"
	if err := os.RemoveAll(temporary); err != nil {
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for _, file := range receipt.Files {
		source := filepath.Join(destination, filepath.FromSlash(file.Destination))
		staged := filepath.Join(temporary, filepath.FromSlash(file.Destination))
		if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if file.Mode == "0755" {
			mode = 0o755
		}
		if err := copyProductViewFile(source, staged, mode); err != nil {
			return err
		}
	}
	if err := validateProductView(temporary, receipt); err != nil {
		return err
	}
	if exists {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		defer os.Rename(backup, target)
	}
	if err := os.Rename(temporary, target); err != nil {
		return err
	}
	return os.RemoveAll(backup)
}

// prune drops every view the live index no longer names: the whole directory
// of a removed product and the superseded digests of an updated one, so neither
// a removal nor an update leaves another source tree behind in state. live maps
// each indexed id to its current state-relative view.
func (v productViews) prune(live map[string]string) error {
	root := filepath.Join(storeStateDir(), v.root)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		view, kept := live[entry.Name()]
		if !kept {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
			continue
		}
		current := filepath.Base(filepath.FromSlash(view))
		digests, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return err
		}
		for _, digest := range digests {
			if digest.Name() == current {
				continue
			}
			if err := os.RemoveAll(filepath.Join(root, entry.Name(), digest.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProductView(view string, receipt Receipt) error {
	info, err := os.Lstat(view)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("product view is not a directory: %s", view)
	}
	for _, file := range receipt.Files {
		path := filepath.Join(view, filepath.FromSlash(file.Destination))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("product view file is not regular: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := validateProductPayload(raw, file.Size, file.SHA256); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func copyProductViewFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
