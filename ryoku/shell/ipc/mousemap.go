package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The mouse remapper seizes each physical mouse (EVIOCGRAB), publishes a uinput
// clone that carries every event straight through, and rewrites only the mapped
// buttons into a key/button sequence. Grabbing at the evdev layer means the
// rewrite lands in every app, the lockscreen, and under any window manager, with
// nothing here that has to know which one is running. A mouse with no mappings is
// left ungrabbed so it stays byte-for-byte transparent.

const (
	// struct input_event on 64-bit: two 64-bit times (sec, usec), then u16 type,
	// u16 code, i32 value. Native endian, the kernel fills timestamps on write.
	mouseEventSize = 24

	// evKey (EV_KEY = 1) is declared in keypress.go; reuse it.
	evSyn = 0x00
	evRel = 0x02
	evMsc = 0x04

	relX      = 0x00
	relY      = 0x01
	relHWheel = 0x06
	relDial   = 0x07
	relWheel  = 0x08
	mscScan   = 0x04

	btnMouseFirst   = 0x110 // BTN_MOUSE
	btnMouseLast    = 0x11f // complete mouse BTN_* block
	btnTriggerFirst = 0x2c0 // BTN_TRIGGER_HAPPY1
	btnTriggerLast  = 0x2e7 // BTN_TRIGGER_HAPPY40
	btnToolFinger   = 0x145 // BTN_TOOL_FINGER
	keyMax          = 0x2ff
	keyNumberFirst  = 2     // KEY_1
	keyNumberLast   = 13    // KEY_EQUAL, completing a 12-button side grid
	keyMacroFirst   = 0x290 // KEY_MACRO1
	keyMacroLast    = 0x2ad // KEY_MACRO30

	busUSB = 0x03 // BUS_USB, the bustype the clone advertises

	// EVIOCGRAB is Linux _IOW('E', 0x90, int): seize the device so its events
	// reach only this reader. Derived like keypress.go's EVIOCSCLOCKID
	// (_IOW('E', 0xa0, int) = 0x400445a0); 'E' is 0x45, sizeof(int) is 4.
	evIoCGrab = 0x40044590

	// uinput control ioctls (linux/uinput.h). 'U' is 0x55; _IO('U', n) for the
	// no-argument pair, _IOW('U', n, T) for the setters.
	uiDevCreate  = 0x5501     // _IO('U', 1)
	uiDevDestroy = 0x5502     // _IO('U', 2)
	uiDevSetup   = 0x405c5503 // _IOW('U', 3, struct uinput_setup), sizeof 92 (0x5c)
	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565 // _IOW('U', 101, int)
	uiSetRelBit  = 0x40045566 // _IOW('U', 102, int)
	uiSetAbsBit  = 0x40045567 // _IOW('U', 103, int) — declared for parity, unused
	uiSetMscBit  = 0x40045568 // _IOW('U', 104, int)

	// struct uinput_setup: struct input_id id (8), char name[80], u32
	// ff_effects_max, padded to 92. The kernel ABI orders id before name.
	uinputSetupSize = 92
	uinputNameSize  = 80
)

// chordKeyCodes maps a chord token to its Linux KEY_* code — the same stable
// evdev ABI keypress.go's inputKeys encodes, left-hand modifier variants. The
// Hub emits these tokens; an unknown one rejects the map rather than binding a
// silent no-op.
var chordKeyCodes = map[string]uint16{
	"a": 30, "b": 48, "c": 46, "d": 32, "e": 18, "f": 33, "g": 34, "h": 35, "i": 23,
	"j": 36, "k": 37, "l": 38, "m": 50, "n": 49, "o": 24, "p": 25, "q": 16, "r": 19,
	"s": 31, "t": 20, "u": 22, "v": 47, "w": 17, "x": 45, "y": 21, "z": 44,

	"1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10, "0": 11,

	"f1": 59, "f2": 60, "f3": 61, "f4": 62, "f5": 63, "f6": 64,
	"f7": 65, "f8": 66, "f9": 67, "f10": 68, "f11": 87, "f12": 88,

	"enter": 28, "esc": 1, "space": 57, "tab": 15, "backspace": 14,
	"left": 105, "right": 106, "up": 103, "down": 108,
	"home": 102, "end": 107, "pgup": 104, "pgdn": 109, "insert": 110, "delete": 111, "print": 99,
	"minus": 12, "equal": 13, "bracketleft": 26, "bracketright": 27,
	"comma": 51, "period": 52, "slash": 53, "semicolon": 39,
	"apostrophe": 40, "backslash": 43, "grave": 41,

	"ctrl": 29, "shift": 42, "alt": 56, "super": 125,
}

// chordCodes resolves the token sequence a chord binds to, in press order. An
// unrecognised token is an error so a bad map never silently drops keys.
func chordCodes(keys []string) ([]uint16, error) {
	out := make([]uint16, 0, len(keys))
	for _, key := range keys {
		code, ok := chordKeyCodes[strings.ToLower(strings.TrimSpace(key))]
		if !ok {
			return nil, fmt.Errorf("unknown chord key %q", key)
		}
		out = append(out, code)
	}
	return out, nil
}

// mouseTarget is one button's rewrite: a key chord, another button, a timed
// sequence, or nothing.
type mouseTarget struct {
	Kind            string           `json:"kind"`
	Keys            []string         `json:"keys,omitempty"`
	Button          uint16           `json:"button,omitempty"`
	Sequence        []mouseMacroStep `json:"sequence,omitempty"`
	Repeat          int              `json:"repeat,omitempty"`
	CancelOnRelease bool             `json:"cancelOnRelease,omitempty"`
}

