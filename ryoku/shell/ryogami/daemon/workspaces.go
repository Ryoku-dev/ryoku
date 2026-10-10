package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wm "ryoku-wm"
)

const (
	workspaceStateVersion = 1
	workspaceDebounce     = 180 * time.Millisecond
)

type workspaceWall struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Output   string `json:"output,omitempty"`
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	WeID     string `json:"we_id,omitempty"`
	Mute     bool   `json:"mute"`
	Volume   int    `json:"volume"`
}

type workspaceWallState struct {
	Version     int                      `json:"version"`
	Assignments map[string]workspaceWall `json:"assignments"`
}

type workspaceTarget struct {
	Provider string
	ID       string
	Name     string
	Output   string
}

type pendingWorkspaceSwitch struct {
	target workspaceTarget
	timer  *time.Timer
}

type workspaceManager struct {
	d *daemon

	mu          sync.Mutex
	state       workspaceWallState
	provider    string
	active      map[string]workspaceTarget
	pending     map[string]*pendingWorkspaceSwitch
	initialized bool
	debounce    time.Duration
	apply       func(workspaceTarget, string)
}

func newWorkspaceManager(d *daemon) *workspaceManager {
	m := &workspaceManager{
		d:        d,
		active:   map[string]workspaceTarget{},
		pending:  map[string]*pendingWorkspaceSwitch{},
		debounce: workspaceDebounce,
	}
	m.apply = func(target workspaceTarget, mode string) {
		if err := d.applyWorkspaceTarget(target, mode); err != nil {
			fmt.Fprintf(os.Stderr, "ryogami: workspace wallpaper for %s: %v\n", target.Output, err)
		}
	}
	m.load()
	return m
}

func (m *workspaceManager) path() string {
	return filepath.Join(m.d.config().cacheDir(), "workspaces.json")
}

func (m *workspaceManager) load() {
	state := workspaceWallState{}
	loadJSON(m.path(), &state)
	if state.Version <= 0 {
		state.Version = workspaceStateVersion
	}
	if state.Assignments == nil {
		state.Assignments = map[string]workspaceWall{}
	}
	m.state = state
}

func (m *workspaceManager) saveLocked() {
	if m.state.Version <= 0 {
		m.state.Version = workspaceStateVersion
	}
	if m.state.Assignments == nil {
		m.state.Assignments = map[string]workspaceWall{}
	}
	_ = os.MkdirAll(filepath.Dir(m.path()), 0o755)
	saveJSON(m.path(), m.state)
}

func workspaceKey(provider, id string) string {
	return provider + ":" + id
}

func currentProviderName() string {
	caps, err := wmClient.Caps()
	if err != nil {
		return ""
	}
	return caps.Name
}

func targetFromWorkspace(provider string, ws wm.Workspace) workspaceTarget {
	return workspaceTarget{Provider: provider, ID: ws.ID, Name: ws.Name, Output: ws.Output}
}

func sameWorkspace(a, b workspaceTarget) bool {
	return a.Provider == b.Provider && a.ID == b.ID && a.Output == b.Output
}
func (m *workspaceManager) updateActive(workspaces []wm.Workspace) {
	provider := currentProviderName()
	if provider == "" {
		return
	}
	m.updateActiveForProvider(provider, workspaces)
}

// restoreActiveWorkspaces seeds the switch tracker from a synchronous neutral
// snapshot. Some providers only emit later changes on their watch stream, so
// waiting for that stream would leave the output fallback painted after a
// daemon-only restart.
func (d *daemon) restoreActiveWorkspaces() {
	provider := currentProviderName()
	if provider == "" {
		return
	}
	snapshot, err := wmClient.State()
	if err != nil {
		return
	}
	d.workspaces.updateActiveForProvider(provider, snapshot.Workspaces)
}

