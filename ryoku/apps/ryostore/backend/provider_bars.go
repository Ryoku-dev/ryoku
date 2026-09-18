// The bar-style provider keeps Sumi built into the shell and adapts optional
// styles from the canonical extras registry into receipt-owned Store products.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const barStyleTransactionKey = "ryoStoreBarStyleTransaction"
const barStyleScene = "Scene.qml"

type barStyleIndexRow struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Scene   string `json:"scene"`
	View    string `json:"view"`
}

type barProvider struct {
	cache       *Cache
	shellConfig string
}

var barStyleShellPatch = func(id string) error {
	return exec.Command("ryoku-shell", "barstyle", id).Run()
}

func newBarProvider(cache *Cache) barProvider {
	if cache == nil {
		cache = newCache()
	}
	return barProvider{
		cache:       cache,
		shellConfig: defaultShellConfigPath(),
	}
}

func (barProvider) Category() Category {
	return Category{
		ID:          "barstyles",
		Name:        "Bar styles",
		Group:       "wear",
		Description: "Complete bar compositions for the Ryoku shell.",
	}
}

func (p barProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	local, err := rebuildBarStyleIndex()
	if err != nil {
		return nil, SourceState{}, err
	}

	active := p.activeStyle()
	items := []Item{{
		ID: "sumi", Category: "barstyles", Name: "Sumi",
		Summary:     "Ink spine",
		Description: "The built-in left rail: paper, ink, and a vertical working edge.",
		Tags:        []string{"rail", "vertical", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "", "core": true},
	}}
	items = append(items, Item{
		ID: "qsbar", Category: "barstyles", Name: "QS Bar",
		Summary:     "Hancore top bar",
		Description: "The default top bar: Hancore's Quickshell Rise, ported to Ryoku.",
		Tags:        []string{"top", "horizontal", "built-in"},
		Installed:   true,
		Active:      active == "qsbar",
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	})
	items = append(items, Item{
		ID: "chroma", Category: "barstyles", Name: "Chroma",
		Summary:     "Matugen signal bar",
		Description: "Chroma Shell's modular top bar, rebuilt on Ryoku's native services and live palette.",
		Tags:        []string{"top", "horizontal", "built-in", "matugen"},
		Installed:   true,
		Active:      active == "chroma",
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	})
	items = append(items, Item{
		ID: "kairos", Category: "barstyles", Name: "Kairos",
		Summary:     "Island clock",
		Description: "One floating island at the top of the screen: the clock that grows on hover.",
		Tags:        []string{"top", "island", "built-in"},
		Installed:   true,
		Active:      active == "kairos",
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	})

	entries, state, registryErr := loadProductRegistry(ctx, p.cache, "barstyles", refresh)
	if registryErr != nil && !barStyleRegistryUnavailable(registryErr) {
		return nil, state, registryErr
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		item, err := productEntryItem(p.cache.base, "barstyles", entry)
		if err != nil {
			return nil, state, fmt.Errorf("barstyles/%s: installed state: %w", entry.ID, err)
		}
		item.Active = item.Installed && entry.ID == active
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata["scene"] = barStyleScene
		items = append(items, item)
		seen[entry.ID] = true
	}
	for _, row := range local {
		if seen[row.ID] {
			continue
		}
		items = append(items, Item{
			ID: row.ID, Category: "barstyles", Name: row.ID,
			Version: row.Version, InstalledVersion: row.Version,
			Installed: true, Active: row.ID == active,
			Metadata: map[string]any{"scene": barStyleScene},
		})
	}
	if registryErr != nil {
		state.Offline = true
	}
	for i := 1; i < len(items); i++ {
		if items[i].Active {
			items[0].Active = false
			return items, state, nil
		}
	}
	items[0].Active = true
	return items, state, nil
}

func barStyleRegistryUnavailable(err error) bool {
	var requestErr *url.Error
	if errors.As(err, &requestErr) {
		return true
	}
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.Status >= 500
}

func (p barProvider) Install(ctx context.Context, id string) error {
	if id == "sumi" || id == "qsbar" || id == "chroma" || id == "kairos" {
		return fmt.Errorf("the built-in %s bar style is already installed", id)
	}
	entries, _, err := loadProductRegistry(ctx, p.cache, "barstyles", false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	return installProduct(ctx, p.cache, "barstyles", entry)
}

func (p barProvider) Remove(ctx context.Context, id string) error {
	if id == "sumi" || id == "qsbar" || id == "chroma" || id == "kairos" {
		return fmt.Errorf("the built-in %s bar style is not removable", id)
	}
	return removeProduct(ctx, "barstyles", id)
}

func barStyleIndexPath() string {
	return filepath.Join(storeStateDir(), "barstyles.json")
}

func installedBarStyleRows() ([]barStyleIndexRow, error) {
	directory := filepath.Join(storeStateDir(), "barstyles")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []barStyleIndexRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := make([]barStyleIndexRow, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		// A broken barstyle receipt (bad name, unreadable, missing Scene.qml, or
		// drifted from disk) is quarantined: skipped so it can't error the
		// provider and report the store offline. It reads as installable again
		// and a reinstall heals it.
		if !productIDPattern.MatchString(id) {
			continue
		}
		receipt, err := readReceipt("barstyles", id)
		if err != nil {
			continue
		}
		dst, _, err := productDestination("barstyles", id)
		if err != nil {
			continue
		}
		if !receiptOwnsFile(receipt, barStyleScene) {
			continue
		}
		if err := verifyInstalledReceipt(dst, receipt); err != nil {
			continue
		}
		row := barStyleIndexRow{ID: id, Version: receipt.Version, Scene: barStyleScene}
		row.View = barStyleView(row, receipt)
		rows = append(rows, row)
	}
	return rows, nil
}

func rebuildBarStyleIndex() ([]barStyleIndexRow, error) {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := recoverStoreTransactions(); err != nil {
		return nil, err
	}
	return rebuildBarStyleIndexLocked()
}

func writeBarStyleIndexLocked() error {
	_, err := rebuildBarStyleIndexLocked()
	return err
}

func rebuildBarStyleIndexLocked() ([]barStyleIndexRow, error) {
	rows, err := installedBarStyleRows()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := ensureBarStyleView(row); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(barStyleIndexPath(), append(raw, '\n'), 0o600); err != nil {
		return nil, err
	}
	return rows, nil
}

// QML caches every relative dependency by URL, so the whole product tree needs
// a durable content-specific copy; changing only Scene.qml's URL leaves stale children.
func barStyleView(row barStyleIndexRow, receipt Receipt) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(row.ID))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(receipt.Version))
	for _, file := range receipt.Files {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(file.Destination))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(file.SHA256))
	}
	return filepath.ToSlash(filepath.Join("barstyle-views", row.ID, fmt.Sprintf("%x", digest.Sum(nil))))
}

