package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// shellFallbackID is the subshell every missing or failed selection falls back to.
const shellFallbackID = "orpheus"

// shellsIndex is $XDG_STATE_HOME/ryoku/store/shells.json, the one file the
// shell, the Hub and the daemon read to find subshells and bar styles.
type shellsIndex struct {
	Schema      int              `json:"schema"`
	GeneratedAt string           `json:"generatedAt"`
	Active      string           `json:"active"`
	Fallback    string           `json:"fallback"`
	ActiveOwns  []string         `json:"activeOwns"`
	ModuleRoot  string           `json:"moduleRoot"`
	Items       []shellIndexItem `json:"items"`
}

type shellIndexItem struct {
	ID          string             `json:"id"`
	Category    string             `json:"category"`
	Source      string             `json:"source"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Summary     string             `json:"summary"`
	Dir         string             `json:"dir"`
	Entry       string             `json:"entry"`
	InUse       bool               `json:"inUse"`
	Prepared    bool               `json:"prepared"`
	FrameHost   string             `json:"frameHost"`
	Owns        []string           `json:"owns"`
	OwnsSetting map[string]string  `json:"ownsSetting"`
	Modules     []string           `json:"modules"`
	Settings    ShellSettings      `json:"settings"`
	Provides    shellIndexProvides `json:"provides"`
	Lifecycle   ShellLifecycle     `json:"lifecycle"`
	Runtime     ShellRuntime       `json:"runtime"`
	Env         map[string]string  `json:"env"`
}

type shellIndexProvides struct {
	Dock           []ShellDockContribution     `json:"dock"`
	Launcher       []ShellLauncherContribution `json:"launcher"`
	DesktopWidgets []shellIndexWidgetSet       `json:"desktopWidgets"`
}

// shellIndexWidgetSet carries the roster records inline, so the shell never
// reads a file from a packed item.
type shellIndexWidgetSet struct {
	ID      string           `json:"id"`
	Widget  string           `json:"widget"`
	Options string           `json:"options"`
	Roster  []map[string]any `json:"roster"`
}

type shellsSyncResult struct {
	Active     string   `json:"active"`
	ActiveOwns []string `json:"activeOwns"`
	InUse      []string `json:"inUse"`
	Prepared   []string `json:"prepared"`
	Pruned     []string `json:"pruned"`
	Problems   []string `json:"problems"`
	Changed    bool     `json:"changed"`
}

type shellSelections struct {
	BarStyle string
	Dock     string
	Launcher string
	Widgets  map[string]any
}

func shellsIndexPath() string {
	return filepath.Join(storeStateDir(), "shells.json")
}

func shellModuleRoot() string {
	return filepath.Join(storeStateDir(), "qml")
}

func shellUnpackedRoot() string {
	return filepath.Join(storeStateDir(), "unpacked")
}

func shellUnpackedDir(category, id, digest string) string {
	return filepath.Join(shellUnpackedRoot(), category, id, digest)
}

func widgetsConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "widgets.json")
}

// installedShellItems lists every store subshell and bar style with a packed
// receipt. A receipt that cannot be read, or a legacy loose install still
// waiting for its migration, is reported in problems and left out.
func installedShellItems() ([]shellInventoryItem, []string, error) {
	items := []shellInventoryItem{}
	problems := []string{}
	for _, category := range []string{"subshells", "barstyles"} {
		entries, err := os.ReadDir(filepath.Join(storeStateDir(), category))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".json")
			if !productIDPattern.MatchString(id) {
				continue
			}
			receipt, err := readReceipt(category, id)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s/%s: %v", category, id, err))
				continue
			}
			if receipt.Shell == nil {
				problems = append(problems, fmt.Sprintf("%s/%s: legacy install waits for migration", category, id))
				continue
			}
			stored := receipt
			items = append(items, shellInventoryItem{
				ID: id, Category: category, Source: "store", Name: receipt.Name,
				Version: receipt.Version, Summary: receipt.Summary, Shell: cloneShellManifest(*receipt.Shell),
				Receipt: &stored, Digest: shellReceiptDigest(receipt),
			})
		}
	}
	return items, problems, nil
}

// shellReceiptDigest names the unpacked folder of one receipt: id, version and
// every packed member path and hash, so any change unpacks into a new folder.
func shellReceiptDigest(receipt Receipt) string {
	members := make([]ReceiptFile, 0, len(receipt.Files))
	for _, file := range receipt.Files {
		if file.ArchiveMember {
			members = append(members, file)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Destination < members[j].Destination })
	hash := sha256.New()
	_, _ = io.WriteString(hash, receipt.ID+"\x00"+receipt.Version)
	for _, member := range members {
		_, _ = io.WriteString(hash, "\x00"+member.Destination+"\x00"+member.SHA256)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// readShellSelections reads every user setting that can put an item in use. An
// unreadable file counts as empty, so a broken config never blocks the index.
func readShellSelections() shellSelections {
	shell, err := readConfigState(defaultShellConfigPath())
	if err != nil {
		shell = map[string]any{}
	}
	selections := shellSelections{}
	selections.BarStyle, _ = shell["barStyle"].(string)
	if selections.BarStyle == "" {
		selections.BarStyle = shellFallbackID
	}
	if section, ok := shell["dock"].(map[string]any); ok {
		selections.Dock, _ = section["design"].(string)
	}
	selections.Launcher = readConfigString(defaultLauncherConfigPath(), "variant")
	selections.Widgets, err = readConfigState(widgetsConfigPath())
	if err != nil {
		selections.Widgets = map[string]any{}
	}
	return selections
}

func markShellsInUse(items []shellInventoryItem, selections shellSelections) {
	for index := range items {
		item := &items[index]
		item.InUse = item.ID == selections.BarStyle
		for _, contribution := range item.Shell.Provides.Dock {
			if selections.Dock != "" && contribution.ID == selections.Dock {
				item.InUse = true
			}
		}
		for _, contribution := range item.Shell.Provides.Launcher {
			if selections.Launcher != "" && contribution.ID == selections.Launcher {
				item.InUse = true
			}
		}
		for _, contribution := range item.Shell.Provides.DesktopWidgets {
			if widgetSetEnabled(selections.Widgets, contribution) {
				item.InUse = true
			}
		}
	}
}

// widgetSetEnabled reports whether any widget of the set is switched on, either
// globally or in one output's override block of widgets.json.
func widgetSetEnabled(config map[string]any, contribution ShellDesktopWidgetContribution) bool {
	for _, record := range contribution.RosterRecords {
		prefix, _ := record["prefix"].(string)
		if prefix != "" && configBoolAnywhere(config, prefix+"Enabled") {
			return true
		}
	}
	return false
}

func configBoolAnywhere(value any, key string) bool {
	switch current := value.(type) {
	case map[string]any:
		if enabled, ok := current[key].(bool); ok && enabled {
			return true
		}
		for _, child := range current {
			if configBoolAnywhere(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range current {
			if configBoolAnywhere(child, key) {
				return true
			}
		}
	}
	return false
}

// shellInventory is every shipped and installed item, sorted by category then id.
func shellInventory() ([]shellInventoryItem, []string, error) {
	shipped, problems, err := discoverShippedShells()
	if err != nil {
		return nil, nil, err
	}
	installed, installedProblems, err := installedShellItems()
	if err != nil {
		return nil, nil, err
	}
	items := append(shipped, installed...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Category != items[j].Category {
			return items[i].Category < items[j].Category
		}
		return items[i].ID < items[j].ID
	})
	return items, append(problems, installedProblems...), nil
}

// runShellsSync is `ryostore internal shells-sync`: it unpacks what is in use,
// links its QML modules and rewrites shells.json. Without prune nothing is
// deleted, so a running shell keeps the files it loaded until it restarts.
func runShellsSync(prune bool) (shellsSyncResult, error) {
	result := shellsSyncResult{
		ActiveOwns: []string{}, InUse: []string{}, Prepared: []string{}, Pruned: []string{}, Problems: []string{},
	}
	unlock, err := lockTree(shellsIndexPath())
	if err != nil {
		return result, err
	}
	defer unlock()

	items, problems, err := shellInventory()
	if err != nil {
		return result, err
	}
	result.Problems = append(result.Problems, problems...)
	selections := readShellSelections()
	result.Active = selections.BarStyle
	markShellsInUse(items, selections)

	for index := range items {
		item := &items[index]
		key := item.Category + "/" + item.ID
		if item.InUse {
			result.InUse = append(result.InUse, key)
		}
		if item.Source != "store" {
			continue
		}
		root := shellUnpackedDir(item.Category, item.ID, item.Digest)
		if item.InUse {
			if err := prepareShellItem(*item, root); err != nil {
				result.Problems = append(result.Problems, fmt.Sprintf("%s: %v", key, err))
				continue
			}
		} else if prune || preparedShellIntact(root, *item.Receipt) != nil {
			// An item not in use is prepared only while its old unpack survives,
			// and prune is about to delete it.
			continue
		}
		item.Dir = root
		item.Prepared = true
		result.Prepared = append(result.Prepared, key)
	}

	modules := expectedShellModules(items, selections.BarStyle, &result)
	if err := refreshShellModuleLinks(modules, prune, &result); err != nil {
		return result, err
	}
	if prune {
		if err := pruneShellUnpacked(items, &result); err != nil {
			return result, err
		}
	}

	index := shellsIndex{
		Schema: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Active: selections.BarStyle, Fallback: shellFallbackID, ActiveOwns: []string{},
		ModuleRoot: shellModuleRoot(), Items: make([]shellIndexItem, 0, len(items)),
	}
	activeFound := false
	for _, item := range items {
		if !activeFound && item.ID == selections.BarStyle && item.Prepared {
			index.ActiveOwns = append([]string{}, item.Shell.Owns...)
			activeFound = true
		}
		// An invalid env costs the item its env only, never its listing.
		if err := validateShellEnv(item.Shell.Env); err != nil {
			problem := fmt.Sprintf("%s/%s: %v; its env is ignored", item.Category, item.ID, err)
			fmt.Fprintln(os.Stderr, "ryostore: "+problem)
			result.Problems = append(result.Problems, problem)
			item.Shell.Env = nil
		}
		index.Items = append(index.Items, shellIndexRecord(item))
	}
	result.ActiveOwns = index.ActiveOwns
	changed, err := writeShellsIndex(index)
	if err != nil {
		return result, err
	}
	result.Changed = changed
	sort.Strings(result.Pruned)
	return result, nil
}

// expectedShellModules maps each QML module of an in-use, prepared item to its
// folder. The active item wins a name two items share; the other is reported.
func expectedShellModules(items []shellInventoryItem, active string, result *shellsSyncResult) map[string]string {
	ordered := make([]shellInventoryItem, 0, len(items))
	for _, item := range items {
		if item.ID == active {
			ordered = append([]shellInventoryItem{item}, ordered...)
		} else {
			ordered = append(ordered, item)
		}
	}
	modules := map[string]string{}
	for _, item := range ordered {
		if !item.InUse || !item.Prepared {
			continue
		}
		root := shellItemFilesystemRoot(item)
		for _, module := range item.Shell.Modules {
			target := filepath.Join(root, "qml", module)
			if existing, taken := modules[module]; taken && existing != target {
				result.Problems = append(result.Problems, fmt.Sprintf("%s/%s: QML module %s is already provided by another item in use", item.Category, item.ID, module))
				continue
			}
			modules[module] = target
		}
	}
	return modules
}

// writeShellsIndex replaces shells.json only when something other than its
// timestamp changed, so watchers do not wake on an idle sync.
func writeShellsIndex(index shellsIndex) (bool, error) {
	if raw, err := os.ReadFile(shellsIndexPath()); err == nil {
		var current shellsIndex
		if json.Unmarshal(raw, &current) == nil {
			current.GeneratedAt = ""
			next := index
			next.GeneratedAt = ""
			before, beforeErr := json.Marshal(current)
			after, afterErr := json.Marshal(next)
			if beforeErr == nil && afterErr == nil && bytes.Equal(before, after) {
				return false, nil
			}
		}
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return false, err
	}
	if err := atomicWrite(shellsIndexPath(), append(raw, '\n'), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// prepareShellItem unpacks the item's bundle into root, checking every member
// against its receipt hash. An intact root is left as it is.
func prepareShellItem(item shellInventoryItem, root string) error {
	if item.Receipt == nil {
		return fmt.Errorf("no receipt")
	}
	if preparedShellIntact(root, *item.Receipt) == nil {
		return nil
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".ryostore-prepare-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	destination, _, err := productDestination(item.Category, item.ID)
	if err != nil {
		return err
	}
	if err := verifyShellBundle(filepath.Join(destination, shellBundleName), *item.Receipt, stage); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return os.Rename(stage, root)
}

// preparedShellIntact is the cheap check run on every sync: the unpacked tree
// holds exactly the receipt's members with their sizes and modes. Hashes are
// checked when the bundle is unpacked.
func preparedShellIntact(root string, receipt Receipt) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}
	members := shellBundleMembers(receipt)
	seen := 0
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == root {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unpacked member %s is a link", name)
		}
		if entry.IsDir() {
			return nil
		}
		member, ok := members[name]
		if !ok {
			return fmt.Errorf("unexpected unpacked file %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() != member.Size || fmt.Sprintf("%04o", info.Mode().Perm()) != member.Mode {
			return fmt.Errorf("unpacked file %s does not match its receipt", name)
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(members) {
		return fmt.Errorf("unpacked item has %d of %d files", seen, len(members))
	}
	return nil
}

func shellItemFilesystemRoot(item shellInventoryItem) string {
	if item.Source == "shipped" {
		return filepath.Join(shellConfigRoot(), filepath.FromSlash(item.Dir))
	}
	return shellUnpackedDir(item.Category, item.ID, item.Digest)
}

// shellIndexRecord applies the shells.json path rule: shipped paths stay
// relative to the shell config root, store paths are absolute inside the
// unpacked folder or empty while packed, and scripts are always absolute.
func shellIndexRecord(item shellInventoryItem) shellIndexItem {
	root := shellItemFilesystemRoot(item)
	itemPath := func(value string) string {
		if value == "" || !item.Prepared {
			return ""
		}
		if item.Source == "shipped" {
			return item.Dir + "/" + value
		}
		return filepath.Join(root, filepath.FromSlash(value))
	}
	script := func(value string) string {
		if value == "" {
			return ""
		}
		return filepath.Join(root, filepath.FromSlash(value))
	}
	provides := shellIndexProvides{
		Dock:           []ShellDockContribution{},
		Launcher:       []ShellLauncherContribution{},
		DesktopWidgets: []shellIndexWidgetSet{},
	}
	for _, dock := range item.Shell.Provides.Dock {
		provides.Dock = append(provides.Dock, ShellDockContribution{ID: dock.ID, Name: dock.Name, Entry: itemPath(dock.Entry)})
	}
	for _, launcher := range item.Shell.Provides.Launcher {
		provides.Launcher = append(provides.Launcher, ShellLauncherContribution{
			ID: launcher.ID, Name: launcher.Name, Description: launcher.Description,
			Entry: itemPath(launcher.Entry), Preview: itemPath(launcher.Preview),
		})
	}
	for _, widgets := range item.Shell.Provides.DesktopWidgets {
		roster := widgets.RosterRecords
		if roster == nil {
			roster = []map[string]any{}
		}
		provides.DesktopWidgets = append(provides.DesktopWidgets, shellIndexWidgetSet{
			ID: widgets.ID, Widget: itemPath(widgets.Widget), Options: itemPath(widgets.Options), Roster: roster,
		})
	}
	dir := item.Dir
	if item.Source == "store" && !item.Prepared {
		dir = ""
	}
	settings := item.Shell.Settings
	settings.Args = nonNilStrings(settings.Args)
	return shellIndexItem{
		ID: item.ID, Category: item.Category, Source: item.Source, Name: item.Name,
		Version: item.Version, Summary: item.Summary, Dir: dir, Entry: itemPath(item.Shell.Entry),
		InUse: item.InUse, Prepared: item.Prepared, FrameHost: item.Shell.FrameHost,
		Owns: nonNilStrings(item.Shell.Owns), OwnsSetting: nonNilSettings(item.Shell.OwnsSetting),
		Env:      nonNilSettings(item.Shell.Env),
		Modules:  nonNilStrings(item.Shell.Modules),
		Settings: settings, Provides: provides,
		Lifecycle: ShellLifecycle{
			Activate: script(item.Shell.Lifecycle.Activate), Deactivate: script(item.Shell.Lifecycle.Deactivate),
			Palette: script(item.Shell.Lifecycle.Palette),
		},
		Runtime: ShellRuntime{
			Check: script(item.Shell.Runtime.Check), Install: script(item.Shell.Runtime.Install),
			Start: script(item.Shell.Runtime.Start),
		},
	}
}

// refreshShellModuleLinks points $XDG_STATE_HOME/ryoku/store/qml/<module> at
// each expected module folder, swapping a link in place with a rename. Only
// prune removes links nothing in use needs.
func refreshShellModuleLinks(expected map[string]string, prune bool, result *shellsSyncResult) error {
	root := shellModuleRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(expected))
	for module := range expected {
		names = append(names, module)
	}
	sort.Strings(names)
	for _, module := range names {
		target := expected[module]
		link := filepath.Join(root, module)
		current, err := os.Readlink(link)
		if err == nil && current == target {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			result.Problems = append(result.Problems, fmt.Sprintf("QML module path %s is not a link", link))
			continue
		}
		temporary := filepath.Join(root, fmt.Sprintf(".ryostore-link-%s-%d", module, os.Getpid()))
		_ = os.Remove(temporary)
		if err := os.Symlink(target, temporary); err != nil {
			return err
		}
		if err := os.Rename(temporary, link); err != nil {
			_ = os.Remove(temporary)
			return err
		}
	}
	if !prune {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, keep := expected[entry.Name()]; keep {
			continue
		}
		link := filepath.Join(root, entry.Name())
		if entry.Type()&os.ModeSymlink == 0 {
			result.Problems = append(result.Problems, fmt.Sprintf("QML module path %s is not a link", link))
			continue
		}
		if err := os.Remove(link); err != nil {
			return err
		}
		result.Pruned = append(result.Pruned, "qml/"+entry.Name())
	}
	return nil
}

// pruneShellUnpacked removes every unpacked folder that is not the current
// digest of an item in use.
func pruneShellUnpacked(items []shellInventoryItem, result *shellsSyncResult) error {
	keep := map[string]bool{}
	for _, item := range items {
		if item.Source == "store" && item.InUse && item.Prepared {
			keep[shellUnpackedDir(item.Category, item.ID, item.Digest)] = true
		}
	}
	for _, category := range []string{"subshells", "barstyles"} {
		categoryRoot := filepath.Join(shellUnpackedRoot(), category)
		ids, err := os.ReadDir(categoryRoot)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, id := range ids {
			itemRoot := filepath.Join(categoryRoot, id.Name())
			if !id.IsDir() {
				if err := os.RemoveAll(itemRoot); err != nil {
					return err
				}
				result.Pruned = append(result.Pruned, category+"/"+id.Name())
				continue
			}
			digests, err := os.ReadDir(itemRoot)
			if err != nil {
				return err
			}
			for _, digest := range digests {
				candidate := filepath.Join(itemRoot, digest.Name())
				if keep[candidate] {
					continue
				}
				if err := os.RemoveAll(candidate); err != nil {
					return err
				}
				result.Pruned = append(result.Pruned, category+"/"+id.Name()+"/"+digest.Name())
			}
			_ = os.Remove(itemRoot)
		}
	}
	return nil
}

// removeShellItemArtifacts deletes the unpacked folders of a removed item and
// every module link that points into them.
func removeShellItemArtifacts(category, id string) error {
	if !shellCategory(category) || !productIDPattern.MatchString(id) {
		return fmt.Errorf("invalid shell item %s/%s", category, id)
	}
	itemRoot := filepath.Join(shellUnpackedRoot(), category, id)
	entries, err := os.ReadDir(shellModuleRoot())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		link := filepath.Join(shellModuleRoot(), entry.Name())
		target, err := os.Readlink(link)
		if err != nil {
			return err
		}
		if strings.HasPrefix(target, itemRoot+string(filepath.Separator)) {
			if err := os.Remove(link); err != nil {
				return err
			}
		}
	}
	return os.RemoveAll(itemRoot)
}
