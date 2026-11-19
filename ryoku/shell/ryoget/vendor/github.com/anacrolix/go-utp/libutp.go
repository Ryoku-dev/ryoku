//go:build cgo && !purego && !purego_utp

package utp

import (
	"errors"
	"fmt"
	"net"
	"time"

	libutp "github.com/anacrolix/go-libutp"
)

// LibUtp is the C++ reference implementation, [github.com/anacrolix/go-libutp], reached through
// cgo. It only exists where libutp is being compiled: not under the purego or purego_utp tags,
// and not with cgo disabled.
var LibUtp Implementation = &implementation{"libutp", newLibutpSocket}

// Default is the implementation this binary was built with: libutp here, because the build set
// neither the purego nor the purego_utp tag, and didn't disable cgo. A program that wants to
// override the build's choice can assign to it before making any sockets.
var Default = LibUtp

// Adapts a libutp Socket to the Socket interface.
type libutpSocket struct {
	*libutp.Socket
}

var _ Socket = libutpSocket{}

// libutp's Socket panics on all three deadline methods rather than implementing them. A panic
// isn't a reasonable thing for a net.PacketConn to do to its caller, so report it as what it is.
// Deadlines on the connections a Socket hands out work normally in both implementations.
var errDeadlinesUnsupported = fmt.Errorf(
	"deadlines on a libutp Socket: %w", errors.ErrUnsupported)

func (libutpSocket) SetDeadline(time.Time) error      { return errDeadlinesUnsupported }
func (libutpSocket) SetReadDeadline(time.Time) error  { return errDeadlinesUnsupported }
func (libutpSocket) SetWriteDeadline(time.Time) error { return errDeadlinesUnsupported }

// The synchronous variant is the one libutp consults for every incoming connection, which is what
// the interface documents. The asynchronous one is only reached when no synchronous callback is
// set, so this can't just forward to SetFirewallCallback.
func (me libutpSocket) SetFirewallCallback(f FirewallCallback) {
	me.Socket.SetSyncFirewallCallback(f)
}

// libutp keeps the buffer sizes as context options, which new connections are created from.
func (me libutpSocket) SetReadBufferLen(n int) {
	me.Socket.SetOption(libutp.RecvBuffer, n)
}

func (me libutpSocket) SetLogging(normal, mtu, debug bool) {
	me.Socket.SetOption(libutp.LogNormal, boolOption(normal))
	me.Socket.SetOption(libutp.LogMtu, boolOption(mtu))
	// The vendored libutp is compiled with UTP_DEBUG_LOGGING=0, so this option is accepted and
	// then never consulted. Set it anyway, so that a build which turns it on works.
	me.Socket.SetOption(libutp.LogDebug, boolOption(debug))
}

func boolOption(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newLibutpSocket(pc net.PacketConn, opts ...Option) (Socket, error) {
	o := newOptions(opts)
	var lopts []libutp.NewSocketOpt
	if o.logger != nil {
		lopts = append(lopts, libutp.WithLogger(o.logger))
	}
	s, err := libutp.NewSocketFromPacketConn(pc, lopts...)
	if err != nil {
		return nil, err
	}
	if o.sendBuffer != 0 {
		s.SetOption(libutp.SendBuffer, o.sendBuffer)
	}
	if o.receiveBuffer != 0 {
		s.SetOption(libutp.RecvBuffer, o.receiveBuffer)
	}
	if o.targetDelay != 0 {
		s.SetOption(libutp.TargetDelay, int(o.targetDelay/time.Microsecond))
	}
	return libutpSocket{s}, nil
}
