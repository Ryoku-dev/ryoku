package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// a device description built the way the SDK sends one, so the parser and the
// mode writer are exercised against the shape a real controller has: a per-LED
// Direct mode, a mode-specific Static, an effect with speed and direction, and
// two zones.
func fixtureDevice(t *testing.T) orgbDevice {
	t.Helper()
	b := []byte{}
	add := func(parts ...[]byte) {
		for _, p := range parts {
			b = append(b, p...)
		}
	}
	add(u32(5)) // type: Keyboard
	add(orgbString("Test Board"), orgbString("Vendor"), orgbString("A test board"),
		orgbString("1.0"), orgbString("SN123"), orgbString("HID: /dev/hidraw9"))
	add(u16(3), u32(0)) // three modes, sitting on the first
	modes := []orgbMode{
		{Name: "Direct", Value: 255, Flags: modeHasPerLEDColor | modeHasBrightness, BriMax: 3, Bri: 3, ColorMode: 1},
		{Name: "Static", Value: 0, Flags: modeHasModeColor | modeHasBrightness | modeManualSave,
			BriMax: 3, Bri: 2, ColorsMin: 1, ColorsMax: 1, ColorMode: 2, Colors: []uint32{0}},
		{Name: "Wave", Value: 3, Flags: modeHasSpeed | modeHasDirLR | modeHasDirUD,
			SpeedMin: 245, SpeedMax: 225, Speed: 235},
	}
	for _, m := range modes {
		add(orgbString(m.Name), u32(uint32(m.Value)), u32(m.Flags), u32(m.SpeedMin), u32(m.SpeedMax),
			u32(m.BriMin), u32(m.BriMax), u32(m.ColorsMin), u32(m.ColorsMax), u32(m.Speed), u32(m.Bri),
			u32(m.Direction), u32(m.ColorMode), u16(uint16(len(m.Colors))))
		for _, c := range m.Colors {
			add(u32(c))
		}
	}
	add(u16(2)) // two zones
	add(orgbString("Keys"), u32(2), u32(4), u32(4), u32(4), u16(0), u16(0), u32(0))
	add(orgbString("Logo"), u32(0), u32(1), u32(1), u32(1), u16(0), u16(0), u32(0))
	add(u16(5)) // five LEDs
	for _, n := range []string{"A", "B", "C", "D", "Logo"} {
		add(orgbString(n), u32(0))
	}
	add(u16(0), u16(0), u32(1)) // no device colours, no alternate names, local flag

	full := append(u32(uint32(len(b)+4)), b...)
	d, err := parseDevice(full, 5)
	if err != nil {
		t.Fatalf("parseDevice: %v", err)
	}
	return d
}

func TestParseDeviceReadsCapabilities(t *testing.T) {
	d := fixtureDevice(t)

	if d.Name != "Test Board" || d.Vendor != "Vendor" || d.Serial != "SN123" {
		t.Fatalf("identity: %+v", d)
	}
	if d.Type != "Keyboard" {
		t.Fatalf("type = %q, want Keyboard", d.Type)
	}
	if len(d.Modes) != 3 || len(d.Zones) != 2 || d.LEDCount != 5 {
		t.Fatalf("shape: %d modes, %d zones, %d leds", len(d.Modes), len(d.Zones), d.LEDCount)
	}
	if d.Zones[1].Start != 4 || d.Zones[1].LEDs != 1 {
		t.Fatalf("second zone starts at %d with %d leds", d.Zones[1].Start, d.Zones[1].LEDs)
	}

	_, direct, _ := d.mode("Direct")
	if !direct.has(modeHasPerLEDColor) || direct.CanSave() {
		t.Fatal("Direct should paint per LED and not offer a device save")
	}
	_, static, _ := d.mode("Static")
	if !static.CanSave() || static.ColorsMax != 1 {
		t.Fatalf("Static: canSave=%v colorsMax=%d", static.CanSave(), static.ColorsMax)
	}
	_, wave, _ := d.mode("Wave")
	if got := wave.Directions(); len(got) != 4 || got[0] != "Left" || got[3] != "Down" {
		t.Fatalf("Wave directions = %v", got)
	}
	if wave.Directions() == nil || static.Directions() != nil {
		t.Fatal("only a mode that advertises a direction should offer one")
	}
}

