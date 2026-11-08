package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

// powerprofiles.go owns the power-profile provider integration: it reads the
// active profile and ordered profile list over the system bus, streams them to
// QML on the "powerprofiles" state topic, and exposes the same state and setter
// over the control socket. The bus name, object path, and interface are the
// freedesktop PowerProfiles spec, reproduced as-is; they are third-party
// protocol, not reference API. Contract 06 sec 2.9 / contract 11 sec 3.1
// (power profiles): the list is service order with no client sort, and
// "Unknown" maps to the balanced icon on the QML side.
const (
	ppBusName = "org.freedesktop.UPower.PowerProfiles"
	ppPath    = "/org/freedesktop/UPower/PowerProfiles"
	ppIface   = "org.freedesktop.UPower.PowerProfiles"
)

type powerProfilesObject interface {
	GetProperty(string) (dbus.Variant, error)
	Call(string, dbus.Flags, ...any) *dbus.Call
}

// powerProfilesState holds the one system-bus connection and the topic the
// active-profile and profile-list frames publish to.
type powerProfilesState struct {
	conn  *dbus.Conn
	obj   powerProfilesObject
	topic *stateTopic

	applyTimer   *time.Timer // debounce for re-applying the active profile after a ppd switch
	applyMu      sync.Mutex  // serialises applies so overlapping fires never race
	lastApplyErr string      // profile whose apply last failed; logged once per change

	// Timestamps guard the persist decision: a change before restoreDone, within
	// restoreReassertWindow of restoreNs, or within autoSwitchWindow of acFlipNs
	// is not a user pick. saved (string) is the live re-assert target.
	restoreDone atomic.Bool
	restoreNs   atomic.Int64
	acFlipNs    atomic.Int64
	saved       atomic.Value // string
}

// startPowerProfiles brings the power-profile integration up, registers the
// topic and the setProfile call, watches PropertiesChanged, and publishes the
// first frame. A missing system bus or absent daemon disables the feature
// without failing the daemon (the QML view simply stays empty).
func (d *daemon) startPowerProfiles() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: power profiles disabled: %v", err)
		return
	}
	p := &powerProfilesState{
		conn:  conn,
		obj:   conn.Object(ppBusName, dbus.ObjectPath(ppPath)),
		topic: d.registerTopic("powerprofiles"),
	}
	d.pp = p
	// Read the pick before wiring signals so a boot PropertiesChanged can't
	// rewrite the store first.
	saved := readPersistedProfile()
	p.saved.Store(saved)

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath(ppPath)),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		log.Printf("ryoku-shell: power profiles signal match failed: %v", err)
	}
	sigs := make(chan *dbus.Signal, 8)
	conn.Signal(sigs)
	go func() {
		for sig := range sigs {
			if sig == nil || sig.Path != dbus.ObjectPath(ppPath) ||
				sig.Name != propertiesInterface+".PropertiesChanged" {
				continue
			}
			p.publish()
			p.scheduleApply()
			p.maybeHandleProfileChange()
			d.kickSaverExtras()
		}
		if p.applyTimer != nil {
			p.applyTimer.Stop()
		}
	}()

	// A pick that came through this call is the user's by construction, so it is
	// banked here rather than inferred from the signal: it must not be mistaken
	// for the provider's boot default and re-asserted away inside the restore
	// window.
	d.registerCall("powerprofiles.setProfile", func(raw json.RawMessage) (any, error) {
		var a struct {
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		if err := p.setUserProfile(a.Profile); err != nil {
			return nil, err
		}
		return nil, nil
	})
	d.registerCall("powerprofiles.state", func(json.RawMessage) (any, error) {
		return p.state(), nil
	})

	// Restore the last pick: ppd resets to a platform default on reboot.
	if saved != "" && saved != p.activeProfile() {
		if err := p.setProfile(saved); err != nil {
			log.Printf("ryoku-shell: restore power profile %q: %v", saved, err)
		}
	}
	if shouldSeedBalanced(saved, p.activeProfile(), gameModeActive()) {
		// setProfile rejects a name ppd does not offer, so a box with no
		// `balanced` simply logs and keeps what it had.
		if err := p.setProfile(seedProfile); err != nil {
			log.Printf("ryoku-shell: seed power profile %q: %v", seedProfile, err)
		} else {
			p.saved.Store(seedProfile)
			p.persistProfile(seedProfile)
		}
	}
	// restoreNs before restoreDone so the first post-restore signal sees both.
	// The first activeProfile() read is not final: ppd can publish its boot
	// default a beat later, which maybeHandleProfileChange re-asserts over.
	p.restoreNs.Store(time.Now().UnixNano())
	p.restoreDone.Store(true)

	p.publish()
}

