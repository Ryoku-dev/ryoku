package main

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func ownerChanged(name, owner string) *dbus.Signal {
	return &dbus.Signal{Name: "org.freedesktop.DBus.NameOwnerChanged", Body: []any{name, "", owner}}
}

func adapterAdded(path string) *dbus.Signal {
	return &dbus.Signal{Name: objectManagerIface + ".InterfacesAdded", Body: []any{
		dbus.ObjectPath(path), map[string]map[string]dbus.Variant{bluezAdapterIface: {"Powered": dbus.MakeVariant(true)}},
	}}
}

func deviceAdded(path string) *dbus.Signal {
	return &dbus.Signal{Name: objectManagerIface + ".InterfacesAdded", Body: []any{
		dbus.ObjectPath(path), map[string]map[string]dbus.Variant{"org.bluez.Device1": {"RSSI": dbus.MakeVariant(int16(-60))}},
	}}
}

func adapterProps(changed map[string]dbus.Variant, invalidated ...string) *dbus.Signal {
	return &dbus.Signal{Name: propertiesInterface + ".PropertiesChanged", Body: []any{bluezAdapterIface, changed, invalidated}}
}

func TestClassifyBluezSignal(t *testing.T) {
	cases := []struct {
		name string
		sig  *dbus.Signal
		want bluezEvent
	}{
		{"bluetoothd exits", ownerChanged(bluezBusName, ""), bluezGone},
		{"bluetoothd starts", ownerChanged(bluezBusName, ":1.42"), bluezRefresh},
		{"another service changes owner", ownerChanged("org.freedesktop.NetworkManager", ""), bluezIgnore},
		{"adapter appears", adapterAdded("/org/bluez/hci0"), bluezRefresh},
		{"scan finds a device", deviceAdded("/org/bluez/hci0/dev_AA"), bluezIgnore},
		{"adapter removed", &dbus.Signal{Name: objectManagerIface + ".InterfacesRemoved",
			Body: []any{dbus.ObjectPath("/org/bluez/hci0"), []string{bluezAdapterIface}}}, bluezRefresh},
		{"device removed", &dbus.Signal{Name: objectManagerIface + ".InterfacesRemoved",
			Body: []any{dbus.ObjectPath("/org/bluez/hci0/dev_AA"), []string{"org.bluez.Device1"}}}, bluezIgnore},
		{"power changes", adapterProps(map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)}), bluezRefresh},
		{"name invalidated", adapterProps(map[string]dbus.Variant{}, "Alias"), bluezRefresh},
		{"scan starts", adapterProps(map[string]dbus.Variant{"Discovering": dbus.MakeVariant(true)}), bluezIgnore},
		{"device property", &dbus.Signal{Name: propertiesInterface + ".PropertiesChanged",
			Body: []any{"org.bluez.Device1", map[string]dbus.Variant{"Powered": dbus.MakeVariant(true)}, []string{}}}, bluezIgnore},
		{"nil", nil, bluezIgnore},
	}
	for _, c := range cases {
		if got := classifyBluezSignal(c.sig); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

type bluezHarness struct {
	sigs    chan *dbus.Signal
	sub     *topicSubscriber
	fetches atomic.Int32
	objs    atomic.Value // bluezObjects
}

func newBluezHarness(t *testing.T, objs bluezObjects) *bluezHarness {
	t.Helper()
	h := &bluezHarness{sigs: make(chan *dbus.Signal, 64)}
	h.objs.Store(objs)
	topic := newStateTopic()
	h.sub = topic.subscribe()
	b := &bluezState{
		topic: topic,
		fetch: func() (bluezObjects, error) {
			h.fetches.Add(1)
			objs := h.objs.Load().(bluezObjects)
			if objs == nil {
				return nil, errors.New("org.freedesktop.DBus.Error.ServiceUnknown")
			}
			return objs, nil
		},
		settle: 20 * time.Millisecond,
	}
	go b.run(h.sigs)
	t.Cleanup(func() { close(h.sigs) })
	return h
}

func (h *bluezHarness) frame(t *testing.T) bluezFrame {
	t.Helper()
	select {
	case raw := <-h.sub.frames:
		var f bluezFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("bad frame %q: %v", raw, err)
		}
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("no frame published")
	}
	return bluezFrame{}
}

