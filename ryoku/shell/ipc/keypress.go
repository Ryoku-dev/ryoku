package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type keyModifier uint8

const (
	modifierNone keyModifier = iota
	modifierCtrl
	modifierAlt
	modifierAltGr
	modifierShift
	modifierSuper
)

var modifierDisplayOrder = [...]keyModifier{
	modifierSuper,
	modifierCtrl,
	modifierAlt,
	modifierAltGr,
	modifierShift,
}

type keyInfo struct {
	label     string
	shifted   string
	printable bool
	modifier  keyModifier
}

// Linux input key codes are a stable userspace ABI. Keep display policy here so
// the daemon emits viewer-ready labels and QML never has to understand evdev.
var inputKeys = map[uint16]keyInfo{
	1: {label: "Esc"},
	2: {label: "1", shifted: "!", printable: true}, 3: {label: "2", shifted: "@", printable: true},
	4: {label: "3", shifted: "#", printable: true}, 5: {label: "4", shifted: "$", printable: true},
	6: {label: "5", shifted: "%", printable: true}, 7: {label: "6", shifted: "^", printable: true},
	8: {label: "7", shifted: "&", printable: true}, 9: {label: "8", shifted: "*", printable: true},
	10: {label: "9", shifted: "(", printable: true}, 11: {label: "0", shifted: ")", printable: true},
	12: {label: "-", shifted: "_", printable: true}, 13: {label: "=", shifted: "+", printable: true},
	14: {label: "Backspace"}, 15: {label: "Tab"},
	16: {label: "Q", printable: true}, 17: {label: "W", printable: true},
	18: {label: "E", printable: true}, 19: {label: "R", printable: true},
	20: {label: "T", printable: true}, 21: {label: "Y", printable: true},
	22: {label: "U", printable: true}, 23: {label: "I", printable: true},
	24: {label: "O", printable: true}, 25: {label: "P", printable: true},
	26: {label: "[", shifted: "{", printable: true}, 27: {label: "]", shifted: "}", printable: true},
	28: {label: "Enter"},
	29: {label: "Ctrl", modifier: modifierCtrl},
	30: {label: "A", printable: true}, 31: {label: "S", printable: true},
	32: {label: "D", printable: true}, 33: {label: "F", printable: true},
	34: {label: "G", printable: true}, 35: {label: "H", printable: true},
	36: {label: "J", printable: true}, 37: {label: "K", printable: true},
	38: {label: "L", printable: true}, 39: {label: ";", shifted: ":", printable: true},
	40: {label: "'", shifted: "\"", printable: true}, 41: {label: "`", shifted: "~", printable: true},
	42: {label: "Shift", modifier: modifierShift},
	43: {label: "\\", shifted: "|", printable: true},
	44: {label: "Z", printable: true}, 45: {label: "X", printable: true},
	46: {label: "C", printable: true}, 47: {label: "V", printable: true},
	48: {label: "B", printable: true}, 49: {label: "N", printable: true},
	50: {label: "M", printable: true}, 51: {label: ",", shifted: "<", printable: true},
	52: {label: ".", shifted: ">", printable: true}, 53: {label: "/", shifted: "?", printable: true},
	54: {label: "Shift", modifier: modifierShift},
	55: {label: "Num ×", printable: true},
	56: {label: "Alt", modifier: modifierAlt},
	57: {label: "Space", printable: true},
	58: {label: "Caps Lock"},
	59: {label: "F1"}, 60: {label: "F2"}, 61: {label: "F3"},
	62: {label: "F4"}, 63: {label: "F5"}, 64: {label: "F6"},
	65: {label: "F7"}, 66: {label: "F8"}, 67: {label: "F9"},
	68: {label: "F10"}, 69: {label: "Num Lock"}, 70: {label: "Scroll Lock"},
	71: {label: "Num 7", printable: true}, 72: {label: "Num 8", printable: true},
	73: {label: "Num 9", printable: true}, 74: {label: "Num -", printable: true},
	75: {label: "Num 4", printable: true}, 76: {label: "Num 5", printable: true},
	77: {label: "Num 6", printable: true}, 78: {label: "Num +", printable: true},
	79: {label: "Num 1", printable: true}, 80: {label: "Num 2", printable: true},
	81: {label: "Num 3", printable: true}, 82: {label: "Num 0", printable: true},
	83: {label: "Num .", printable: true},
	87: {label: "F11"}, 88: {label: "F12"},
	96: {label: "Num Enter"},
	97: {label: "Ctrl", modifier: modifierCtrl},
	98: {label: "Num /", printable: true}, 99: {label: "Print Screen"},
	100: {label: "AltGr", modifier: modifierAltGr},
	102: {label: "Home"}, 103: {label: "↑"}, 104: {label: "PgUp"},
	105: {label: "←"}, 106: {label: "→"}, 107: {label: "End"},
	108: {label: "↓"}, 109: {label: "PgDn"}, 110: {label: "Insert"},
	111: {label: "Delete"},
	113: {label: "Mute"}, 114: {label: "Volume −"}, 115: {label: "Volume +"},
	116: {label: "Power"}, 119: {label: "Pause"},
	125: {label: "Super", modifier: modifierSuper},
	126: {label: "Super", modifier: modifierSuper},
	127: {label: "Compose"}, 128: {label: "Stop"},
	130: {label: "Properties"}, 131: {label: "Undo"}, 133: {label: "Copy"},
	134: {label: "Open"}, 135: {label: "Paste"}, 136: {label: "Find"},
	137: {label: "Cut"}, 138: {label: "Help"}, 139: {label: "Menu"},
	140: {label: "Calculator"}, 142: {label: "Sleep"}, 143: {label: "Wake"},
	155: {label: "Mail"}, 156: {label: "Bookmarks"}, 157: {label: "Computer"},
	158: {label: "Back"}, 159: {label: "Forward"},
	163: {label: "Next"}, 164: {label: "Play / Pause"},
	165: {label: "Previous"}, 166: {label: "Media Stop"},
	172: {label: "Home Page"}, 173: {label: "Refresh"},
	207: {label: "Play"}, 208: {label: "Fast Forward"},
	212: {label: "Camera"}, 215: {label: "Email"}, 217: {label: "Search"},
	224: {label: "Brightness −"}, 225: {label: "Brightness +"},
	226: {label: "Media"}, 229: {label: "Keyboard Light −"},
	230: {label: "Keyboard Light +"}, 237: {label: "Bluetooth"},
	238: {label: "Wi-Fi"}, 248: {label: "Mic Mute"},
}

