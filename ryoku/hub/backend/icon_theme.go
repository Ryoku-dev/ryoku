package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type iconThemeOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type iconThemeMetadata struct {
	Name        string
	Hidden      bool
	Directories []string
}

var (
	runIconCommand = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	runIconCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

func parseIconTheme(path string) (iconThemeMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return iconThemeMetadata{}, err
	}
	defer f.Close()

	var meta iconThemeMetadata
	section := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "Icon Theme" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "Name":
			meta.Name = value
		case "Hidden":
			meta.Hidden = strings.EqualFold(value, "true")
		case "Directories", "ScaledDirectories":
			for _, dir := range strings.Split(value, ",") {
				if dir = strings.TrimSpace(dir); dir != "" {
					meta.Directories = append(meta.Directories, dir)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return iconThemeMetadata{}, err
	}
	return meta, nil
}

func listIconThemes() []iconThemeOption {
	return listIconThemesFromDirs(iconSearchDirs())
}

func listIconThemesFromDirs(roots []string) []iconThemeOption {
	seen := map[string]bool{}
	var out []iconThemeOption
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			code := entry.Name()
			if seen[code] || strings.ContainsAny(code, "\r\n") {
				continue
			}
			themeDir := filepath.Join(root, code)
			info, err := os.Stat(themeDir)
			if err != nil || !info.IsDir() {
				continue
			}
			seen[code] = true
			meta, err := parseIconTheme(filepath.Join(themeDir, "index.theme"))
			if err != nil || meta.Hidden || meta.Name == "" {
				continue
			}
			hasIcons := false
			for _, dir := range meta.Directories {
				clean := filepath.Clean(dir)
				if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
					strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
					strings.EqualFold(filepath.Base(clean), "cursors") {
					continue
				}
				if info, err := os.Stat(filepath.Join(themeDir, dir)); err == nil && info.IsDir() {
					hasIcons = true
					break
				}
			}
			if !hasIcons {
				continue
			}
			out = append(out, iconThemeOption{Code: code, Name: meta.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if left == right {
			return out[i].Code < out[j].Code
		}
		return left < right
	})
	return out
}

func iconThemeStatePath() string {
	return filepath.Join(ryokuConfigDir(), "icon-theme")
}

func storedIconTheme() string {
	b, err := os.ReadFile(iconThemeStatePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func currentIconTheme() string {
	if theme := storedIconTheme(); theme != "" {
		return theme
	}
	out, err := runIconCommandOutput("gsettings", "get", "org.gnome.desktop.interface", "icon-theme")
	if err == nil {
		if theme := strings.Trim(strings.TrimSpace(string(out)), "'\""); theme != "" {
			return theme
		}
	}
	for _, candidate := range []struct {
		path    string
		section string
		key     string
	}{
		{filepath.Join(configHome(), "gtk-3.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
		{filepath.Join(configHome(), "qt6ct", "qt6ct.conf"), "Appearance", "icon_theme"},
	} {
		if b, err := os.ReadFile(candidate.path); err == nil {
			if value := iniValue(b, candidate.section, candidate.key); value != "" {
				return strings.Trim(value, "'\"")
			}
		}
	}
	return "hicolor"
}

// iconThemeInstalled: a theme applies when any search dir holds its index.theme, offered or
// not; the list leaves out hidden themes such as Adwaita, which can still be the one in use
func iconThemeInstalled(theme string) bool {
	if theme == "" || theme == "." || theme == ".." || strings.ContainsAny(theme, "/\r\n") {
		return false
	}
	for _, root := range iconSearchDirs() {
		if info, err := os.Stat(filepath.Join(root, theme, "index.theme")); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func applyIconTheme(theme string) error {
	if !iconThemeInstalled(theme) {
		return fmt.Errorf("icon theme %q is not installed", theme)
	}

	writes := []struct {
		path    string
		section string
		key     string
	}{
		{filepath.Join(configHome(), "gtk-3.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
		{filepath.Join(configHome(), "gtk-4.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
		{filepath.Join(configHome(), "qt6ct", "qt6ct.conf"), "Appearance", "icon_theme"},
		{filepath.Join(configHome(), "kdeglobals"), "Icons", "Theme"},
	}
	paths := []string{iconThemeStatePath()}
	for _, write := range writes {
		paths = append(paths, write.path)
	}
	before, err := snapshotFiles(paths...)
	if err != nil {
		return err
	}
	// gsettings runs last, so any failure leaves the session on the previous theme
	err = func() error {
		for _, write := range writes {
			if err := setINIFileValue(write.path, write.section, write.key, theme); err != nil {
				return fmt.Errorf("write %s: %w", write.path, err)
			}
		}
		if err := atomicWrite(iconThemeStatePath(), []byte(theme+"\n"), 0o644); err != nil {
			return fmt.Errorf("remember icon theme: %w", err)
		}
		if err := runIconCommand("gsettings", "set", "org.gnome.desktop.interface", "icon-theme", theme); err != nil {
			return fmt.Errorf("set org.gnome.desktop.interface icon-theme: %w", err)
		}
		return nil
	}()
	if err == nil {
		return nil
	}
	if rerr := before.restore(); rerr != nil {
		return errors.Join(err, fmt.Errorf("restore icon theme files: %w", rerr))
	}
	return err
}

func setINIFileValue(path, section, key, value string) error {
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return atomicWrite(path, setINIValue(body, section, key, value), 0o644)
}

func iniValue(body []byte, wantedSection, wantedKey string) string {
	section := ""
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			continue
		}
		if section != wantedSection || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(key) == wantedKey {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func setINIValue(body []byte, wantedSection, wantedKey, value string) []byte {
	text := string(body)
	hadNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	section := ""
	sectionFound := false
	insertAt := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if section == wantedSection && insertAt < 0 {
				insertAt = i
			}
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			if section == wantedSection {
				sectionFound = true
			}
			continue
		}
		if section != wantedSection || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(key) == wantedKey {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + wantedKey + "=" + value
			return []byte(strings.Join(lines, "\n") + "\n")
		}
	}
	if sectionFound {
		if insertAt < 0 {
			insertAt = len(lines)
		}
		lines = append(lines, "")
		copy(lines[insertAt+1:], lines[insertAt:])
		lines[insertAt] = wantedKey + "=" + value
	} else {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "["+wantedSection+"]", wantedKey+"="+value)
	}
	out := strings.Join(lines, "\n")
	if hadNewline || out != "" {
		out += "\n"
	}
	return []byte(out)
}
