package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const shellBundleName = "bundle.tar.gz"

type shellItemRequires struct {
	PackageManagers []string `toml:"packageManagers"`
	WindowManager   string   `toml:"windowManager"`
	Reason          string   `toml:"reason"`
	QMLModules      []string `toml:"qmlModules"`
}

type shellItemFile struct {
	ID          string            `toml:"id"`
	Name        string            `toml:"name"`
	Version     string            `toml:"version"`
	Summary     string            `toml:"summary"`
	Description string            `toml:"description"`
	License     string            `toml:"license"`
	Authors     []string          `toml:"authors"`
	Tags        []string          `toml:"tags"`
	Upstream    string            `toml:"upstream"`
	Accent      string            `toml:"accent"`
	Surface     string            `toml:"surface"`
	Preview     string            `toml:"preview"`
	Screenshots []string          `toml:"screenshots"`
	Official    bool              `toml:"official"`
	Listed      *bool             `toml:"listed"`
	Requires    shellItemRequires `toml:"requires"`
	Shell       struct {
		Entry       string            `toml:"entry"`
		FrameHost   string            `toml:"frameHost"`
		Owns        []string          `toml:"owns"`
		OwnsSetting map[string]string `toml:"ownsSetting"`
		Modules     []string          `toml:"modules"`
		Settings    ShellSettings     `toml:"settings"`
		Env         map[string]string `toml:"env"`
	} `toml:"shell"`
	Provides struct {
		Dock           []ShellDockContribution          `toml:"dock"`
		Launcher       []ShellLauncherContribution      `toml:"launcher"`
		DesktopWidgets []ShellDesktopWidgetContribution `toml:"desktopWidgets"`
	} `toml:"provides"`
	Lifecycle ShellLifecycle `toml:"lifecycle"`
	Runtime   ShellRuntime   `toml:"runtime"`
}

// shellInventoryItem is one subshell or bar style on this machine: shipped
// with Ryoku (Metadata from its item.toml) or installed from the store
// (Receipt). Dir is shell-root relative for shipped items and the absolute
// unpacked folder for store items once they are prepared.
type shellInventoryItem struct {
	ID       string
	Category string
	Source   string
	Name     string
	Version  string
	Summary  string
	Dir      string
	Shell    ShellManifest
	Metadata *shellItemFile
	Receipt  *Receipt
	InUse    bool
	Prepared bool
	Digest   string
}

func shellConfigRoot() string {
	if root := strings.TrimSpace(os.Getenv("RYOSTORE_SHIPPED_ROOT")); root != "" {
		return filepath.Clean(root)
	}
	return filepath.Join(configHome(), "quickshell", "shell")
}

func shellItemFromTOML(path, category, expectedID string) (shellItemFile, error) {
	var item shellItemFile
	metadata, err := toml.DecodeFile(path, &item)
	if err != nil {
		return item, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		return item, fmt.Errorf("unknown item.toml field %s", undecoded[0].String())
	}
	if item.ID != expectedID || !productIDPattern.MatchString(item.ID) || item.Name == "" || item.Version == "" || item.Summary == "" {
		return item, fmt.Errorf("invalid %s/%s item.toml identity", category, expectedID)
	}
	if item.Preview == "" {
		item.Preview = "preview.webp"
	}
	item.Shell.Settings.Args = nonNilStrings(item.Shell.Settings.Args)
	shell := item.shellManifest()
	if err := loadShellRosterRecords(filepath.Dir(path), &shell); err != nil {
		return item, err
	}
	item.Provides.DesktopWidgets = shell.Provides.DesktopWidgets
	if err := validateShellManifest(category+"/"+expectedID, shell); err != nil {
		return item, err
	}
	return item, nil
}

func (item shellItemFile) shellManifest() ShellManifest {
	return ShellManifest{
		Entry: item.Shell.Entry, FrameHost: item.Shell.FrameHost,
		Owns: nonNilStrings(item.Shell.Owns), OwnsSetting: nonNilSettings(item.Shell.OwnsSetting),
		Modules:  nonNilStrings(item.Shell.Modules),
		Settings: item.Shell.Settings,
		Provides: ShellProvides{
			Dock: nonNilDocks(item.Provides.Dock), Launcher: nonNilLaunchers(item.Provides.Launcher),
			DesktopWidgets: nonNilDesktopWidgets(item.Provides.DesktopWidgets),
		},
		Lifecycle: item.Lifecycle, Runtime: item.Runtime, Env: nonNilSettings(item.Shell.Env),
	}
}

func loadShellRosterRecords(root string, shell *ShellManifest) error {
	for index := range shell.Provides.DesktopWidgets {
		widgets := &shell.Provides.DesktopWidgets[index]
		if widgets.Roster == "" {
			if widgets.RosterRecords == nil {
				widgets.RosterRecords = []map[string]any{}
			}
			continue
		}
		if !validProductPath(widgets.Roster) {
			return fmt.Errorf("invalid desktop widget roster %q", widgets.Roster)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgets.Roster)))
		if err != nil {
			return err
		}
		var records []map[string]any
		if err := decodeOneJSON(raw, &records); err != nil || records == nil {
			if err == nil {
				err = fmt.Errorf("roster must be an array")
			}
			return fmt.Errorf("%s: %w", widgets.Roster, err)
		}
		widgets.RosterRecords = records
	}
	return nil
}

// discoverShippedShells lists the subshells Ryoku ships under the shell config
// root. A folder whose item.toml is unreadable is reported in problems and left
// out, so one damaged item never hides the others.
func discoverShippedShells() ([]shellInventoryItem, []string, error) {
	root := filepath.Join(shellConfigRoot(), "subshells")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []shellInventoryItem{}, []string{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	items := make([]shellInventoryItem, 0, len(entries))
	problems := []string{}
	for _, entry := range entries {
		if !productIDPattern.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		manifest := filepath.Join(dir, "item.toml")
		if _, err := os.Stat(manifest); os.IsNotExist(err) {
			continue
		}
		item, err := shellItemFromTOML(manifest, "subshells", entry.Name())
		if err != nil {
			problems = append(problems, fmt.Sprintf("shipped subshell %s: %v", entry.Name(), err))
			continue
		}
		items = append(items, shellInventoryItem{
			ID: item.ID, Category: "subshells", Source: "shipped", Name: item.Name,
			Version: item.Version, Summary: item.Summary, Dir: path.Join("subshells", item.ID),
			Shell: item.shellManifest(), Metadata: &item, Prepared: true,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, problems, nil
}

func shippedShellIDs() map[string]bool {
	shipped, _, _ := discoverShippedShells()
	ids := make(map[string]bool, len(shipped))
	for _, item := range shipped {
		ids[item.ID] = true
	}
	return ids
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilDocks(values []ShellDockContribution) []ShellDockContribution {
	if values == nil {
		return []ShellDockContribution{}
	}
	return values
}

func nonNilLaunchers(values []ShellLauncherContribution) []ShellLauncherContribution {
	if values == nil {
		return []ShellLauncherContribution{}
	}
	return values
}

func nonNilDesktopWidgets(values []ShellDesktopWidgetContribution) []ShellDesktopWidgetContribution {
	if values == nil {
		return []ShellDesktopWidgetContribution{}
	}
	for index := range values {
		if values[index].RosterRecords == nil {
			values[index].RosterRecords = []map[string]any{}
		}
	}
	return values
}

func cloneShellManifest(shell ShellManifest) ShellManifest {
	raw, _ := json.Marshal(shell)
	var cloned ShellManifest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func nonNilSettings(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}
