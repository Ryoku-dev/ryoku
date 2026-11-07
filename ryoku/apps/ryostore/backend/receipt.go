package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// ReceiptFile is one installed file owned by a Store receipt.
type ReceiptFile struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	Mode        string `json:"mode"`
	Size        int64  `json:"size"`
	// ArchiveMember marks a file packed inside bundle.tar.gz rather than on disk.
	ArchiveMember bool `json:"archiveMember,omitempty"`
}

// ReceiptComponent records bundle ownership without claiming pre-existing or
// independently shared components.
type ReceiptComponent struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	PreExisting      bool   `json:"preExisting"`
	InstalledByRyoku bool   `json:"installedByRyoku"`
}

// Receipt is the authoritative ownership record for one installed product.
type Receipt struct {
	Category    string             `json:"category"`
	ID          string             `json:"id"`
	Version     string             `json:"version"`
	Destination string             `json:"destination"`
	Files       []ReceiptFile      `json:"files"`
	Components  []ReceiptComponent `json:"components,omitempty"`
	// Name, Summary and Shell describe a packed subshell or bar style, so the
	// shells index is built without unpacking or fetching anything.
	Name    string         `json:"name,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Shell   *ShellManifest `json:"shell,omitempty"`
}

// StoreRevision is the last committed Store change. Revision increases once for
// every successful install, update, or removal.
type StoreRevision struct {
	Revision  uint64 `json:"revision"`
	Category  string `json:"category"`
	ID        string `json:"id"`
	Version   string `json:"version"`
	Operation string `json:"operation"`
}

func stateHome() string {
	if root := os.Getenv("XDG_STATE_HOME"); root != "" {
		return root
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state")
}

func storeStateDir() string {
	return filepath.Join(stateHome(), "ryoku", "store")
}

func receiptPath(category, id string) string {
	return filepath.Join(storeStateDir(), category, id+".json")
}

func storeRevisionPath() string {
	return filepath.Join(storeStateDir(), "revision.json")
}

func readReceipt(category, id string) (Receipt, error) {
	if !validProductCategory(category) || !productIDPattern.MatchString(id) {
		return Receipt{}, fmt.Errorf("invalid receipt identity %s/%s", category, id)
	}
	info, err := os.Lstat(receiptPath(category, id))
	if err != nil {
		return Receipt{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Receipt{}, fmt.Errorf("receipt %s/%s is not a regular file", category, id)
	}
	raw, err := os.ReadFile(receiptPath(category, id))
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := decodeOneJSON(raw, &receipt); err != nil {
		return Receipt{}, fmt.Errorf("receipt %s/%s: %w", category, id, err)
	}
	if err := validateReceipt(category, id, receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func writeReceipt(receipt Receipt) error {
	if err := validateReceipt(receipt.Category, receipt.ID, receipt); err != nil {
		return err
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return atomicWrite(receiptPath(receipt.Category, receipt.ID), raw, 0o600)
}

func validateReceipt(category, id string, receipt Receipt) error {
	label := category + "/" + id
	if !validProductCategory(category) || !productIDPattern.MatchString(id) {
		return fmt.Errorf("invalid receipt identity %s", label)
	}
	if receipt.Category != category || receipt.ID != id || receipt.Version == "" {
		return fmt.Errorf("receipt %s identity or version mismatch", label)
	}
	_, expected, err := productDestination(category, id)
	if err != nil {
		return err
	}
	if receipt.Destination != expected {
		return fmt.Errorf("receipt %s has forbidden destination %q", label, receipt.Destination)
	}
	seen := make(map[string]struct{}, len(receipt.Files))
	bundleCount := 0
	archiveMemberCount := 0
	for index, file := range receipt.Files {
		if !validProductPath(file.Source) || !validProductPath(file.Destination) {
			return fmt.Errorf("receipt %s file %d has invalid path", label, index)
		}
		key := file.Destination
		if file.ArchiveMember {
			key = "archive:" + key
			archiveMemberCount++
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("receipt %s has duplicate destination %q", label, file.Destination)
		}
		seen[key] = struct{}{}
		if !productHashPattern.MatchString(file.SHA256) || (file.Mode != "0644" && file.Mode != "0755") || file.Size < 0 || file.Size > maxProductFileSize {
			return fmt.Errorf("receipt %s file %d is invalid", label, index)
		}
		if !file.ArchiveMember && file.Destination == shellBundleName {
			bundleCount++
		}
	}
	if shellCategory(category) {
		legacyLoose := bundleCount == 0 && archiveMemberCount == 0 && receipt.Shell == nil
		if !legacyLoose {
			if bundleCount != 1 || archiveMemberCount == 0 || receipt.Shell == nil || receipt.Name == "" {
				return fmt.Errorf("receipt %s has invalid shell bundle metadata", label)
			}
			if err := validateShellManifest(label, *receipt.Shell); err != nil {
				return err
			}
		}
	} else if receipt.Shell != nil || bundleCount != 0 || archiveMemberCount != 0 {
		return fmt.Errorf("receipt %s has unexpected shell bundle metadata", label)
	}
	if category == vesktopThemesCategory {
		_, err := vesktopThemeReceiptFile(receipt.Files)
		return err
	}
	return nil
}

func receiptOwnsFile(receipt Receipt, destination string) bool {
	for _, file := range receipt.Files {
		if file.Destination == destination {
			return true
		}
	}
	return false
}

func writeStoreRevision(change StoreRevision) error {
	if !validProductCategory(change.Category) || !productIDPattern.MatchString(change.ID) || change.Version == "" {
		return fmt.Errorf("invalid Store revision identity")
	}
	switch change.Operation {
	case "install", "update", "remove":
	default:
		return fmt.Errorf("invalid Store revision operation %q", change.Operation)
	}

	path := storeRevisionPath()
	unlock, err := lockTree(path)
	if err != nil {
		return err
	}
	defer unlock()

	current, err := readStoreRevision()
	if os.IsNotExist(err) {
		current = StoreRevision{}
	} else if err != nil {
		return err
	}
	if current.Revision == math.MaxUint64 {
		return fmt.Errorf("Store revision overflow")
	}
	change.Revision = current.Revision + 1
	raw, err := json.Marshal(change)
	if err != nil {
		return err
	}
	return atomicWrite(path, append(raw, '\n'), 0o600)
}
func readStoreRevision() (StoreRevision, error) {
	raw, err := os.ReadFile(storeRevisionPath())
	if err != nil {
		return StoreRevision{}, err
	}
	var revision StoreRevision
	if err := decodeOneJSON(raw, &revision); err != nil {
		return StoreRevision{}, fmt.Errorf("Store revision: %w", err)
	}
	if revision.Revision == 0 || !validProductCategory(revision.Category) ||
		!productIDPattern.MatchString(revision.ID) || revision.Version == "" {
		return StoreRevision{}, fmt.Errorf("Store revision has invalid identity")
	}
	switch revision.Operation {
	case "install", "update", "remove":
	default:
		return StoreRevision{}, fmt.Errorf("Store revision has invalid operation %q", revision.Operation)
	}
	return revision, nil
}

func decodeOneJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
