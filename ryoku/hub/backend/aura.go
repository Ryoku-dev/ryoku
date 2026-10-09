package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	auraProvider   = "asus-aura"
	auraService    = "xyz.ljones.Asusd"
	auraRoot       = dbus.ObjectPath("/xyz/ljones/aura")
	auraLegacyPath = dbus.ObjectPath("/xyz/ljones/Aura")
	auraIface      = "xyz.ljones.Aura"

	auraStatic       uint32 = 0
	auraBreathe      uint32 = 1
	auraRainbowCycle uint32 = 2
	auraRainbowWave  uint32 = 3
	auraStars        uint32 = 4
	auraRain         uint32 = 5
	auraHighlight    uint32 = 6
	auraLaser        uint32 = 7
	auraRipple       uint32 = 8
	auraPulse        uint32 = 10
	auraComet        uint32 = 11
	auraFlash        uint32 = 12
)

type auraColour struct {
	R uint8
	G uint8
	B uint8
}

type auraEffect struct {
	Mode      uint32
	Zone      uint32
	Colour1   auraColour
	Colour2   auraColour
	Speed     string
	Direction string
}

type auraState struct {
	Path       string
	Name       string
	DeviceType uint32
	Mode       uint32
	Modes      []uint32
	Brightness uint32
	ModeData   map[uint32]auraEffect
}

var (
	auraRead    = readAuraState
	auraWrite   = writeAuraEffect
	auraRestore = writeAuraMode
)

func auraInstalled() bool {
	_, err := exec.LookPath("asusd")
	return err == nil
}

func auraServerUp() bool {
	return exec.Command("ryoku-host", "svc", "--system", "is-active", "asusd").Run() == nil
}

type auraNode struct {
	Name string `xml:"name,attr"`
}

type auraIntrospection struct {
	Nodes []auraNode `xml:"node"`
}

func readAuraState() ([]auraState, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	paths := auraObjectPaths(conn)
	states := make([]auraState, 0, len(paths))
	var lastErr error
	for _, path := range paths {
		state, err := readAuraAt(conn, path)
		if err != nil {
			lastErr = err
			continue
		}
		states = append(states, state)
	}
	if len(states) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return states, nil
}

func auraObjectPaths(conn *dbus.Conn) []dbus.ObjectPath {
	var raw string
	err := conn.Object(auraService, auraRoot).
		Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&raw)
	if err == nil {
		var doc auraIntrospection
		if xml.Unmarshal([]byte(raw), &doc) == nil {
			paths := make([]dbus.ObjectPath, 0, len(doc.Nodes))
			for _, node := range doc.Nodes {
				if node.Name != "" && !strings.Contains(node.Name, "/") {
					paths = append(paths, dbus.ObjectPath(string(auraRoot)+"/"+node.Name))
				}
			}
			if len(paths) > 0 {
				return paths
			}
		}
	}
	return []dbus.ObjectPath{auraLegacyPath}
}

func readAuraAt(conn *dbus.Conn, path dbus.ObjectPath) (auraState, error) {
	obj := conn.Object(auraService, path)
	var modes []uint32
	if err := obj.StoreProperty(auraIface+".SupportedBasicModes", &modes); err != nil {
		return auraState{}, err
	}
	var mode, brightness, deviceType uint32
	if err := obj.StoreProperty(auraIface+".LedMode", &mode); err != nil {
		return auraState{}, err
	}
	if err := obj.StoreProperty(auraIface+".Brightness", &brightness); err != nil {
		return auraState{}, err
	}
	if err := obj.StoreProperty(auraIface+".DeviceType", &deviceType); err != nil {
		return auraState{}, err
	}
	effect := auraEffect{Mode: mode, Speed: "Med", Direction: "Right"}
	if err := obj.StoreProperty(auraIface+".LedModeData", &effect); err != nil {
		return auraState{}, err
	}
	modeData := make(map[uint32]auraEffect)
	if err := obj.Call(auraIface+".AllModeData", 0).Store(&modeData); err != nil {
		return auraState{}, err
	}
	modeData[mode] = effect
	return auraState{
		Path:       string(path),
		Name:       auraDeviceName(deviceType),
		DeviceType: deviceType,
		Mode:       mode,
		Modes:      modes,
		Brightness: brightness,
		ModeData:   modeData,
	}, nil
}