func TestParseDeviceRejectsTruncated(t *testing.T) {
	if _, err := parseDevice(u32(4), 5); err == nil {
		t.Fatal("a description with no body should not parse")
	}
	full := append(u32(64), make([]byte, 8)...)
	if _, err := parseDevice(full, 5); err == nil {
		t.Fatal("a size that does not match the buffer should not parse")
	}
}

func TestModePayloadRoundTrips(t *testing.T) {
	d := fixtureDevice(t)
	idx, m, ok := d.mode("Static")
	if !ok {
		t.Fatal("no Static mode")
	}
	m.Colors = []uint32{0x0056F2} // packed R=F2 G=56 B=00
	m.Bri = 1

	p := modePayload(5, idx, m)
	r := &reader{b: p}
	if size := r.u32(); int(size) != len(p) {
		t.Fatalf("payload declares %d bytes, is %d", size, len(p))
	}
	if got := r.u32(); int(got) != idx {
		t.Fatalf("mode index = %d, want %d", got, idx)
	}
	if got := r.str(); got != "Static" {
		t.Fatalf("mode name = %q", got)
	}
	if got := r.i32(); got != m.Value {
		t.Fatalf("mode value = %d, want %d", got, m.Value)
	}
	for i, want := range []uint32{m.Flags, m.SpeedMin, m.SpeedMax, m.BriMin, m.BriMax,
		m.ColorsMin, m.ColorsMax, m.Speed, m.Bri, m.Direction, m.ColorMode} {
		if got := r.u32(); got != want {
			t.Fatalf("field %d = %d, want %d", i, got, want)
		}
	}
	if n := r.u16(); n != 1 {
		t.Fatalf("colour count = %d, want 1", n)
	}
	if got := r.u32(); got != 0x0056F2 {
		t.Fatalf("colour = %#x", got)
	}
	if r.err != nil || r.i != len(p) {
		t.Fatalf("payload not fully consumed: %d of %d (%v)", r.i, len(p), r.err)
	}
}

func TestColorValuePacksRedLow(t *testing.T) {
	// OpenRGB packs red in the low byte; getting this backwards silently swaps
	// every colour a user picks.
	v, ok := colorValue("#F25623")
	if !ok || v != 0x2356F2 {
		t.Fatalf("colorValue(#F25623) = %#x, %v", v, ok)
	}
	if _, ok := colorValue("nope"); ok {
		t.Fatal("a non-colour should not pack")
	}
}

func TestTuneModeOnlyTouchesWhatTheModeOffers(t *testing.T) {
	d := fixtureDevice(t)
	s := &lightingSettings{Source: "fixed", Colors: []string{"#F25623"}, Brightness: 100, Speed: 0}

	_, static, _ := d.mode("Static")
	got := tuneMode(static, s, "#000000")
	if len(got.Colors) != 1 || got.Colors[0] != 0x2356F2 {
		t.Fatalf("Static colours = %v", got.Colors)
	}
	if got.Bri != static.BriMax {
		t.Fatalf("brightness = %d, want %d", got.Bri, static.BriMax)
	}
	if got.Speed != static.Speed {
		t.Fatal("a mode without speed must keep the device's own value")
	}

	_, wave, _ := d.mode("Wave")
	got = tuneMode(wave, s, "#000000")
	if len(got.Colors) != 0 {
		t.Fatal("a mode with no colour slots must not be sent colours")
	}
	// the device reports speed_min above speed_max, so 0% is its slow end.
	if got.Speed != wave.SpeedMin {
		t.Fatalf("speed = %d, want %d", got.Speed, wave.SpeedMin)
	}
	if got.Bri != wave.Bri {
		t.Fatal("a mode without brightness must keep the device's own value")
	}

	s.Brightness, s.Speed = -1, -1
	if got := tuneMode(static, s, "#000000"); got.Bri != static.Bri {
		t.Fatalf("an unset knob must not move: %d, want %d", got.Bri, static.Bri)
	}
}

