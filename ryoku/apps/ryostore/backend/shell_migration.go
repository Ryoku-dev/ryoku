package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// migrateLegacyShellProducts converts bar styles installed as loose files
// under $XDG_DATA_HOME/ryoku/barstyles/<id>/ by earlier Ryoku releases. The
// old payload has the old store's layout, which the shell can no longer load,
// so a verified loose install is reinstalled from the catalogue as a packed
// item, in subshells/ when the catalogue lists the id there, else in
// barstyles/, and only then is the loose copy deleted. barStyle keeps its
// value, because the id is unchanged. A paused catalogue item still migrates:
// the user already has it. An id the catalogue no longer offers is removed and
// barStyle falls back to orpheus. While the catalogue cannot be read nothing
// is touched, and the next run tries again. A modified payload stays where it
// is. The old barstyle-views copies and barstyles.json index are removed.
func migrateLegacyShellProducts(ctx context.Context, cache *Cache) error {
	if cache == nil {
		return fmt.Errorf("shell migration cache is nil")
	}
	unlock, err := lockTree(filepath.Join(storeStateDir(), "legacy-shells"))
	if err != nil {
		return err
	}
	defer unlock()

	legacy, err := legacyShellReceipts()
	if err != nil {
		return err
	}
	var problems []error
	if len(legacy) > 0 {
		registries, err := loadShellRegistries(ctx, cache)
		if err != nil {
			return err
		}
		for _, receipt := range legacy {
			if err := migrateLegacyShellReceipt(ctx, cache, registries, receipt); err != nil {
				problems = append(problems, fmt.Errorf("barstyles/%s: %w", receipt.ID, err))
			}
		}
	}
	if err := removeLegacyBarStyleState(); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

// legacyShellReceipts are the barstyles receipts that own loose files instead
// of a bundle.
func legacyShellReceipts() ([]Receipt, error) {
	entries, err := os.ReadDir(filepath.Join(storeStateDir(), "barstyles"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receipts := []Receipt{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !productIDPattern.MatchString(id) {
			continue
		}
		receipt, err := readReceipt("barstyles", id)
		if err != nil || receipt.Shell != nil {
			continue
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

// loadShellRegistries reads the subshells and barstyles registries live. A
// cached or missing registry is an error: deciding that an id is gone needs
// the current catalogue.
func loadShellRegistries(ctx context.Context, cache *Cache) (map[string][]ProductEntry, error) {
	registries := map[string][]ProductEntry{}
	for _, category := range []string{"subshells", "barstyles"} {
		entries, state, err := loadProductRegistry(ctx, cache, category, true)
		if err != nil {
			return nil, err
		}
		if state.Offline {
			return nil, fmt.Errorf("%s registry is unreachable", category)
		}
		registries[category] = entries
	}
	return registries, nil
}

// shellCatalogueEntry finds id in the catalogue, subshells first.
func shellCatalogueEntry(registries map[string][]ProductEntry, id string) (string, ProductEntry, bool) {
	for _, category := range []string{"subshells", "barstyles"} {
		for _, entry := range registries[category] {
			if entry.ID == id {
				return category, entry, true
			}
		}
	}
	return "", ProductEntry{}, false
}

func migrateLegacyShellReceipt(ctx context.Context, cache *Cache, registries map[string][]ProductEntry, legacy Receipt) error {
	category, entry, offered := shellCatalogueEntry(registries, legacy.ID)
	if !offered {
		return removeProduct(ctx, "barstyles", legacy.ID)
	}
	oldDestination, _, err := productDestination("barstyles", legacy.ID)
	if err != nil {
		return err
	}
	matches, err := destinationMatchesReceipt(oldDestination, legacy)
	if err != nil {
		return err
	}
	if !matches {
		return fmt.Errorf("installed files differ from their receipt; left in place")
	}
	// In barstyles/ the install is an ordinary update of the legacy receipt:
	// the transaction swaps the whole folder for the verified bundle.
	if err := migrateProduct(ctx, cache, category, entry); err != nil {
		return err
	}
	if category == "barstyles" {
		return nil
	}
	return removeLegacyShellInstall(legacy, oldDestination)
}

// removeLegacyShellInstall deletes a verified loose install and its receipt
// without the removal transaction, which would also reset barStyle.
func removeLegacyShellInstall(legacy Receipt, destination string) error {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return err
	}
	defer unlock()
	current, err := readReceipt("barstyles", legacy.ID)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Shell != nil || current.Version != legacy.Version {
		return nil
	}
	if err := rejectSymlinkPath(productDestinationRoot("barstyles"), filepath.FromSlash(current.Destination)); err != nil {
		return err
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Remove(receiptPath("barstyles", legacy.ID))
}

// removeLegacyBarStyleState deletes what the old shell loaded bar styles from.
func removeLegacyBarStyleState() error {
	if err := os.RemoveAll(filepath.Join(storeStateDir(), "barstyle-views")); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(storeStateDir(), "barstyles.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