func (m *workspaceManager) updateActiveForProvider(provider string, workspaces []wm.Workspace) {
	next := map[string]workspaceTarget{}
	for _, ws := range workspaces {
		if ws.Active && ws.Output != "" {
			next[ws.Output] = targetFromWorkspace(provider, ws)
		}
	}

	type applyCall struct {
		target workspaceTarget
		mode   string
	}
	var calls []applyCall

	m.mu.Lock()
	if m.provider != "" && m.provider != provider {
		for _, pending := range m.pending {
			pending.timer.Stop()
		}
		m.pending = map[string]*pendingWorkspaceSwitch{}
		m.active = map[string]workspaceTarget{}
		m.initialized = false
	}
	m.provider = provider
	initial := !m.initialized
	for output, target := range next {
		previous, existed := m.active[output]
		if existed && sameWorkspace(previous, target) {
			continue
		}
		if initial {
			calls = append(calls, applyCall{target: target, mode: "init"})
			continue
		}
		if pending := m.pending[output]; pending != nil {
			pending.timer.Stop()
			calls = append(calls, applyCall{target: pending.target, mode: "init"})
		}
		pending := &pendingWorkspaceSwitch{target: target}
		pending.timer = time.AfterFunc(m.debounce, func() { m.finishSwitch(output, pending) })
		m.pending[output] = pending
	}
	for output, pending := range m.pending {
		if _, alive := next[output]; !alive {
			pending.timer.Stop()
			delete(m.pending, output)
		}
	}
	m.active = next
	if len(next) > 0 {
		m.initialized = true
	}
	m.mu.Unlock()

	for _, call := range calls {
		m.apply(call.target, call.mode)
	}
}

func (m *workspaceManager) finishSwitch(output string, pending *pendingWorkspaceSwitch) {
	m.mu.Lock()
	if m.pending[output] != pending {
		m.mu.Unlock()
		return
	}
	delete(m.pending, output)
	target, active := m.active[output]
	m.mu.Unlock()
	if active && sameWorkspace(target, pending.target) {
		m.apply(target, "set")
	}
}

func (m *workspaceManager) activeTarget(output string) (workspaceTarget, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target, ok := m.active[output]
	return target, ok
}

func (m *workspaceManager) current(output string) (workspaceTarget, error) {
	provider := currentProviderName()
	if provider == "" {
		return workspaceTarget{}, fmt.Errorf("window manager provider is not available")
	}
	if output == "" {
		wmCache.mu.Lock()
		output = wmCache.focusedOutput
		wmCache.mu.Unlock()
		if output == "" {
			for _, candidate := range outputs.list() {
				if candidate.Focused {
					output = candidate.Name
					break
				}
			}
		}
	}
	m.mu.Lock()
	target, ok := m.active[output]
	m.mu.Unlock()
	if ok && target.Provider == provider {
		return target, nil
	}
	snapshot, err := wmClient.State()
	if err != nil {
		return workspaceTarget{}, err
	}
	for _, ws := range snapshot.Workspaces {
		if !ws.Active {
			continue
		}
		if output == "" || ws.Output == output {
			return targetFromWorkspace(provider, ws), nil
		}
	}
	return workspaceTarget{}, fmt.Errorf("no active workspace on output %q", output)
}

func (m *workspaceManager) resolve(target workspaceTarget, includeWorkspace bool) (workspaceWall, bool) {
	m.mu.Lock()
	if includeWorkspace {
		if wall, ok := m.resolveAssignmentLocked(target); ok {
			m.mu.Unlock()
			return wall, true
		}
	}
	m.mu.Unlock()
	return m.outputFallback(target), false
}

func (m *workspaceManager) resolveAssignmentLocked(target workspaceTarget) (workspaceWall, bool) {
	key := workspaceKey(target.Provider, target.ID)
	if wall, ok := m.state.Assignments[key]; ok {
		if wall.Name != target.Name || wall.Output != target.Output {
			wall.Name = target.Name
			wall.Output = target.Output
			m.state.Assignments[key] = wall
			m.saveLocked()
		}
		return wall, true
	}
	if target.Name == "" {
		return workspaceWall{}, false
	}
	keys := make([]string, 0, len(m.state.Assignments))
	for oldKey, wall := range m.state.Assignments {
		if wall.Provider == target.Provider && wall.Name == target.Name {
			keys = append(keys, oldKey)
		}
	}
	if len(keys) == 0 {
		return workspaceWall{}, false
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a := m.state.Assignments[keys[i]].Output == target.Output
		b := m.state.Assignments[keys[j]].Output == target.Output
		if a != b {
			return a
		}
		return keys[i] < keys[j]
	})
	oldKey := keys[0]
	wall := m.state.Assignments[oldKey]
	delete(m.state.Assignments, oldKey)
	wall.ID = target.ID
	wall.Name = target.Name
	wall.Output = target.Output
	m.state.Assignments[key] = wall
	m.saveLocked()
	return wall, true
}