type powerProfilesFrame struct {
	ActiveProfile string   `json:"active_profile"`
	Profiles      []string `json:"profiles"`
}

func (p *powerProfilesState) state() powerProfilesFrame {
	return powerProfilesFrame{
		ActiveProfile: p.activeProfile(),
		Profiles:      p.profiles(),
	}
}

// publish marshals the whole power-profile state and hands it to the topic,
// which drops it if byte-identical to the last frame.
func (p *powerProfilesState) publish() {
	if p.topic == nil {
		return
	}
	frame, err := json.Marshal(p.state())
	if err != nil {
		return
	}
	p.topic.publish(frame)
}

// activeProfile reads the current profile name ("power-saver"/"balanced"/
// "performance"), empty when the property is unreadable.
func (p *powerProfilesState) activeProfile() string {
	v, err := p.obj.GetProperty(ppIface + ".ActiveProfile")
	if err != nil {
		return ""
	}
	s, _ := v.Value().(string)
	return s
}

// profiles reads the ordered profile list. The daemon returns an array of
// dicts; the "Profile" key of each carries the name. Order is the daemon's
// (service order); no client sort.
func (p *powerProfilesState) profiles() []string {
	v, err := p.obj.GetProperty(ppIface + ".Profiles")
	if err != nil {
		return nil
	}
	return profileNames(v.Value())
}

// setProfile writes the active-profile property. An empty or unknown name is
// rejected before the bus call so a bad request never reaches the daemon.
func (p *powerProfilesState) setProfile(name string) error {
	if name == "" {
		return fmt.Errorf("empty profile")
	}
	known := false
	for _, n := range p.profiles() {
		if n == name {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown profile: %s", name)
	}
	return p.obj.Call("org.freedesktop.DBus.Properties.Set", 0,
		ppIface, "ActiveProfile", dbus.MakeVariant(name)).Err
}

func (p *powerProfilesState) setUserProfile(name string) error {
	if err := p.setProfile(name); err != nil {
		return err
	}
	if !gameModeActive() {
		p.saved.Store(name)
		p.persistProfile(name)
	}
	return nil
}

func (d *daemon) powerProfilesCLI(args []string) string {
	if d.pp == nil {
		return "err power profiles unavailable"
	}
	switch {
	case len(args) == 1 && args[0] == "state":
		body, err := json.Marshal(d.pp.state())
		if err != nil {
			return "err powerprofiles: " + err.Error()
		}
		return string(body)
	case len(args) == 2 && args[0] == "set":
		if err := d.pp.setUserProfile(args[1]); err != nil {
			return "err powerprofiles: " + err.Error()
		}
		return "ok"
	default:
		return "err powerprofiles: expected state or set <profile>"
	}
}

var openDirectPowerProfiles = func() (*powerProfilesState, func(), error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, nil, err
	}
	p := &powerProfilesState{
		conn: conn,
		obj:  conn.Object(ppBusName, dbus.ObjectPath(ppPath)),
	}
	return p, func() { conn.Close() }, nil
}

func directPowerProfilesCLI(args []string) error {
	p, closeConn, err := openDirectPowerProfiles()
	if err != nil {
		return fmt.Errorf("power profiles unavailable: %w", err)
	}
	defer closeConn()
	reply := (&daemon{pp: p}).powerProfilesCLI(args)
	if len(reply) >= 4 && reply[:4] == "err " {
		return fmt.Errorf("%s", reply[4:])
	}
	if reply != "" && reply != "ok" {
		fmt.Println(reply)
	}
	return nil
}

// persistedProfilePath is the daemon-owned store for the user's last explicit
// power-profile choice, kept beside power.json but separate so a ryoku-power
// write of the CPU knobs never clobbers it (and vice versa).
func persistedProfilePath() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "power-profile.json")
}