func keyForCode(code uint16) (keyInfo, bool) {
	key, ok := inputKeys[code]
	return key, ok
}

// sysfs writes one native-word hex value per field, most-significant first.
// Lower words are not guaranteed to carry leading zeroes, so concatenate by
// shifting a machine word rather than joining text.
func parseKeyCapabilities(raw string) (*big.Int, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil, false
	}
	out := new(big.Int)
	for _, field := range fields {
		word, err := strconv.ParseUint(field, 16, 64)
		if err != nil {
			return nil, false
		}
		out.Lsh(out, 64)
		out.Or(out, new(big.Int).SetUint64(word))
	}
	return out, true
}

func isKeyboardCapabilities(raw string) bool {
	bits, ok := parseKeyCapabilities(raw)
	if !ok {
		return false
	}
	for _, code := range []int{28, 30, 44, 57} { // Enter, A, Z, Space
		if bits.Bit(code) == 0 {
			return false
		}
	}
	return true
}

func normalizeKeypressMode(mode string) string {
	if mode == "shortcuts" {
		return mode
	}
	return "all"
}

type heldModifier struct {
	count int
	used  bool
}

type modifierState struct {
	ctrl  heldModifier
	alt   heldModifier
	altGr heldModifier
	shift heldModifier
	super heldModifier
}

type heldKeyID struct {
	source uint64
	code   uint16
}

type keyComposer struct {
	sources map[uint64]*modifierState
	held    map[heldKeyID][]string
}

type keypressEvent struct {
	keys   []string
	repeat bool
	state  string
}

func sameKeyLabels(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *keyComposer) source(id uint64) *modifierState {
	if c.sources == nil {
		c.sources = make(map[uint64]*modifierState)
	}
	source := c.sources[id]
	if source == nil {
		source = &modifierState{}
		c.sources[id] = source
	}
	return source
}