func (h *bluezHarness) noFrame(t *testing.T) {
	t.Helper()
	select {
	case raw := <-h.sub.frames:
		t.Fatalf("unexpected frame %s", raw)
	case <-time.After(120 * time.Millisecond):
	}
}

func oneAdapter(powered bool) bluezObjects {
	return bluezObjects{
		"/org/bluez/hci0": {bluezAdapterIface: {
			"Alias": dbus.MakeVariant("laptop"), "Address": dbus.MakeVariant("AA:BB:CC:DD:EE:FF"),
			"Powered": dbus.MakeVariant(powered),
		}},
		"/org/bluez/hci0/dev_11": {"org.bluez.Device1": {"Name": dbus.MakeVariant("buds")}},
	}
}

func TestBluezFirstFrameListsAdaptersOnly(t *testing.T) {
	h := newBluezHarness(t, oneAdapter(true))
	f := h.frame(t)
	if !f.Running || len(f.Adapters) != 1 {
		t.Fatalf("frame = %+v, want running with one adapter", f)
	}
	a := f.Adapters[0]
	if a.Path != "/org/bluez/hci0" || a.Name != "laptop" || a.Address != "AA:BB:CC:DD:EE:FF" || !a.Powered {
		t.Fatalf("adapter = %+v", a)
	}
}

func TestBluezAbsentAtStartReadsNotRunning(t *testing.T) {
	h := newBluezHarness(t, nil)
	select {
	case raw := <-h.sub.frames:
		if string(raw) != `{"running":false,"adapters":[]}` {
			t.Fatalf("frame = %s", raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame")
	}
}

func TestBluezBurstOfAdapterSignalsReadsOnce(t *testing.T) {
	h := newBluezHarness(t, oneAdapter(true))
	h.frame(t)
	h.objs.Store(oneAdapter(false))
	for range 5 {
		h.sigs <- adapterProps(map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)})
	}
	if f := h.frame(t); f.Adapters[0].Powered {
		t.Fatalf("frame after power-off still powered: %+v", f)
	}
	if n := h.fetches.Load(); n != 2 {
		t.Fatalf("fetches = %d, want the first read plus one for the burst", n)
	}
}

func TestBluezScanTrafficNeverReadsTheBus(t *testing.T) {
	h := newBluezHarness(t, oneAdapter(true))
	h.frame(t)
	for range 50 {
		h.sigs <- deviceAdded("/org/bluez/hci0/dev_XX")
		h.sigs <- adapterProps(map[string]dbus.Variant{"Discovering": dbus.MakeVariant(true)})
	}
	h.noFrame(t)
	if n := h.fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, want only the first read", n)
	}
}

func TestBluezExitPublishesNotRunningWithoutABusCall(t *testing.T) {
	h := newBluezHarness(t, oneAdapter(true))
	h.frame(t)
	h.sigs <- ownerChanged(bluezBusName, "")
	if f := h.frame(t); f.Running || len(f.Adapters) != 0 {
		t.Fatalf("frame after exit = %+v", f)
	}
	if n := h.fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, an exit must not query the bus", n)
	}
}

func TestBluezRestartEndsOnAFreshRead(t *testing.T) {
	h := newBluezHarness(t, oneAdapter(true))
	h.frame(t)
	h.objs.Store(oneAdapter(false))
	h.sigs <- ownerChanged(bluezBusName, "")
	h.sigs <- ownerChanged(bluezBusName, ":1.99")
	f := h.frame(t)
	if !f.Running || len(f.Adapters) != 1 || f.Adapters[0].Powered {
		t.Fatalf("frame after restart = %+v", f)
	}
}

func TestBluezStartedLaterIsSeen(t *testing.T) {
	h := newBluezHarness(t, nil)
	if f := h.frame(t); f.Running {
		t.Fatalf("first frame with bluetoothd absent = %+v", f)
	}
	h.objs.Store(oneAdapter(true))
	h.sigs <- ownerChanged(bluezBusName, ":1.7")
	h.sigs <- adapterAdded("/org/bluez/hci0")
	f := h.frame(t)
	if !f.Running || len(f.Adapters) != 1 || !f.Adapters[0].Powered {
		t.Fatalf("frame after bluetoothd started = %+v", f)
	}
}
