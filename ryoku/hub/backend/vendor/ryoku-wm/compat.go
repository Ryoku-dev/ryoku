package wm

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ForeignAPI identifies a shell or IPC interface plugins may consume directly.
// Providers declare which interfaces they serve; plugin hosts use Compatibility
// before loading code authored for a different environment.
type ForeignAPI string

const (
	ForeignAPIHyprctl            ForeignAPI = "hyprctl"
	ForeignAPIHyprlandIPC        ForeignAPI = "hyprland-ipc"
	ForeignAPIQuickshellHyprland ForeignAPI = "quickshell-hyprland"
	compatMaxFileBytes                      = int64(1 << 20)
	compatMaxTotalBytes                     = int64(8 << 20)
	compatMaxFiles                          = 4096
)

// Compatibility is a complete scan result. Requires contains every detected
// interface, including interfaces the running provider serves. Reason is empty
// when Ok is true and names only the missing interfaces when blocked.
type Compatibility struct {
	OK       bool         `json:"ok"`
	Requires []ForeignAPI `json:"requires"`
	Reason   string       `json:"reason"`
}

type foreignAPIPattern struct {
	api     ForeignAPI
	needles [][]byte
}

var foreignAPIPatterns = []foreignAPIPattern{
	{api: ForeignAPIHyprctl, needles: [][]byte{[]byte("hyprctl")}},
	{api: ForeignAPIHyprlandIPC, needles: [][]byte{[]byte("HYPRLAND_INSTANCE_SIGNATURE"), []byte(".socket.sock"), []byte(".socket2.sock")}},
	{api: ForeignAPIQuickshellHyprland, needles: [][]byte{[]byte("Quickshell.Hyprland")}},
}

// CompatibilityFor scans dir without executing plugin code and compares its
// requirements with the interfaces the active provider declares. The bounds
// make this suitable for add/enable paths. A scan that cannot cover the full
// tree returns an error instead of incorrectly declaring the plugin compatible.
func CompatibilityFor(dir string, served []ForeignAPI) (Compatibility, error) {
	requires, err := scanForeignAPIs(dir)
	if err != nil {
		return Compatibility{}, err
	}

	available := make(map[ForeignAPI]struct{}, len(served))
	for _, api := range served {
		available[api] = struct{}{}
	}
	missing := make([]string, 0, len(requires))
	for _, api := range requires {
		if _, ok := available[api]; !ok {
			missing = append(missing, string(api))
		}
	}

	result := Compatibility{OK: len(missing) == 0, Requires: requires, Reason: ""}
	if len(missing) != 0 {
		result.Reason = "running window manager does not provide: " + strings.Join(missing, ", ")
	}
	return result, nil
}

func scanForeignAPIs(dir string) ([]ForeignAPI, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("compat: resolve plugin directory: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("compat: resolve plugin directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("compat: inspect plugin directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("compat: plugin path is not a directory: %s", dir)
	}

	found := make(map[ForeignAPI]struct{}, len(foreignAPIPatterns))
	seenDirs := make(map[string]struct{})
	seenFiles := make(map[string]struct{})
	var totalBytes int64
	entryCount := 0

	var walk func(string) error
	walk = func(current string) error {
		realDir, err := filepath.EvalSymlinks(current)
		if err != nil {
			return fmt.Errorf("compat: resolve %s: %w", current, err)
		}
		if !pathWithin(root, realDir) {
			return fmt.Errorf("compat: symlink escapes plugin directory: %s", current)
		}
		if _, ok := seenDirs[realDir]; ok {
			return nil
		}
		seenDirs[realDir] = struct{}{}

		entries, err := os.ReadDir(realDir)
		if err != nil {
			return fmt.Errorf("compat: read %s: %w", current, err)
		}
		for _, entry := range entries {
			entryCount++
			if entryCount > compatMaxFiles {
				return fmt.Errorf("compat: plugin has more than %d entries", compatMaxFiles)
			}
			path := filepath.Join(realDir, entry.Name())
			entryInfo, err := os.Lstat(path)
			if err != nil {
				return fmt.Errorf("compat: inspect %s: %w", path, err)
			}
			if entryInfo.Mode()&os.ModeSymlink != 0 {
				target, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("compat: resolve %s: %w", path, err)
				}
				if !pathWithin(root, target) {
					return fmt.Errorf("compat: symlink escapes plugin directory: %s", path)
				}
				targetInfo, err := os.Stat(target)
				if err != nil {
					return fmt.Errorf("compat: inspect %s: %w", path, err)
				}
				if targetInfo.IsDir() {
					if err := walk(target); err != nil {
						return err
					}
					continue
				}
				if targetInfo.Mode().IsRegular() && compatibilitySource(entry.Name(), targetInfo.Mode()) {
					if err := scanCompatibilityFile(target, targetInfo, found, seenFiles, &totalBytes); err != nil {
						return err
					}
				}
				continue
			}
			if entryInfo.IsDir() {
				if err := walk(path); err != nil {
					return err
				}
				continue
			}
			if entryInfo.Mode().IsRegular() && compatibilitySource(entry.Name(), entryInfo.Mode()) {
				if err := scanCompatibilityFile(path, entryInfo, found, seenFiles, &totalBytes); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(root); err != nil {
		return nil, err
	}
	requires := make([]ForeignAPI, 0, len(found))
	for api := range found {
		requires = append(requires, api)
	}
	sort.Slice(requires, func(i, j int) bool { return requires[i] < requires[j] })
	return requires, nil
}

func scanCompatibilityFile(path string, info fs.FileInfo, found map[ForeignAPI]struct{}, seen map[string]struct{}, total *int64) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("compat: resolve %s: %w", path, err)
	}
	if _, ok := seen[realPath]; ok {
		return nil
	}
	seen[realPath] = struct{}{}
	if info.Size() > compatMaxFileBytes {
		return fmt.Errorf("compat: source exceeds %d bytes: %s", compatMaxFileBytes, path)
	}

	file, err := os.Open(realPath)
	if err != nil {
		return fmt.Errorf("compat: read %s: %w", path, err)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, compatMaxFileBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return fmt.Errorf("compat: read %s: %w", path, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("compat: close %s: %w", path, closeErr)
	}
	if int64(len(content)) > compatMaxFileBytes {
		return fmt.Errorf("compat: source exceeds %d bytes: %s", compatMaxFileBytes, path)
	}
	if *total+int64(len(content)) > compatMaxTotalBytes {
		return fmt.Errorf("compat: plugin sources exceed %d bytes", compatMaxTotalBytes)
	}
	*total += int64(len(content))
	for _, pattern := range foreignAPIPatterns {
		if _, ok := found[pattern.api]; ok {
			continue
		}
		for _, needle := range pattern.needles {
			if bytes.Contains(content, needle) {
				found[pattern.api] = struct{}{}
				break
			}
		}
	}
	return nil
}

func compatibilitySource(name string, mode fs.FileMode) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".qml", ".js", ".mjs", ".cjs", ".sh", ".py":
		return true
	default:
		return mode.Perm()&0o111 != 0
	}
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