func TestTuneModeDirectionMustBeOffered(t *testing.T) {
	d := fixtureDevice(t)
	_, wave, _ := d.mode("Wave")
	_, static, _ := d.mode("Static")

	if got := tuneMode(wave, &lightingSettings{Direction: "Down", Brightness: -1, Speed: -1}, ""); got.Direction != 3 {
		t.Fatalf("direction = %d, want 3", got.Direction)
	}
	if got := tuneMode(wave, &lightingSettings{Direction: "Horizontal", Brightness: -1, Speed: -1}, ""); got.Direction != wave.Direction {
		t.Fatal("a direction the mode does not advertise must be ignored")
	}
	if got := tuneMode(static, &lightingSettings{Direction: "Left", Brightness: -1, Speed: -1}, ""); got.Direction != static.Direction {
		t.Fatal("a mode with no direction must keep its own")
	}
}

func TestAccentSourceFollowsThePalette(t *testing.T) {
	s := &lightingSettings{Source: "accent", Brightness: -1, Speed: -1}
	got := colorValues(s, "#112233", 1)
	if len(got) != 1 || got[0] != 0x332211 {
		t.Fatalf("accent colour = %v", got)
	}
	s.Source = "fixed"
	s.Colors = []string{"#F25623"}
	if got := colorValues(s, "#112233", 1); got[0] != 0x2356F2 {
		t.Fatalf("fixed colour = %v", got)
	}
	// a two-colour mode with one stored colour repeats it rather than sending black.
	if got := colorValues(s, "#112233", 2); len(got) != 2 || got[1] != got[0] {
		t.Fatalf("padded colours = %v", got)
	}
}

func TestScaleToHandlesInvertedRanges(t *testing.T) {
	if got := scaleTo(100, 0, 3); got != 3 {
		t.Fatalf("100%% of 0..3 = %d", got)
	}
	if got := scaleTo(50, 0, 100); got != 50 {
		t.Fatalf("50%% of 0..100 = %d", got)
	}
	// a device that counts down: min above max, 100% still means fastest.
	if got := scaleTo(100, 245, 225); got != 225 {
		t.Fatalf("100%% of 245..225 = %d", got)
	}
	if got := asPercent(225, 245, 225); got != 100 {
		t.Fatalf("asPercent(225 of 245..225) = %d", got)
	}
	if got := asPercent(2, 0, 3); got != 67 {
		t.Fatalf("asPercent(2 of 0..3) = %d", got)
	}
}

func TestLiftColorKeepsDarkPalettesVisible(t *testing.T) {
	if got := liftColor("#0A0806"); got != "#604C39" {
		t.Fatalf("lifted = %s", got)
	}
	if got := liftColor("#DEC2A2"); got != "#DEC2A2" {
		t.Fatal("a palette already bright enough must pass through untouched")
	}
	if got := liftColor("#000000"); got != lightingFallback {
		t.Fatalf("black = %s, want the Ryoku fallback", got)
	}
	if got := liftColor("garbage"); got != lightingFallback {
		t.Fatalf("nonsense = %s, want the Ryoku fallback", got)
	}
}

func TestPunchColorRestoresWashedOutAccents(t *testing.T) {
	// a bright, bunched-up pastel gains chroma: the top channel stays, the others
	// are pulled twice as far from it (230,210,200 -> 230,190,170).
	if got := punchColor("#E6D2C8"); got != "#E6BEAA" {
		t.Fatalf("washed pastel = %s, want #E6BEAA", got)
	}
	// an already-saturated colour is left alone.
	if got := punchColor("#F25623"); got != "#F25623" {
		t.Fatalf("saturated = %s, want unchanged", got)
	}
	// a bright grey has no hue to restore, so it passes through untouched.
	if got := punchColor("#C8C8C8"); got != "#C8C8C8" {
		t.Fatalf("grey = %s, want unchanged", got)
	}
	// a dark accent is below the wash-out threshold and untouched.
	if got := punchColor("#303030"); got != "#303030" {
		t.Fatalf("dark = %s, want unchanged", got)
	}
	if got := punchColor("garbage"); got != lightingFallback {
		t.Fatalf("nonsense = %s, want the Ryoku fallback", got)
	}
}

