package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	errProductTransactionInterrupted = errors.New("product transaction interrupted")
	productTransactionCheckpoint     = func(string) error { return nil }
	removeProductArtifact            = os.RemoveAll
	writeProductRevision             = writeStoreRevision
)

type productTransactionJournal struct {
	Schema                  int             `json:"schema"`
	Category                string          `json:"category"`
	ID                      string          `json:"id"`
	Version                 string          `json:"version"`
	Operation               string          `json:"operation"`
	Phase                   string          `json:"phase"`
	HadDestination          bool            `json:"hadDestination,omitempty"`
	Adopted                 bool            `json:"adopted,omitempty"`
	BaseRevision            uint64          `json:"baseRevision"`
	PriorReceipt            *Receipt        `json:"priorReceipt,omitempty"`
	SelectionToken          string          `json:"selectionToken,omitempty"`
	PluginPlacement         json.RawMessage `json:"pluginPlacement,omitempty"`
	PluginPlacementPresent  bool            `json:"pluginPlacementPresent,omitempty"`
	PluginConfigPresent     bool            `json:"pluginConfigPresent,omitempty"`
	PluginPlacementCaptured bool            `json:"pluginPlacementCaptured,omitempty"`
}

// installProduct runs the install transaction fetching the manifest and file
// bytes from the cache.
func installProduct(ctx context.Context, cache *Cache, category string, entry ProductEntry) error {
	return installProductFrom(ctx, cache, category, entry, nil)
}