func (c *keyComposer) removeSource(id uint64) []*keypressEvent {
	delete(c.sources, id)
	removed := make([][]string, 0, 1)
	for key, labels := range c.held {
		if key.source == id {
			removed = append(removed, labels)
			delete(c.held, key)
		}
	}
	released := make([]*keypressEvent, 0, len(removed))
	for _, labels := range removed {
		stillHeld := false
		for _, active := range c.held {
			if sameKeyLabels(active, labels) {
				stillHeld = true
				break
			}
		}
		if stillHeld {
			continue
		}
		duplicate := false
		for _, event := range released {
			if sameKeyLabels(event.keys, labels) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			released = append(released, &keypressEvent{keys: labels, state: "released"})
		}
	}
	return released
}

func (s *modifierState) modifier(which keyModifier) *heldModifier {
	switch which {
	case modifierCtrl:
		return &s.ctrl
	case modifierAlt:
		return &s.alt
	case modifierAltGr:
		return &s.altGr
	case modifierShift:
		return &s.shift
	case modifierSuper:
		return &s.super
	default:
		return nil
	}
}

func modifierLabel(which keyModifier) string {
	switch which {
	case modifierCtrl:
		return "Ctrl"
	case modifierAlt:
		return "Alt"
	case modifierAltGr:
		return "AltGr"
	case modifierShift:
		return "Shift"
	case modifierSuper:
		return "Super"
	default:
		return ""
	}
}

func (c *keyComposer) modifierHeld(which keyModifier) bool {
	for _, source := range c.sources {
		if source.modifier(which).count > 0 {
			return true
		}
	}
	return false
}

func (c *keyComposer) appendModifiers(keys []string, markUsed bool) []string {
	for _, which := range modifierDisplayOrder {
		down := false
		for _, source := range c.sources {
			held := source.modifier(which)
			if held.count == 0 {
				continue
			}
			down = true
			if markUsed {
				held.used = true
			}
		}
		if down {
			keys = append(keys, modifierLabel(which))
		}
	}
	return keys
}

func (c *keyComposer) shortcutHeld() bool {
	for _, source := range c.sources {
		if source.ctrl.count > 0 || source.alt.count > 0 || source.super.count > 0 {
			return true
		}
	}
	return false
}

// handleSource turns one source-tagged EV_KEY transition into at most one
// display chord. Modifier state remains isolated by device until composition,
// so unplugging one keyboard cannot poison another while split keyboards still
// form a single shortcut.
func (c *keyComposer) handleSource(sourceID uint64, code uint16, value int32, mode string) *keypressEvent {
	key, ok := keyForCode(code)
	if !ok || value < 0 || value > 2 {
		return nil
	}
	source := c.source(sourceID)
	if key.modifier != modifierNone {
		held := source.modifier(key.modifier)
		switch value {
		case 1:
			if held.count == 0 {
				held.used = false
			}
			held.count++
		case 0:
			if held.count == 0 {
				return nil
			}
			wasStandalone := held.count == 1 && !held.used
			held.count--
			if held.count == 0 {
				held.used = false
			}
			if wasStandalone {
				return &keypressEvent{keys: []string{key.label}, state: "tap"}
			}
		}
		return nil
	}
	id := heldKeyID{source: sourceID, code: code}
	if value == 0 {
		keys, found := c.held[id]
		if !found {
			return nil
		}
		delete(c.held, id)
		for _, active := range c.held {
			if sameKeyLabels(active, keys) {
				return nil
			}
		}
		return &keypressEvent{keys: keys, state: "released"}
	}
	if keys, found := c.held[id]; found {
		if value != 2 {
			return nil
		}
		return &keypressEvent{keys: keys, repeat: true, state: "pressed"}
	}

	label := key.label
	if key.shifted != "" && c.modifierHeld(modifierShift) {
		label = key.shifted
	}
	shortcut := c.shortcutHeld()
	keys := c.appendModifiers(make([]string, 0, 6), true)
	if normalizeKeypressMode(mode) == "shortcuts" && key.printable && !shortcut {
		return nil
	}
	keys = append(keys, label)
	if c.held == nil {
		c.held = make(map[heldKeyID][]string)
	}
	c.held[id] = keys
	return &keypressEvent{keys: keys, repeat: value == 2, state: "pressed"}
}

