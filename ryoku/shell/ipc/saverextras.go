package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	wm "ryoku-wm"
)

const saverExtrasActionTimeout = 2 * time.Second

type brightnessChange struct {
	Original int `json:"original"`
	Set      int `json:"set"`
}

type refreshChange struct {
	Original string `json:"original"`
	Set      string `json:"set"`
}

type saverExtrasState struct {
	Keyboard map[string]brightnessChange `json:"keyboard,omitempty"`
	Screen   map[string]brightnessChange `json:"screen,omitempty"`
	Outputs  map[string]refreshChange    `json:"outputs,omitempty"`
}

type saverExtrasFlags struct {
	Keyboard bool
	Panel    bool
	Screen   bool
}

type brightnessSetter func(subsystem, name string, value int) error

type saverExtrasController struct {
	mu sync.Mutex

	path          string
	ledsRoot      string
	backlightRoot string
	setBrightness brightnessSetter
	outputs       func() ([]wm.Output, error)
	setRefresh    func(context.Context, string, string) error

	loaded bool
	active bool
	state  saverExtrasState
	logged map[string]bool
}

func saverExtrasPath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "ryoku", "saver-extras.json")
}

func newSaverExtrasController(d *daemon) *saverExtrasController {
	return &saverExtrasController{
		path:          saverExtrasPath(),
		ledsRoot:      "/sys/class/leds",
		backlightRoot: "/sys/class/backlight",
		setBrightness: func(subsystem, name string, value int) error {
			var conn *dbus.Conn
			if d.pp != nil {
				conn = d.pp.conn
			}
			return setSaverBrightness(conn, subsystem, name, value)
		},
		outputs: func() ([]wm.Output, error) {
			snapshot, err := d.wmc.State()
			return snapshot.Outputs, err
		},
		setRefresh: func(ctx context.Context, name, hz string) error {
			return d.wmc.ActContext(ctx, wm.ActionOutputRefresh, name, hz)
		},
		logged: make(map[string]bool),
	}
}

func (d *daemon) kickSaverExtras() {
	if d.saverExtrasKick == nil {
		return
	}
	select {
	case d.saverExtrasKick <- struct{}{}:
	default:
	}
}

func (d *daemon) reconcileSaverExtras(ctx context.Context, onBattery bool, activeProfile string) {
	if d.saverExtras == nil {
		d.saverExtras = newSaverExtrasController(d)
	}
	active := onBattery && activeProfile == ppSaver && !gameModeActive()
	d.saverExtras.reconcile(ctx, active, saverExtrasFlags{
		Keyboard: perfFlagDefault("saverDimKeyboard", true),
		Panel:    perfFlagDefault("saverPanel60", true),
		Screen:   perfFlagDefault("saverDimScreen", true),
	})
}

func (c *saverExtrasController) reconcile(ctx context.Context, active bool, flags saverExtrasFlags) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.load()
	if active && !c.active {
		c.logged = make(map[string]bool)
	}
	c.active = active

	c.reconcileBrightness(c.ledsRoot, "::kbd_backlight", "leds", active && flags.Keyboard, c.state.Keyboard, keyboardTarget)
	c.reconcileBrightness(c.backlightRoot, "", "backlight", active && flags.Screen, c.state.Screen, brightnessTarget)
	c.reconcilePanels(ctx, active && flags.Panel)
}

func (c *saverExtrasController) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	c.ensureMaps()
	if c.path == "" {
		return
	}
	b, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		c.logOnce("state-read", "read state: %v", err)
		return
	}
	if err := json.Unmarshal(b, &c.state); err != nil {
		c.logOnce("state-read", "read state: %v", err)
		c.state = saverExtrasState{}
	}
	c.ensureMaps()
}

func (c *saverExtrasController) ensureMaps() {
	if c.state.Keyboard == nil {
		c.state.Keyboard = make(map[string]brightnessChange)
	}
	if c.state.Screen == nil {
		c.state.Screen = make(map[string]brightnessChange)
	}
	if c.state.Outputs == nil {
		c.state.Outputs = make(map[string]refreshChange)
	}
}