func ensureBarStyleView(row barStyleIndexRow) error {
	destination, _, err := productDestination("barstyles", row.ID)
	if err != nil {
		return err
	}
	receipt, err := readReceipt("barstyles", row.ID)
	if err != nil {
		return err
	}
	if row.View != barStyleView(row, receipt) {
		return fmt.Errorf("barstyles/%s changed while rebuilding its view", row.ID)
	}
	view := filepath.Join(storeStateDir(), filepath.FromSlash(row.View))
	viewExists := false
	if _, err := os.Lstat(view); err == nil {
		viewExists = true
		if validateBarStyleView(view, receipt) == nil {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(view), 0o700); err != nil {
		return err
	}
	temp := view + ".tmp"
	backup := view + ".old"
	if err := os.RemoveAll(temp); err != nil {
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.MkdirAll(temp, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	for _, file := range receipt.Files {
		source := filepath.Join(destination, filepath.FromSlash(file.Destination))
		target := filepath.Join(temp, filepath.FromSlash(file.Destination))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if file.Mode == "0755" {
			mode = 0o755
		}
		if err := copyBarStyleViewFile(source, target, mode); err != nil {
			return err
		}
	}
	if err := validateBarStyleView(temp, receipt); err != nil {
		return err
	}
	if viewExists {
		if err := os.Rename(view, backup); err != nil {
			return err
		}
		defer os.Rename(backup, view)
	}
	if err := os.Rename(temp, view); err != nil {
		return err
	}
	return os.RemoveAll(backup)
}

func validateBarStyleView(view string, receipt Receipt) error {
	info, err := os.Lstat(view)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("barstyle view is not a directory: %s", view)
	}
	for _, file := range receipt.Files {
		path := filepath.Join(view, filepath.FromSlash(file.Destination))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("barstyle view file is not regular: %s", path)
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

func copyBarStyleViewFile(source, target string, mode os.FileMode) error {
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

func receiptOwnsFile(receipt Receipt, destination string) bool {
	for _, file := range receipt.Files {
		if file.Destination == destination {
			return true
		}
	}
	return false
}

func defaultShellConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "shell.json")
}

func setBarStyleSelection(path, id string) error {
	if path == defaultShellConfigPath() && barStyleShellPatch(id) == nil {
		return nil
	}
	unlock, err := lockShellConfig(path)
	if err != nil {
		return err
	}
	defer unlock()
	return writeBarStyleSelection(path, id)
}

func lockShellConfig(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func writeBarStyleSelection(path, id string) error {
	state := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := decodeOneJSON(raw, &state); err != nil {
			return fmt.Errorf("parse shell config: %w", err)
		}
		if state == nil {
			return fmt.Errorf("parse shell config: root must be an object")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	state["barStyle"] = id
	return writeShellConfigState(path, state)
}

func writeShellConfigState(path string, state map[string]any) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(raw, '\n'), 0o644)
}

// The marker makes the selection write part of the removable transaction:
// recovery reverts only its own fallback and preserves a newer user selection.
func applyBarStyleFallback(path, id, token string) error {
	return mutateBarStyleSelection(path, func(state map[string]any) bool {
		if state["barStyle"] != id {
			return false
		}
		state["barStyle"] = "sumi"
		state[barStyleTransactionKey] = token
		return true
	})
}

func finishBarStyleFallback(path, id, token string, committed bool) error {
	return mutateBarStyleSelection(path, func(state map[string]any) bool {
		marker, owned := state[barStyleTransactionKey].(string)
		owned = owned && marker == token
		changed := false
		if committed {
			if state["barStyle"] == id {
				state["barStyle"] = "sumi"
				changed = true
			}
		} else if owned && state["barStyle"] == "sumi" {
			state["barStyle"] = id
			changed = true
		}
		if owned {
			delete(state, barStyleTransactionKey)
			changed = true
		}
		return changed
	})
}

func mutateBarStyleSelection(path string, mutate func(map[string]any) bool) error {
	unlock, err := lockShellConfig(path)
	if err != nil {
		return err
	}
	defer unlock()
	state := map[string]any{}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err := decodeOneJSON(raw, &state); err != nil {
			return fmt.Errorf("parse shell config: %w", err)
		}
		if state == nil {
			return fmt.Errorf("parse shell config: root must be an object")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if !mutate(state) {
		return nil
	}
	return writeShellConfigState(path, state)
}

func (p barProvider) activeStyle() string {
	return readBarStyleSelection(p.shellConfig)
}

func readBarStyleSelection(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var state struct {
		BarStyle string `json:"barStyle"`
	}
	if decodeOneJSON(raw, &state) != nil {
		return ""
	}
	return state.BarStyle
}
