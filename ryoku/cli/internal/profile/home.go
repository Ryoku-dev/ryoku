package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func LayHome(home, payload string, m *Manifest, v Vars, prev *AccountReceipt) (*AccountReceipt, []Change, error) {
	if err := verifyHomeRoot(home); err != nil {
		return nil, nil, err
	}
	sourceRoot := payload
	prevFiles := map[string]ManagedFile{}
	seeded := map[string]bool{}
	if prev != nil {
		for _, file := range prev.Files {
			prevFiles[file.Path] = file
		}
		for _, path := range prev.Seeded {
			seeded[path] = true
		}
	}
	r := &AccountReceipt{Schema: API, Payload: filepath.Dir(payload), Files: []ManagedFile{}, Seeded: []string{}, EnvKeys: []string{}, UserData: append([]string{}, m.UserData...)}
	r.Scope.Hidden = []HiddenEntry{}
	r.Profile.ID, r.Profile.Version = m.ID, m.Version
	if prev != nil {
		r.Scope = prev.Scope
	}
	changes := []Change{}
	managedSeen := map[string]bool{}
	seedSet, expandSet := stringSet(m.SeedOnce), stringSet(m.Expand)
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("payload path %s is a symlink", path)
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return ensureSafeDir(home, rel, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("payload path %s is not a regular file", rel)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if expandSet[rel] {
			body = []byte(expandTokens(string(body), v))
		}
		dst, err := safeDestination(home, rel)
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		if seedSet[rel] {
			if _, err := os.Lstat(dst); errorsIsNotExist(err) {
				if err := writeHomeFile(home, rel, body, mode); err != nil {
					return err
				}
				changes = append(changes, Change{Path: rel, Action: "seed"})
			}
			seeded[rel] = true
			return nil
		}
		managedSeen[rel] = true
		expected := fileDigest(body)
		if old, ok := prevFiles[rel]; ok {
			if actual, err := hashFile(dst); err == nil {
				if actual != old.SHA256 {
					r.Files = append(r.Files, old)
					changes = append(changes, Change{Path: rel, Action: "keep-user-modified"})
					return nil
				}
				if actual == expected {
					if dstInfo, statErr := os.Lstat(dst); statErr == nil && dstInfo.Mode().Perm() == mode.Perm() {
						r.Files = append(r.Files, ManagedFile{Path: rel, SHA256: expected, Mode: fmt.Sprintf("%04o", mode.Perm())})
						return nil
					}
				}
			}
		}
		action := "create"
		if _, err := os.Lstat(dst); err == nil {
			action = "update"
		}
		if err := writeHomeFile(home, rel, body, mode); err != nil {
			return err
		}
		r.Files = append(r.Files, ManagedFile{Path: rel, SHA256: expected, Mode: fmt.Sprintf("%04o", mode.Perm())})
		changes = append(changes, Change{Path: rel, Action: action})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for rel, old := range prevFiles {
		if managedSeen[rel] {
			continue
		}
		path, pathErr := safeDestination(home, rel)
		if pathErr != nil {
			return nil, nil, pathErr
		}
		actual, hashErr := hashFile(path)
		if os.IsNotExist(hashErr) {
			continue
		}
		if hashErr != nil {
			return nil, nil, hashErr
		}
		if actual != old.SHA256 {
			changes = append(changes, Change{Path: rel, Action: "keep-user-modified"})
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, nil, err
		}
		changes = append(changes, Change{Path: rel, Action: "remove"})
	}
	for _, rel := range m.UserData {
		if err := ensureSafeDir(home, rel, 0o700); err != nil {
			return nil, nil, err
		}
	}
	for rel := range seeded {
		r.Seeded = append(r.Seeded, rel)
	}
	sort.Strings(r.Seeded)
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	return r, changes, nil
}

func ResetHome(home, payload string, m *Manifest, v Vars, r *AccountReceipt, dryRun bool) ([]Change, error) {
	if err := verifyHomeRoot(home); err != nil {
		return nil, err
	}
	if err := verifyHomeRoot(payload); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	expandSet := stringSet(m.Expand)
	changes := []Change{}
	for i := range r.Files {
		managed := r.Files[i]
		src, err := safeDestination(payload, managed.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(src)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("payload path %s is not a regular file", managed.Path)
		}
		body, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if expandSet[managed.Path] {
			body = []byte(expandTokens(string(body), v))
		}
		dst, err := safeDestination(home, managed.Path)
		if err != nil {
			return nil, err
		}
		expected := fileDigest(body)
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		actual, hashErr := hashFile(dst)
		dstInfo, statErr := os.Lstat(dst)
		if hashErr == nil && statErr == nil && actual == expected && dstInfo.Mode().Perm() == mode {
			if !dryRun {
				r.Files[i].SHA256 = expected
				r.Files[i].Mode = fmt.Sprintf("%04o", mode)
			}
			continue
		}
		changes = append(changes, Change{Path: managed.Path, Action: "restore"})
		if dryRun {
			continue
		}

		if err := writeHomeFile(home, managed.Path, body, mode); err != nil {
			return nil, err
		}
		r.Files[i].SHA256 = expected
		r.Files[i].Mode = fmt.Sprintf("%04o", mode)
	}
	return changes, nil
}

func verifyHomeRoot(home string) error {
	if !filepath.IsAbs(home) {
		return fmt.Errorf("home must be absolute")
	}
	info, err := os.Lstat(home)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("home must be a real directory")
	}
	return nil
}

func safeDestination(home, rel string) (string, error) {
	if err := safeRelative(rel); err != nil {
		return "", err
	}
	dst := filepath.Join(home, filepath.FromSlash(rel))
	cleanHome := filepath.Clean(home)
	if relative, err := filepath.Rel(cleanHome, dst); err != nil || relative == ".." || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes home: %s", rel)
	}
	current := cleanHome
	parts := splitRelative(rel)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errorsIsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path contains symlink: %s", current)
		}
	}
	return dst, nil
}

func ensureSafeDir(home, rel string, mode os.FileMode) error {
	if rel == "." || rel == "" {
		return nil
	}
	if mode == 0 {
		mode = 0o755
	}
	current := filepath.Clean(home)
	for _, part := range splitRelative(rel) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errorsIsNotExist(err) {
			if err := os.Mkdir(current, mode); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("path component is not a real directory: %s", current)
		}
	}
	return nil
}

func writeHomeFile(home, rel string, body []byte, mode os.FileMode) error {
	if _, err := safeDestination(home, rel); err != nil {
		return err
	}
	parent := filepath.ToSlash(filepath.Dir(rel))
	if parent != "." {
		if err := ensureSafeDir(home, parent, 0o755); err != nil {
			return err
		}
	}
	dst, err := safeDestination(home, rel)
	if err != nil {
		return err
	}
	return atomicFile(dst, body, mode, 0o755)
}

func hashFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fileDigest(body), nil
}
func fileDigest(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[filepath.ToSlash(value)] = true
	}
	return out
}
func splitRelative(path string) []string {
	var out []string
	for _, part := range splitSlash(filepath.ToSlash(path)) {
		if part != "" && part != "." {
			out = append(out, part)
		}
	}
	return out
}
func splitSlash(path string) []string {
	var out []string
	start := 0
	for i, r := range path {
		if r == '/' {
			out = append(out, path[start:i])
			start = i + 1
		}
	}
	return append(out, path[start:])
}
func errorsIsNotExist(err error) bool { return os.IsNotExist(err) }
