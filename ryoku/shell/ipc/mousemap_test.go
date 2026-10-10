package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// capKey renders a capabilities/key value: hex words, most-significant first,
// exactly the sysfs shape parseKeyCapabilities decodes.
func capKey(bits ...int) string {
	maxWord := 0
	for _, bit := range bits {
		if bit/64 > maxWord {
			maxWord = bit / 64
		}
	}
	words := make([]uint64, maxWord+1)
	for _, bit := range bits {
		words[bit/64] |= 1 << uint(bit%64)
	}
	parts := make([]string, 0, len(words))
	for i := maxWord; i >= 0; i-- {
		parts = append(parts, strconv.FormatUint(words[i], 16))
	}
	return strings.Join(parts, " ")
}

// addFakeInput lays out one event node: the real device dir (carrying the key
// capabilities and name) lives under realSub, and class/input/<event>/device
// symlinks to it, so EvalSymlinks reports the bus/virtual path the classifier
// keys on.
func addFakeInput(t testing.TB, sysRoot, devRoot, event, realSub, key, name string, makeNode bool) {
	t.Helper()
	realDir := filepath.Join(sysRoot, realSub)
	if err := os.MkdirAll(filepath.Join(realDir, "capabilities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "capabilities", "key"), []byte(key), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "name"), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	eventDir := filepath.Join(sysRoot, "class", "input", event)
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(eventDir, "device")); err != nil {
		t.Fatal(err)
	}
	if makeNode {
		if err := os.WriteFile(filepath.Join(devRoot, event), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
func addFakeUSBIdentity(t testing.TB, sysRoot, sub, vendor, product string) {
	t.Helper()
	dir := filepath.Join(sysRoot, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "idVendor"), []byte(vendor), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "idProduct"), []byte(product), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyMouseDevices(t *testing.T) {
	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	devRoot := filepath.Join(root, "dev", "input")
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.MkdirAll(byIDRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mouseKey := capKey(272, 273, 274, 275, 276, 286, 287, 704, 743)

	// A real USB mouse with a stable by-id symlink and catalogue identity.
	addFakeUSBIdentity(t, sysRoot, "devices/pci0000/usb1/1-1", "046d", "c08b")
	addFakeInput(t, sysRoot, devRoot, "event0", "devices/pci0000/usb1/1-1/1-1:1.0/input/input5", mouseKey, "Logitech G502 Hero", true)
	if err := os.Symlink(filepath.Join(devRoot, "event0"), filepath.Join(byIDRoot, "usb-Logitech_G502-event-mouse")); err != nil {
		t.Fatal(err)
	}
	// A bluetooth mouse with no by-id entry: the id falls back to the event name.
	addFakeInput(t, sysRoot, devRoot, "event6", "devices/pci0000/bluetooth/hci0/input11", capKey(272, 273), "BT Mouse", true)
	// A keyboard: no BTN_MOUSE.
	addFakeInput(t, sysRoot, devRoot, "event1", "devices/pci0000/usb1/1-2/input6", capKey(30, 44, 28, 57, 704), "AT Keyboard", true)
	// A touchpad: BTN_TOOL_FINGER set.
	addFakeInput(t, sysRoot, devRoot, "event2", "devices/platform/i2c/input7", capKey(272, 325), "Synaptics TouchPad", true)
	// A built-in virtual mouse: realpath under the virtual bus.
	addFakeInput(t, sysRoot, devRoot, "event3", "devices/virtual/input/input8", capKey(272, 273), "Virtual core pointer", true)
	// One of our own clones by name, on a non-virtual path.
	addFakeInput(t, sysRoot, devRoot, "event5", "devices/pci0000/usb1/1-3/input10", capKey(272, 273), "Ryoku Mouse Logitech G502", true)

	got := classifyMouseDevices(sysRoot, devRoot, byIDRoot)
	if len(got) != 2 {
		t.Fatalf("expected 2 mice, got %d: %+v", len(got), got)
	}

	byID := map[string]mouseDevice{}
	for _, dev := range got {
		byID[dev.ID] = dev
	}

	logi, ok := byID["usb-Logitech_G502-event-mouse"]
	if !ok {
		t.Fatalf("logitech mouse id not resolved from by-id: %+v", got)
	}
	if logi.Name != "Logitech G502 Hero" {
		t.Errorf("name = %q, want Logitech G502 Hero", logi.Name)
	}
	if logi.Brand != "Logitech" || logi.Model != "G502 Hero" {
		t.Errorf("catalogue identity = %q %q, want Logitech G502 Hero", logi.Brand, logi.Model)
	}
	if logi.Labels["277"] != "DPI shift" {
		t.Errorf("friendly label 277 = %q, want DPI shift", logi.Labels["277"])
	}
	if logi.Bus != "usb" {
		t.Errorf("bus = %q, want usb", logi.Bus)
	}
	wantButtons := []uint16{272, 273, 274, 275, 276, 286, 287, 704, 743}
	if !reflect.DeepEqual(logi.Buttons, wantButtons) {
		t.Errorf("buttons = %v, want %v", logi.Buttons, wantButtons)
	}

	bt, ok := byID["event6"]
	if !ok {
		t.Fatalf("bluetooth mouse id did not fall back to event name: %+v", got)
	}
	if bt.Bus != "bluetooth" {
		t.Errorf("bus = %q, want bluetooth", bt.Bus)
	}
	if bt.Brand != "" || bt.Model != "" || len(bt.Labels) != 0 {
		t.Errorf("unknown mouse should stay generic, got %+v", bt)
	}
}

func TestMouseInputFactsCacheInvalidatesRecreatedNode(t *testing.T) {
	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	devRoot := filepath.Join(root, "dev", "input")
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.MkdirAll(byIDRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	addFakeUSBIdentity(t, sysRoot, "devices/pci0000/usb1/1-1", "046d", "c08b")
	addFakeInput(t, sysRoot, devRoot, "event4", "devices/pci0000/usb1/1-1/1-1:1.0/input/input4",
		capKey(272, 273, 274, 275), "Logitech G502 Hero", true)

	cache := mouseInputFactsCache{}
	if got := classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache); len(got) != 1 {
		t.Fatalf("first classification = %+v", got)
	}
	capPath := filepath.Join(sysRoot, "devices/pci0000/usb1/1-1/1-1:1.0/input/input4/capabilities/key")
	if err := os.Remove(capPath); err != nil {
		t.Fatal(err)
	}
	if got := classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache); len(got) != 1 {
		t.Fatalf("unchanged node reread sysfs instead of using facts cache: %+v", got)
	}

	time.Sleep(time.Millisecond)
	node := filepath.Join(devRoot, "event4")
	if err := os.Remove(node); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache); len(got) != 0 {
		t.Fatalf("recreated node inherited stale facts: %+v", got)
	}
}

func TestMouseInputWatcherReportsCreateAndDelete(t *testing.T) {
	devRoot := t.TempDir()
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.Mkdir(byIDRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	changes, stop, err := watchMouseInputChanges(devRoot, byIDRoot)
	if err != nil {
		t.Fatal(err)
	}
	wait := func(action string) {
		t.Helper()
		select {
		case _, ok := <-changes:
			if !ok {
				t.Fatalf("watcher closed while waiting for %s", action)
			}
		case <-time.After(time.Second):
			t.Fatalf("watcher missed %s", action)
		}
	}
	node := filepath.Join(devRoot, "event42")
	if err := os.WriteFile(node, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	wait("create")
	if err := os.Remove(node); err != nil {
		t.Fatal(err)
	}
	wait("delete")
	stop()
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
}

func BenchmarkMouseClassificationCache(b *testing.B) {
	root := b.TempDir()
	sysRoot := filepath.Join(root, "sys")
	devRoot := filepath.Join(root, "dev", "input")
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.MkdirAll(byIDRoot, 0o755); err != nil {
		b.Fatal(err)
	}
	addFakeUSBIdentity(b, sysRoot, "devices/pci0000/usb1/1-1", "1532", "00a7")
	addFakeInput(b, sysRoot, devRoot, "event4", "devices/pci0000/usb1/1-1/1-1:1.0/input/input4",
		capKey(272, 273, 274, 275, 276), "Razer Naga V2 Pro", true)
	addFakeInput(b, sysRoot, devRoot, "event5", "devices/pci0000/usb1/1-1/1-1:1.1/input/input5",
		capKey(2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 30), "Razer Naga V2 Pro Keyboard", true)

	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			classifyMouseDevices(sysRoot, devRoot, byIDRoot)
		}
	})
	b.Run("cached", func(b *testing.B) {
		cache := mouseInputFactsCache{}
		classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache)
		b.ResetTimer()
		b.ReportAllocs()
		for range b.N {
			classifyMouseDevicesCached(sysRoot, devRoot, byIDRoot, cache)
		}
	})
}

func TestMouseCatalogueMatchesPopularModels(t *testing.T) {
	tests := []struct {
		vendor, product uint16
		brand, model    string
	}{
		{0x046d, 0xc08b, "Logitech", "G502 Hero"},
		{0x046d, 0xc099, "Logitech", "G502 X"},
		{0x046d, 0xc094, "Logitech", "G Pro X Superlight"},
		{0x046d, 0xb034, "Logitech", "MX Master 3S"},
		{0x046d, 0x4074, "Logitech", "G305"},
		{0x1532, 0x00b2, "Razer", "DeathAdder V3"},
		{0x1532, 0x0099, "Razer", "Basilisk V3"},
		{0x1532, 0x00c1, "Razer", "Viper V3 Pro"},
		{0x1532, 0x00a7, "Razer", "Naga V2 / Pro"},
		{0x1038, 0x1824, "SteelSeries", "Rival 3"},
		{0x1038, 0x1836, "SteelSeries", "Aerox 3"},
		{0x1b1c, 0x1b5a, "Corsair", "M65"},
		{0x1b1c, 0x1b8b, "Corsair", "Scimitar RGB Elite"},
		{0x258a, 0x0036, "Glorious", "Model O"},
		{0x258a, 0x0033, "Glorious", "Model D"},
		{0x1af3, 0x0001, "Zowie", "EC2"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := matchMouseCatalogue(tt.vendor, tt.product, "")
			if got == nil || got.Brand != tt.brand || got.Model != tt.model {
				t.Fatalf("match %04x:%04x = %+v, want %s %s", tt.vendor, tt.product, got, tt.brand, tt.model)
			}
		})
	}
	if got := matchMouseCatalogue(0xffff, 0xffff, "Logitech MX Master 3S"); got == nil || got.Model != "MX Master 3S" {
		t.Fatalf("name alias match = %+v, want MX Master 3S", got)
	}
	if got := matchMouseCatalogue(0xffff, 0xffff, "Uncatalogued Mouse"); got != nil {
		t.Fatalf("unknown mouse unexpectedly matched %+v", got)
	}
}