// validateTarget rejects a mapping the reader could not honour, so mouse.map
// fails loudly instead of arming a reader that will misbehave.
func validateTarget(t mouseTarget) error {
	switch t.Kind {
	case "disabled":
		return nil
	case "button":
		if t.Button == 0 {
			return fmt.Errorf("button target requires a button code")
		}
		return nil
	case "chord":
		if len(t.Keys) == 0 {
			return fmt.Errorf("chord target requires at least one key")
		}
		_, err := chordCodes(t.Keys)
		return err
	case "sequence":
		return validateMouseSequence(t)
	default:
		return fmt.Errorf("unknown target kind %q", t.Kind)
	}
}

// synthEvent is one decoded event the reader forwards to the clone.
type synthEvent struct {
	etype uint16
	code  uint16
	value int32
}

// downUp turns a target into the events to emit on press and on release. The
// release list is the press list reversed, so a chord unwinds outermost-last.
func (t mouseTarget) downUp() (down, up []synthEvent, err error) {
	switch t.Kind {
	case "disabled":
		return nil, nil, nil
	case "button":
		return []synthEvent{{evKey, t.Button, 1}}, []synthEvent{{evKey, t.Button, 0}}, nil
	case "chord":
		codes, err := chordCodes(t.Keys)
		if err != nil {
			return nil, nil, err
		}
		down = make([]synthEvent, 0, len(codes))
		up = make([]synthEvent, 0, len(codes))
		for _, c := range codes {
			down = append(down, synthEvent{evKey, c, 1})
		}
		for i := len(codes) - 1; i >= 0; i-- {
			up = append(up, synthEvent{evKey, codes[i], 0})
		}
		return down, up, nil
	case "sequence":
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown target kind %q", t.Kind)
	}
}

type mouseBinding struct {
	down []synthEvent
	up   []synthEvent
}

// mouseSubstitutor is the pure rewrite core: it turns a source EV_KEY on a mapped
// button into the target's events and reports whether the source event was
// consumed. It tracks the outstanding press per button so a release unwinds the
// exact down sequence even if pointer motion arrives in between.
type mouseSubstitutor struct {
	bindings map[uint16]mouseBinding
	held     map[uint16][]synthEvent
}

func newMouseSubstitutor(mapping map[uint16]mouseTarget) (*mouseSubstitutor, error) {
	s := &mouseSubstitutor{
		bindings: make(map[uint16]mouseBinding, len(mapping)),
		held:     make(map[uint16][]synthEvent),
	}
	for code, target := range mapping {
		down, up, err := target.downUp()
		if err != nil {
			return nil, err
		}
		s.bindings[code] = mouseBinding{down: down, up: up}
	}
	return s, nil
}

// apply returns the events to emit for one decoded source event and whether the
// source event was consumed (dropped from pass-through). Unmapped codes and
// every non-EV_KEY event pass through untouched.
func (s *mouseSubstitutor) apply(etype, code uint16, value int32) (out []synthEvent, consumed bool) {
	if etype != evKey {
		return nil, false
	}
	binding, ok := s.bindings[code]
	if !ok {
		return nil, false
	}
	switch value {
	case 1:
		s.held[code] = binding.up
		return binding.down, true
	case 0:
		up, held := s.held[code]
		delete(s.held, code)
		if !held {
			return nil, true
		}
		return up, true
	default:
		// Autorepeat and any other value: swallow, mouse buttons never repeat.
		return nil, true
	}
}

// mouseInterface is one evdev interface in a physical mouse. MMO mice often put
// their side grid on a keyboard-class sibling, so the logical card may own more
// than one source and clone.
type mouseInterface struct {
	node      string
	sysDevice string
	buttons   []uint16
	keys      []uint16
	keyboard  bool
}

// mouseDevice is one enumerated physical mouse. interfaces and the numeric USB
// identity stay internal; the Hub receives the catalogue-enriched wire fields.
type mouseDevice struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Bus         string            `json:"bus"`
	Brand       string            `json:"brand"`
	Model       string            `json:"model"`
	Labels      map[string]string `json:"labels"`
	Buttons     []uint16          `json:"buttons"`
	SideButtons []uint16          `json:"sideButtons,omitempty"`

	vendor     uint16
	product    uint16
	interfaces []mouseInterface
}

type mousePressed struct {
	Device string `json:"device"`
	Button uint16 `json:"button"`
}

// mouseFrame is the one line published on every state change. pressed rides only
// on the frame that answers a capture.
type mouseFrame struct {
	Devices []mouseDevice                     `json:"devices"`
	Maps    map[string]map[string]mouseTarget `json:"maps"`
	Pressed *mousePressed                     `json:"pressed,omitempty"`
}

// mouseSettings is the on-disk store, deviceKey -> buttonCode(string) -> target.
type mouseSettings struct {
	Maps map[string]map[string]mouseTarget `json:"maps"`
}

func mouseSettingsPath() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mousemap.json")
}

// loadMouseSettings reads the store, dropping any entry the reader could not
// honour so a hand-edited or stale file can never wedge the daemon.
func loadMouseSettings(path string) mouseSettings {
	settings := mouseSettings{Maps: map[string]map[string]mouseTarget{}}
	body, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	var saved mouseSettings
	if json.Unmarshal(body, &saved) != nil {
		return settings
	}
	for device, buttons := range saved.Maps {
		clean := map[string]mouseTarget{}
		for code, target := range buttons {
			if _, err := strconv.ParseUint(code, 10, 16); err != nil {
				continue
			}
			if validateTarget(target) != nil {
				continue
			}
			clean[code] = target
		}
		if len(clean) > 0 {
			settings.Maps[device] = clean
		}
	}
	return settings
}

