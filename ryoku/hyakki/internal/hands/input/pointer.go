package input

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Pointer moves, presses and scrolls one display's pointer.
type Pointer interface {
	// Move puts the pointer at x,y of a width by height display.
	Move(x, y, width, height int) error
	// Button holds (down) or lets go of ButtonLeft, ButtonRight or
	// ButtonMiddle where the pointer is.
	Button(code uint32, down bool) error
	// Scroll turns the wheel by whole notches, down and right positive, at
	// most MaxNotches each way.
	Scroll(dx, dy int) error
	// Close ends the connection; the compositor drops the device with it.
	Close()
}

// Linux input button codes, the only buttons a Pointer presses.
const (
	ButtonLeft   = 0x110
	ButtonRight  = 0x111
	ButtonMiddle = 0x112
)

// MaxNotches bounds one Scroll on each axis.
const MaxNotches = 100

// ErrPointer is returned for a move off the display, an unknown button or
// too long a scroll; nothing is sent.
var ErrPointer = errors.New("input: pointer action out of range")

// wire drives a pointer through wlr-virtual-pointer, spoken over the
// Wayland wire directly: one small connection held for the display's life,
// with nothing to start per click.
type wire struct {
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
	opPointerFrame          = 4
	opPointerAxisSource     = 5
	opPointerAxisDiscrete   = 7

	evDisplayError   = 0
	evRegistryGlobal = 0
	evCallbackDone   = 0

	managerInterface = "zwlr_virtual_pointer_manager_v1"
)

// DialPointer connects to the Wayland socket at socket, binds the virtual
// pointer manager and creates one pointer with no seat (the compositor's
// default).
func DialPointer(socket string) (Pointer, error) {
	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return nil, err
	}
	p := &wire{conn: conn, next: 2, started: time.Now()}
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
		return nil, errors.New("the display offers no virtual pointer")
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

func (p *wire) newID() uint32 {
	id := p.next
	p.next++
	return id
}

func (p *wire) millis() uint32 { return uint32(time.Since(p.started).Milliseconds()) }

// roundtrip sends a sync and reads until its callback is done, collecting
// the registry's globals on the way and failing on a protocol error
func (p *wire) roundtrip(registry uint32) (map[string]global, error) {
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
			return nil, errors.New("a malformed message from the display")
		}
		body := make([]byte, size-8)
		if _, err := io.ReadFull(p.conn, body); err != nil {
			return nil, err
		}
		switch {
		case object == displayID && opcode == evDisplayError:
			if len(body) >= 12 {
				message, _ := readString(body[8:])
				return nil, fmt.Errorf("the display refused input: %s", message)
			}
			return nil, errors.New("the display refused input")
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

func (p *wire) send(object, opcode uint32, args ...[]byte) {
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

func (p *wire) Move(x, y, width, height int) error {
	if width <= 0 || height <= 0 || x < 0 || y < 0 || x >= width || y >= height {
		return ErrPointer
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send(p.device, opPointerMotionAbsolute, uintArg(p.millis()), uintArg(uint32(x)), uintArg(uint32(y)), uintArg(uint32(width)), uintArg(uint32(height)))
	p.send(p.device, opPointerFrame)
	_, err := p.roundtrip(0)
	return err
}

func (p *wire) Button(code uint32, down bool) error {
	switch code {
	case ButtonLeft, ButtonRight, ButtonMiddle:
	default:
		return ErrPointer
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	state := uint32(0)
	if down {
		state = 1
	}
	p.send(p.device, opPointerButton, uintArg(p.millis()), uintArg(code), uintArg(state))
	p.send(p.device, opPointerFrame)
	_, err := p.roundtrip(0)
	return err
}

func (p *wire) Scroll(dx, dy int) error {
	if dx < -MaxNotches || dx > MaxNotches || dy < -MaxNotches || dy > MaxNotches {
		return ErrPointer
	}
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

func (p *wire) Close() {
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