func TestDeviceKeyIgnoresThePort(t *testing.T) {
	a := orgbDevice{Name: "Board", Serial: "SN1", Location: "HID: /dev/hidraw0"}
	b := orgbDevice{Name: "Board", Serial: "SN1", Location: "HID: /dev/hidraw3"}
	if deviceKey(a) != deviceKey(b) {
		t.Fatal("a device that moved port must keep its settings")
	}
	if deviceKey(orgbDevice{Name: "Board"}) == deviceKey(a) {
		t.Fatal("a serial must distinguish two boards")
	}
}

func dramStick(address uint32, bus int, slot string) orgbDevice {
	return orgbDevice{
		Name: "ENE DRAM", Vendor: "ENE", Type: "DRAM", Address: address,
		Location: fmt.Sprintf("I2C: SMBus PIIX4 adapter port 0 at 0b00 (/dev/i2c-%d), address %s", bus, slot),
		Modes: []orgbMode{
			{Name: "Direct", Value: 0, Flags: modeHasPerLEDColor},
			{Name: "Breathing", Value: 2, Flags: modeHasSpeed | modeHasPerLEDColor},
			{Name: "Rainbow", Value: 5, Flags: modeHasSpeed},
		},
	}
}

func TestDeviceKeySeparatesIdenticalSMBusSticks(t *testing.T) {
	first, second := dramStick(0, 8, "0x71"), dramStick(1, 8, "0x73")
	if deviceKey(first) == deviceKey(second) {
		t.Fatalf("two sticks share %q", deviceKey(first))
	}
	if moved := dramStick(0, 9, "0x71"); deviceKey(moved) != deviceKey(first) {
		t.Fatalf("a renumbered i2c bus moved the stick from %q to %q", deviceKey(first), deviceKey(moved))
	}
	if got, want := deviceKey(first), "ENE DRAM#smbus:SMBus PIIX4 adapter port 0 at 0b00@0x71"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	if got := deviceKey(orgbDevice{Name: "Pad", Location: "HID: /dev/hidraw4"}); got != "Pad" {
		t.Fatalf("a serial-less USB device keyed as %q, want its bare name", got)
	}
}

func TestFirstSightGivesEverySharedStickItsOwnCopy(t *testing.T) {
	setupFollowTest(t)
	if err := saveLighting(lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		"ENE DRAM": {
			Name: "ENE DRAM", Managed: true, Mode: "Breathing", Effect: "pulse", Source: "accent",
			Colors: []string{"#E79389"}, Brightness: -1, Speed: 40, Restore: "Rainbow",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	first, second := dramStick(0, 8, "0x71"), dramStick(1, 8, "0x73")

	st := rememberFirstSight([]orgbDevice{first, second})
	if _, ok := st.Devices["ENE DRAM"]; ok {
		t.Fatal("the shared entry outlived the migration")
	}
	a, b := st.Devices[deviceKey(first)], st.Devices[deviceKey(second)]
	for _, s := range []*lightingSettings{a, b} {
		if s == nil || !s.Managed || s.Mode != "Breathing" || s.Effect != "pulse" ||
			s.Speed != 40 || s.Restore != "Rainbow" || len(s.Colors) != 1 {
			t.Fatalf("stick settings = %+v, want the shared look", s)
		}
	}
	a.Colors[0] = "#000000"
	if b.Colors[0] != "#E79389" {
		t.Fatal("the sticks still share one settings value")
	}
	if saved := loadLighting(); len(saved.Devices) != 2 {
		t.Fatalf("saved devices = %d, want 2", len(saved.Devices))
	}
}

func TestReleaseRestoresOnlyTheChosenStick(t *testing.T) {
	setupFollowTest(t)
	first, second := dramStick(4, 8, "0x71"), dramStick(7, 8, "0x73")
	managed := func() *lightingSettings {
		return &lightingSettings{Name: "ENE DRAM", Managed: true, Mode: "Breathing",
			Source: "accent", Brightness: -1, Speed: -1, Restore: "Rainbow"}
	}
	if err := saveLighting(lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		deviceKey(first): managed(), deviceKey(second): managed(),
	}}); err != nil {
		t.Fatal(err)
	}

	oldAccess := lightingAccess
	t.Cleanup(func() { lightingAccess = oldAccess })
	packet := captureWrite(t, 5, func(conn *orgbConn) error {
		lightingAccess = func(_ bool, _ bool, _ lightingProviders, fn func(*orgbConn, []orgbDevice) error) error {
			return fn(conn, []orgbDevice{first, second})
		}
		captureStdout(t, func() {
			if err := lightingRelease(deviceKey(second)); err != nil {
				t.Fatal(err)
			}
		})
		return nil
	})
	if packet.id != pktUpdateMode || packet.device != second.Address {
		t.Fatalf("restore went to device %d (packet %d), want %d", packet.device, packet.id, second.Address)
	}
	st := loadLighting()
	if st.Devices[deviceKey(second)].Managed {
		t.Fatal("the released stick is still managed")
	}
	if !st.Devices[deviceKey(first)].Managed {
		t.Fatal("releasing one stick released the other")
	}
}