func (m *workspaceManager) outputFallback(target workspaceTarget) workspaceWall {
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(m.d.config().cacheDir(), "outputs.json"), &state)
	entry := state[target.Output]
	if entry == nil {
		entry = state["*"]
	}
	if entry != nil {
		wall := workspaceWall{Provider: target.Provider, ID: target.ID, Name: target.Name, Output: target.Output}
		wall.Type, _ = entry["type"].(string)
		wall.Path, _ = entry["path"].(string)
		wall.Mute, wall.Volume = entryAudio(entry, wallAudioDefaults())
		if wall.Type == "we" {
			wall.WeID = filepath.Base(wall.Path)
		}
		if wall.Type == "" {
			wall.Type = typeOf(wall.Path)
		}
		return wall
	}
	if path := m.d.defaultWallpaper(); path != "" {
		def := wallAudioDefaults()
		return workspaceWall{Provider: target.Provider, ID: target.ID, Name: target.Name, Output: target.Output, Type: "static", Path: path, Mute: def.mute, Volume: def.volume}
	}
	return workspaceWall{}
}

func (m *workspaceManager) assign(target workspaceTarget, wall workspaceWall) (workspaceWall, error) {
	if target.Provider == "" || target.ID == "" || target.Output == "" {
		return workspaceWall{}, fmt.Errorf("workspace target is incomplete")
	}
	if wall.Type == "" && wall.Path != "" {
		wall.Type = typeOf(wall.Path)
	}
	if wall.Type == "" {
		var ok bool
		wall, ok = m.resolve(target, true)
		if !ok && wall.Path == "" && wall.WeID == "" {
			return workspaceWall{}, fmt.Errorf("no wallpaper is active on %s", target.Output)
		}
	}
	if wall.Type == "we" {
		if wall.WeID == "" {
			wall.WeID = filepath.Base(wall.Path)
		}
		if !validWeID(wall.WeID) {
			return workspaceWall{}, fmt.Errorf("invalid Wallpaper Engine id: %s", wall.WeID)
		}
		if !m.d.featureSteam() {
			return workspaceWall{}, fmt.Errorf("Wallpaper Engine support is disabled")
		}
		itemDir := filepath.Join(m.d.workshop.workshopDir(), wall.WeID)
		if _, err := validateProject(itemDir); err != nil {
			return workspaceWall{}, fmt.Errorf("workshop item %s: %w", wall.WeID, err)
		}
		wall.Path = ""
	} else {
		if wall.Path == "" {
			return workspaceWall{}, fmt.Errorf("missing wallpaper path")
		}
		if _, err := os.Stat(wall.Path); err != nil {
			return workspaceWall{}, fmt.Errorf("wallpaper not readable: %v", err)
		}
		wall.WeID = ""
	}
	wall.Provider = target.Provider
	wall.ID = target.ID
	wall.Name = target.Name
	wall.Output = target.Output
	wall.Volume = clampVolume(wall.Volume)
	m.mu.Lock()
	m.state.Assignments[workspaceKey(target.Provider, target.ID)] = wall
	m.saveLocked()
	m.mu.Unlock()
	return wall, nil
}

func (m *workspaceManager) unassign(target workspaceTarget) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := workspaceKey(target.Provider, target.ID)
	if _, ok := m.state.Assignments[key]; ok {
		delete(m.state.Assignments, key)
		m.saveLocked()
		return true
	}
	for oldKey, wall := range m.state.Assignments {
		if target.Name != "" && wall.Provider == target.Provider && wall.Name == target.Name {
			delete(m.state.Assignments, oldKey)
			m.saveLocked()
			return true
		}
	}
	return false
}

func workspaceTargetParam(p map[string]interface{}) workspaceTarget {
	value, _ := p["workspace"].(map[string]interface{})
	if value == nil {
		value = p
	}
	return workspaceTarget{
		ID:     strParam(value, "id", ""),
		Name:   strParam(value, "name", ""),
		Output: strParam(value, "output", ""),
	}
}

func (d *daemon) requestedWorkspace(p map[string]interface{}) (workspaceTarget, error) {
	target := workspaceTargetParam(p)
	provider := currentProviderName()
	if provider == "" {
		return workspaceTarget{}, fmt.Errorf("window manager provider is not available")
	}
	target.Provider = provider
	if target.ID != "" {
		if target.Output == "" {
			if current, err := d.workspaces.current(""); err == nil && current.ID == target.ID {
				target.Output = current.Output
				if target.Name == "" {
					target.Name = current.Name
				}
			}
		}
		if target.Output == "" {
			return workspaceTarget{}, fmt.Errorf("workspace output is required")
		}
		return target, nil
	}
	return d.workspaces.current(target.Output)
}

