package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func SystemReceiptPath(root, id, account string) string {
	return filepath.Join(root, id, "accounts", account+".json")
}
func StagedPayloadDir(root, id, version string) string {
	return filepath.Join(root, id, "payload", version)
}
func AccountReceiptPath(home string) string {
	return filepath.Join(home, ".local", "state", "ryoku", "profile", "receipt.json")
}

func ReadSystemReceipts(root string) ([]SystemReceipt, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []SystemReceipt{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SystemReceipt
	for _, profileDir := range entries {
		if !profileDir.IsDir() {
			continue
		}
		accountDir := filepath.Join(root, profileDir.Name(), "accounts")
		files, readErr := os.ReadDir(accountDir)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}
			body, readErr := os.ReadFile(filepath.Join(accountDir, file.Name()))
			if readErr != nil {
				return nil, readErr
			}
			var receipt SystemReceipt
			if err := json.Unmarshal(body, &receipt); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Join(accountDir, file.Name()), err)
			}
			out = append(out, receipt)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Profile.ID == out[j].Profile.ID {
			return out[i].Account.Name < out[j].Account.Name
		}
		return out[i].Profile.ID < out[j].Profile.ID
	})
	return out, nil
}

func ReadSystemReceipt(root, id, account string) (*SystemReceipt, error) {
	body, err := os.ReadFile(SystemReceiptPath(root, id, account))
	if err != nil {
		return nil, err
	}
	var receipt SystemReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func WriteSystemReceipt(root string, r *SystemReceipt) error {
	if r == nil {
		return fmt.Errorf("system receipt is nil")
	}
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicFile(SystemReceiptPath(root, r.Profile.ID, r.Account.Name), body, 0o644, 0o755)
}

func ReadAccountReceipt(home string) (*AccountReceipt, error) {
	body, err := os.ReadFile(AccountReceiptPath(home))
	if err != nil {
		return nil, err
	}
	var receipt AccountReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func WriteAccountReceipt(home string, r *AccountReceipt) error {
	if r == nil {
		return fmt.Errorf("account receipt is nil")
	}
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeHomeFile(home, ".local/state/ryoku/profile/receipt.json", body, 0o600)
}

func atomicFile(path string, body []byte, mode, dirMode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