// writeMouseSettings persists atomically (temp + rename) so a crash mid-write
// never leaves a half-written map.
func writeMouseSettings(path string, settings mouseSettings) error {
	if path == "" {
		return fmt.Errorf("mouse settings path is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// cloneMouseMaps deep-copies the store so a patch never mutates a map a reader
// still holds.
func cloneMouseMaps(maps map[string]map[string]mouseTarget) map[string]map[string]mouseTarget {
	out := make(map[string]map[string]mouseTarget, len(maps))
	for device, buttons := range maps {
		copied := make(map[string]mouseTarget, len(buttons))
		for code, target := range buttons {
			copied[code] = target
		}
		out[device] = copied
	}
	return out
}

// patchMouseMaps applies one edit: a nil target removes the button, and a device
// left with no buttons drops out entirely.
func patchMouseMaps(maps map[string]map[string]mouseTarget, device string, button uint16, target *mouseTarget) map[string]map[string]mouseTarget {
	out := cloneMouseMaps(maps)
	key := strconv.FormatUint(uint64(button), 10)
	if target == nil {
		if buttons, ok := out[device]; ok {
			delete(buttons, key)
			if len(buttons) == 0 {
				delete(out, device)
			}
		}
		return out
	}
	buttons := out[device]
	if buttons == nil {
		buttons = map[string]mouseTarget{}
		out[device] = buttons
	}
	buttons[key] = *target
	return out
}

// deviceMapCodes converts one device's string-keyed map into the uint16-keyed
// form the substitutor and clone want.
func deviceMapCodes(buttons map[string]mouseTarget) map[uint16]mouseTarget {
	out := make(map[uint16]mouseTarget, len(buttons))
	for code, target := range buttons {
		parsed, err := strconv.ParseUint(code, 10, 16)
		if err != nil {
			continue
		}
		out[uint16(parsed)] = target
	}
	return out
}

func mouseInterfaceMap(mapping map[uint16]mouseTarget, buttons []uint16) map[uint16]mouseTarget {
	if len(mapping) == 0 {
		return nil
	}
	out := map[uint16]mouseTarget{}
	for _, button := range buttons {
		if target, ok := mapping[button]; ok {
			out[button] = target
		}
	}
	return out
}

func mouseCodeMapSig(mapping map[uint16]mouseTarget) string {
	if len(mapping) == 0 {
		return ""
	}
	body, _ := json.Marshal(mapping)
	return string(body)
}

func mouseDevicesEqual(a, b []mouseDevice) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name || a[i].Bus != b[i].Bus ||
			a[i].Brand != b[i].Brand || a[i].Model != b[i].Model ||
			len(a[i].Buttons) != len(b[i].Buttons) ||
			len(a[i].SideButtons) != len(b[i].SideButtons) ||
			len(a[i].interfaces) != len(b[i].interfaces) {
			return false
		}
		for j := range a[i].Buttons {
			if a[i].Buttons[j] != b[i].Buttons[j] {
				return false
			}
		}
		for j := range a[i].SideButtons {
			if a[i].SideButtons[j] != b[i].SideButtons[j] {
				return false
			}
		}
		for j := range a[i].interfaces {
			if a[i].interfaces[j].node != b[i].interfaces[j].node {
				return false
			}
		}
	}
	return true
}

type mouseInputCandidate struct {
	node, real, name, phys, uniq, usbRoot string
	vendor, product                       uint16
	bits                                  *big.Int
	keys                                  []uint16
	mouseButtons                          []uint16
	sideButtons                           []uint16
}

type mouseInputCacheEntry struct {
	inode     uint64
	ctime     unix.Timespec
	candidate mouseInputCandidate
	usable    bool
}

type mouseInputFactsCache map[string]mouseInputCacheEntry

func sameMouseInputCacheKey(entry mouseInputCacheEntry, stat unix.Stat_t) bool {
	return entry.inode == stat.Ino &&
		entry.ctime.Sec == stat.Ctim.Sec &&
		entry.ctime.Nsec == stat.Ctim.Nsec
}

func inspectMouseInput(base, node, real, virtualPrefix string) (mouseInputCandidate, bool) {
	capRaw, err := os.ReadFile(filepath.Join(base, "device", "capabilities", "key"))
	if err != nil {
		return mouseInputCandidate{}, false
	}
	bits, ok := parseKeyCapabilities(string(capRaw))
	if !ok {
		return mouseInputCandidate{}, false
	}
	name := readMouseSysText(filepath.Join(base, "device", "name"))
	isTestVirtual := os.Getenv("RYOKU_MOUSE_TEST_VIRTUAL") == "1" &&
		strings.HasPrefix(name, "Ryoku Test Mouse ")
	if strings.HasPrefix(real, virtualPrefix) && !isTestVirtual {
		return mouseInputCandidate{}, false
	}
	if strings.HasPrefix(name, "Ryoku Mouse") {
		return mouseInputCandidate{}, false
	}
	vendor, product, usbRoot := sysMouseIdentity(real)
	if vendor == 0 || product == 0 {
		vendor, _ = readSysHex(filepath.Join(base, "device", "id", "vendor"))
		product, _ = readSysHex(filepath.Join(base, "device", "id", "product"))
	}
	return mouseInputCandidate{
		node:         node,
		real:         real,
		name:         name,
		phys:         readMouseSysText(filepath.Join(base, "device", "phys")),
		uniq:         readMouseSysText(filepath.Join(base, "device", "uniq")),
		usbRoot:      usbRoot,
		vendor:       vendor,
		product:      product,
		bits:         bits,
		keys:         mouseAllKeys(bits),
		mouseButtons: mousePointerButtons(bits),
		sideButtons:  mouseSideGridButtons(bits),
	}, true
}

func classifyMouseDevices(sysRoot, devRoot, byIDRoot string) []mouseDevice {
	cache := mouseInputFactsCache{}
	return classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache)
}