// installProductFrom is the install transaction. With local nil it fetches the
// manifest and every file from the cache; with local set it takes the manifest
// and reads the file bytes from a local directory instead. Every other check is
// identical either way: destination allowlist, symlink rejection, per-file hash
// verification, receipt, journal, setPluginPlacementEnabled, and syncProductDerivedState.
func installProductFrom(ctx context.Context, cache *Cache, category string, entry ProductEntry, local *localProductSource) error {
	dst, expectedDestination, err := productDestination(category, entry.ID)
	if err != nil {
		return err
	}
	if err := rejectSymlinkPath(productDestinationRoot(category), filepath.FromSlash(expectedDestination)); err != nil {
		return err
	}
	// A source can pause downloads of a still-listed product, or list one that is
	// written for another window manager, without delisting it. Re-read the
	// authoritative registry (never the provider's cached listing, never a
	// client-supplied field) and refuse before any manifest or payload byte is
	// fetched. A local install carries no registry and skips it.
	if local == nil {
		if err := assertProductInstallable(ctx, cache, category, entry.ID); err != nil {
			return err
		}
	}
	var manifest ProductManifest
	if local != nil {
		manifest = local.manifest
		if err := validateProductManifest(category, entry, manifest); err != nil {
			return err
		}
	} else {
		manifest, err = loadProductManifest(ctx, cache, category, entry)
		if err != nil {
			return err
		}
	}
	if manifest.Destination != expectedDestination {
		return fmt.Errorf("%s/%s: destination %q is outside the category allowlist", category, entry.ID, manifest.Destination)
	}

	globalUnlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return err
	}
	defer globalUnlock()
	if err := recoverStoreTransactions(); err != nil {
		return err
	}
	if err := rejectSymlinkPath(productDestinationRoot(category), filepath.FromSlash(expectedDestination)); err != nil {
		return err
	}
	unlock, err := lockTree(dst)
	if err != nil {
		return err
	}
	defer unlock()
	if err := cleanupProductStages(filepath.Dir(dst), entry.ID); err != nil {
		return err
	}

	prior, receiptErr := readReceipt(category, entry.ID)
	hadReceipt := receiptErr == nil
	if receiptErr != nil && !os.IsNotExist(receiptErr) {
		return receiptErr
	}
	info, destinationErr := os.Lstat(dst)
	hadDestination := destinationErr == nil
	if destinationErr != nil && !os.IsNotExist(destinationErr) {
		return destinationErr
	}
	adopting := false
	if hadDestination {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", dst)
		}
		if !info.IsDir() {
			return fmt.Errorf("tracked destination %s is not a directory", dst)
		}
		if !hadReceipt {
			// Adopt a pre-receipt install: a receipt-less directory that still
			// carries a product manifest.json predates the receipt system (or a
			// failed migration), not foreign user data, so replace it (the
			// transaction backs up the old tree) and write a receipt below rather
			// than refuse. A directory with no manifest is genuinely untracked.
			if mInfo, mErr := os.Stat(filepath.Join(dst, "manifest.json")); mErr != nil || mInfo.IsDir() {
				return fmt.Errorf("refusing untracked destination %s", dst)
			}
			adopting = true
		}
	}
	if hadReceipt && prior.Destination != expectedDestination {
		return fmt.Errorf("receipt destination %q is outside the category allowlist", prior.Destination)
	}
	backup := productInstallBackupPath(dst)
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("reserved transaction backup exists: %s", backup)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dst), ".ryostore-stage-"+entry.ID+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	receiptFiles := make([]ReceiptFile, 0, len(manifest.Files))
	for index, file := range manifest.Files {
		if !file.Install {
			continue
		}
		var data []byte
		if local != nil {
			data, err = readLocalProductFile(local.root, file.Source, file.Size, file.SHA256)
		} else {
			rel := path.Join(entry.Path, file.Source)
			data, err = fetchProductFile(ctx, cache, rel, file.Size, file.SHA256)
		}
		if err != nil {
			return fmt.Errorf("%s/%s: files[%d] %s: %w", category, entry.ID, index, file.Source, err)
		}
		target := filepath.Join(stage, filepath.FromSlash(file.Destination))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if file.Mode == "0755" {
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return err
		}
		if err := os.Chmod(target, mode); err != nil {
			return err
		}
		receiptFiles = append(receiptFiles, ReceiptFile{
			Source:      file.Source,
			Destination: file.Destination,
			SHA256:      file.SHA256,
			Mode:        file.Mode,
			Size:        file.Size,
		})
	}
	if len(receiptFiles) == 0 {
		return fmt.Errorf("%s/%s: manifest has no installable files", category, entry.ID)
	}

	receipt := Receipt{
		Category:    category,
		ID:          entry.ID,
		Version:     entry.Version,
		Destination: expectedDestination,
		Files:       receiptFiles,
	}
	operation := "install"
	if hadReceipt {
		operation = "update"
	}
	baseRevision, err := currentStoreRevisionNumber()
	if err != nil {
		return err
	}
	journal := productTransactionJournal{
		Schema:         1,
		Category:       category,
		ID:             entry.ID,
		Version:        entry.Version,
		Operation:      operation,
		Phase:          "install-prepared",
		BaseRevision:   baseRevision,
		HadDestination: hadDestination,
		Adopted:        adopting,
	}
	if hadReceipt {
		priorCopy := prior
		journal.PriorReceipt = &priorCopy
	}
	if category == "plugins" && operation == "install" {
		placement, present, configPresent, err := snapshotPluginPlacement(entry.ID)
		if err != nil {
			return err
		}
		journal.PluginPlacement = placement
		journal.PluginPlacementPresent = present
		journal.PluginConfigPresent = configPresent
		journal.PluginPlacementCaptured = true
	}
	if err := writeProductJournal(journal); err != nil {
		return err
	}

	rollback := func(cause error) error {
		if errors.Is(cause, errProductTransactionInterrupted) {
			return cause
		}
		if rollbackErr := rollbackProductJournal(journal); rollbackErr != nil {
			return fmt.Errorf("%w; rollback: %v", cause, rollbackErr)
		}
		return cause
	}
	if hadDestination {
		if err := os.Rename(dst, backup); err != nil {
			return rollback(err)
		}
	}
	if err := os.Rename(stage, dst); err != nil {
		return rollback(err)
	}
	journal.Phase = "install-published"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		return rollback(err)
	}
	if err := writeReceipt(receipt); err != nil {
		return rollback(err)
	}
	journal.Phase = "install-receipt"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		return rollback(err)
	}
	if err := verifyInstalledReceipt(dst, receipt); err != nil {
		return rollback(err)
	}
	if category == "plugins" && operation == "install" {
		if err := setPluginPlacementEnabled(entry.ID, pluginAutoEnable(dst)); err != nil {
			return rollback(err)
		}
		journal.Phase = "install-placement"
		if err := writeProductJournal(journal); err != nil {
			return rollback(err)
		}
		if err := productTransactionCheckpoint(journal.Phase); err != nil {
			return rollback(err)
		}
	}
	if err := syncProductDerivedState(journal, true); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint("derived-state"); err != nil {
		return rollback(err)
	}
	journal.Phase = "ready"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		if errors.Is(err, errProductTransactionInterrupted) {
			return err
		}
		return beginProductRollback(journal, err)
	}
	if err := writeProductRevision(journalRevision(journal)); err != nil {
		return beginProductRollback(journal, err)
	}
	_ = cleanupCommittedProductJournal(journal)
	return nil
}

