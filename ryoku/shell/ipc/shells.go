package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const shellsSyncDelay = 150 * time.Millisecond

type shellIndex struct {
	Schema     int         `json:"schema"`
	Active     string      `json:"active"`
	Fallback   string      `json:"fallback"`
	ModuleRoot string      `json:"moduleRoot"`
	Items      []shellItem `json:"items"`
}

type shellItem struct {
	ID        string            `json:"id"`
	Category  string            `json:"category"`
	Source    string            `json:"source"`
	Dir       string            `json:"dir"`
	InUse     bool              `json:"inUse"`
	Prepared  bool              `json:"prepared"`
	Lifecycle shellLifecycle    `json:"lifecycle"`
	Runtime   shellRuntime      `json:"runtime"`
	Env       map[string]string `json:"env"`
}

type shellLifecycle struct {
	Activate   string `json:"activate"`
	Deactivate string `json:"deactivate"`
	Palette    string `json:"palette"`
}

type shellRuntime struct {
	Check   string `json:"check"`
	Install string `json:"install"`
	Start   string `json:"start"`
}

type shellSnapshot struct {
	Active string
	InUse  []string
}

var runRyostoreShellsSync = func(prune bool) error {
	args := []string{"internal", "shells-sync"}
	if prune {
		args = append(args, "--prune")
	}
	out, err := exec.Command("ryostore", args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
	}
	return err
}

var runItemScript = func(path string, env []string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message != "" {
		return fmt.Errorf("%w: %s", err, message)
	}
	return err
}

// commitReload restarts the shell under the reload cover; tests replace it.
var commitReload = func(d *daemon) string {
	return d.reload()
}

func shellsIndexPath() string {
	return filepath.Join(stateDir(), "ryoku", "store", "shells.json")
}

func readShellIndex(path string) (*shellIndex, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var index shellIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, err
	}
	if index.Schema != 1 {
		return nil, fmt.Errorf("unsupported shells index schema %d", index.Schema)
	}
	return &index, nil
}

func (index *shellIndex) item(id string) *shellItem {
	if index == nil || id == "" {
		return nil
	}
	for i := range index.Items {
		if index.Items[i].ID == id {
			return &index.Items[i]
		}
	}
	return nil
}

func shellItemKey(item *shellItem) string {
	if item == nil {
		return ""
	}
	return item.Category + "/" + item.ID
}

func shellSelectionSig(frame []byte) string {
	var settings struct {
		BarStyle string `json:"barStyle"`
		Dock     struct {
			Design string `json:"design"`
		} `json:"dock"`
	}
	if json.Unmarshal(frame, &settings) != nil {
		return ""
	}
	return settings.BarStyle + "\x00" + settings.Dock.Design
}

// snapshotFor records what the running shell depends on. In-use items carry
// their unpacked dir, so an update of an item in use counts as a change; the
// active item also carries its env, which only a restart applies.
func snapshotFor(active string, index *shellIndex) shellSnapshot {
	inUse := make([]string, 0)
	if index != nil {
		for i := range index.Items {
			if index.Items[i].InUse {
				inUse = append(inUse, shellItemKey(&index.Items[i])+"@"+shellItemDir(&index.Items[i]))
			}
		}
	}
	sort.Strings(inUse)
	if item := index.item(active); item != nil {
		active += "@" + shellItemDir(item) + "\x00" + strings.Join(itemEnv(item), "\x00")
	}
	return shellSnapshot{Active: active, InUse: inUse}
}

// shellRestartNeeded reports whether the running shell holds anything the
// current selection no longer wants: a different active item (id, dir or env),
// or an item it saw in use since it started that is no longer in use. Items
// newly in use load live and need no restart.
func shellRestartNeeded(started shellSnapshot, seen map[string]bool, current shellSnapshot) bool {
	if started.Active != current.Active {
		return true
	}
	now := make(map[string]bool, len(current.InUse))
	for _, key := range current.InUse {
		now[key] = true
	}
	for _, key := range started.InUse {
		if !now[key] {
			return true
		}
	}
	for key := range seen {
		if !now[key] {
			return true
		}
	}
	return false
}