func (c *saverExtrasController) save() error {
	if c.path == "" {
		return nil
	}
	if len(c.state.Keyboard) == 0 && len(c.state.Screen) == 0 && len(c.state.Outputs) == 0 {
		if err := os.Remove(c.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeJSONFile(c.path, c.state)
}

func (c *saverExtrasController) saveOrLog() bool {
	if err := c.save(); err != nil {
		c.logOnce("state-write", "write state: %v", err)
		return false
	}
	return true
}

func (c *saverExtrasController) logOnce(key, format string, args ...any) {
	if c.logged == nil {
		c.logged = make(map[string]bool)
	}
	if c.logged[key] {
		return
	}
	c.logged[key] = true
	log.Printf("ryoku-shell: saver extras: "+format, args...)
}

func keyboardTarget(current, maximum int) (int, bool) {
	if maximum <= 1 || current <= 1 {
		return current, false
	}
	return 1, true
}

func brightnessTarget(current, maximum int) (int, bool) {
	if current <= 0 || maximum <= 0 {
		return current, false
	}
	floor := int(math.Round(float64(maximum) * 0.10))
	target := int(math.Round(float64(current) * 0.60))
	if target < floor {
		target = floor
	}
	if target >= current {
		return current, false
	}
	return target, true
}

func shouldRestore(current, set int) bool {
	return current == set
}

type brightnessTargetFunc func(current, maximum int) (int, bool)

func (c *saverExtrasController) reconcileBrightness(root, suffix, subsystem string, enabled bool, changes map[string]brightnessChange, target brightnessTargetFunc) {
	if !enabled {
		for name, change := range changes {
			current, err := readBrightness(filepath.Join(root, name), "brightness")
			if errors.Is(err, os.ErrNotExist) {
				delete(changes, name)
				c.saveOrLog()
				continue
			}
			if err != nil {
				c.logOnce(subsystem+":"+name+":read-restore", "%s %q restore read failed: %v", subsystem, name, err)
				continue
			}
			if !shouldRestore(current, change.Set) {
				delete(changes, name)
				c.saveOrLog()
				continue
			}
			if err := c.setBrightness(subsystem, name, change.Original); err != nil {
				c.logOnce(subsystem+":"+name+":restore", "%s %q restore failed: %v", subsystem, name, err)
				continue
			}
			delete(changes, name)
			c.saveOrLog()
		}
		return
	}

	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		c.logOnce(subsystem+":scan", "scan %s: %v", subsystem, err)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if suffix != "" && !strings.HasSuffix(name, suffix) {
			continue
		}
		if _, tracked := changes[name]; tracked {
			continue
		}
		device := filepath.Join(root, name)
		maximum, err := readBrightness(device, "max_brightness")
		if err != nil {
			c.logOnce(subsystem+":"+name+":read-max", "%s %q max read failed: %v", subsystem, name, err)
			continue
		}
		current, err := readBrightness(device, "brightness")
		if err != nil {
			c.logOnce(subsystem+":"+name+":read", "%s %q read failed: %v", subsystem, name, err)
			continue
		}
		value, change := target(current, maximum)
		if !change {
			continue
		}
		changes[name] = brightnessChange{Original: current, Set: value}
		if !c.saveOrLog() {
			delete(changes, name)
			continue
		}
		if err := c.setBrightness(subsystem, name, value); err != nil {
			c.logOnce(subsystem+":"+name+":set", "%s %q dim failed: %v", subsystem, name, err)
			delete(changes, name)
			c.saveOrLog()
		}
	}
}

func readBrightness(device, field string) (int, error) {
	b, err := os.ReadFile(filepath.Join(device, field))
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	return value, nil
}

func setSaverBrightness(conn *dbus.Conn, subsystem, name string, value int) error {
	var dbusErr error
	if conn == nil {
		owned, err := dbus.ConnectSystemBus()
		if err != nil {
			dbusErr = err
		} else {
			conn = owned
			defer owned.Close()
		}
	}
	if conn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), saverExtrasActionTimeout)
		call := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1/session/auto")).CallWithContext(
			ctx, "org.freedesktop.login1.Session.SetBrightness", 0, subsystem, name, uint32(value))
		cancel()
		dbusErr = call.Err
		if dbusErr == nil {
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), saverExtrasActionTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "brightnessctl", "-d", name, "s", strconv.Itoa(value)).Run(); err != nil {
		return fmt.Errorf("logind: %v; brightnessctl: %w", dbusErr, err)
	}
	return nil
}