func TestStateDefaultsToOffAndSurvivesGarbage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if st := loadLighting(); st.Enabled || len(st.Devices) != 0 {
		t.Fatal("with no file, lighting must read as off with nothing adopted")
	}

	want := lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Managed: true, Mode: "Static", Source: "fixed",
			Colors: []string{"#F25623"}, Brightness: 80, Speed: -1, Restore: "Direct"},
	}}
	if err := saveLighting(want); err != nil {
		t.Fatal(err)
	}
	got := loadLighting()
	if !got.Enabled || got.Devices["Board#SN1"].Mode != "Static" || got.Devices["Board#SN1"].Restore != "Direct" {
		t.Fatalf("round trip lost settings: %+v", got.Devices["Board#SN1"])
	}

	if err := os.WriteFile(filepath.Join(dir, "ryoku", "lighting.json"), []byte("{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st := loadLighting(); st.Enabled || len(st.Devices) != 0 {
		t.Fatal("an unreadable file must fail closed, never into writing to devices")
	}
}

func TestSetMergesWithoutClobbering(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("PATH", dir) // no openrgb, so nothing can be applied while merging

	if err := saveLighting(lightingState{Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Mode: "Static", Source: "accent", Brightness: -1, Speed: -1, Restore: "Direct"},
	}}); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := lightingSet("Board#SN1", `{"colors":["f25623"],"source":"fixed","brightness":40}`); err != nil {
			t.Fatal(err)
		}
	})
	var rep lightReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("set printed %q: %v", out, err)
	}

	got := loadLighting().Devices["Board#SN1"]
	if got.Source != "fixed" || len(got.Colors) != 1 || got.Colors[0] != "#F25623" {
		t.Fatalf("colour patch: %+v", got)
	}
	if got.Brightness != 40 || got.Speed != -1 {
		t.Fatalf("brightness/speed: %+v", got)
	}
	if got.Mode != "Static" || got.Restore != "Direct" || got.Name != "Board" {
		t.Fatalf("a patch must not drop untouched fields: %+v", got)
	}
	if got.Managed {
		t.Fatal("patching settings must not adopt a device on its own")
	}
}

func TestReleaseKeepsSettingsAndDropsControl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("PATH", dir)

	if err := saveLighting(lightingState{Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Managed: true, Mode: "Static", Source: "fixed",
			Colors: []string{"#F25623"}, Brightness: 80, Speed: -1, Restore: "Direct"},
	}}); err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		if err := lightingSet("Board#SN1", `{"managed":false}`); err != nil {
			t.Fatal(err)
		}
	})

	got := loadLighting().Devices["Board#SN1"]
	if got.Managed {
		t.Fatal("handing a device back must clear managed")
	}
	if got.Mode != "Static" || got.Brightness != 80 || len(got.Colors) != 1 {
		t.Fatalf("handing a device back must keep its look for next time: %+v", got)
	}
}