func removeProduct(_ context.Context, category, id string) error {
	dst, expectedDestination, err := productDestination(category, id)
	if err != nil {
		return err
	}
	if err := rejectSymlinkPath(productDestinationRoot(category), filepath.FromSlash(expectedDestination)); err != nil {
		return err
	}
	globalUnlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return err
	}
	defer globalUnlock()
	if err := recoverStoreTransactions(); err != nil {
		return err
	}
	if err := rejectSymlinkPath(productDestinationRoot(category), filepath.FromSlash(expectedDestination)); err != nil {
		return err
	}
	unlock, err := lockTree(dst)
	if err != nil {
		return err
	}
	defer unlock()

	receipt, err := readReceipt(category, id)
	if err != nil {
		return err
	}
	if receipt.Destination != expectedDestination {
		return fmt.Errorf("receipt destination %q is outside the category allowlist", receipt.Destination)
	}
	for _, file := range receipt.Files {
		source := filepath.Join(dst, filepath.FromSlash(file.Destination))
		if err := rejectSymlinkPath(dst, filepath.FromSlash(file.Destination)); err != nil {
			return err
		}
		info, err := os.Lstat(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", source)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("owned path %s is not a regular file", source)
		}
	}

	hold := productRemovalHoldPath(dst)
	if _, err := os.Lstat(hold); err == nil {
		return fmt.Errorf("reserved removal hold exists: %s", hold)
	} else if !os.IsNotExist(err) {
		return err
	}
	baseRevision, err := currentStoreRevisionNumber()
	if err != nil {
		return err
	}
	selectionToken := ""
	if category == "barstyles" {
		selectionToken = fmt.Sprintf("%s-%d", id, time.Now().UnixNano())
	}
	journal := productTransactionJournal{
		Schema:         1,
		Category:       category,
		ID:             id,
		Version:        receipt.Version,
		Operation:      "remove",
		Phase:          "remove-prepared",
		BaseRevision:   baseRevision,
		PriorReceipt:   &receipt,
		SelectionToken: selectionToken,
	}
	if err := writeProductJournal(journal); err != nil {
		return err
	}
	rollback := func(cause error) error {
		if errors.Is(cause, errProductTransactionInterrupted) {
			return cause
		}
		return rollbackProductJournalWithCause(journal, cause)
	}
	if err := os.MkdirAll(hold, 0o700); err != nil {
		return rollback(err)
	}
	for _, file := range receipt.Files {
		source := filepath.Join(dst, filepath.FromSlash(file.Destination))
		if _, err := os.Lstat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return rollback(err)
		}
		target := filepath.Join(hold, filepath.FromSlash(file.Destination))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return rollback(err)
		}
		if err := os.Rename(source, target); err != nil {
			return rollback(err)
		}
	}
	pruneEmptyProductDirs(dst, receipt.Files)
	journal.Phase = "remove-files-moved"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		return rollback(err)
	}
	if err := os.Remove(receiptPath(category, id)); err != nil {
		return rollback(err)
	}
	journal.Phase = "remove-receipt"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		return rollback(err)
	}
	if err := syncProductDerivedState(journal, true); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint("derived-state"); err != nil {
		return rollback(err)
	}
	journal.Phase = "ready"
	if err := writeProductJournal(journal); err != nil {
		return rollback(err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		if errors.Is(err, errProductTransactionInterrupted) {
			return err
		}
		return beginProductRollback(journal, err)
	}
	if err := writeProductRevision(journalRevision(journal)); err != nil {
		return beginProductRollback(journal, err)
	}
	_ = cleanupCommittedProductJournal(journal)
	return nil
}