// persistProfile records the user's pick for restore after a reboot. Only
// decideProfileChange's profilePersist verdict reaches here.
func (p *powerProfilesState) persistProfile(name string) {
	if name == "" {
		return
	}
	if path := persistedProfilePath(); path != "" {
		_ = writeJSONFile(path, map[string]string{"profile": name})
	}
}

const (
	autoSwitchWindow = 6 * time.Second
	// Wider than autoSwitchWindow: ppd can settle platform_profile a few seconds
	// into the session, and a user won't reconfigure power that fast.
	restoreReassertWindow = 12 * time.Second
)

// gameModeProfilePath mirrors ryoku-cmd-game-mode's stash file, present only while
// a game-mode override holds the profile at performance. Env overrides match the
// script's.
func gameModeProfilePath() string {
	if f := os.Getenv("RYOKU_GAMEMODE_PROFILE_FILE"); f != "" {
		return f
	}
	dir := os.Getenv("RYOKU_STATE_PATH")
	if dir == "" {
		dir = filepath.Join(stateDir(), "ryoku")
	}
	return filepath.Join(dir, "game-mode.profile")
}

// gameModeActive reports whether game mode holds the profile. Its switch is
// Ryoku's, not the user's, so the daemon neither banks nor fights it; game mode
// restores the prior profile on stop.
func gameModeActive() bool {
	_, err := os.Stat(gameModeProfilePath())
	return err == nil
}

// profileAction is what the daemon does with an observed active profile.
type profileAction int

const (
	profileNone     profileAction = iota // Ryoku-caused change; leave it, do not record it
	profilePersist                       // a genuine user pick (either write path); save it
	profileReassert                      // ppd's boot default landed after restore; put the pick back
)

// decideProfileChange classifies an active profile seen on a PropertiesChanged.
// Pure, so the persist/restore precedence is unit-tested without a bus or clock.
func decideProfileChange(active, saved string, restoreDone, onBattery, saverFeature, gameMode bool, sinceACFlip, sinceRestore time.Duration) profileAction {
	switch {
	case active == "" || !restoreDone:
		return profileNone
	case gameMode:
		return profileNone
	case active == ppSaver && onBattery && saverFeature:
		return profileNone
	case sinceACFlip < autoSwitchWindow:
		return profileNone
	case saved != "" && active != saved && sinceRestore < restoreReassertWindow:
		return profileReassert
	default:
		return profilePersist
	}
}

// noteACFlip records that AC just plugged or unplugged.
func (p *powerProfilesState) noteACFlip() { p.acFlipNs.Store(time.Now().UnixNano()) }

// maybeHandleProfileChange persists a real pick (from either write path) or
// re-asserts the saved pick over a late ppd default, per decideProfileChange.
// Runs on every PropertiesChanged.
func (p *powerProfilesState) maybeHandleProfileChange() {
	active := p.activeProfile()
	saved, _ := p.saved.Load().(string)
	st := readPowerState()
	onBattery := st.present && st.discharging
	switch decideProfileChange(active, saved, p.restoreDone.Load(), onBattery,
		perfFlag("autoPowerSaverOnBattery"), gameModeActive(),
		p.sinceACFlip(), p.sinceRestore()) {
	case profilePersist:
		p.persistProfile(active)
		p.saved.Store(active)
	case profileReassert:
		if err := p.setProfile(saved); err != nil {
			log.Printf("ryoku-shell: re-assert power profile %q over ppd default %q: %v", saved, active, err)
		}
	}
}

// A never-set stamp reads as effectively infinite.
func (p *powerProfilesState) sinceACFlip() time.Duration  { return sinceStamp(p.acFlipNs.Load()) }
func (p *powerProfilesState) sinceRestore() time.Duration { return sinceStamp(p.restoreNs.Load()) }

func sinceStamp(ns int64) time.Duration {
	if ns == 0 {
		return time.Duration(1) << 62
	}
	return time.Since(time.Unix(0, ns))
}

// seedProfile is what a box that has never been told which profile to run gets
// put on, when the alternative is the firmware's own choice.
const seedProfile = "balanced"