func builtInOutput(name string) bool {
	return strings.HasPrefix(name, "eDP") || strings.HasPrefix(name, "LVDS") || strings.HasPrefix(name, "DSI")
}

func parseMode(mode string) (resolution, refresh string, hz float64, ok bool) {
	at := strings.LastIndexByte(mode, '@')
	if at <= 0 || at == len(mode)-1 {
		return "", "", 0, false
	}
	resolution = mode[:at]
	refresh = mode[at+1:]
	if !strings.Contains(resolution, "x") {
		return "", "", 0, false
	}
	hz, err := strconv.ParseFloat(refresh, 64)
	if err != nil || hz <= 0 {
		return "", "", 0, false
	}
	return resolution, refresh, hz, true
}

func panelModePick(output wm.Output) (original, target string, ok bool) {
	if !builtInOutput(output.Name) {
		return "", "", false
	}
	resolution, original, currentHz, ok := parseMode(output.Mode)
	if !ok || currentHz <= 75 {
		return "", "", false
	}
	bestDistance := math.MaxFloat64
	for _, mode := range output.Modes {
		candidateResolution, candidate, hz, valid := parseMode(mode)
		if !valid || candidateResolution != resolution || hz < 59 || hz > 61 {
			continue
		}
		distance := math.Abs(hz - 60)
		if distance < bestDistance {
			bestDistance = distance
			target = candidate
		}
	}
	return original, target, target != ""
}

func refreshUnchanged(current, set string) bool {
	currentHz, err := strconv.ParseFloat(current, 64)
	if err != nil {
		return false
	}
	setHz, err := strconv.ParseFloat(set, 64)
	if err != nil {
		return false
	}
	return math.Abs(currentHz-setHz) < 0.001
}

func (c *saverExtrasController) reconcilePanels(ctx context.Context, enabled bool) {
	if !enabled && len(c.state.Outputs) == 0 {
		return
	}
	if c.outputs == nil || c.setRefresh == nil {
		return
	}
	outputs, err := c.outputs()
	if err != nil {
		c.logOnce("outputs:state", "read outputs: %v", err)
		return
	}
	byName := make(map[string]wm.Output, len(outputs))
	for _, output := range outputs {
		byName[output.Name] = output
	}

	if !enabled {
		for name, change := range c.state.Outputs {
			output, exists := byName[name]
			if !exists {
				delete(c.state.Outputs, name)
				c.saveOrLog()
				continue
			}
			_, current, _, valid := parseMode(output.Mode)
			if !valid || !refreshUnchanged(current, change.Set) {
				delete(c.state.Outputs, name)
				c.saveOrLog()
				continue
			}
			actionCtx, cancel := context.WithTimeout(ctx, saverExtrasActionTimeout)
			err := c.setRefresh(actionCtx, name, change.Original)
			cancel()
			if err != nil {
				c.logOnce("output:"+name+":restore", "output %q restore failed: %v", name, err)
				continue
			}
			delete(c.state.Outputs, name)
			c.saveOrLog()
		}
		return
	}

	for _, output := range outputs {
		if _, tracked := c.state.Outputs[output.Name]; tracked {
			continue
		}
		original, target, change := panelModePick(output)
		if !change {
			continue
		}
		c.state.Outputs[output.Name] = refreshChange{Original: original, Set: target}
		if !c.saveOrLog() {
			delete(c.state.Outputs, output.Name)
			continue
		}
		actionCtx, cancel := context.WithTimeout(ctx, saverExtrasActionTimeout)
		err := c.setRefresh(actionCtx, output.Name, "60")
		cancel()
		if err != nil {
			c.logOnce("output:"+output.Name+":set", "output %q 60 Hz failed: %v", output.Name, err)
			delete(c.state.Outputs, output.Name)
			c.saveOrLog()
		}
	}
}