func workspaceWallParam(p map[string]interface{}, output string) workspaceWall {
	wall := workspaceWall{
		Type:   strParam(p, "type", ""),
		Path:   strParam(p, "path", ""),
		WeID:   weIDParam(p),
		Volume: wallAudioDefaults().volume,
		Mute:   wallAudioDefaults().mute,
	}
	if mute, ok := muteParam(p)[output]; ok {
		wall.Mute = mute
	}
	if volume, ok := volumeParam(p)[output]; ok {
		wall.Volume = volume
	}
	if wall.Type == "" && wall.WeID != "" {
		wall.Type = "we"
	}
	return wall
}

func (d *daemon) assignWorkspace(p map[string]interface{}) (workspaceWall, error) {
	target, err := d.requestedWorkspace(p)
	if err != nil {
		return workspaceWall{}, err
	}
	wall, err := d.workspaces.assign(target, workspaceWallParam(p, target.Output))
	if err != nil {
		return workspaceWall{}, err
	}
	if current, err := d.workspaces.current(target.Output); err == nil && sameWorkspace(current, target) {
		if err := d.paintWorkspaceWall(target, wall, "set"); err != nil {
			return workspaceWall{}, err
		}
	}
	return wall, nil
}

func (d *daemon) unassignWorkspace(p map[string]interface{}) (bool, error) {
	target, err := d.requestedWorkspace(p)
	if err != nil {
		return false, err
	}
	removed := d.workspaces.unassign(target)
	if removed {
		if current, err := d.workspaces.current(target.Output); err == nil && sameWorkspace(current, target) {
			if err := d.applyWorkspaceTarget(target, "set"); err != nil {
				return false, err
			}
		}
	}
	return removed, nil
}

func (d *daemon) applyWorkspaceTarget(target workspaceTarget, mode string) error {
	wall, assigned := d.workspaces.resolve(target, true)
	if wall.Type == "" {
		return nil
	}
	if err := d.paintWorkspaceWall(target, wall, mode); err != nil {
		if !assigned {
			return err
		}
		fallback, _ := d.workspaces.resolve(target, false)
		if fallback.Type == "" {
			return err
		}
		fmt.Fprintf(os.Stderr, "ryogami: workspace %s wallpaper unavailable, using its display fallback: %v\n",
			workspaceTargetLabel(target), err)
		return d.paintWorkspaceWall(target, fallback, mode)
	}
	return nil
}

func (d *daemon) paintWorkspaceWall(target workspaceTarget, wall workspaceWall, mode string) error {
	// Moving between workspaces that resolve to the wallpaper already on this
	// output (the usual case: no assignment, so both use the display's own)
	// must not replay the reveal on every switch.
	if shows := wall.shows(); shows != "" && d.showing(target.Output) == shows {
		return nil
	}
	outputs := []string{target.Output}
	mute := map[string]bool{target.Output: wall.Mute}
	volume := map[string]int{target.Output: wall.Volume}
	if wall.Type == "we" {
		itemDir := filepath.Join(d.workshop.workshopDir(), wall.WeID)
		return d.driveWE(wall.WeID, itemDir, outputs, mute, volume)
	}
	return d.paintWallpaperReason("workspace", wall.Type, wall.Path, mode, outputs, mute, volume)
}

// shows names the wallpaper the way beginPaint records it.
func (w workspaceWall) shows() string {
	if w.Type == "we" {
		return "we:" + w.WeID
	}
	return w.Path
}

func (m *workspaceManager) setAudio(mute *bool, volume *int, outputs []string) (claimed map[string]bool, weOutputs []string) {
	claimed = map[string]bool{}
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for output, target := range m.active {
		if len(outputs) > 0 && !contains(outputs, output) && !contains(outputs, "*") {
			continue
		}
		key := workspaceKey(target.Provider, target.ID)
		wall, ok := m.state.Assignments[key]
		if !ok {
			continue
		}
		claimed[output] = true
		if mute != nil {
			wall.Mute = *mute
		}
		if volume != nil {
			wall.Volume = clampVolume(*volume)
		}
		m.state.Assignments[key] = wall
		changed = true
		if wall.Type == "we" {
			weOutputs = append(weOutputs, output)
		}
	}
	if changed {
		m.saveLocked()
	}
	return claimed, weOutputs
}

func (m *workspaceManager) snapshot() workspaceWallState {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyState := workspaceWallState{Version: m.state.Version, Assignments: map[string]workspaceWall{}}
	for key, wall := range m.state.Assignments {
		copyState.Assignments[key] = wall
	}
	return copyState
}

func workspaceTargetLabel(target workspaceTarget) string {
	if strings.TrimSpace(target.Name) != "" {
		return target.Name
	}
	return target.ID
}