func TestClassifyMousePairsMMOSideGrid(t *testing.T) {
	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	devRoot := filepath.Join(root, "dev", "input")
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.MkdirAll(byIDRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	addFakeUSBIdentity(t, sysRoot, "devices/pci0000/usb2/2-1", "1532", "00a7")
	addFakeInput(t, sysRoot, devRoot, "event10", "devices/pci0000/usb2/2-1/2-1:1.0/input/input10", capKey(272, 273, 274, 275, 276), "Razer Naga V2 Pro", true)
	sideKeys := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 30}
	addFakeInput(t, sysRoot, devRoot, "event11", "devices/pci0000/usb2/2-1/2-1:1.1/input/input11", capKey(sideKeys...), "Razer Naga V2 Pro Keyboard", true)

	// Even an identical VID:PID is not a sibling when sysfs gives it a different
	// physical USB parent.
	addFakeUSBIdentity(t, sysRoot, "devices/pci0000/usb2/2-2", "1532", "00a7")
	addFakeInput(t, sysRoot, devRoot, "event12", "devices/pci0000/usb2/2-2/2-2:1.0/input/input12", capKey(2, 3, 30), "Separate keyboard", true)

	got := classifyMouseDevices(sysRoot, devRoot, byIDRoot)
	if len(got) != 1 {
		t.Fatalf("expected one logical mouse, got %d: %+v", len(got), got)
	}
	dev := got[0]
	wantSide := []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	if !reflect.DeepEqual(dev.SideButtons, wantSide) {
		t.Fatalf("side buttons = %v, want %v", dev.SideButtons, wantSide)
	}
	if len(dev.interfaces) != 2 || !dev.interfaces[1].keyboard {
		t.Fatalf("paired interfaces = %+v, want pointer and keyboard side grid", dev.interfaces)
	}
	if mouseCodeIn(dev.SideButtons, 30) {
		t.Fatal("ordinary keyboard key leaked into side-grid buttons")
	}
	if !mouseCodeIn(dev.interfaces[1].keys, 30) {
		t.Fatal("paired keyboard did not retain ordinary keys for clone pass-through")
	}
	if dev.Labels["2"] != "Side 1" || dev.Labels["13"] != "Side 12" {
		t.Fatalf("side labels = %+v", dev.Labels)
	}
}