// classifyMouseDevicesCached avoids reopening immutable sysfs attributes while
// an event node keeps the same inode and ctime. Recreated nodes are inspected
// before they can inherit facts from a device that was unplugged.
func classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot string, cache mouseInputFactsCache) []mouseDevice {
	if cache == nil {
		cache = mouseInputFactsCache{}
	}
	inputDir := filepath.Join(sysRoot, "class", "input")
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return []mouseDevice{}
	}
	virtualPrefix := filepath.Join(sysRoot, "devices", "virtual", "input")
	candidates := []mouseInputCandidate{}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		event := entry.Name()
		if !strings.HasPrefix(event, "event") {
			continue
		}
		node := filepath.Join(devRoot, event)
		var stat unix.Stat_t
		if err := unix.Stat(node, &stat); err != nil {
			continue
		}
		seen[event] = true
		if cached, ok := cache[event]; ok && sameMouseInputCacheKey(cached, stat) {
			if cached.usable {
				candidates = append(candidates, cached.candidate)
			}
			continue
		}
		base := filepath.Join(inputDir, event)
		real, err := filepath.EvalSymlinks(filepath.Join(base, "device"))
		if err != nil {
			real = filepath.Join(base, "device")
		}
		candidate, usable := inspectMouseInput(base, node, real, virtualPrefix)
		cache[event] = mouseInputCacheEntry{
			inode: stat.Ino, ctime: stat.Ctim, candidate: candidate, usable: usable,
		}
		if usable {
			candidates = append(candidates, candidate)
		}
	}
	for event := range cache {
		if !seen[event] {
			delete(cache, event)
		}
	}

	out := []mouseDevice{}
	for i := range candidates {
		base := candidates[i]
		if base.bits.Bit(btnMouseFirst) == 0 || base.bits.Bit(btnToolFinger) != 0 {
			continue
		}
		entry := matchMouseCatalogue(base.vendor, base.product, base.name)
		dev := mouseDevice{
			ID:      mouseDeviceID(byIDRoot, base.node),
			Name:    base.name,
			Bus:     mouseBus(base.real),
			Labels:  map[string]string{},
			Buttons: append([]uint16(nil), base.mouseButtons...),
			vendor:  base.vendor,
			product: base.product,
			interfaces: []mouseInterface{{
				node:      base.node,
				sysDevice: base.real,
				buttons:   append([]uint16(nil), base.mouseButtons...),
				keys:      append([]uint16(nil), base.keys...),
			}},
		}
		if entry != nil {
			dev.Brand = entry.Brand
			dev.Model = entry.Model
			dev.Labels = copyMouseLabels(entry.Labels)
		}

		for j := range candidates {
			if i == j {
				continue
			}
			sibling := candidates[j]
			if sibling.bits.Bit(btnMouseFirst) != 0 || len(sibling.sideButtons) == 0 ||
				!samePhysicalMouseInput(base, sibling) {
				continue
			}
			dev.interfaces = append(dev.interfaces, mouseInterface{
				node:      sibling.node,
				sysDevice: sibling.real,
				buttons:   append([]uint16(nil), sibling.sideButtons...),
				keys:      append([]uint16(nil), sibling.keys...),
				keyboard:  true,
			})
			for _, code := range sibling.sideButtons {
				dev.SideButtons = appendMouseCode(dev.SideButtons, code)
				dev.Buttons = appendMouseCode(dev.Buttons, code)
			}
		}
		sort.Slice(dev.Buttons, func(i, j int) bool { return dev.Buttons[i] < dev.Buttons[j] })
		sort.Slice(dev.SideButtons, func(i, j int) bool { return dev.SideButtons[i] < dev.SideButtons[j] })
		out = append(out, dev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func readMouseSysText(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func mouseAllKeys(bits *big.Int) []uint16 {
	keys := []uint16{}
	for code := range keyMax + 1 {
		if bits.Bit(code) != 0 {
			keys = append(keys, uint16(code))
		}
	}
	return keys
}

func mousePointerButtons(bits *big.Int) []uint16 {
	buttons := []uint16{}
	for code := btnMouseFirst; code <= btnMouseLast; code++ {
		if bits.Bit(code) != 0 {
			buttons = append(buttons, uint16(code))
		}
	}
	for code := btnTriggerFirst; code <= btnTriggerLast; code++ {
		if bits.Bit(code) != 0 {
			buttons = append(buttons, uint16(code))
		}
	}
	return buttons
}

func mouseSideGridButtons(bits *big.Int) []uint16 {
	buttons := []uint16{}
	addRange := func(first, last int) {
		for code := first; code <= last; code++ {
			if bits.Bit(code) != 0 {
				buttons = appendMouseCode(buttons, uint16(code))
			}
		}
	}
	addRange(keyNumberFirst, keyNumberLast)
	addRange(148, 149) // KEY_PROG1..KEY_PROG2
	addRange(183, 194) // KEY_F13..KEY_F24
	addRange(202, 203) // KEY_PROG3..KEY_PROG4
	addRange(keyMacroFirst, keyMacroLast)
	if bits.Bit(112) != 0 { // KEY_MACRO
		buttons = appendMouseCode(buttons, 112)
	}
	sort.Slice(buttons, func(i, j int) bool { return buttons[i] < buttons[j] })
	return buttons
}

func appendMouseCode(codes []uint16, code uint16) []uint16 {
	for _, existing := range codes {
		if existing == code {
			return codes
		}
	}
	return append(codes, code)
}

func samePhysicalMouseInput(a, b mouseInputCandidate) bool {
	if a.usbRoot != "" && b.usbRoot != "" {
		return a.usbRoot == b.usbRoot
	}
	if a.uniq != "" && b.uniq != "" {
		return a.uniq == b.uniq
	}
	aPhys, bPhys := mousePhysRoot(a.phys), mousePhysRoot(b.phys)
	if aPhys != "" && bPhys != "" {
		return aPhys == bPhys
	}
	return a.vendor != 0 && a.vendor == b.vendor && a.product != 0 && a.product == b.product
}

func mousePhysRoot(phys string) string {
	if at := strings.LastIndex(phys, "/input"); at >= 0 {
		return phys[:at]
	}
	return phys
}

// mouseDeviceID prefers the by-id symlink resolving to this event node, which
// survives a replug; the raw eventN name (which does not) is the fallback.
func mouseDeviceID(byIDRoot, node string) string {
	target, err := filepath.EvalSymlinks(node)
	if err != nil {
		target = node
	}
	entries, err := os.ReadDir(byIDRoot)
	if err == nil {
		for _, entry := range entries {
			link := filepath.Join(byIDRoot, entry.Name())
			resolved, err := filepath.EvalSymlinks(link)
			if err != nil {
				continue
			}
			if resolved == target {
				return entry.Name()
			}
		}
	}
	return filepath.Base(node)
}

func mouseBus(realpath string) string {
	switch {
	case strings.Contains(realpath, "/usb"):
		return "usb"
	case strings.Contains(realpath, "/bluetooth"):
		return "bluetooth"
	case strings.Contains(realpath, "/serio"):
		return "ps2"
	default:
		return "other"
	}
}

func decodeInputEvent(raw []byte) (etype, code uint16, value int32) {
	etype = binary.NativeEndian.Uint16(raw[16:18])
	code = binary.NativeEndian.Uint16(raw[18:20])
	value = int32(binary.NativeEndian.Uint32(raw[20:24]))
	return
}

func writeInputEvent(w io.Writer, event synthEvent) error {
	var raw [mouseEventSize]byte
	binary.NativeEndian.PutUint16(raw[16:18], event.etype)
	binary.NativeEndian.PutUint16(raw[18:20], event.code)
	binary.NativeEndian.PutUint32(raw[20:24], uint32(event.value))
	_, err := w.Write(raw[:])
	return err
}

func openMouseUinput() (*os.File, error) {
	return os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
}

func ioctlPointer(fd, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// buildUinputSetup lays out struct uinput_setup: id first, then the NUL-padded
// name, then ff_effects_max (left zero).
func buildUinputSetup(name string, vendor, product uint16) [uinputSetupSize]byte {
	var buf [uinputSetupSize]byte
	binary.NativeEndian.PutUint16(buf[0:2], busUSB)
	binary.NativeEndian.PutUint16(buf[2:4], vendor)
	binary.NativeEndian.PutUint16(buf[4:6], product)
	binary.NativeEndian.PutUint16(buf[6:8], 1)
	label := []byte(name)
	if len(label) > uinputNameSize-1 {
		label = label[:uinputNameSize-1]
	}
	copy(buf[8:8+len(label)], label)
	return buf
}

// readSysHex reads a sysfs hex attribute (no 0x prefix, e.g. idVendor).
func readSysHex(path string) (uint16, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(value), true
}

// sysMouseIdentity walks to the USB device carrying the shared VID:PID. The
// directory is also the strongest sibling key for composite HID interfaces.
func sysMouseIdentity(dir string) (uint16, uint16, string) {
	for range 12 {
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		vendor, vok := readSysHex(filepath.Join(dir, "idVendor"))
		product, pok := readSysHex(filepath.Join(dir, "idProduct"))
		if vok && pok {
			return vendor, product, dir
		}
		dir = filepath.Dir(dir)
	}
	return 0, 0, ""
}

func sysMouseIDs(dir string) (uint16, uint16) {
	vendor, product, _ := sysMouseIdentity(dir)
	if vendor == 0 || product == 0 {
		return 0x0001, 0x0001
	}
	return vendor, product
}

const (
	cmdState = iota
	cmdMap
	cmdCapture
)

type mouseCmd struct {
	kind    int
	device  string
	button  uint16
	target  *mouseTarget
	timeout time.Duration
	reply   chan mouseCmdReply
}

type mouseCmdReply struct {
	frame mouseFrame
	err   error
}

type capturePress struct {
	device string
	button uint16
}

type mouseReader struct {
	cancel context.CancelFunc
	sig    string
}

type mouseMapManager struct {
	topic        *stateTopic
	sysRoot      string
	devRoot      string
	byIDRoot     string
	settingsPath string
	openUinput   func() (*os.File, error)

	commands chan mouseCmd
	done     chan struct{}

	mu     sync.Mutex
	cancel context.CancelFunc

	// curMaps is owned by the run loop once started; newMouseMapManager seeds it.
	curMaps map[string]map[string]mouseTarget
}

func newMouseMapManager(sysRoot, devRoot, byIDRoot, settingsPath string, openUinput func() (*os.File, error)) *mouseMapManager {
	settings := loadMouseSettings(settingsPath)
	return &mouseMapManager{
		sysRoot:      sysRoot,
		devRoot:      devRoot,
		byIDRoot:     byIDRoot,
		settingsPath: settingsPath,
		openUinput:   openUinput,
		commands:     make(chan mouseCmd),
		done:         make(chan struct{}),
		curMaps:      settings.Maps,
	}
}

// watchMouseInputChanges replaces the periodic sysfs walk when inotify is
// available. The wake pipe lets shutdown stop the poll immediately.
func watchMouseInputChanges(devRoot, byIDRoot string) (<-chan struct{}, func(), error) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return nil, nil, err
	}
	mask := uint32(unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO)
	if _, err := unix.InotifyAddWatch(fd, devRoot, mask); err != nil {
		unix.Close(fd)
		return nil, nil, err
	}
	_, _ = unix.InotifyAddWatch(fd, byIDRoot, mask)

	var wake [2]int
	if err := unix.Pipe2(wake[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		unix.Close(fd)
		return nil, nil, err
	}
	changes := make(chan struct{}, 1)
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			_, _ = unix.Write(wake[1], []byte{1})
		})
	}
	go func() {
		defer close(changes)
		defer unix.Close(fd)
		defer unix.Close(wake[0])
		defer unix.Close(wake[1])
		poll := []unix.PollFd{
			{Fd: int32(fd), Events: unix.POLLIN},
			{Fd: int32(wake[0]), Events: unix.POLLIN},
		}
		buf := make([]byte, 4096)
		for {
			if _, err := unix.Poll(poll, -1); err != nil {
				if err == unix.EINTR {
					continue
				}
				return
			}
			if poll[1].Revents != 0 {
				return
			}
			if poll[0].Revents == 0 {
				continue
			}
			for {
				n, err := unix.Read(fd, buf)
				if err != nil {
					if err == unix.EAGAIN {
						break
					}
					if err == unix.EINTR {
						continue
					}
					return
				}
				if n == 0 {
					break
				}
			}
			_, _ = unix.InotifyAddWatch(fd, byIDRoot, mask)
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}()
	return changes, stop, nil
}

func (m *mouseMapManager) start() {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()
	go m.run(ctx)
}

// stop tears down every reader before the process is replaced, so the grabs and
// clones are gone before a fresh daemon tries to seize the same devices.
func (m *mouseMapManager) stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *mouseMapManager) currentFrame(devices []mouseDevice, pressed *mousePressed) mouseFrame {
	if devices == nil {
		devices = []mouseDevice{}
	}
	maps := m.curMaps
	if maps == nil {
		maps = map[string]map[string]mouseTarget{}
	}
	return mouseFrame{Devices: devices, Maps: maps, Pressed: pressed}
}