func (c *keyComposer) handle(code uint16, value int32, mode string) *keypressEvent {
	return c.handleSource(0, code, value, mode)
}

// Independent keyboard readers can invert near-simultaneous messages.
// Buffering for half a 60 Hz frame preserves monotonic kernel order without
// visible overlay lag. EVIOCSCLOCKID is Linux _IOW('E', 0xa0, int).
const (
	linuxInputEventSize  = 24
	evKey                = 1
	evIoCsClockID        = 0x400445a0
	keyEventReorderDelay = 8 * time.Millisecond
)

type linuxKeyEvent struct {
	code        uint16
	value       int32
	timestampUS int64
}

func decodeLinuxInputEvent(raw []byte) (linuxKeyEvent, bool) {
	if len(raw) != linuxInputEventSize || binary.NativeEndian.Uint16(raw[16:18]) != evKey {
		return linuxKeyEvent{}, false
	}
	return linuxKeyEvent{
		code:  binary.NativeEndian.Uint16(raw[18:20]),
		value: int32(binary.NativeEndian.Uint32(raw[20:24])),
		timestampUS: int64(binary.NativeEndian.Uint64(raw[0:8]))*1_000_000 +
			int64(binary.NativeEndian.Uint64(raw[8:16])),
	}, true
}

func monotonicTimestampUS() int64 {
	var now unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &now); err != nil {
		return 0
	}
	return now.Sec*1_000_000 + now.Nsec/1_000
}

func configureKeyEventClock(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return nil
	}
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var clockErr error
	if err := connection.Control(func(fd uintptr) {
		clockErr = unix.IoctlSetPointerInt(int(fd), evIoCsClockID, unix.CLOCK_MONOTONIC)
	}); err != nil {
		return err
	}
	return clockErr
}

func decodeKeypressConfigure(raw json.RawMessage) (enabled bool, mode string, err error) {
	var args struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return false, "", err
	}
	return args.Enabled, normalizeKeypressMode(args.Mode), nil
}

type keypressFrame struct {
	Status string   `json:"status"`
	Keys   []string `json:"keys"`
	Repeat bool     `json:"repeat,omitempty"`
	State  string   `json:"state,omitempty"`
	Serial uint64   `json:"serial,omitempty"`
	Time   int64    `json:"time,omitempty"`
	Error  string   `json:"error,omitempty"`
}

func marshalKeypressEvent(event *keypressEvent, nowMS int64, serial uint64) ([]byte, error) {
	return json.Marshal(keypressFrame{
		Status: "ready",
		Keys:   event.keys,
		Repeat: event.repeat,
		State:  event.state,
		Time:   nowMS,
		Serial: serial,
	})
}

type keypressSettings struct {
	Theme   string   `json:"theme"`
	Mode    string   `json:"mode"`
	PX      *float64 `json:"px,omitempty"`
	PY      *float64 `json:"py,omitempty"`
	Monitor string   `json:"monitor,omitempty"`
}

func keypressSettingsPath() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "keypresses.json")
}

func loadKeypressSettings(path string) keypressSettings {
	settings := keypressSettings{Theme: "dark", Mode: "all"}
	body, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	var saved keypressSettings
	if json.Unmarshal(body, &saved) != nil {
		return settings
	}
	if saved.Theme == "dark" || saved.Theme == "light" {
		settings.Theme = saved.Theme
	}
	if saved.Mode == "all" || saved.Mode == "shortcuts" {
		settings.Mode = saved.Mode
	}
	if saved.PX != nil && saved.PY != nil {
		settings.PX = saved.PX
		settings.PY = saved.PY
		settings.Monitor = saved.Monitor
	}
	return settings
}

