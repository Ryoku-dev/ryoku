package input

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type message struct {
	object, opcode uint32
	body           []byte
}

func (m message) word(i int) uint32 { return binary.LittleEndian.Uint32(m.body[4*i:]) }

// fakeDisplay is a Wayland server on a private socket that answers the
// registry and syncs and records every request. offer says whether it
// advertises the pointer manager; refuse makes it send a protocol error to
// the first sync after the device exists.
type fakeDisplay struct {
	socket string
	offer  bool
	refuse bool

	mu       sync.Mutex
	requests []message
	done     chan struct{}
}

func serveDisplay(t *testing.T, offer, refuse bool) *fakeDisplay {
	t.Helper()
	dir, err := os.MkdirTemp("", "ptr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	d := &fakeDisplay{socket: filepath.Join(dir, "wayland-9"), offer: offer, refuse: refuse, done: make(chan struct{})}
	l, err := net.Listen("unix", d.socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		defer close(d.done)
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		d.serve(conn)
	}()
	return d
}

func (d *fakeDisplay) serve(conn net.Conn) {
	var registry uint32
	created := false
	reply := func(object, opcode uint32, args ...[]byte) {
		size := 8
		for _, a := range args {
			size += len(a)
		}
		out := binary.LittleEndian.AppendUint32(nil, object)
		out = binary.LittleEndian.AppendUint32(out, uint32(size)<<16|opcode)
		for _, a := range args {
			out = append(out, a...)
		}
		_, _ = conn.Write(out)
	}
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		m := message{object: binary.LittleEndian.Uint32(header), opcode: binary.LittleEndian.Uint32(header[4:]) & 0xffff}
		m.body = make([]byte, int(binary.LittleEndian.Uint32(header[4:])>>16)-8)
		if _, err := io.ReadFull(conn, m.body); err != nil {
			return
		}
		d.mu.Lock()
		d.requests = append(d.requests, m)
		d.mu.Unlock()
		switch {
		case m.object == displayID && m.opcode == opDisplayGetRegistry:
			registry = m.word(0)
		case m.object == displayID && m.opcode == opDisplaySync:
			if registry != 0 {
				reply(registry, evRegistryGlobal, uintArg(7), stringArg("wl_seat"), uintArg(7))
				if d.offer {
					reply(registry, evRegistryGlobal, uintArg(9), stringArg(managerInterface), uintArg(2))
				}
				registry = 0
			}
			if created && d.refuse {
				reply(displayID, evDisplayError, uintArg(5), uintArg(0), stringArg("no motion here"))
				continue
			}
			reply(m.word(0), evCallbackDone, uintArg(1))
		case m.opcode == opManagerCreate && m.object != displayID && len(m.body) == 8:
			created = true
		}
	}
}

// pointerRequests is every request after the handshake, without syncs.
func (d *fakeDisplay) pointerRequests(device uint32) []message {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []message
	for _, m := range d.requests {
		if m.object == device {
			out = append(out, m)
		}
	}
	return out
}

func dial(t *testing.T, d *fakeDisplay) *wire {
	t.Helper()
	p, err := DialPointer(d.socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p.(*wire)
}

func TestPointerBindsTheManagerAndCreatesOneDevice(t *testing.T) {
	d := serveDisplay(t, true, false)
	p := dial(t, d)
	d.mu.Lock()
	defer d.mu.Unlock()
	var bind, create *message
	for i, m := range d.requests {
		switch {
		case m.object != displayID && m.opcode == opRegistryBind && len(m.body) > 8:
			bind = &d.requests[i]
		case m.object != displayID && m.opcode == opManagerCreate && len(m.body) == 8:
			create = &d.requests[i]
		}
	}
	if bind == nil || create == nil {
		t.Fatalf("requests = %+v", d.requests)
	}
	iface, rest := readString(bind.body[4:])
	if bind.word(0) != 9 || iface != managerInterface || binary.LittleEndian.Uint32(rest) != 1 {
		t.Fatalf("bind = %q %x", iface, bind.body)
	}
	if create.word(0) != 0 || create.word(1) != p.device {
		t.Fatalf("create = %x, device %d", create.body, p.device)
	}
}

func TestPointerMovesPressesAndScrollsOnTheWire(t *testing.T) {
	d := serveDisplay(t, true, false)
	p := dial(t, d)
	if err := p.Move(640, 400, 1280, 800); err != nil {
		t.Fatal(err)
	}
	if err := p.Button(ButtonRight, true); err != nil {
		t.Fatal(err)
	}
	if err := p.Button(ButtonRight, false); err != nil {
		t.Fatal(err)
	}
	if err := p.Scroll(-2, 3); err != nil {
		t.Fatal(err)
	}
	got := d.pointerRequests(p.device)
	var ops []string
	for _, m := range got {
		ops = append(ops, map[uint32]string{1: "motion", 2: "button", 4: "frame", 5: "source", 7: "discrete"}[m.opcode])
	}
	if strings.Join(ops, ",") != "motion,frame,button,frame,button,frame,source,discrete,frame,source,discrete,frame" {
		t.Fatalf("ops = %v", ops)
	}
	if m := got[0]; m.word(1) != 640 || m.word(2) != 400 || m.word(3) != 1280 || m.word(4) != 800 {
		t.Errorf("motion = %x", m.body)
	}
	if m := got[2]; m.word(1) != ButtonRight || m.word(2) != 1 {
		t.Errorf("press = %x", m.body)
	}
	if m := got[4]; m.word(1) != ButtonRight || m.word(2) != 0 {
		t.Errorf("release = %x", m.body)
	}
	// vertical first: axis 0 by 3 notches, then horizontal: axis 1 by -2
	if m := got[7]; m.word(1) != 0 || int32(m.word(2)) != 3*15*256 || int32(m.word(3)) != 3 {
		t.Errorf("vertical = %x", m.body)
	}
	if m := got[10]; m.word(1) != 1 || int32(m.word(2)) != -2*15*256 || int32(m.word(3)) != -2 {
		t.Errorf("horizontal = %x", m.body)
	}
}

func TestPointerRefusesOutOfRangeWithoutSending(t *testing.T) {
	d := serveDisplay(t, true, false)
	p := dial(t, d)
	for name, err := range map[string]error{
		"off the right": p.Move(1280, 0, 1280, 800),
		"negative":      p.Move(-1, 5, 1280, 800),
		"no display":    p.Move(0, 0, 0, 0),
		"side button":   p.Button(0x113, true),
		"key code":      p.Button(30, true),
		"long scroll":   p.Scroll(0, MaxNotches+1),
		"long sideways": p.Scroll(-MaxNotches-1, 0),
	} {
		if !errors.Is(err, ErrPointer) {
			t.Errorf("%s: %v; want ErrPointer", name, err)
		}
	}
	if got := d.pointerRequests(p.device); len(got) != 0 {
		t.Fatalf("sent %d requests", len(got))
	}
}

func TestPointerNeedsTheManager(t *testing.T) {
	d := serveDisplay(t, false, false)
	if p, err := DialPointer(d.socket); err == nil {
		p.Close()
		t.Fatal("dialed a display with no virtual pointer")
	}
	select {
	case <-d.done:
	case <-time.After(3 * time.Second):
		t.Fatal("the connection stayed open")
	}
}

func TestPointerReportsAProtocolError(t *testing.T) {
	d := serveDisplay(t, true, true)
	_, err := DialPointer(d.socket)
	if err == nil || !strings.Contains(err.Error(), "no motion here") {
		t.Fatalf("err = %v", err)
	}
}