// run owns the readers, the current map, and the capture waiter. Every mutation
// flows through here, so nothing needs a lock and the published frames stay in
// order.
func (m *mouseMapManager) run(ctx context.Context) {
	defer close(m.done)

	events := make(chan capturePress, 64)
	deaths := make(chan string, 8)
	readers := map[string]*mouseReader{}
	var curDevices []mouseDevice
	facts := mouseInputFactsCache{}

	defer func() {
		for _, reader := range readers {
			reader.cancel()
		}
	}()

	captureArmed := false
	var captureTimer *time.Timer
	var captureFire <-chan time.Time
	disarm := func() {
		captureArmed = false
		captureFire = nil
		if captureTimer != nil {
			captureTimer.Stop()
		}
	}

	reconcile := func() bool {
		devices := classifyMouseDevicesCached(m.sysRoot, m.devRoot, m.byIDRoot, facts)
		present := map[string]bool{}
		for _, dev := range devices {
			allCodes := deviceMapCodes(m.curMaps[dev.ID])
			for _, iface := range dev.interfaces {
				readerID := dev.ID + "\x00" + iface.node
				present[readerID] = true
				codes := mouseInterfaceMap(allCodes, iface.buttons)
				sig := mouseCodeMapSig(codes)
				if reader, ok := readers[readerID]; ok {
					if reader.sig == sig {
						continue
					}
					reader.cancel()
					delete(readers, readerID)
				}
				readerCtx, cancel := context.WithCancel(ctx)
				readers[readerID] = &mouseReader{cancel: cancel, sig: sig}
				go m.readDevice(readerCtx, dev, iface, len(codes) > 0, codes, events, deaths, readerID)
			}
		}
		for id, reader := range readers {
			if !present[id] {
				reader.cancel()
				delete(readers, id)
			}
		}
		changed := !mouseDevicesEqual(curDevices, devices)
		curDevices = devices
		return changed
	}

	publish := func(pressed *mousePressed) {
		if m.topic == nil {
			return
		}
		body, err := json.Marshal(m.currentFrame(curDevices, pressed))
		if err == nil {
			m.topic.publish(body)
		}
	}

	reconcile()
	publish(nil)

	inputChanges, stopInputWatch, watchErr := watchMouseInputChanges(m.devRoot, m.byIDRoot)
	var fallbackTicker *time.Ticker
	var fallbackPoll <-chan time.Time
	startFallbackPoll := func() {
		if fallbackTicker == nil {
			fallbackTicker = time.NewTicker(2 * time.Second)
			fallbackPoll = fallbackTicker.C
		}
	}
	if watchErr != nil {
		inputChanges = nil
		startFallbackPoll()
	}
	defer func() {
		if stopInputWatch != nil {
			stopInputWatch()
		}
		if fallbackTicker != nil {
			fallbackTicker.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-inputChanges:
			if !ok {
				inputChanges = nil
				startFallbackPoll()
				continue
			}
			if reconcile() {
				publish(nil)
			}
		case <-fallbackPoll:
			if reconcile() {
				publish(nil)
			}
		case id := <-deaths:
			if reader, ok := readers[id]; ok {
				reader.cancel()
				delete(readers, id)
			}
			if reconcile() {
				publish(nil)
			}
		case press := <-events:
			if captureArmed {
				disarm()
				publish(&mousePressed{Device: press.device, Button: press.button})
			}
		case <-captureFire:
			if captureArmed {
				disarm()
				publish(&mousePressed{})
			}
		case cmd := <-m.commands:
			switch cmd.kind {
			case cmdState:
				reconcile()
				cmd.reply <- mouseCmdReply{frame: m.currentFrame(curDevices, nil)}
			case cmdMap:
				next := patchMouseMaps(m.curMaps, cmd.device, cmd.button, cmd.target)
				if err := writeMouseSettings(m.settingsPath, mouseSettings{Maps: next}); err != nil {
					cmd.reply <- mouseCmdReply{err: err}
					break
				}
				m.curMaps = next
				reconcile()
				publish(nil)
				cmd.reply <- mouseCmdReply{frame: m.currentFrame(curDevices, nil)}
			case cmdCapture:
				captureArmed = true
				if captureTimer == nil {
					captureTimer = time.NewTimer(cmd.timeout)
				} else {
					if !captureTimer.Stop() {
						select {
						case <-captureTimer.C:
						default:
						}
					}
					captureTimer.Reset(cmd.timeout)
				}
				captureFire = captureTimer.C
				cmd.reply <- mouseCmdReply{}
			}
		}
	}
}

type lockedMouseWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedMouseWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(body)
}

// readDevice observes one interface of a logical mouse. Only the interface with
// a live binding is grabbed, so an MMO side grid can be rewritten without
// seizing an unrelated keyboard or even the pointer half of the same mouse.
func (m *mouseMapManager) readDevice(ctx context.Context, dev mouseDevice, iface mouseInterface, grab bool, codes map[uint16]mouseTarget, events chan<- capturePress, deaths chan<- string, readerID string) {
	src, err := os.Open(iface.node)
	if err != nil {
		m.signalDeath(ctx, deaths, readerID)
		return
	}
	defer src.Close()

	// Poll with a short timeout rather than blocking in read: an evdev fd is a
	// character device, so Close does not wake a blocked Go read.
	raw := make([]byte, mouseEventSize)
	pfd := []unix.PollFd{{Fd: int32(src.Fd()), Events: unix.POLLIN}}

	var clone *os.File
	var sub *mouseSubstitutor
	var macro *mouseMacroExecutor
	var output *lockedMouseWriter
	if grab {
		if err := unix.IoctlSetInt(int(src.Fd()), evIoCGrab, 1); err != nil {
			m.signalDeath(ctx, deaths, readerID)
			return
		}
		sub, err = newMouseSubstitutor(codes)
		if err != nil {
			return
		}
		clone, err = m.makeClone(dev, iface)
		if err != nil {
			return
		}
		defer m.destroyClone(clone)
		output = &lockedMouseWriter{w: clone}
		macro = newMouseMacroExecutor(ctx, func(event synthEvent) error {
			return writeInputEvent(output, event)
		}, nil)
		defer macro.stop()
	}

	for {
		if ctx.Err() != nil {
			return
		}
		n, err := unix.Poll(pfd, 200)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			m.signalDeath(ctx, deaths, readerID)
			return
		}
		if n == 0 {
			continue
		}
		if pfd[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			m.signalDeath(ctx, deaths, readerID)
			return
		}
		if _, err := io.ReadFull(src, raw); err != nil {
			m.signalDeath(ctx, deaths, readerID)
			return
		}
		etype, code, value := decodeInputEvent(raw)
		if etype == evKey && value == 1 && mouseCodeIn(iface.buttons, code) {
			select {
			case events <- capturePress{device: dev.ID, button: code}:
			case <-ctx.Done():
				return
			}
		}
		if !grab {
			continue
		}
		if target, ok := codes[code]; etype == evKey && ok && target.Kind == "sequence" {
			switch value {
			case 1:
				macro.start(code, target)
			case 0:
				macro.release(code, target.CancelOnRelease)
			}
			continue
		}
		if out, consumed := sub.apply(etype, code, value); consumed {
			for _, event := range out {
				if writeInputEvent(output, event) != nil {
					return
				}
			}
			continue
		}
		if _, err := output.Write(raw); err != nil {
			return
		}
	}
}