func recoverStoreTransactions() error {
	root := storeTransactionsDir()
	for _, category := range []string{"rices", "lockscreens", "barstyles", "fastfetch", "plugins", "bundles"} {
		directory := filepath.Join(root, category)
		entries, err := os.ReadDir(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			journalPath := filepath.Join(directory, entry.Name())
			info, err := os.Lstat(journalPath)
			if err != nil {
				return err
			}
			if strings.HasPrefix(entry.Name(), ".tmp-") && info.Mode().IsRegular() {
				if err := os.Remove(journalPath); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
				return fmt.Errorf("invalid Store transaction journal %s", journalPath)
			}
			raw, err := os.ReadFile(journalPath)
			if err != nil {
				return err
			}
			var journal productTransactionJournal
			if err := decodeOneJSON(raw, &journal); err != nil {
				return fmt.Errorf("Store transaction journal: %w", err)
			}
			if err := validateProductJournal(journal); err != nil {
				return err
			}
			if entry.Name() != journal.ID+".json" || journal.Category != category {
				return fmt.Errorf("Store transaction journal path does not match %s/%s", journal.Category, journal.ID)
			}
			if err := recoverProductJournal(journal); err != nil {
				return err
			}
		}
	}
	return nil
}

func recoverProductJournal(journal productTransactionJournal) error {
	dst, expected, err := productDestination(journal.Category, journal.ID)
	if err != nil {
		return err
	}
	if err := rejectSymlinkPath(productDestinationRoot(journal.Category), filepath.FromSlash(expected)); err != nil {
		return err
	}
	unlock, err := lockTree(dst)
	if err != nil {
		return err
	}
	defer unlock()
	if journal.Phase == "ready" {
		if err := syncProductDerivedState(journal, true); err != nil {
			return err
		}
		current, err := readStoreRevision()
		if os.IsNotExist(err) {
			current = StoreRevision{}
		} else if err != nil {
			return err
		}
		switch {
		case current.Revision == journal.BaseRevision:
			if err := writeProductRevision(journalRevision(journal)); err != nil {
				return err
			}
		case journal.BaseRevision < math.MaxUint64 &&
			current.Revision == journal.BaseRevision+1 &&
			storeRevisionChangeMatches(current, journalRevision(journal)):
		default:
			return fmt.Errorf("Store revision no longer matches transaction baseline %d", journal.BaseRevision)
		}
		return cleanupCommittedProductJournal(journal)
	}
	return rollbackProductJournal(journal)
}

func beginProductRollback(journal productTransactionJournal, cause error) error {
	switch journal.Operation {
	case "install", "update":
		journal.Phase = "install-rollback"
	case "remove":
		journal.Phase = "remove-rollback"
	default:
		return fmt.Errorf("%w; rollback: invalid transaction operation %q", cause, journal.Operation)
	}
	if err := writeProductJournal(journal); err != nil {
		return fmt.Errorf("%w; record rollback intent: %v", cause, err)
	}
	if err := productTransactionCheckpoint(journal.Phase); err != nil {
		if errors.Is(err, errProductTransactionInterrupted) {
			return err
		}
		cause = fmt.Errorf("%w; rollback checkpoint: %v", cause, err)
	}
	return rollbackProductJournalWithCause(journal, cause)
}

func rollbackProductJournalWithCause(journal productTransactionJournal, cause error) error {
	if err := rollbackProductJournal(journal); err != nil {
		return fmt.Errorf("%w; rollback: %v", cause, err)
	}
	return cause
}