func writeKeypressSettings(path string, settings keypressSettings) error {
	if path == "" {
		return fmt.Errorf("keypress settings path is unavailable")
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

type keypressSettingsPatch struct {
	Theme          *string  `json:"theme"`
	Mode           *string  `json:"mode"`
	PX             *float64 `json:"px"`
	PY             *float64 `json:"py"`
	Monitor        *string  `json:"monitor"`
	ResetPlacement bool     `json:"resetPlacement"`
}

type keypressManager struct {
	topic        *stateTopic
	sysRoot      string
	devRoot      string
	settingsPath string

	mu          sync.Mutex
	cancel      context.CancelFunc
	enabled     bool
	mode        string
	settings    keypressSettings
	generation  uint64
	eventSerial uint64
}

func newKeypressManager(topic *stateTopic, sysRoot, devRoot, settingsPath string) *keypressManager {
	settings := loadKeypressSettings(settingsPath)
	return &keypressManager{
		topic:        topic,
		sysRoot:      sysRoot,
		devRoot:      devRoot,
		settingsPath: settingsPath,
		mode:         settings.Mode,
		settings:     settings,
	}
}

// configure turns the visualiser overlay on or off. The device reader runs
// only while the overlay is enabled, so nothing reads the keyboard when the
// visualiser is idle.
func (m *keypressManager) configure(enabled bool, mode string) {
	mode = normalizeKeypressMode(mode)
	m.mu.Lock()
	m.mode = mode
	m.enabled = enabled
	quiet := !enabled
	generation, started := m.restartLocked()
	m.mu.Unlock()
	if started {
		m.publish(generation, keypressFrame{Status: "starting", Keys: []string{}})
	} else if quiet {
		m.publishDisabled()
	}
}

// restartLocked brings the reader in line with the visualiser state: it starts
// when the overlay turns on while stopped, and stops when it turns off. A mode
// change while running never bounces the reader. It returns the new generation
// when the reader started, so the caller can publish the starting frame once
// the lock is released. Call with mu held.
func (m *keypressManager) restartLocked() (uint64, bool) {
	want := m.enabled
	if want == (m.cancel != nil) {
		return 0, false
	}
	if !want {
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		return 0, false
	}
	m.generation++
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.run(ctx, m.generation)
	return m.generation, true
}

func (m *keypressManager) currentMode() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

func (m *keypressManager) patchSettings(raw json.RawMessage) (keypressSettings, error) {
	var patch keypressSettingsPatch
	if err := json.Unmarshal(raw, &patch); err != nil {
		return keypressSettings{}, err
	}
	if patch.Theme != nil && *patch.Theme != "dark" && *patch.Theme != "light" {
		return keypressSettings{}, fmt.Errorf("theme must be dark or light")
	}
	if patch.Mode != nil && *patch.Mode != "all" && *patch.Mode != "shortcuts" {
		return keypressSettings{}, fmt.Errorf("mode must be all or shortcuts")
	}
	if (patch.PX == nil) != (patch.PY == nil) {
		return keypressSettings{}, fmt.Errorf("px and py must be set together")
	}
	if patch.ResetPlacement && patch.PX != nil {
		return keypressSettings{}, fmt.Errorf("placement cannot be set and reset together")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.settings
	if patch.Theme != nil {
		next.Theme = *patch.Theme
	}
	if patch.Mode != nil {
		next.Mode = *patch.Mode
	}
	if patch.ResetPlacement {
		next.PX = nil
		next.PY = nil
		next.Monitor = ""
	} else if patch.PX != nil {
		next.PX = patch.PX
		next.PY = patch.PY
		if patch.Monitor != nil {
			next.Monitor = *patch.Monitor
		}
	}
	if err := writeKeypressSettings(m.settingsPath, next); err != nil {
		return keypressSettings{}, err
	}
	m.settings = next
	if patch.Mode != nil {
		m.mode = next.Mode
	}
	return next, nil
}

func (m *keypressManager) publish(generation uint64, frame keypressFrame) {
	m.mu.Lock()
	current := m.enabled && m.generation == generation
	m.mu.Unlock()
	if !current {
		return
	}
	body, err := json.Marshal(frame)
	if err == nil {
		m.topic.publish(body)
	}
}

func (m *keypressManager) publishDisabled() {
	body, _ := json.Marshal(keypressFrame{Status: "disabled", Keys: []string{}})
	m.topic.publish(body)
}

type keyDeviceMessage struct {
	path        string
	readerID    uint64
	opened      bool
	hasEvent    bool
	event       linuxKeyEvent
	timestampUS int64
	err         error
}

func keyDeviceRemovalFrame(readErr error, remaining int) keypressFrame {
	if remaining > 0 {
		return keypressFrame{Status: "ready", Keys: []string{}}
	}
	message := "No keyboard input device found"
	if readErr != nil {
		message = fmt.Sprintf("Cannot read keyboard input: %v", readErr)
	}
	return keypressFrame{
		Status: "error",
		Keys:   []string{},
		Error:  message,
	}
}

func insertOrderedKeyMessage(pending []keyDeviceMessage, message keyDeviceMessage) []keyDeviceMessage {
	pending = append(pending, message)
	for i := len(pending) - 1; i > 0; i-- {
		current := pending[i].timestampUS
		previous := pending[i-1].timestampUS
		if current == 0 || previous == 0 || current >= previous {
			break
		}
		pending[i], pending[i-1] = pending[i-1], pending[i]
	}
	return pending
}

func sendKeyDeviceMessage(ctx context.Context, messages chan<- keyDeviceMessage, message keyDeviceMessage) {
	select {
	case messages <- message:
	case <-ctx.Done():
	}
}

type keyDeviceReader struct {
	id       uint64
	cancel   context.CancelFunc
	opened   bool
	removing bool
}

func healthyKeyDeviceCount(active map[string]keyDeviceReader) int {
	count := 0
	for _, reader := range active {
		if reader.opened && !reader.removing {
			count++
		}
	}
	return count
}

func appendKeyDeviceBatch(
	pending []keyDeviceMessage,
	message keyDeviceMessage,
) []keyDeviceMessage {
	return insertOrderedKeyMessage(pending, message)
}

func drainKeyDeviceBatch(
	messages <-chan keyDeviceMessage,
	active map[string]keyDeviceReader,
	pending []keyDeviceMessage,
) []keyDeviceMessage {
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				return pending
			}
			reader, current := active[message.path]
			if !current || reader.id != message.readerID {
				continue
			}
			pending = appendKeyDeviceBatch(pending, message)
		default:
			return pending
		}
	}
}