func mouseCodeIn(codes []uint16, code uint16) bool {
	for _, candidate := range codes {
		if candidate == code {
			return true
		}
	}
	return false
}

// signalDeath tells the run loop a reader died from a read error (an unplug) so
// it republishes at once. A cancelled context is an orderly stop, not a death.
func (m *mouseMapManager) signalDeath(ctx context.Context, deaths chan<- string, id string) {
	if ctx.Err() != nil {
		return
	}
	select {
	case deaths <- id:
	case <-ctx.Done():
	}
}

// makeClone mirrors every source key so unbound events pass through, then adds
// the chord table needed by remaps. Pointer axes belong only on pointer clones.
func (m *mouseMapManager) makeClone(dev mouseDevice, iface mouseInterface) (*os.File, error) {
	ui, err := m.openUinput()
	if err != nil {
		return nil, err
	}
	fd := int(ui.Fd())
	fail := func(err error) (*os.File, error) {
		_ = ui.Close()
		return nil, err
	}

	if err := unix.IoctlSetInt(fd, uiSetEvBit, evKey); err != nil {
		return fail(err)
	}
	keyset := map[uint16]bool{}
	for _, code := range iface.keys {
		keyset[code] = true
	}
	for _, code := range chordKeyCodes {
		keyset[code] = true
	}
	for code := range keyset {
		if err := unix.IoctlSetInt(fd, uiSetKeyBit, int(code)); err != nil {
			return fail(err)
		}
	}
	if !iface.keyboard {
		if err := unix.IoctlSetInt(fd, uiSetEvBit, evRel); err != nil {
			return fail(err)
		}
		for _, axis := range []int{relX, relY, relHWheel, relWheel, relDial} {
			if err := unix.IoctlSetInt(fd, uiSetRelBit, axis); err != nil {
				return fail(err)
			}
		}
	}
	if err := unix.IoctlSetInt(fd, uiSetEvBit, evMsc); err != nil {
		return fail(err)
	}
	if err := unix.IoctlSetInt(fd, uiSetMscBit, mscScan); err != nil {
		return fail(err)
	}
	if err := unix.IoctlSetInt(fd, uiSetEvBit, evSyn); err != nil {
		return fail(err)
	}

	name := dev.Model
	if name == "" {
		name = dev.Name
	}
	if iface.keyboard {
		name += " Side Grid"
	}
	vendor, product := dev.vendor, dev.product
	if vendor == 0 || product == 0 {
		vendor, product = sysMouseIDs(iface.sysDevice)
	}
	setup := buildUinputSetup("Ryoku Mouse "+name, vendor, product)
	if err := ioctlPointer(uintptr(fd), uiDevSetup, unsafe.Pointer(&setup[0])); err != nil {
		return fail(err)
	}
	if err := unix.IoctlSetInt(fd, uiDevCreate, 0); err != nil {
		return fail(err)
	}
	return ui, nil
}

