package host

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

type storeProfileReceipt struct {
	Category    string                    `json:"category"`
	ID          string                    `json:"id"`
	Version     string                    `json:"version"`
	Destination string                    `json:"destination"`
	Files       []storeProfileReceiptFile `json:"files"`
}

type storeProfileReceiptFile struct {
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
}

func (a *App) callerUser() (localUser, error) {
	users, _, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return localUser{}, fmt.Errorf(i18n.T("Read the caller account: %v"), err)
	}
	uid := a.cfg.UID
	if a.cfg.UID == 0 {
		if value := a.getenv("PKEXEC_UID"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return localUser{}, fmt.Errorf(i18n.T("The authenticated caller identity is invalid."))
			}
			uid = parsed
		}
	}
	for _, user := range users {
		if user.UID == uid && filepath.IsAbs(user.Home) {
			return user, nil
		}
	}
	return localUser{}, fmt.Errorf(i18n.T("The caller account could not be found."))
}

func (a *App) verifiedProfileSource(source string) (profile.Source, error) {
	caller, err := a.callerUser()
	if err != nil {
		return profile.Source{}, err
	}
	source = filepath.Clean(source)
	id := filepath.Base(source)
	expected := filepath.Join(caller.Home, ".local", "share", "ryoku", "profiles", id)
	if source != expected {
		return profile.Source{}, fmt.Errorf(i18n.T("The profile source must be the installed Store copy for the caller."))
	}
	receiptPath := filepath.Join(caller.Home, ".local", "state", "ryoku", "store", "profiles", id+".json")
	if err := rejectSymlinkPath(caller.Home, source); err != nil {
		return profile.Source{}, reinstallProfileError(err)
	}
	if err := rejectSymlinkPath(caller.Home, receiptPath); err != nil {
		return profile.Source{}, reinstallProfileError(err)
	}
	receipt, err := readStoreProfileReceipt(receiptPath, id)
	if err != nil {
		return profile.Source{}, reinstallProfileError(err)
	}
	files := make([]profile.FileHash, 0, len(receipt.Files))
	manifestHash := ""
	seen := map[string]bool{}
	for _, file := range receipt.Files {
		rel, err := safeProfileRelative(file.Destination)
		if err != nil || seen[rel] || !validSHA256(file.SHA256) {
			if err == nil && seen[rel] {
				err = fmt.Errorf("duplicate receipt path %s", rel)
			}
			return profile.Source{}, reinstallProfileError(err)
		}
		seen[rel] = true
		path, err := regularFileBelow(source, rel)
		if err != nil {
			return profile.Source{}, reinstallProfileError(err)
		}
		digest, err := hashFile(path)
		if err != nil || digest != file.SHA256 {
			if err == nil {
				err = fmt.Errorf("hash mismatch for %s", rel)
			}
			return profile.Source{}, reinstallProfileError(err)
		}
		files = append(files, profile.FileHash{Path: filepath.ToSlash(rel), SHA256: digest})
		if filepath.ToSlash(rel) == "profile.json" {
			manifestHash = digest
		}
	}
	if manifestHash == "" {
		return profile.Source{}, reinstallProfileError(fmt.Errorf("profile.json is not receipt-owned"))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return profile.Source{Dir: source, ManifestSHA256: manifestHash, Files: files}, nil
}

func readStoreProfileReceipt(path, id string) (storeProfileReceipt, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return storeProfileReceipt{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return storeProfileReceipt{}, fmt.Errorf("receipt is not a regular file")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return storeProfileReceipt{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		syscall.Close(fd)
		return storeProfileReceipt{}, fmt.Errorf("open receipt")
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 8<<20))
	var receipt storeProfileReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return storeProfileReceipt{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return storeProfileReceipt{}, fmt.Errorf("receipt contains trailing data")
	}
	if receipt.Category != "profiles" || receipt.ID != id || receipt.Version == "" || receipt.Destination != filepath.ToSlash(filepath.Join("ryoku", "profiles", id)) || len(receipt.Files) == 0 {
		return storeProfileReceipt{}, fmt.Errorf("receipt identity does not match")
	}
	return receipt, nil
}

func rejectSymlinkPath(base, target string) error {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	relative, err := filepath.Rel(base, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes the caller home")
	}
	current := base
	info, err := os.Lstat(current)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("caller home is not a regular directory")
	}
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains a symbolic link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("path parent is not a directory")
		}
	}
	return nil
}

func safeProfileRelative(value string) (string, error) {
	if value == "" || strings.Contains(value, "\\") || filepath.IsAbs(value) {
		return "", fmt.Errorf("invalid receipt path")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("receipt path escapes the profile")
	}
	return clean, nil
}

func regularFileBelow(root, relative string) (string, error) {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("profile source is not a regular directory")
	}
	current := root
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("profile path contains a symbolic link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("profile path parent is not a directory")
		}
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("profile payload entry is not a regular file")
	}
	return current, nil
}

func hashFile(path string) (string, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		syscall.Close(fd)
		return "", fmt.Errorf("open %s", path)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func reinstallProfileError(err error) error {
	if err == nil {
		err = fmt.Errorf("invalid profile receipt")
	}
	return fmt.Errorf(i18n.T("The installed profile cannot be trusted (%v). Reinstall it from the Store."), err)
}