func shellConfigRoot() string {
	if override := os.Getenv("RYOSTORE_SHIPPED_ROOT"); override != "" {
		return override
	}
	if shellDir != "" {
		return filepath.Join(shellDir, "quickshell", "shell")
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, _ := os.UserHomeDir()
		config = filepath.Join(home, ".config")
	}
	return filepath.Join(config, "quickshell", "shell")
}

func shellItemDir(item *shellItem) string {
	if item == nil || item.Dir == "" {
		return ""
	}
	if filepath.IsAbs(item.Dir) {
		return filepath.Clean(item.Dir)
	}
	return filepath.Join(shellConfigRoot(), filepath.FromSlash(item.Dir))
}

func shellItemEnv(item *shellItem) []string {
	return shellItemEnvFrom(os.Environ(), item)
}

// shellItemEnvFrom is the environment of an item's hooks: base plus the item's
// own env and its RYOKU_ITEM_DIR / RYOKU_ITEM_ID.
func shellItemEnvFrom(base []string, item *shellItem) []string {
	base = withItemEnv(base, item)
	env := make([]string, 0, len(base)+2)
	for _, value := range base {
		if strings.HasPrefix(value, "RYOKU_ITEM_DIR=") || strings.HasPrefix(value, "RYOKU_ITEM_ID=") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "RYOKU_ITEM_DIR="+shellItemDir(item), "RYOKU_ITEM_ID="+item.ID)
}

// activeItemSig identifies the active item together with its unpacked dir, so
// a new version of the same item is a new activation.
func activeItemSig(index *shellIndex) string {
	if index == nil || index.Active == "" {
		return ""
	}
	return index.Active + "\x00" + shellItemDir(index.item(index.Active))
}

func runLifecycleTransition(previous, next *shellIndex) {
	if activeItemSig(previous) == activeItemSig(next) {
		return
	}
	// A packed item never ran its activate hook, and has no scripts to run yet.
	if previous != nil {
		if old := previous.item(previous.Active); old != nil && old.Prepared && old.Lifecycle.Deactivate != "" {
			if err := runItemScript(old.Lifecycle.Deactivate, shellItemEnv(old)); err != nil {
				log.Printf("ryoku-shell: deactivate shell item %s: %v", old.ID, err)
			}
		}
	}
	if next != nil {
		if current := next.item(next.Active); current != nil && current.Prepared && current.Lifecycle.Activate != "" {
			if err := runItemScript(current.Lifecycle.Activate, shellItemEnv(current)); err != nil {
				log.Printf("ryoku-shell: activate shell item %s: %v", current.ID, err)
			}
		}
	}
}

func (d *daemon) activeBarStyle() string {
	if d.settings == nil {
		return ""
	}
	d.settings.mu.Lock()
	defer d.settings.mu.Unlock()
	active, _ := d.settings.raw["barStyle"].(string)
	return active
}

func (d *daemon) currentShellIndex() *shellIndex {
	d.shellIndexMu.RLock()
	defer d.shellIndexMu.RUnlock()
	return d.shellIndex
}

func (d *daemon) applyShellIndex(next *shellIndex) {
	d.shellIndexMu.RLock()
	previous := d.shellIndex
	d.shellIndexMu.RUnlock()
	runLifecycleTransition(previous, next)
	d.shellIndexMu.Lock()
	d.shellIndex = next
	d.shellIndexMu.Unlock()
	d.noteShellsInUse(next)
}