func (m *mouseMapManager) destroyClone(clone *os.File) {
	if clone == nil {
		return
	}
	_ = unix.IoctlSetInt(int(clone.Fd()), uiDevDestroy, 0)
	_ = clone.Close()
}

func (m *mouseMapManager) state() (any, error) {
	reply := make(chan mouseCmdReply, 1)
	select {
	case m.commands <- mouseCmd{kind: cmdState, reply: reply}:
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
	select {
	case result := <-reply:
		return result.frame, result.err
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
}

func (m *mouseMapManager) mapButton(raw json.RawMessage) (any, error) {
	var args struct {
		Device string       `json:"device"`
		Button uint16       `json:"button"`
		Target *mouseTarget `json:"target"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Device == "" {
		return nil, fmt.Errorf("device is required")
	}
	if args.Button == 0 {
		return nil, fmt.Errorf("button is required")
	}
	if args.Target != nil {
		if err := validateTarget(*args.Target); err != nil {
			return nil, err
		}
	}
	reply := make(chan mouseCmdReply, 1)
	select {
	case m.commands <- mouseCmd{kind: cmdMap, device: args.Device, button: args.Button, target: args.Target, reply: reply}:
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
	select {
	case result := <-reply:
		return result.frame, result.err
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
}

func (m *mouseMapManager) testSequence(raw json.RawMessage) (any, error) {
	var args struct {
		Target mouseTarget `json:"target"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Target.Kind != "sequence" {
		return nil, fmt.Errorf("test target must be a sequence")
	}
	if err := validateTarget(args.Target); err != nil {
		return nil, err
	}
	dev := mouseDevice{Name: "Macro Test", Model: "Macro Test", vendor: 1, product: 1}
	iface := mouseInterface{keyboard: true}
	clone, err := m.makeClone(dev, iface)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	output := &lockedMouseWriter{w: clone}
	executor := newMouseMacroExecutor(ctx, func(event synthEvent) error {
		return writeInputEvent(output, event)
	}, nil)
	done := executor.start(0, args.Target)
	go func() {
		<-done
		cancel()
		m.destroyClone(clone)
	}()
	return map[string]any{"started": true}, nil
}

func (m *mouseMapManager) capture(raw json.RawMessage) (any, error) {
	var args struct {
		TimeoutMs int `json:"timeoutMs"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
	}
	timeout := 5 * time.Second
	if args.TimeoutMs > 0 {
		timeout = time.Duration(args.TimeoutMs) * time.Millisecond
	}
	if timeout < 100*time.Millisecond {
		timeout = 100 * time.Millisecond
	}
	if timeout > 120*time.Second {
		timeout = 120 * time.Second
	}
	reply := make(chan mouseCmdReply, 1)
	select {
	case m.commands <- mouseCmd{kind: cmdCapture, timeout: timeout, reply: reply}:
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
	select {
	case <-reply:
		return map[string]any{"armed": true}, nil
	case <-m.done:
		return nil, fmt.Errorf("mouse remapper is not running")
	}
}

func (d *daemon) startMouseMap() {
	topic := d.registerEventTopic("mouse")
	manager := newMouseMapManager("/sys", "/dev/input", "/dev/input/by-id", mouseSettingsPath(), openMouseUinput)
	manager.topic = topic
	d.mousemap = manager
	manager.start()
	d.registerCall("mouse.state", func(json.RawMessage) (any, error) {
		return d.mousemap.state()
	})
	d.registerCall("mouse.map", func(raw json.RawMessage) (any, error) {
		return d.mousemap.mapButton(raw)
	})
	d.registerCall("mouse.test", func(raw json.RawMessage) (any, error) {
		return d.mousemap.testSequence(raw)
	})
	d.registerCall("mouse.capture", func(raw json.RawMessage) (any, error) {
		return d.mousemap.capture(raw)
	})
}
