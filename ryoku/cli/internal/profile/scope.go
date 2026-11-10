package profile

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func SystemApplicationDirs() []string {
	return []string{"/usr/share/applications", "/usr/local/share/applications", "/var/lib/flatpak/exports/share/applications"}
}

func ScanDesktopEntries(dirs []string) ([]DesktopEntry, error) {
	seen := map[string]bool{}
	var out []DesktopEntry
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			id := strings.ReplaceAll(filepath.ToSlash(rel), "/", "-")
			if seen[id] {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			parsed, err := parseDesktopEntry(body)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			parsed.ID, parsed.Path = id, path
			seen[id] = true
			out = append(out, parsed)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func parseDesktopEntry(body []byte) (DesktopEntry, error) {
	var out DesktopEntry
	group := ""
	found := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			group = line
			if line == "[Desktop Entry]" {
				found = true
			}
			continue
		}
		if group != "[Desktop Entry]" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Name":
			if out.Name == "" {
				out.Name = strings.TrimSpace(value)
			}
		case "NoDisplay":
			out.NoDisplay = strings.EqualFold(strings.TrimSpace(value), "true")
		case "Hidden":
			out.Hidden = strings.EqualFold(strings.TrimSpace(value), "true")
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if !found {
		return out, fmt.Errorf("missing [Desktop Entry] group")
	}
	return out, nil
}

func Essentials() []string {
	return []string{
		"compress-video.desktop", "install-app.desktop", "kitty.desktop", "nemo.desktop",
		"org.gnome.Nautilus.desktop", "org.kde.dolphin.desktop", "pcmanfm.desktop", "pcmanfm-qt.desktop",
		"dev.ryoku.rashin.desktop", "ryogami.desktop", "ryoku-hub.desktop", "ryoku-nvim.desktop",
		"ryomanager.desktop", "ryostore.desktop", "dev.ryoku.ryoport.desktop", "thunar.desktop",
	}
}

func Allowed(m *Manifest, installed []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range Essentials() {
		out[id] = true
	}
	for _, id := range m.Launcher.Allow {
		out[id] = true
	}
	installedSet := stringSet(installed)
	for _, c := range m.Components {
		if installedSet[c.Name] {
			for _, id := range c.DesktopEntries {
				out[id] = true
			}
		}
	}
	return out
}

func HiddenOverride(e DesktopEntry) ([]byte, error) {
	body, err := os.ReadFile(e.Path)
	if err != nil {
		return nil, err
	}
	lines := bytes.SplitAfter(body, []byte{'\n'})
	header, end, existing := -1, len(lines), -1
	inDesktop := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inDesktop {
				end = i
				break
			}
			inDesktop = trimmed == "[Desktop Entry]"
			if inDesktop {
				header = i
			}
			continue
		}
		if inDesktop {
			if key, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == "NoDisplay" {
				existing = i
			}
		}
	}
	if header < 0 {
		return nil, fmt.Errorf("%s has no [Desktop Entry] group", e.ID)
	}
	if existing >= 0 {
		lines[existing] = append([]byte("NoDisplay=true"), lineEnding(lines[existing])...)
		return bytes.Join(lines, nil), nil
	}
	ending := lineEnding(lines[header])
	if len(ending) == 0 {
		ending = []byte{'\n'}
		lines[header] = append(lines[header], ending...)
	}
	insert := append([]byte("NoDisplay=true"), ending...)
	out := make([][]byte, 0, len(lines)+1)
	out = append(out, lines[:header+1]...)
	out = append(out, insert)
	out = append(out, lines[header+1:end]...)
	out = append(out, lines[end:]...)
	return bytes.Join(out, nil), nil
}

func lineEnding(line []byte) []byte {
	if bytes.HasSuffix(line, []byte("\r\n")) {
		return []byte("\r\n")
	}
	if bytes.HasSuffix(line, []byte{'\n'}) {
		return []byte{'\n'}
	}
	return nil
}

func ApplyScope(home string, entries []DesktopEntry, allow map[string]bool, r *AccountReceipt) (added, removed []string, err error) {
	if r == nil {
		return nil, nil, fmt.Errorf("account receipt is nil")
	}
	added, removed = []string{}, []string{}
	previous := map[string]HiddenEntry{}
	for _, hidden := range r.Scope.Hidden {
		previous[hidden.ID] = hidden
	}
	present := map[string]bool{}
	next := []HiddenEntry{}
	for _, entry := range entries {
		present[entry.ID] = true
		if allow[entry.ID] || entry.NoDisplay || entry.Hidden {
			if _, wasHidden := previous[entry.ID]; wasHidden {
				if err := removeOverride(home, entry.ID); err != nil {
					return nil, nil, err
				}
				removed = append(removed, entry.ID)
			}
			continue
		}
		digest, err := hashFile(entry.Path)
		if err != nil {
			return nil, nil, err
		}
		old, exists := previous[entry.ID]
		if !exists {
			rel := filepath.ToSlash(filepath.Join(".local", "share", "applications", entry.ID))
			dst, pathErr := safeDestination(home, rel)
			if pathErr != nil {
				return nil, nil, pathErr
			}
			if _, statErr := os.Lstat(dst); statErr == nil {
				continue
			} else if !os.IsNotExist(statErr) {
				return nil, nil, statErr
			}
		}
		if !exists || old.SourceSHA256 != digest {
			body, err := HiddenOverride(entry)
			if err != nil {
				return nil, nil, err
			}
			rel := filepath.ToSlash(filepath.Join(".local", "share", "applications", entry.ID))
			if err := writeHomeFile(home, rel, body, 0o644); err != nil {
				return nil, nil, err
			}
			added = append(added, entry.ID)
		}
		next = append(next, HiddenEntry{ID: entry.ID, SourceSHA256: digest})
	}
	for id := range previous {
		if !present[id] {
			if err := removeOverride(home, id); err != nil {
				return nil, nil, err
			}
			removed = append(removed, id)
		}
	}
	sort.Slice(next, func(i, j int) bool { return next[i].ID < next[j].ID })
	sort.Strings(added)
	sort.Strings(removed)
	r.Scope.Hidden = next
	return added, removed, nil
}

func removeOverride(home, id string) error {
	rel := filepath.ToSlash(filepath.Join(".local", "share", "applications", id))
	path, err := safeDestination(home, rel)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
