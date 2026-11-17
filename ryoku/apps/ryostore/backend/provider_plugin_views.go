package main

import (
	"encoding/json"
	"path/filepath"
)

type pluginIndexRow struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	View    string `json:"view"`
}

func pluginIndexPath() string {
	return filepath.Join(storeStateDir(), "plugins.json")
}

func rebuildPluginIndex() ([]pluginIndexRow, error) {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := recoverStoreTransactions(); err != nil {
		return nil, err
	}
	return rebuildPluginIndexLocked()
}

func writePluginIndexLocked() error {
	_, err := rebuildPluginIndexLocked()
	return err
}

func rebuildPluginIndexLocked() ([]pluginIndexRow, error) {
	// Official and user store receipts alike: the collision rule keeps one
	// install per id, so the index and its views stay keyed by id. A broken
	// receipt (unreadable, malformed, or drifted from disk) is quarantined:
	// skipped so it can't blank the whole plugins tab. It then reads as
	// installable and a reinstall heals it; only index writes below are fatal.
	receipts, _, err := installedReceipts("plugins")
	if err != nil {
		return nil, err
	}
	rows := make([]pluginIndexRow, 0, len(receipts))
	seen := map[string]bool{}
	for _, receipt := range receipts {
		id := receipt.ID
		if seen[id] {
			continue
		}
		destination, _, err := sourceProductDestination(receipt.Source, "plugins", id)
		if err != nil {
			continue
		}
		if !receiptOwnsFile(receipt, "manifest.json") {
			continue
		}
		if err := verifyInstalledReceipt(destination, receipt); err != nil {
			continue
		}
		row := pluginIndexRow{ID: id, Version: receipt.Version}
		row.View = pluginViews.path(id, receipt)
		if err := pluginViews.ensure(id, row.View, receipt); err != nil {
			continue
		}
		seen[id] = true
		rows = append(rows, row)
	}
	live := make(map[string]string, len(rows))
	for _, row := range rows {
		live[row.ID] = row.View
	}
	if err := pluginViews.prune(live); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(pluginIndexPath(), append(raw, '\n'), 0o600); err != nil {
		return nil, err
	}
	return rows, nil
}