// noteShellsInUse adds the in-use entries of a freshly loaded index to the set
// the running shell may have loaded live.
func (d *daemon) noteShellsInUse(index *shellIndex) {
	keys := snapshotFor("", index).InUse
	d.shellSnapshotMu.Lock()
	defer d.shellSnapshotMu.Unlock()
	if !d.shellSnapshotSet {
		return
	}
	if d.shellSeen == nil {
		d.shellSeen = map[string]bool{}
	}
	for _, key := range keys {
		d.shellSeen[key] = true
	}
}

func (d *daemon) loadShellIndex() error {
	index, err := readShellIndex(shellsIndexPath())
	if err != nil {
		return err
	}
	d.applyShellIndex(index)
	return nil
}

// loadShellIndexAtStart reads the last shells.json before the shell starts, so
// the first launch already knows the active item's runtime and lifecycle.
func (d *daemon) loadShellIndexAtStart() {
	if err := d.loadShellIndex(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("ryoku-shell: read shells index: %v", err)
	}
}

func (d *daemon) syncShellsNow(prune bool) error {
	d.shellSyncRunMu.Lock()
	defer d.shellSyncRunMu.Unlock()
	if err := runRyostoreShellsSync(prune); err != nil {
		return err
	}
	return d.loadShellIndex()
}

func (d *daemon) scheduleShellsSync() {
	d.shellSyncMu.Lock()
	defer d.shellSyncMu.Unlock()
	if d.shellSyncTimer == nil {
		d.shellSyncTimer = time.AfterFunc(shellsSyncDelay, d.runScheduledShellsSync)
		return
	}
	d.shellSyncTimer.Reset(shellsSyncDelay)
}

func (d *daemon) runScheduledShellsSync() {
	select {
	case <-d.quit:
		return
	default:
	}
	if err := d.syncShellsNow(false); err != nil {
		log.Printf("ryoku-shell: shells sync: %v", err)
	}
}

// captureShellSnapshot records what a freshly started shell process loaded and
// resets the set of items it has seen in use.
func (d *daemon) captureShellSnapshot() {
	index := d.currentShellIndex()
	if index == nil {
		index, _ = readShellIndex(shellsIndexPath())
	}
	snapshot := snapshotFor(d.activeBarStyle(), index)
	seen := make(map[string]bool, len(snapshot.InUse))
	for _, key := range snapshot.InUse {
		seen[key] = true
	}
	d.shellSnapshotMu.Lock()
	d.shellSnapshot = snapshot
	d.shellSeen = seen
	d.shellSnapshotSet = true
	d.shellSnapshotMu.Unlock()
}

func (d *daemon) shellCommitNeeded() bool {
	index := d.currentShellIndex()
	if index == nil {
		index, _ = readShellIndex(shellsIndexPath())
	}
	current := snapshotFor(d.activeBarStyle(), index)
	d.shellSnapshotMu.Lock()
	defer d.shellSnapshotMu.Unlock()
	return d.shellSnapshotSet && shellRestartNeeded(d.shellSnapshot, d.shellSeen, current)
}

// commitShells restarts the shell when it holds something no longer selected,
// then prunes. Without a restart the prune is safe: the shell only holds what
// is in use now.
func (d *daemon) commitShells() string {
	if err := d.syncShellsNow(false); err != nil {
		return "err shells commit: " + err.Error()
	}
	if d.shellCommitNeeded() {
		if reply := commitReload(d); reply != "ok" {
			return reply
		}
		d.captureShellSnapshot()
	}
	if err := d.syncShellsNow(true); err != nil {
		return "err shells commit prune: " + err.Error()
	}
	return "ok"
}

func (d *daemon) runActivePaletteHook() {
	index := d.currentShellIndex()
	if index == nil {
		return
	}
	item := index.item(index.Active)
	if item == nil || item.Lifecycle.Palette == "" {
		return
	}
	if err := runItemScript(item.Lifecycle.Palette, shellItemEnv(item), matugenColorsPath()); err != nil {
		log.Printf("ryoku-shell: palette shell item %s: %v", item.ID, err)
	}
}
