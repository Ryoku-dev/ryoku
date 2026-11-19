// Package utp provides one interface over both µTP implementations for Go, and picks one of them
// at build time.
//
// By default it uses libutp, the C++ reference implementation, through cgo and
// [github.com/anacrolix/go-libutp]. Either of two build tags, or cgo being disabled, uses the
// pure Go implementation in [github.com/anacrolix/go-utp/purego] instead:
//
//	go build -tags purego ./...       // the package this tag selects shares its name
//	go build -tags purego_utp ./...   // the same choice, for this module alone
//	CGO_ENABLED=0 go build ./...
//
// purego is the tag a build sets to say it wants no cgo anywhere, and plenty of modules answer
// to it. purego_utp says it about µTP and nothing else, for a build that wants the pure
// implementation here without making that choice for everything it depends on, or that wants to
// keep cgo out of µTP while leaving it on elsewhere.
//
// [Default] is whichever one the build selected. [NewSocket] and [NewSocketFromPacketConn] use
// it and have the same signatures as the constructors of those names in both underlying packages,
// so switching a caller over to this package is an import change. Code that wants a particular
// implementation rather than the default can name [PureGo] or [LibUtp] directly; LibUtp only exists
// where libutp is being compiled, which is to say under neither tag and not with cgo disabled.
//
// Implementations are values, so code that has to work with either can take an [Implementation]
// and be handed one, rather than choosing at its own build time. An Implementation makes a Socket
// over a PacketConn; [Listen] opens one for it.
package utp

import (
	"context"
	"log/slog"
	"net"
	"time"
)

// An Implementation makes Sockets over a PacketConn. The two in this module are [PureGo] and
// [LibUtp], and [Default] is whichever the build selected.
//
// Owning a port is not part of it: use [Listen] to have one opened for an implementation. Both
// values here also print as their name, so fmt.Sprint(utp.Default) is "libutp" or "purego".
type Implementation interface {
	// NewSocket runs µTP over a PacketConn. The Socket takes ownership of it: closing the Socket
	// closes the PacketConn.
	NewSocket(pc net.PacketConn, opts ...Option) (Socket, error)
}

// A FirewallCallback reports whether an incoming connection should be ignored. Rejecting one this
// way is better than accepting and immediately closing it, because the peer sees no response at
// all rather than an acknowledgement followed by a reset.
//
// It is called while the Socket holds its own lock, so it must not block or call back into the
// Socket.
//
// It's an alias rather than a defined type, so that an implementation's own callback type is the
// same type and no conversion is needed either way.
type FirewallCallback = func(net.Addr) bool

// Socket is a µTP endpoint: it owns a packet conn and multiplexes connections over it.
//
// It is a [net.Listener], so Accept hands out incoming connections, and a [net.PacketConn], whose
// ReadFrom and WriteTo carry the datagrams on the port that aren't µTP. That's what lets one port
// serve µTP and another protocol, a DHT say, at the same time. Datagrams that aren't read
// promptly are dropped.
//
// The connections Accept and Dial return are ordinary [net.Conn]s, deadlines included. Deadlines
// on the Socket itself, which only affect ReadFrom and WriteTo, are not supported by libutp: it
// returns an error wrapping [errors.ErrUnsupported] from the three deadline methods.
type Socket interface {
	net.Listener
	net.PacketConn

	// Dial connects to addr on the Socket's own network.
	Dial(addr string) (net.Conn, error)
	// DialTimeout connects to addr, giving up after timeout. A zero timeout means no limit.
	DialTimeout(addr string, timeout time.Duration) (net.Conn, error)
	// DialContext connects to addr. An empty network means the Socket's own.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)

	// SetFirewallCallback sets the function consulted before each incoming connection is
	// accepted. Pass nil to accept everything, which is the default.
	SetFirewallCallback(FirewallCallback)

	// ReadBufferLen and WriteBufferLen report the buffer sizes new connections are given. The
	// write buffer also caps the congestion window; the read buffer is what the receive window
	// advertised to peers is computed from. Connections already open keep the sizes they were
	// created with.
	ReadBufferLen() int
	WriteBufferLen() int
	SetReadBufferLen(int)
	SetWriteBufferLen(int)

	// SetLogging turns the protocol's own log categories on and off. They go to the Socket's
	// logger at debug level, so it has to be passing debug records for them to appear. All are
	// off by default, and they're noisy: normal covers connection lifecycle and packet loss, mtu
	// the path MTU search, and debug every packet. libutp compiles its debug logging out, so
	// debug does nothing there.
	SetLogging(normal, mtu, debug bool)
}

// The options both implementations have in common. Anything left zero is left at the
// implementation's own default.
type options struct {
	logger        *slog.Logger
	sendBuffer    int
	receiveBuffer int
	targetDelay   time.Duration
}

// An Option configures a Socket as it's created.
type Option func(*options)

// WithLogger gives a Socket its own logger, instead of the implementation's package level one:
// [github.com/anacrolix/go-libutp.Logger] or [github.com/anacrolix/go-utp/purego.Logger],
// either of which is slog's default until it's set. This is the logger the categories in
// [Socket.SetLogging] write to, along with anything either implementation has to report about the
// socket itself.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// WithBufferSizes sets the send and receive buffers new connections are given, in bytes. Zero
// leaves either at the default, which is a megabyte.
func WithBufferSizes(send, receive int) Option {
	return func(o *options) {
		o.sendBuffer = send
		o.receiveBuffer = receive
	}
}

// WithTargetDelay sets the one way queuing delay the congestion controller aims for. Lower means
// yielding sooner to whatever else is sharing the link. Zero leaves it at the default of 100ms,
// which is what the rest of the µTP network uses.
func WithTargetDelay(d time.Duration) Option {
	return func(o *options) { o.targetDelay = d }
}

func newOptions(opts []Option) (o options) {
	for _, opt := range opts {
		opt(&o)
	}
	return
}

// The two implementations in this module, which differ only in the constructor they wrap.
//
// Used through a pointer so that Implementation values are comparable: Default == LibUtp is a
// reasonable thing to write, and a struct with a func field in it would panic.
type implementation struct {
	name      string
	newSocket func(pc net.PacketConn, opts ...Option) (Socket, error)
}

func (me *implementation) String() string { return me.name }

func (me *implementation) NewSocket(pc net.PacketConn, opts ...Option) (Socket, error) {
	return me.newSocket(pc, opts...)
}

// Listen opens a port for impl and returns a Socket over it. Only UDP networks are supported; for
// anything else, listen yourself and pass the PacketConn to [Implementation.NewSocket].
//
// The Socket owns the port: closing the Socket closes it.
func Listen(impl Implementation, network, addr string, opts ...Option) (Socket, error) {
	pc, err := net.ListenPacket(network, addr)
	if err != nil {
		return nil, err
	}
	s, err := impl.NewSocket(pc, opts...)
	if err != nil {
		pc.Close()
		return nil, err
	}
	return s, nil
}

// NewSocket listens on the given network and address using [Default]. It is [Listen] with the
// implementation the build selected, and has the same signature as the constructor of the same
// name in both underlying packages.
func NewSocket(network, addr string, opts ...Option) (Socket, error) {
	return Listen(Default, network, addr, opts...)
}

// NewSocketFromPacketConn runs µTP over a PacketConn you already have, using [Default]. The
// Socket takes ownership of it: closing the Socket closes the PacketConn.
func NewSocketFromPacketConn(pc net.PacketConn, opts ...Option) (Socket, error) {
	return Default.NewSocket(pc, opts...)
}
