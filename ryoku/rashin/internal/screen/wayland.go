package screen

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// pointer drives the screen's pointer through wlr-virtual-pointer, spoken
// over the Wayland wire directly: one small connection held for the screen's
// life, with nothing to start per click.
type pointer struct {
	mu      sync.Mutex
	conn    net.Conn
	next    uint32
	device  uint32
	started time.Time
}

const (
	displayID = 1

	opDisplaySync        = 0
	opDisplayGetRegistry = 1
	opRegistryBind       = 0
	opManagerCreate      = 0

	opPointerMotionAbsolute = 1
	opPointerButton         = 2
	opPointerAxis           = 3
	opPointerFrame          = 4
	opPointerAxisSource     = 5
	opPointerAxisDiscrete   = 7

	evDisplayError   = 0
	evRegistryGlobal = 0
	evCallbackDone   = 0

	managerInterface = "zwlr_virtual_pointer_manager_v1"
)

// Linux input button codes.
const (
	buttonLeft   = 0x110
	buttonRight  = 0x111
	buttonMiddle = 0x112
)

func dialPointer(socket string) (*pointer, error) {
	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return nil, err
	}
	p := &pointer{conn: conn, next: 2, started: time.Now()}
	registry := p.newID()
	p.send(displayID, opDisplayGetRegistry, uintArg(registry))
	globals, err := p.roundtrip(registry)
	if err != nil {
		conn.Close()
		return nil, err
	}
	global, ok := globals[managerInterface]
	if !ok {
		conn.Close()
		return nil, errors.New("the screen offers no virtual pointer")
	}
	manager := p.newID()
	p.send(registry, opRegistryBind, uintArg(global.name), stringArg(managerInterface), uintArg(1), uintArg(manager))
	p.device = p.newID()
	p.send(manager, opManagerCreate, uintArg(0), uintArg(p.device))
	if _, err := p.roundtrip(0); err != nil {
		conn.Close()
		return nil, err
	}
	return p, nil
}

type global struct{ name, version uint32 }

func (p *pointer) newID() uint32 {
	id := p.next
	p.next++
	return id
}

func (p *pointer) millis() uint32 { return uint32(time.Since(p.started).Milliseconds()) }

// roundtrip sends a sync and reads until its callback is done, collecting
// the registry's globals on the way and failing on a protocol error
func (p *pointer) roundtrip(registry uint32) (map[string]global, error) {
	callback := p.newID()
	p.send(displayID, opDisplaySync, uintArg(callback))
	globals := map[string]global{}
	_ = p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	defer func() { _ = p.conn.SetReadDeadline(time.Time{}) }()
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(p.conn, header); err != nil {
			return nil, err
		}
		object := binary.LittleEndian.Uint32(header)
		word := binary.LittleEndian.Uint32(header[4:])
		size, opcode := int(word>>16), word&0xffff
		if size < 8 {
			return nil, errors.New("a malformed message from the screen")
		}
		body := make([]byte, size-8)
		if _, err := io.ReadFull(p.conn, body); err != nil {
			return nil, err
		}
		switch {
		case object == displayID && opcode == evDisplayError:
			if len(body) >= 12 {
				message, _ := readString(body[8:])
				return nil, fmt.Errorf("the screen refused input: %s", message)
			}
			return nil, errors.New("the screen refused input")
		case object == registry && registry != 0 && opcode == evRegistryGlobal && len(body) >= 8:
			name := binary.LittleEndian.Uint32(body)
			iface, rest := readString(body[4:])
			if len(rest) >= 4 {
				globals[iface] = global{name: name, version: binary.LittleEndian.Uint32(rest)}
			}
		case object == callback && opcode == evCallbackDone:
			return globals, nil
		}
	}
}

func (p *pointer) send(object, opcode uint32, args ...[]byte) {
	size := 8
	for _, arg := range args {
		size += len(arg)
	}
	message := make([]byte, 8, size)
	binary.LittleEndian.PutUint32(message, object)
	binary.LittleEndian.PutUint32(message[4:], uint32(size)<<16|opcode)
	for _, arg := range args {
		message = append(message, arg...)
	}
	_, _ = p.conn.Write(message)
}

// move puts the pointer at x,y of a width by height screen
func (p *pointer) move(x, y, width, height int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send(p.device, opPointerMotionAbsolute, uintArg(p.millis()), uintArg(uint32(x)), uintArg(uint32(y)), uintArg(uint32(width)), uintArg(uint32(height)))
	p.send(p.device, opPointerFrame)
	_, err := p.roundtrip(0)
	return err
}

// press holds or lets go of a button where the pointer is
func (p *pointer) press(button uint32, down bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := uint32(0)
	if down {
		state = 1
	}
	p.send(p.device, opPointerButton, uintArg(p.millis()), uintArg(button), uintArg(state))
	p.send(p.device, opPointerFrame)
	_, err := p.roundtrip(0)
	return err
}

// scroll turns the wheel by whole notches, down and right positive
func (p *pointer) scroll(dx, dy int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for axis, notches := range [2]int{dy, dx} {
		if notches == 0 {
			continue
		}
		p.send(p.device, opPointerAxisSource, uintArg(0))
		p.send(p.device, opPointerAxisDiscrete, uintArg(p.millis()), uintArg(uint32(axis)), fixedArg(float64(notches)*15), intArg(int32(notches)))
		p.send(p.device, opPointerFrame)
	}
	_, err := p.roundtrip(0)
	return err
}

func (p *pointer) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.conn.Close()
}

func uintArg(value uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, value)
}

func intArg(value int32) []byte {
	return binary.LittleEndian.AppendUint32(nil, uint32(value))
}

func fixedArg(value float64) []byte {
	return intArg(int32(value * 256))
}

func stringArg(value string) []byte {
	length := len(value) + 1
	out := binary.LittleEndian.AppendUint32(nil, uint32(length))
	out = append(out, value...)
	out = append(out, 0)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

func readString(body []byte) (string, []byte) {
	if len(body) < 4 {
		return "", nil
	}
	length := int(binary.LittleEndian.Uint32(body))
	padded := (length + 3) &^ 3
	if len(body) < 4+padded || length == 0 {
		return "", nil
	}
	return string(body[4 : 4+length-1]), body[4+padded:]
}