func TestClassifyMouseVirtualPairRequiresExplicitTestOptIn(t *testing.T) {
	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	devRoot := filepath.Join(root, "dev", "input")
	byIDRoot := filepath.Join(devRoot, "by-id")
	if err := os.MkdirAll(byIDRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	addFakeInput(t, sysRoot, devRoot, "event20", "devices/virtual/input/input20",
		capKey(272, 273, 274, 275, 276), "Ryoku Test Mouse Razer Naga V2 Pro", true)
	addFakeInput(t, sysRoot, devRoot, "event21", "devices/virtual/input/input21",
		capKey(2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13), "Ryoku Test Mouse Razer Naga V2 Pro Keyboard", true)
	for i, phys := range []string{"usb-ryoku-naga/input0", "usb-ryoku-naga/input1"} {
		dir := filepath.Join(sysRoot, "devices", "virtual", "input", fmt.Sprintf("input%d", 20+i))
		if err := os.WriteFile(filepath.Join(dir, "phys"), []byte(phys), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "id"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "id", "vendor"), []byte("1532"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "id", "product"), []byte("00a7"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := classifyMouseDevices(sysRoot, devRoot, byIDRoot); len(got) != 0 {
		t.Fatalf("virtual pair was visible without opt-in: %+v", got)
	}
	t.Setenv("RYOKU_MOUSE_TEST_VIRTUAL", "1")
	got := classifyMouseDevices(sysRoot, devRoot, byIDRoot)
	if len(got) != 1 || len(got[0].interfaces) != 2 || len(got[0].SideButtons) != 12 {
		t.Fatalf("opted-in virtual Naga pair = %+v", got)
	}
}

func TestMouseMapPatchPersistRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mousemap.json")

	maps := map[string]map[string]mouseTarget{}
	maps = patchMouseMaps(maps, "dev1", 275, &mouseTarget{Kind: "chord", Keys: []string{"ctrl", "c"}})
	maps = patchMouseMaps(maps, "dev1", 276, &mouseTarget{Kind: "button", Button: 273})
	maps = patchMouseMaps(maps, "dev1", 277, &mouseTarget{Kind: "disabled"})
	maps = patchMouseMaps(maps, "dev1", 278, &mouseTarget{
		Kind: "sequence", Repeat: 2, CancelOnRelease: true,
		Sequence: []mouseMacroStep{{Kind: "tap", Keys: []string{"ctrl", "c"}}, {Kind: "delay", DelayMS: 25}},
	})
	if err := writeMouseSettings(path, mouseSettings{Maps: maps}); err != nil {
		t.Fatal(err)
	}
	loaded := loadMouseSettings(path)
	if !reflect.DeepEqual(loaded.Maps, maps) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", loaded.Maps, maps)
	}

	// Removing one button leaves the rest.
	trimmed := patchMouseMaps(loaded.Maps, "dev1", 276, nil)
	if _, present := trimmed["dev1"]["276"]; present {
		t.Fatalf("button 276 not removed: %+v", trimmed)
	}
	if len(trimmed["dev1"]) != 3 {
		t.Fatalf("expected 3 remaining buttons, got %+v", trimmed["dev1"])
	}

	// Removing the last button drops the device entirely.
	trimmed = patchMouseMaps(trimmed, "dev1", 275, nil)
	trimmed = patchMouseMaps(trimmed, "dev1", 277, nil)
	trimmed = patchMouseMaps(trimmed, "dev1", 278, nil)
	if _, present := trimmed["dev1"]; present {
		t.Fatalf("empty device should be dropped: %+v", trimmed)
	}
}

func TestMouseSubstitution(t *testing.T) {
	sub, err := newMouseSubstitutor(map[uint16]mouseTarget{
		275: {Kind: "chord", Keys: []string{"ctrl", "c"}},
		276: {Kind: "button", Button: 273},
		277: {Kind: "disabled"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Chord press emits key-downs in order; release unwinds in reverse.
	out, consumed := sub.apply(evKey, 275, 1)
	if !consumed {
		t.Fatal("chord press not consumed")
	}
	if !reflect.DeepEqual(out, []synthEvent{{evKey, 29, 1}, {evKey, 46, 1}}) {
		t.Fatalf("chord down = %v, want ctrl+c down (29,46)", out)
	}
	out, consumed = sub.apply(evKey, 275, 0)
	if !consumed {
		t.Fatal("chord release not consumed")
	}
	if !reflect.DeepEqual(out, []synthEvent{{evKey, 46, 0}, {evKey, 29, 0}}) {
		t.Fatalf("chord up = %v, want reverse (46,29)", out)
	}

	// Button remap forwards a single BTN_ press/release.
	out, consumed = sub.apply(evKey, 276, 1)
	if !consumed || !reflect.DeepEqual(out, []synthEvent{{evKey, 273, 1}}) {
		t.Fatalf("button down = %v consumed=%v", out, consumed)
	}
	out, consumed = sub.apply(evKey, 276, 0)
	if !consumed || !reflect.DeepEqual(out, []synthEvent{{evKey, 273, 0}}) {
		t.Fatalf("button up = %v consumed=%v", out, consumed)
	}

	// Disabled swallows both edges.
	if out, consumed := sub.apply(evKey, 277, 1); !consumed || len(out) != 0 {
		t.Fatalf("disabled press = %v consumed=%v", out, consumed)
	}
	if out, consumed := sub.apply(evKey, 277, 0); !consumed || len(out) != 0 {
		t.Fatalf("disabled release = %v consumed=%v", out, consumed)
	}

	// Unmapped button and non-EV_KEY events pass through untouched.
	if out, consumed := sub.apply(evKey, 280, 1); consumed || out != nil {
		t.Fatalf("unmapped button should pass through, got %v consumed=%v", out, consumed)
	}
	if out, consumed := sub.apply(evRel, relX, 5); consumed || out != nil {
		t.Fatalf("pointer motion should pass through, got %v consumed=%v", out, consumed)
	}
}

func TestMouseChordRejectsUnknownToken(t *testing.T) {
	if err := validateTarget(mouseTarget{Kind: "chord", Keys: []string{"ctrl", "notakey"}}); err == nil {
		t.Fatal("expected unknown chord token to be rejected")
	}
	if _, err := newMouseSubstitutor(map[uint16]mouseTarget{
		275: {Kind: "chord", Keys: []string{"ctrl", "notakey"}},
	}); err == nil {
		t.Fatal("expected substitutor build to reject unknown chord token")
	}
	if err := validateTarget(mouseTarget{Kind: "weird"}); err == nil {
		t.Fatal("expected unknown kind to be rejected")
	}
	if err := validateTarget(mouseTarget{Kind: "button", Button: 0}); err == nil {
		t.Fatal("expected button target without a code to be rejected")
	}
}

func TestMouseSequenceValidation(t *testing.T) {
	valid := mouseTarget{
		Kind: "sequence", Repeat: 3, CancelOnRelease: true,
		Sequence: []mouseMacroStep{
			{Kind: "down", Keys: []string{"ctrl", "shift"}},
			{Kind: "tap", Keys: []string{"c"}},
			{Kind: "delay", DelayMS: 40},
			{Kind: "up", Keys: []string{"ctrl", "shift"}},
		},
	}
	if err := validateTarget(valid); err != nil {
		t.Fatalf("valid sequence rejected: %v", err)
	}
	invalid := []mouseTarget{
		{Kind: "sequence"},
		{Kind: "sequence", Repeat: -1, Sequence: []mouseMacroStep{{Kind: "tap", Keys: []string{"a"}}}},
		{Kind: "sequence", Repeat: maxMouseSequenceRepeat + 1, Sequence: []mouseMacroStep{{Kind: "tap", Keys: []string{"a"}}}},
		{Kind: "sequence", Sequence: []mouseMacroStep{{Kind: "tap", Keys: []string{"notakey"}}}},
		{Kind: "sequence", Sequence: []mouseMacroStep{{Kind: "delay", DelayMS: 0}}},
		{Kind: "sequence", Sequence: []mouseMacroStep{{Kind: "delay", DelayMS: int(maxMouseSequenceDelay/time.Millisecond) + 1}}},
		{Kind: "sequence", Sequence: []mouseMacroStep{{Kind: "hold", Keys: []string{"a"}}}},
	}
	for i, target := range invalid {
		if err := validateTarget(target); err == nil {
			t.Errorf("invalid sequence %d was accepted: %+v", i, target)
		}
	}
}

func TestMouseSequenceExecutionUsesDelaysAndRepeat(t *testing.T) {
	var mu sync.Mutex
	events := []synthEvent{}
	eventTimes := []time.Duration{}
	delays := []time.Duration{}
	var elapsed time.Duration
	executor := newMouseMacroExecutor(context.Background(), func(event synthEvent) error {
		mu.Lock()
		events = append(events, event)
		eventTimes = append(eventTimes, elapsed)
		mu.Unlock()
		return nil
	}, func(ctx context.Context, delay time.Duration) bool {
		mu.Lock()
		delays = append(delays, delay)
		elapsed += delay
		mu.Unlock()
		return ctx.Err() == nil
	})
	target := mouseTarget{
		Kind: "sequence", Repeat: 2,
		Sequence: []mouseMacroStep{
			{Kind: "tap", Keys: []string{"ctrl", "c"}},
			{Kind: "delay", DelayMS: 25},
			{Kind: "down", Keys: []string{"shift"}},
			{Kind: "up", Keys: []string{"shift"}},
		},
	}
	select {
	case <-executor.start(275, target):
	case <-time.After(time.Second):
		t.Fatal("sequence did not finish")
	}

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(delays, []time.Duration{25 * time.Millisecond, 25 * time.Millisecond}) {
		t.Fatalf("delays = %v, want two 25ms waits", delays)
	}
	keyEvents := []synthEvent{}
	keyTimes := []time.Duration{}
	for i, event := range events {
		if event.etype == evKey {
			keyEvents = append(keyEvents, event)
			keyTimes = append(keyTimes, eventTimes[i])
		}
	}
	oneCycle := []synthEvent{
		{evKey, 29, 1}, {evKey, 46, 1}, {evKey, 46, 0}, {evKey, 29, 0},
		{evKey, 42, 1}, {evKey, 42, 0},
	}
	want := append(append([]synthEvent{}, oneCycle...), oneCycle...)
	if !reflect.DeepEqual(keyEvents, want) {
		t.Fatalf("sequence events = %v, want %v", keyEvents, want)
	}
	if keyTimes[0] != 0 || keyTimes[4] != 25*time.Millisecond ||
		keyTimes[6] != 25*time.Millisecond || keyTimes[10] != 50*time.Millisecond {
		t.Fatalf("sequence event times = %v", keyTimes)
	}
}

func TestMouseSequenceCancelOnReleaseUnwindsHeldKeys(t *testing.T) {
	var mu sync.Mutex
	events := []synthEvent{}
	waiting := make(chan struct{})
	executor := newMouseMacroExecutor(context.Background(), func(event synthEvent) error {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		return nil
	}, func(ctx context.Context, _ time.Duration) bool {
		close(waiting)
		<-ctx.Done()
		return false
	})
	target := mouseTarget{
		Kind: "sequence", CancelOnRelease: true,
		Sequence: []mouseMacroStep{
			{Kind: "down", Keys: []string{"ctrl"}},
			{Kind: "delay", DelayMS: 1000},
			{Kind: "tap", Keys: []string{"a"}},
		},
	}
	done := executor.start(275, target)
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("sequence start blocked before its delay")
	}
	executor.release(275, target.CancelOnRelease)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sequence did not cancel on source release")
	}
	mu.Lock()
	defer mu.Unlock()
	keyEvents := []synthEvent{}
	for _, event := range events {
		if event.etype == evKey {
			keyEvents = append(keyEvents, event)
		}
	}
	want := []synthEvent{{evKey, 29, 1}, {evKey, 29, 0}}
	if !reflect.DeepEqual(keyEvents, want) {
		t.Fatalf("cancel events = %v, want ctrl down then release", keyEvents)
	}
}