func (m *keypressManager) run(ctx context.Context, generation uint64) {
	var composer keyComposer
	publishEvent := func(event *keypressEvent) {
		if event == nil {
			return
		}
		m.mu.Lock()
		current := m.enabled && m.generation == generation
		if current {
			m.eventSerial++
		}
		serial := m.eventSerial
		m.mu.Unlock()
		if current {
			body, err := marshalKeypressEvent(event, time.Now().UnixMilli(), serial)
			if err == nil {
				m.topic.publish(body)
			}
		}
	}
	runKeyboardReader(ctx, m.sysRoot, m.devRoot, func(message keyDeviceMessage, remaining int) {
		switch {
		case message.hasEvent:
			publishEvent(composer.handleSource(message.readerID, message.event.code, message.event.value, m.currentMode()))
		case message.opened:
			composer.source(message.readerID)
			m.publish(generation, keypressFrame{Status: "ready", Keys: []string{}})
		default:
			for _, event := range composer.removeSource(message.readerID) {
				publishEvent(event)
			}
			m.publish(generation, keyDeviceRemovalFrame(message.err, remaining))
		}
	})
}

func (d *daemon) startKeypress() {
	topic := d.registerEventTopic("keypress")
	d.keypress = newKeypressManager(topic, "/sys/class/input", "/dev/input", keypressSettingsPath())
	d.keypress.configure(false, d.keypress.currentMode())
	d.registerCall("keypress.configure", func(raw json.RawMessage) (any, error) {
		enabled, mode, err := decodeKeypressConfigure(raw)
		if err != nil {
			return nil, err
		}
		d.keypress.configure(enabled, mode)
		return map[string]any{"enabled": enabled, "mode": mode}, nil
	})
	d.registerCall("keypress.settings", func(raw json.RawMessage) (any, error) {
		return d.keypress.patchSettings(raw)
	})
}