func TestAuraReleaseFailureKeepsDeviceManaged(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"asusd", "openrgb"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	state := auraState{
		Path:       "/xyz/ljones/aura/keyboard",
		Name:       "Keyboard",
		Mode:       auraStatic,
		Modes:      []uint32{auraStatic},
		Brightness: 3,
	}
	device := auraDeviceView(state)
	key := deviceKey(device)
	if err := saveLighting(lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		key: {
			Name:       device.Name,
			Managed:    true,
			Mode:       "Static",
			Source:     "accent",
			Brightness: 90,
			Speed:      -1,
			Restore:    "Static",
		},
	}}); err != nil {
		t.Fatal(err)
	}

	oldRead := auraRead
	defer func() { auraRead = oldRead }()
	auraRead = func() ([]auraState, error) { return []auraState{state}, nil }
	oldRestore := auraRestore
	defer func() { auraRestore = oldRestore }()
	auraRestore = func(string, uint32) error { return errors.New("restore failed") }

	out := captureStdout(t, func() {
		if err := lightingRelease(key); err != nil {
			t.Fatal(err)
		}
	})
	var rep lightReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("release printed %q: %v", out, err)
	}
	if rep.Error != "restore failed" {
		t.Fatalf("release error = %q", rep.Error)
	}
	if !loadLighting().Devices[key].Managed {
		t.Fatal("failed hand-back must stay managed so it can be retried")
	}
}

func TestApplyIsANoOpUntilTheUserOptsIn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	// lighting off: nothing to do even with a device on file.
	if err := saveLighting(lightingState{Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Managed: true, Mode: "Static", Brightness: -1, Speed: -1},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := lightingApply(""); err != nil {
		t.Fatalf("apply with lighting off: %v", err)
	}
	// lighting on, but no device adopted: still nothing to do.
	if err := saveLighting(lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Mode: "Static", Brightness: -1, Speed: -1},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := lightingApply(""); err != nil {
		t.Fatalf("apply with nothing adopted: %v", err)
	}
}

func TestStateReportDoesNotTouchHardware(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("PATH", dir)

	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ryoku", "hypr-colors.lua"),
		[]byte("return {\n    active = \"#dec2a2\",\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveLighting(lightingState{Devices: map[string]*lightingSettings{
		"Board#SN1": {Name: "Board", Mode: "Static", Brightness: -1, Speed: -1},
	}}); err != nil {
		t.Fatal(err)
	}

	rep := lightingStateReport()
	if rep.Available {
		t.Fatal("openrgb is not on PATH here; the report must say so")
	}
	if rep.Enabled {
		t.Fatal("state must report lighting off")
	}
	// accentColor() punches a washed-out accent so an LED shows the hue rather
	// than near-white, and the report carries that effective colour. #DEC2A2 is
	// pale enough to be punched, so the expected value here is the punched one;
	// punchColor is idempotent, so this does not drift on a re-read.
	if rep.Accent != "#DEA666" {
		t.Fatalf("accent = %s", rep.Accent)
	}
	if len(rep.Devices) != 1 || rep.Devices[0].Key != "Board#SN1" || rep.Devices[0].Online {
		t.Fatalf("saved devices should list offline: %+v", rep.Devices)
	}
	if len(rep.Devices[0].Modes) != 0 {
		t.Fatal("a report that never probed cannot know a device's modes")
	}
}

func TestAuraOnlyApplySkipsOpenRGB(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"asusd", "openrgb"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	state := auraState{
		Path:       "/xyz/ljones/aura/keyboard",
		Name:       "Keyboard",
		Mode:       auraStatic,
		Modes:      []uint32{auraStatic},
		Brightness: 3,
	}
	device := auraDeviceView(state)
	if err := saveLighting(lightingState{Enabled: true, Devices: map[string]*lightingSettings{
		deviceKey(device): {
			Name:       device.Name,
			Managed:    true,
			Mode:       "Static",
			Source:     "accent",
			Brightness: 90,
			Speed:      -1,
		},
	}}); err != nil {
		t.Fatal(err)
	}

	oldDial := openrgbDial
	defer func() { openrgbDial = oldDial }()
	openrgbDial = func(time.Duration) (*orgbConn, error) {
		t.Fatal("Aura-only apply dialed OpenRGB")
		return nil, nil
	}
	oldRead := auraRead
	defer func() { auraRead = oldRead }()
	auraRead = func() ([]auraState, error) { return []auraState{state}, nil }
	oldWrite := auraWrite
	defer func() { auraWrite = oldWrite }()
	wrote := false
	auraWrite = func(string, auraEffect, int) error {
		wrote = true
		return nil
	}

	if err := lightingApply("#112233"); err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("Aura-only apply did not write the native keyboard")
	}
}

// captureStdout runs fn with stdout redirected, so a command's JSON can be read
// back the way the Hub reads it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	read := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		read <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = saved
	out := <-read
	r.Close()
	return out
}