func writeAuraEffect(path string, effect auraEffect, brightness int) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	obj := conn.Object(auraService, dbus.ObjectPath(path))
	if err := obj.SetProperty(auraIface+".LedModeData", dbus.MakeVariant(effect)); err != nil {
		return err
	}
	if brightness >= 0 {
		level := uint32((clampPercent(brightness)*3 + 50) / 100)
		if err := obj.SetProperty(auraIface+".Brightness", dbus.MakeVariant(level)); err != nil {
			return err
		}
	}
	return nil
}

func writeAuraMode(path string, mode uint32) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Object(auraService, dbus.ObjectPath(path)).
		SetProperty(auraIface+".LedMode", dbus.MakeVariant(mode))
}

func auraDeviceName(deviceType uint32) string {
	base := "ASUS"
	for _, path := range []string{"/sys/class/dmi/id/product_family", "/sys/class/dmi/id/product_name"} {
		if raw, err := os.ReadFile(path); err == nil {
			if name := strings.TrimSpace(string(raw)); name != "" {
				base = name
				break
			}
		}
	}
	switch deviceType {
	case 0, 1, 2:
		return base + " keyboard"
	case 3:
		return base + " external lighting"
	case 4:
		return base + " handheld lighting"
	default:
		return base + " Aura lighting"
	}
}

func auraDeviceView(state auraState) orgbDevice {
	modes := make([]orgbMode, 0, len(state.Modes))
	active := 0
	for _, id := range state.Modes {
		mode := auraModeView(id, state)
		if id == state.Mode {
			active = len(modes)
		}
		modes = append(modes, mode)
	}
	path := state.Path
	if path == "" {
		path = string(auraLegacyPath)
	}
	name := state.Name
	if name == "" {
		name = auraDeviceName(state.DeviceType)
	}
	class := auraDeviceClass(state.DeviceType)
	serial := auraProvider + ":" + path
	var aliases []string
	if class == "Keyboard" {
		aliases = []string{name + "#" + serial}
		serial = auraProvider + ":keyboard"
	}
	return orgbDevice{
		Provider:     auraProvider,
		ProviderPath: path,
		Aliases:      aliases,
		Type:         class,
		Name:         name,
		Vendor:       "ASUS",
		Description:  "ASUS Aura lighting",
		Serial:       serial,
		Location:     "asusd",
		ActiveMode:   active,
		Modes:        modes,
	}
}

func auraDeviceClass(deviceType uint32) string {
	if deviceType <= 2 {
		return "Keyboard"
	}
	return "Lighting"
}

func auraModeView(id uint32, state auraState) orgbMode {
	flags := uint32(modeHasModeColor | modeHasBrightness)
	colors := uint32(1)
	if id != auraStatic {
		flags |= modeHasSpeed
	}
	if id == auraBreathe || id == auraStars {
		colors = 2
	}
	if id == auraRainbowWave {
		flags |= modeHasDirLR | modeHasDirUD
	}
	effect := auraEffect{Mode: id, Speed: "Med", Direction: "Right"}
	if stored, ok := state.ModeData[id]; ok {
		effect = stored
		effect.Mode = id
	}
	m := orgbMode{
		Name:      auraModeName(id),
		Value:     int32(id),
		Flags:     flags,
		SpeedMin:  0,
		SpeedMax:  100,
		BriMin:    0,
		BriMax:    100,
		ColorsMin: colors,
		ColorsMax: colors,
		Speed:     auraSpeedPercent(effect.Speed),
		Bri:       state.Brightness * 100 / 3,
		Direction: auraDirectionValue(effect.Direction),
		ColorMode: 2,
		Colors:    []uint32{auraPackedColour(effect.Colour1)},
	}
	if colors == 2 {
		m.Colors = append(m.Colors, auraPackedColour(effect.Colour2))
	}
	return m
}

func auraModeName(id uint32) string {
	switch id {
	case auraBreathe:
		return "Breathe"
	case auraRainbowCycle:
		return "RainbowCycle"
	case auraRainbowWave:
		return "RainbowWave"
	case auraStars:
		return "Stars"
	case auraRain:
		return "Rain"
	case auraHighlight:
		return "Highlight"
	case auraLaser:
		return "Laser"
	case auraRipple:
		return "Ripple"
	case auraPulse:
		return "Pulse"
	case auraComet:
		return "Comet"
	case auraFlash:
		return "Flash"
	default:
		return "Static"
	}
}

