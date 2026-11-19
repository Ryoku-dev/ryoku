package purego

import (
	"net"
	"net/netip"
	"time"
)

// uTP timestamps only have to be consistent within a connection, and the wire format truncates
// them to 32 bits, so a process-wide monotonic origin is all that's needed. Using the monotonic
// clock also means a wall clock adjustment can't be mistaken for queuing delay.
var timeOrigin = time.Now()

func nowMicros() uint64 {
	return uint64(time.Since(timeOrigin).Microseconds())
}

func nowMillis() uint64 {
	return uint64(time.Since(timeOrigin).Milliseconds())
}

// The comparable part of a connection's identity that comes from its peer address. Addresses that
// denote the same peer have to compare equal, so IP addresses are normalized rather than compared
// as strings.
type connAddrKey struct {
	ap  netip.AddrPort
	str string
}

func connKey(a net.Addr) connAddrKey {
	if ua, ok := a.(*net.UDPAddr); ok {
		if addr, ok := netip.AddrFromSlice(ua.IP); ok {
			return connAddrKey{ap: netip.AddrPortFrom(addr.Unmap().WithZone(ua.Zone), uint16(ua.Port))}
		}
	}
	return connAddrKey{str: a.String()}
}

// A connection is identified by the peer address and the connection ID it sends to us.
type socketKey struct {
	addr   connAddrKey
	recvID uint16
}

func addrIsIPv6(a net.Addr) bool {
	ua, ok := a.(*net.UDPAddr)
	if !ok {
		return false
	}
	return ua.IP != nil && ua.IP.To4() == nil
}

type deadlineExceededError struct{}

var errDeadlineExceeded net.Error = deadlineExceededError{}

func (deadlineExceededError) Error() string   { return "i/o timeout" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }

// Buffers received payload until the application reads it. Payload arrives in packet sized pieces
// and is usually read in larger ones, so it's kept as a list of chunks rather than being copied
// into a single growing buffer.
type readBuffer struct {
	chunks [][]byte
	n      int
}

func (b *readBuffer) len() int { return b.n }

// Copies p, which aliases a receive buffer that's about to be reused.
func (b *readBuffer) write(p []byte) {
	b.writeOwned(append([]byte(nil), p...))
}

// Takes ownership of p.
func (b *readBuffer) writeOwned(p []byte) {
	if len(p) == 0 {
		return
	}
	b.chunks = append(b.chunks, p)
	b.n += len(p)
}

func (b *readBuffer) read(p []byte) (n int) {
	for len(p) > 0 && len(b.chunks) > 0 {
		c := b.chunks[0]
		n1 := copy(p, c)
		p = p[n1:]
		n += n1
		b.n -= n1
		if n1 == len(c) {
			b.chunks[0] = nil
			b.chunks = b.chunks[1:]
		} else {
			b.chunks[0] = c[n1:]
		}
	}
	return
}