func rollbackProductJournal(journal productTransactionJournal) error {
	dst, _, err := productDestination(journal.Category, journal.ID)
	if err != nil {
		return err
	}
	switch journal.Operation {
	case "install", "update":
		backup := productInstallBackupPath(dst)
		_, backupErr := os.Lstat(backup)
		if backupErr == nil {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
			if err := os.Rename(backup, dst); err != nil {
				return err
			}
		} else if !os.IsNotExist(backupErr) {
			return backupErr
		} else if !journal.HadDestination {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
		}
		if journal.PriorReceipt != nil {
			if err := writeReceipt(*journal.PriorReceipt); err != nil {
				return err
			}
		} else if err := os.Remove(receiptPath(journal.Category, journal.ID)); err != nil && !os.IsNotExist(err) {
			return err
		}
	case "remove":
		if journal.PriorReceipt == nil {
			return fmt.Errorf("removal journal has no prior receipt")
		}
		hold := productRemovalHoldPath(dst)
		holdInfo, err := os.Lstat(hold)
		holdExists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if holdExists && (!holdInfo.IsDir() || holdInfo.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("removal hold %s is not a directory", hold)
		}
		for _, file := range journal.PriorReceipt.Files {
			relative := filepath.FromSlash(file.Destination)
			if err := rejectSymlinkPath(hold, relative); err != nil {
				return err
			}
			if err := rejectSymlinkPath(dst, relative); err != nil {
				return err
			}
			source := filepath.Join(hold, relative)
			if _, err := os.Lstat(source); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return err
			}
			target := filepath.Join(dst, relative)
			if _, err := os.Lstat(target); err == nil {
				return fmt.Errorf("cannot restore owned path over %s", target)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		for _, file := range journal.PriorReceipt.Files {
			relative := filepath.FromSlash(file.Destination)
			source := filepath.Join(hold, relative)
			if _, err := os.Lstat(source); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return err
			}
			target := filepath.Join(dst, relative)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Rename(source, target); err != nil {
				return err
			}
		}
		if err := writeReceipt(*journal.PriorReceipt); err != nil {
			return err
		}
		if err := os.RemoveAll(hold); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid transaction operation %q", journal.Operation)
	}
	if journal.Category == "plugins" && journal.Operation == "install" && journal.PluginPlacementCaptured {
		if err := restorePluginPlacement(
			journal.ID, journal.PluginPlacement, journal.PluginPlacementPresent, journal.PluginConfigPresent,
		); err != nil {
			return err
		}
	}
	if err := syncProductDerivedState(journal, false); err != nil {
		return err
	}
	if err := cleanupProductStages(filepath.Dir(dst), journal.ID); err != nil {
		return err
	}
	if err := os.Remove(productJournalPath(journal.Category, journal.ID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cleanupCommittedProductJournal(journal productTransactionJournal) error {
	if journal.Category == "barstyles" && journal.Operation == "remove" {
		if err := finishBarStyleFallback(defaultShellConfigPath(), journal.ID, journal.SelectionToken, true); err != nil {
			return err
		}
	}
	dst, _, err := productDestination(journal.Category, journal.ID)
	if err != nil {
		return err
	}
	artifact := productInstallBackupPath(dst)
	if journal.Operation == "remove" {
		artifact = productRemovalHoldPath(dst)
	}
	if err := removeProductArtifact(artifact); err != nil {
		return err
	}
	if err := cleanupProductStages(filepath.Dir(dst), journal.ID); err != nil {
		return err
	}
	return os.Remove(productJournalPath(journal.Category, journal.ID))
}

func writeProductJournal(journal productTransactionJournal) error {
	if err := validateProductJournal(journal); err != nil {
		return err
	}
	raw, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return atomicWrite(productJournalPath(journal.Category, journal.ID), append(raw, '\n'), 0o600)
}

func validateProductJournal(journal productTransactionJournal) error {
	if journal.Schema != 1 || !validProductCategory(journal.Category) || !productIDPattern.MatchString(journal.ID) || journal.Version == "" {
		return fmt.Errorf("invalid product transaction journal identity")
	}
	if journal.PriorReceipt != nil {
		if err := validateReceipt(journal.Category, journal.ID, *journal.PriorReceipt); err != nil {
			return fmt.Errorf("invalid prior receipt: %w", err)
		}
	}
	if journal.SelectionToken != "" && (journal.Category != "barstyles" || journal.Operation != "remove") {
		return fmt.Errorf("invalid product selection transaction")
	}
	if journal.Category == "barstyles" && journal.Operation == "remove" && journal.SelectionToken == "" {
		return fmt.Errorf("barstyle removal has no selection transaction")
	}
	if journal.Category == "plugins" && journal.Operation == "install" {
		if !journal.PluginPlacementCaptured {
			return fmt.Errorf("plugin install journal has no placement snapshot")
		}
	} else if journal.PluginPlacementCaptured || journal.PluginPlacementPresent || journal.PluginConfigPresent || len(journal.PluginPlacement) > 0 {
		return fmt.Errorf("non-plugin transaction has plugin placement state")
	}
	switch journal.Operation {
	case "install":
		if journal.PriorReceipt != nil || (journal.HadDestination && !journal.Adopted) {
			return fmt.Errorf("install journal unexpectedly owns prior state")
		}
		if journal.Phase != "install-prepared" && journal.Phase != "install-published" && journal.Phase != "install-receipt" && journal.Phase != "install-placement" && journal.Phase != "install-rollback" && journal.Phase != "ready" {
			return fmt.Errorf("invalid install transaction phase %q", journal.Phase)
		}
	case "update":
		if journal.PriorReceipt == nil {
			return fmt.Errorf("update journal has no prior receipt")
		}
		if journal.Phase != "install-prepared" && journal.Phase != "install-published" && journal.Phase != "install-receipt" && journal.Phase != "install-rollback" && journal.Phase != "ready" {
			return fmt.Errorf("invalid install transaction phase %q", journal.Phase)
		}
	case "remove":
		if journal.PriorReceipt == nil || journal.PriorReceipt.Version != journal.Version {
			return fmt.Errorf("removal journal has no matching prior receipt")
		}
		if journal.Phase != "remove-prepared" && journal.Phase != "remove-files-moved" && journal.Phase != "remove-receipt" && journal.Phase != "remove-rollback" && journal.Phase != "ready" {
			return fmt.Errorf("invalid removal transaction phase %q", journal.Phase)
		}
	default:
		return fmt.Errorf("invalid transaction operation %q", journal.Operation)
	}
	return nil
}

func syncProductDerivedState(journal productTransactionJournal, committed bool) error {
	if journal.Category == "plugins" {
		return writePluginIndexLocked()
	}
	if journal.Category != "barstyles" {
		return nil
	}
	if err := writeBarStyleIndexLocked(); err != nil {
		return err
	}
	if journal.Operation != "remove" {
		return nil
	}
	if committed {
		return applyBarStyleFallback(defaultShellConfigPath(), journal.ID, journal.SelectionToken)
	}
	return finishBarStyleFallback(defaultShellConfigPath(), journal.ID, journal.SelectionToken, false)
}

func journalRevision(journal productTransactionJournal) StoreRevision {
	return StoreRevision{Category: journal.Category, ID: journal.ID, Version: journal.Version, Operation: journal.Operation}
}

func currentStoreRevisionNumber() (uint64, error) {
	current, err := readStoreRevision()
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return current.Revision, nil
}

func storeRevisionChangeMatches(current, change StoreRevision) bool {
	return current.Category == change.Category &&
		current.ID == change.ID &&
		current.Version == change.Version &&
		current.Operation == change.Operation
}

func storeTransactionsDir() string {
	return filepath.Join(storeStateDir(), "transactions")
}

func storeTransactionLockPath() string {
	return filepath.Join(storeStateDir(), "transactions.lock")
}

func productJournalPath(category, id string) string {
	return filepath.Join(storeTransactionsDir(), category, id+".json")
}

func productInstallBackupPath(dst string) string {
	return filepath.Join(filepath.Dir(dst), ".ryostore-product-backup-"+filepath.Base(dst))
}

func productRemovalHoldPath(dst string) string {
	return filepath.Join(filepath.Dir(dst), ".ryostore-product-remove-"+filepath.Base(dst))
}

func productDestination(category, id string) (string, string, error) {
	if !validProductCategory(category) || !productIDPattern.MatchString(id) {
		return "", "", fmt.Errorf("invalid product identity %s/%s", category, id)
	}
	var relative string
	switch category {
	case "rices":
		relative = path.Join("ryoku", "rices", id)
	case "lockscreens":
		relative = path.Join("qylock", "themes", id)
	default:
		relative = path.Join("ryoku", category, id)
	}
	root := productDestinationRoot(category)
	return filepath.Join(root, filepath.FromSlash(relative)), relative, nil
}

func productDestinationRoot(category string) string {
	if category == "rices" {
		return configHome()
	}
	return dataHome()
}

func fetchProductFile(ctx context.Context, cache *Cache, rel string, size int64, expectedHash string) ([]byte, error) {
	if cache == nil || cache.client == nil || !validProductPath(rel) || size < 0 || size > maxProductFileSize || !productHashPattern.MatchString(expectedHash) {
		return nil, fmt.Errorf("invalid product fetch %q", rel)
	}
	data, fetchErr := fetchProductFileLive(ctx, cache, rel, size)
	if fetchErr == nil {
		if err := validateProductPayload(data, size, expectedHash); err != nil {
			return nil, err
		}
		cache.writeDisk(rel, data)
		return data, nil
	}
	cached := filepath.Join(cache.dir, filepath.FromSlash(rel))
	info, err := os.Lstat(cached)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > size {
		return nil, fetchErr
	}
	data, err = os.ReadFile(cached)
	if err != nil {
		return nil, fetchErr
	}
	if err := validateProductPayload(data, size, expectedHash); err != nil {
		return nil, fmt.Errorf("%w; cached payload: %v", fetchErr, err)
	}
	return data, nil
}

func fetchProductFileLive(ctx context.Context, cache *Cache, rel string, limit int64) ([]byte, error) {
	if root, ok := localBase(cache.base); ok {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("%s: exceeds %d bytes", rel, limit)
		}
		return data, nil
	}
	url := cache.base + "/" + rel
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s_=%d", url, separator, time.Now().UnixNano()), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Cache-Control", "no-cache")
	response, err := cache.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{URL: url, Status: response.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: response exceeds %d bytes", url, limit)
	}
	return data, nil
}

func validateProductPayload(data []byte, size int64, expectedHash string) error {
	if int64(len(data)) != size {
		return fmt.Errorf("payload size mismatch")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != expectedHash {
		return fmt.Errorf("payload hash mismatch")
	}
	return nil
}

func verifyInstalledReceipt(dst string, receipt Receipt) error {
	stored, err := readReceipt(receipt.Category, receipt.ID)
	if err != nil {
		return err
	}
	if stored.Version != receipt.Version || stored.Destination != receipt.Destination || len(stored.Files) != len(receipt.Files) {
		return fmt.Errorf("installed receipt did not persist")
	}
	for _, file := range receipt.Files {
		path := filepath.Join(dst, filepath.FromSlash(file.Destination))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != file.Size {
			return fmt.Errorf("installed file %s does not match receipt", path)
		}
	}
	return nil
}

func cleanupProductStages(parent, id string) error {
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	prefix := ".ryostore-stage-" + id + "-"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		candidate := filepath.Join(parent, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(candidate); err != nil {
				return err
			}
		} else if err := os.RemoveAll(candidate); err != nil {
			return err
		}
	}
	return nil
}

func pruneEmptyProductDirs(dst string, files []ReceiptFile) {
	directories := make(map[string]struct{})
	for _, file := range files {
		directory := filepath.Dir(filepath.Join(dst, filepath.FromSlash(file.Destination)))
		for directory != dst && strings.HasPrefix(directory, dst+string(filepath.Separator)) {
			directories[directory] = struct{}{}
			directory = filepath.Dir(directory)
		}
	}
	ordered := make([]string, 0, len(directories))
	for directory := range directories {
		ordered = append(ordered, directory)
	}
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, directory := range ordered {
		_ = os.Remove(directory)
	}
	_ = os.Remove(dst)
}

func (riceProvider) Remove(ctx context.Context, id string) error {
	return removeProduct(ctx, "rices", id)
}

func (fastfetchProvider) Remove(ctx context.Context, id string) error {
	return removeProduct(ctx, "fastfetch", id)
}

func (ryotunesSkinsProvider) Remove(ctx context.Context, id string) error {
	return removeProduct(ctx, "ryotunes-skins", id)
}

func (pluginProvider) Remove(ctx context.Context, id string) error {
	return removeProduct(ctx, "plugins", id)
}

func (bundleProvider) Remove(ctx context.Context, id string) error {
	return removeProduct(ctx, "bundles", id)
}