// shouldSeedBalanced: nothing saved means this box has never been told which
// profile to run, so it is sitting on whatever power-profiles-daemon inherited
// from the firmware. On a gaming laptop that is `performance`, which pins the
// package power limit and holds the CPU near 90 C with the fans up during light
// use, and nothing in the desktop ever asked for it (#157). Seed `balanced`
// once and bank it, so the box behaves like every other install and the choice
// is visible in the Hub instead of buried in firmware.
//
// Only `performance` is corrected: a firmware default of `power-saver` is a
// deliberately quiet machine, a saved pick is the user's and is restored above,
// and a running game already owns the profile.
func shouldSeedBalanced(saved, active string, gaming bool) bool {
	return saved == "" && active == "performance" && !gaming
}

// readPersistedProfile returns the user's last saved profile, or "" when none is
// stored or it is unreadable.
func readPersistedProfile() string {
	path := persistedProfilePath()
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	return s.Profile
}

// profileNames extracts profile names from the raw Profiles property value
// (aa{sv}). It is pure so the extraction is unit-tested without a live bus.
func profileNames(v any) []string {
	rows, ok := v.([]map[string]dbus.Variant)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if pv, ok := row["Profile"]; ok {
			if name, ok := pv.Value().(string); ok && name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// applyProfileCmd re-applies a profile's definition through ryoku-power. It is a
// package var so a test can record calls without spawning the helper.
var applyProfileCmd = func(profile string) error {
	return exec.Command("ryoku-power", "apply-profile", profile).Run()
}

// powerConfigPath is ~/.config/ryoku/power.json, the CPU/battery knob store the
// Machine page and ryoku-power write. Empty when the home dir is unknowable.
func powerConfigPath() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "power.json")
}

// applyDelay is the debounce window before re-applying the active profile, from
// applyDelayMs in power.json (400ms on a missing, absent, or non-positive value).
func applyDelay() time.Duration {
	const def = 400 * time.Millisecond
	b, err := os.ReadFile(powerConfigPath())
	if err != nil {
		return def
	}
	var m struct {
		ApplyDelayMs int `json:"applyDelayMs"`
	}
	if json.Unmarshal(b, &m) != nil || m.ApplyDelayMs <= 0 {
		return def
	}
	return time.Duration(m.ApplyDelayMs) * time.Millisecond
}

// shouldApplyProfile returns the profile to re-apply, or "" to leave the hardware
// alone. Pure, so the safety guard is unit-tested without a bus or exec: an empty
// active profile, a power.json with no "profiles" block, or no entry for the
// active profile all yield "", so the hook execs nothing and never prompts a user
// who has not defined a profile.
func shouldApplyProfile(cfg []byte, active string) string {
	if active == "" {
		return ""
	}
	var m struct {
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	if json.Unmarshal(cfg, &m) != nil {
		return ""
	}
	if _, ok := m.Profiles[active]; !ok {
		return ""
	}
	return active
}

// scheduleApply debounces re-applying the active profile. ppd writes its own
// governor/EPP/platform_profile when the profile changes, so applying the user's
// definition immediately would be overwritten; the delay lets ppd settle first.
// A burst of PropertiesChanged coalesces into one apply by restarting the timer.
//
// The profile is resolved when the timer fires, not when it is scheduled: at
// signal time ppd may not have published the new ActiveProfile yet, and reading
// it then would apply the outgoing profile's definition over the incoming one.
func (p *powerProfilesState) scheduleApply() {
	if p.applyTimer != nil {
		p.applyTimer.Stop()
	}
	p.applyTimer = time.AfterFunc(applyDelay(), func() { p.applyActiveProfile(p.activeProfile()) })
}

// applyActiveProfile writes the user's definition for active to sysfs via
// ryoku-power. It runs on the timer goroutine, never the signal goroutine, so a
// slow or prompting helper cannot stall signal handling. It no-ops (never execs)
// when the profile is not configured, and logs a failure at most once per profile.
func (p *powerProfilesState) applyActiveProfile(active string) {
	cfg, _ := os.ReadFile(powerConfigPath())
	if shouldApplyProfile(cfg, active) == "" {
		return
	}
	p.applyMu.Lock()
	defer p.applyMu.Unlock()
	if err := applyProfileCmd(active); err != nil {
		if p.lastApplyErr != active {
			p.lastApplyErr = active
			log.Printf("ryoku-shell: apply-profile %q failed: %v", active, err)
		}
		return
	}
	p.lastApplyErr = ""
}