func auraModeForEffect(effect string, device orgbDevice) string {
	mode := "Static"
	switch effect {
	case "breathe":
		mode = "Breathe"
	case "pulse":
		mode = "Pulse"
	case "spectrum":
		mode = "RainbowCycle"
	case "wave":
		mode = "RainbowWave"
	case "comet":
		mode = "Comet"
	case "scanner":
		mode = "Laser"
	}
	return supportedAuraMode(mode, device)
}

func auraModeForLegacyMode(mode string, device orgbDevice) string {
	switch mode {
	case "Breathing":
		mode = "Breathe"
	case "Flashing":
		mode = "Pulse"
	case "Spectrum Cycle":
		mode = "RainbowCycle"
	case "Rainbow Wave":
		mode = "RainbowWave"
	case "Starry Night":
		mode = "Stars"
	case "Reactive - Fade":
		mode = "Highlight"
	case "Reactive - Laser":
		mode = "Laser"
	case "Reactive - Ripple":
		mode = "Ripple"
	case "Flash N Dash":
		mode = "Flash"
	case "Direct", "Off", "Keystone", "":
		mode = "Static"
	}
	return supportedAuraMode(mode, device)
}

func supportedAuraMode(mode string, device orgbDevice) string {
	if _, _, ok := device.mode(mode); ok {
		return mode
	}
	if _, _, ok := device.mode("Static"); ok {
		return "Static"
	}
	if len(device.Modes) > 0 {
		return device.Modes[0].Name
	}
	return ""
}

func auraEffectFromMode(mode orgbMode) auraEffect {
	effect := auraEffect{
		Mode:      uint32(mode.Value),
		Speed:     auraSpeedName(mode.Speed),
		Direction: auraDirectionName(mode.Direction),
	}
	if len(mode.Colors) > 0 {
		effect.Colour1 = auraUnpackColour(mode.Colors[0])
	}
	if len(mode.Colors) > 1 {
		effect.Colour2 = auraUnpackColour(mode.Colors[1])
	}
	return effect
}

func auraPackedColour(c auraColour) uint32 {
	return uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16
}

func auraUnpackColour(v uint32) auraColour {
	return auraColour{R: uint8(v), G: uint8(v >> 8), B: uint8(v >> 16)}
}

func auraSpeedPercent(speed string) uint32 {
	switch speed {
	case "Low":
		return 0
	case "High":
		return 100
	default:
		return 50
	}
}

func auraSpeedName(percent uint32) string {
	if percent < 34 {
		return "Low"
	}
	if percent > 66 {
		return "High"
	}
	return "Med"
}

func auraDirectionValue(direction string) uint32 {
	for _, d := range orgbDirections {
		if d.Name == direction {
			return d.Val
		}
	}
	return 1
}

func auraDirectionName(value uint32) string {
	for _, d := range orgbDirections {
		if d.Val == value {
			return d.Name
		}
	}
	return "Right"
}

func combineLightingDevices(openrgb, aura []orgbDevice) []orgbDevice {
	hasAuraKeyboard := false
	for _, device := range aura {
		if device.Type == "Keyboard" {
			hasAuraKeyboard = true
			break
		}
	}
	var aliases []string
	out := make([]orgbDevice, 0, len(openrgb)+len(aura))
	for _, device := range openrgb {
		if hasAuraKeyboard && isOpenRGBAsusLaptopAlias(device) {
			aliases = append(aliases, deviceKey(device))
			continue
		}
		out = append(out, device)
	}
	for _, device := range aura {
		if device.Type == "Keyboard" {
			device.Aliases = append(device.Aliases, aliases...)
		}
		out = append(out, device)
	}
	return out
}

func isOpenRGBAsusLaptopAlias(device orgbDevice) bool {
	if !strings.EqualFold(device.Vendor, "ASUS") {
		return false
	}
	return device.Type == "Laptop" || strings.Contains(strings.ToUpper(device.Description), "N-KEY")
}

func readAuraDevices() ([]orgbDevice, error) {
	if !auraInstalled() {
		return nil, nil
	}
	states, err := auraRead()
	if err != nil {
		return nil, fmt.Errorf("ASUS Aura: %w", err)
	}
	devices := make([]orgbDevice, 0, len(states))
	for _, state := range states {
		devices = append(devices, auraDeviceView(state))
	}
	return devices, nil
}
