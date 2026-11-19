package purego

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

const (
	// How often every connection's timers are checked.
	timeoutCheckInterval = 500 * time.Millisecond
	// How long acks are held back after a packet arrives, so that a burst produces one ack rather
	// than one per packet.
	deferredAckDelay = 1000 * time.Microsecond
	// How long a reset we sent to an unknown connection is remembered, and how many are
	// remembered at once. Both bound how much a peer can make us send by replaying packets.
	rstInfoTimeout = 10 * time.Second
	rstInfoLimit   = 1000
	// Non-uTP datagrams buffered for ReadFrom before they start being dropped.
	nonUtpReadBufferPackets = 100
	// The largest datagram we'll receive. IPv4 UDP tops out around 64KiB, and Windows fails reads
	// outright if the buffer is smaller than the datagram.
	maxDatagramSize = 0x10000
)

type logLevel int

const (
	logNormal logLevel = iota
	logMTU
	logDebug
	numLogLevels
)

// A reset we've already sent for an unrecognized connection, so that a replayed packet doesn't
// make us send another.
type rstInfo struct {
	addr      connAddrKey
	connID    uint16
	ackNr     uint16
	timestamp uint64
}

// A firewall callback returns true if an incoming connection should be ignored. This is better
// than accepting and immediately closing, because the peer sees no response at all rather than an
// acknowledgement followed by a reset.
//
// It's an alias rather than a defined type so that this Socket satisfies interfaces declaring the
// callback in their own terms, such as [github.com/anacrolix/go-utp.Socket], without a
// wrapper to convert between two identical function types.
type FirewallCallback = func(net.Addr) bool

// Socket multiplexes uTP connections over a net.PacketConn. It implements net.Listener and
// net.PacketConn: uTP traffic is dispatched to connections, and everything else is handed to
// ReadFrom, so a single port can carry uTP and another protocol at once.
type Socket struct {
	mu sync.Mutex
	pc net.PacketConn

	conns   map[socketKey]*Conn
	backlog chan *Conn

	// Datagrams that weren't uTP, waiting for ReadFrom.
	nonUtpReads     []nonUtpPacket
	nonUtpReadCond  sync.Cond
	readDeadline    time.Time
	writeDeadline   time.Time
	readDeadlineTmr *time.Timer

	closed bool

	// Cached from the last time anything asked, the way libutp's context does it, so that a batch
	// of work sees one consistent time.
	currentMS uint64

	targetDelay int
	optSndBuf   int
	optRcvBuf   int

	// Connections with an ack owed, to be flushed once the current read batch is done.
	ackConns []*Conn
	ackTimer *time.Timer
	// Whether ackTimer is armed.
	acksScheduled bool

	timeoutTimer *time.Timer

	rstInfo []rstInfo

	firewallCallback FirewallCallback
	resolveAddr      AddrResolver

	logger    *slog.Logger
	logLevels [numLogLevels]bool
}

var (
	_ net.PacketConn = (*Socket)(nil)
	_ net.Listener   = (*Socket)(nil)

	// ErrSocketClosed is returned by Socket methods after Close.
	ErrSocketClosed = errors.New("socket closed")
)

type nonUtpPacket struct {
	b    []byte
	from net.Addr
}

// NewSocketOpt configures a Socket at construction.
type NewSocketOpt func(s *Socket)

// WithLogger gives a Socket its own logger instead of the package level [Logger].
func WithLogger(l *slog.Logger) NewSocketOpt {
	return func(s *Socket) { s.logger = l }
}

// WithBufferSizes sets the initial send and receive buffer sizes for connections on this Socket.
// The send buffer caps the congestion window; the receive buffer is what the advertised receive
// window is computed from.
func WithBufferSizes(send, receive int) NewSocketOpt {
	return func(s *Socket) {
		s.optSndBuf = send
		s.optRcvBuf = receive
	}
}

// WithTargetDelay sets the one way queuing delay the congestion controller aims for on
// connections from this Socket. The default is 100ms, as in libutp.
func WithTargetDelay(d time.Duration) NewSocketOpt {
	return func(s *Socket) { s.targetDelay = int(d / time.Microsecond) }
}

// WithAddrResolver sets how the address strings passed to Dial are resolved. The default is
// net.ResolveUDPAddr, which is what a Socket over a UDP PacketConn wants. Pass this when the
// PacketConn given to NewSocketFromPacketConn uses addresses of some other type.
func WithAddrResolver(r AddrResolver) NewSocketOpt {
	return func(s *Socket) { s.resolveAddr = r }
}

// NewSocket listens on the given network and address and returns a Socket carrying uTP over it.
func NewSocket(network, addr string, opts ...NewSocketOpt) (*Socket, error) {
	pc, err := net.ListenPacket(network, addr)
	if err != nil {
		return nil, err
	}
	return NewSocketFromPacketConn(pc, opts...)
}

// AddrResolver turns the network and address strings given to Dial into an address the underlying
// PacketConn will accept.
type AddrResolver func(network, addr string) (net.Addr, error)

// NewSocketFromPacketConn runs uTP over a net.PacketConn you already have. The Socket takes
// ownership of it: closing the Socket closes the PacketConn.
func NewSocketFromPacketConn(pc net.PacketConn, opts ...NewSocketOpt) (*Socket, error) {
	s := &Socket{
		pc:          pc,
		conns:       make(map[socketKey]*Conn),
		backlog:     make(chan *Conn, 5),
		targetDelay: defaultTargetDelay,
		optSndBuf:   defaultBufferSize,
		optRcvBuf:   defaultBufferSize,
		resolveAddr: func(network, addr string) (net.Addr, error) { return net.ResolveUDPAddr(network, addr) },
	}
	s.nonUtpReadCond.L = &s.mu
	for _, opt := range opts {
		opt(s)
	}
	if s.logger == nil {
		s.logger = defaultLogger()
	}
	s.currentMS = nowMillis()
	s.ackTimer = time.AfterFunc(math.MaxInt64, s.ackTimerFunc)
	s.ackTimer.Stop()
	s.readDeadlineTmr = time.AfterFunc(math.MaxInt64, s.broadcastReaders)
	s.readDeadlineTmr.Stop()
	s.timeoutTimer = time.AfterFunc(timeoutCheckInterval, s.timeoutTimerFunc)
	go s.reader()
	return s, nil
}

func (s *Socket) broadcastReaders() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonUtpReadCond.Broadcast()
}

// Logs one of the categories [Socket.SetLogging] turns on. The messages are formatted the way
// libutp formats its own, so that logs from the two implementations can be read side by side.
func (s *Socket) logf(level logLevel, format string, args ...any) {
	if !s.logLevels[level] {
		return
	}
	s.logger.Debug(fmt.Sprintf(format, args...), "category", level)
}

// SetLogging enables or disables the log categories libutp distinguishes: normal covers the
// connection lifecycle and packet loss, mtu the path MTU search, and debug every packet. All are
// off by default. They're logged at debug level, so the Socket's logger has to be passing debug
// records for them to appear.
func (s *Socket) SetLogging(normal, mtu, debug bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLevels[logNormal] = normal
	s.logLevels[logMTU] = mtu
	s.logLevels[logDebug] = debug
}

// SetFirewallCallback sets a function consulted before each incoming connection is accepted. It
// is called with the Socket's lock held, so it must not call back into this package or block.
func (s *Socket) SetFirewallCallback(f FirewallCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firewallCallback = f
}

func (s *Socket) updateCurrentMS() {
	s.currentMS = nowMillis()
}

// Writes a datagram. dontFragment asks for the packet not to be fragmented, which MTU probes want;
// it isn't honoured yet, so a probe that's too large is dropped somewhere along the path instead
// of reported back, which the discovery search handles either way.
func (s *Socket) send(b []byte, addr net.Addr, dontFragment bool) {
	_, err := s.pc.WriteTo(b, addr)
	if err != nil {
		s.logf(logNormal, "error writing to %v: %v", addr, err)
	}
}

func (s *Socket) reader() {
	b := make([]byte, maxDatagramSize)
	// Some systems raise read errors that don't mean the socket is finished, so tolerate a run of
	// them before giving up.
	consecutiveErrors := 0
	for {
		n, addr, err := s.pc.ReadFrom(b)
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			s.logger.Warn("ignoring socket read error", "err", err)
			consecutiveErrors++
			if consecutiveErrors >= 100 {
				s.logger.Error("too many consecutive errors, closing socket",
					"errors", consecutiveErrors)
				s.Close()
				return
			}
			continue
		}
		consecutiveErrors = 0
		s.processReceivedMessage(b[:n], addr)
	}
}

func (s *Socket) processReceivedMessage(b []byte, addr net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.processPacket(b, addr) {
		s.afterReceivingUtpMessage()
	} else {
		s.onReadNonUtp(b, addr)
	}
}

// Arms the deferred ack timer. Acks are held back briefly so that a burst of packets produces one
// ack, which is what libutp achieves by flushing after draining the socket.
func (s *Socket) afterReceivingUtpMessage() {
	if s.acksScheduled || len(s.ackConns) == 0 {
		return
	}
	s.acksScheduled = true
	s.ackTimer.Reset(deferredAckDelay)
}

func (s *Socket) ackTimerFunc() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.acksScheduled || s.closed {
		return
	}
	s.acksScheduled = false
	s.updateCurrentMS()
	s.issueDeferredAcks()
}

func (s *Socket) issueDeferredAcks() {
	// sendAck removes the connection from the list, so keep taking the first one.
	for len(s.ackConns) > 0 {
		s.ackConns[0].sendAck(false)
	}
}

func (s *Socket) timeoutTimerFunc() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.checkTimeouts()
	s.timeoutTimer.Reset(timeoutCheckInterval)
}

func (s *Socket) checkTimeouts() {
	s.updateCurrentMS()
	// Forget resets we sent long enough ago that a replay isn't worth suppressing.
	kept := s.rstInfo[:0]
	for _, r := range s.rstInfo {
		if s.currentMS-r.timestamp < uint64(rstInfoTimeout/time.Millisecond) {
			kept = append(kept, r)
		}
	}
	s.rstInfo = kept
	for _, c := range s.connSlice() {
		c.checkTimeouts()
		c.maybeDestroy()
	}
}

// A snapshot of the connections, so the map can be mutated while it's walked.
func (s *Socket) connSlice() []*Conn {
	cs := make([]*Conn, 0, len(s.conns))
	for _, c := range s.conns {
		cs = append(cs, c)
	}
	return cs
}

// Dispatches an incoming datagram. Reports whether it was uTP. Mirrors utp_process_udp.
func (s *Socket) processPacket(b []byte, addr net.Addr) bool {
	p, err := parsePacket(b)
	if err != nil {
		return false
	}
	s.updateCurrentMS()
	ak := connKey(addr)
	id := p.h.ConnID

	if p.h.Type == stReset {
		// The ID in a reset is the sender's send ID, which is our receive ID for connections we
		// accepted and our receive ID either side of it for connections we opened, so check all
		// three.
		c := s.conns[socketKey{ak, id}]
		if c == nil {
			if c2 := s.conns[socketKey{ak, id + 1}]; c2 != nil && c2.connIDSend == id {
				c = c2
			} else if c2 := s.conns[socketKey{ak, id - 1}]; c2 != nil && c2.connIDSend == id {
				c = c2
			}
		}
		if c != nil {
			// Read the state before the reset overwrites it. libutp reads it afterwards, so it
			// never actually reports a refused connection.
			wasSynSent := c.state == csSynSent
			if c.closeRequested {
				c.state = csDestroy
			} else {
				c.state = csReset
			}
			if wasSynSent {
				c.onError(ErrConnRefused)
			} else {
				c.onError(ErrConnReset)
			}
			c.maybeDestroy()
		}
		return true
	}

	if p.h.Type != stSyn {
		if c := s.conns[socketKey{ak, id}]; c != nil {
			c.processIncoming(&p, false)
			c.maybeDestroy()
			return true
		}
		// Not a connection we know about. Tell the peer, but only once per distinct packet, and
		// only up to a limit, so this can't be turned into an amplifier.
		for i := range s.rstInfo {
			r := &s.rstInfo[i]
			if r.connID == id && r.addr == ak && r.ackNr == p.h.SeqNr {
				r.timestamp = s.currentMS
				return true
			}
		}
		if len(s.rstInfo) > rstInfoLimit {
			return true
		}
		s.rstInfo = append(s.rstInfo, rstInfo{
			addr:      ak,
			connID:    id,
			ackNr:     p.h.SeqNr,
			timestamp: s.currentMS,
		})
		s.sendRST(addr, id, p.h.SeqNr, uint16(rand.Uint32()))
		return true
	}

	// A SYN: an incoming connection.
	if c := s.conns[socketKey{ak, id + 1}]; c != nil {
		// We already have this connection. If it's still handshaking, our syn-ack was lost and
		// the peer is asking again: nothing retransmits it otherwise, so an accepted connection
		// would be stranded until the peer gave up. libutp drops the duplicate SYN instead.
		if c.state == csSynRecv {
			c.sendAck(true)
		}
		return true
	}
	if len(s.conns) > 3000 {
		return true
	}
	if s.firewallCallback != nil && s.firewallCallback(addr) {
		return true
	}
	c := newConn(s, addr)
	c.initialize(id+1, id)
	c.ackNr = p.h.SeqNr
	c.seqNr = uint16(rand.Uint32())
	c.fastResendSeqNr = c.seqNr
	c.state = csSynRecv
	c.processIncoming(&p, true)
	c.sendAck(true)
	s.pushBacklog(c)
	return true
}

func (s *Socket) sendRST(addr net.Addr, connIDSend, ackNr, seqNr uint16) {
	var b [headerSize]byte
	h := header{
		Type:    stReset,
		Version: protocolVersion,
		ConnID:  connIDSend,
		AckNr:   ackNr,
		SeqNr:   seqNr,
	}
	h.marshalTo(b[:])
	stampHeader(b[:], uint32(nowMicros()), 0)
	s.send(b[:], addr, false)
}

func (s *Socket) pushBacklog(c *Conn) {
	select {
	case s.backlog <- c:
	default:
		// Nobody is accepting fast enough. Drop it the way libutp's caller would.
		c.closeLocked()
	}
}

func (s *Socket) onReadNonUtp(b []byte, from net.Addr) {
	if len(s.nonUtpReads) >= nonUtpReadBufferPackets {
		return
	}
	s.nonUtpReads = append(s.nonUtpReads, nonUtpPacket{append([]byte(nil), b...), from})
	s.nonUtpReadCond.Broadcast()
}

// Accept returns the next incoming connection. It implements net.Listener.
func (s *Socket) Accept() (net.Conn, error) {
	c, ok := <-s.backlog
	if !ok {
		return nil, ErrSocketClosed
	}
	return c, nil
}

// Addr returns the local address. It implements net.Listener.
func (s *Socket) Addr() net.Addr { return s.pc.LocalAddr() }

// LocalAddr returns the local address. It implements net.PacketConn.
func (s *Socket) LocalAddr() net.Addr { return s.pc.LocalAddr() }

// Close closes the Socket and the PacketConn under it. Connections on it are torn down without
// waiting for outstanding data to be delivered.
func (s *Socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *Socket) closeLocked() error {
	if s.closed {
		return nil
	}
	s.closed = true
	for _, c := range s.connSlice() {
		c.state = csDestroy
		c.onError(ErrSocketClosed)
		c.destroy()
	}
	s.ackTimer.Stop()
	s.timeoutTimer.Stop()
	s.readDeadlineTmr.Stop()
	s.acksScheduled = false
	s.ackConns = nil
	close(s.backlog)
	s.nonUtpReadCond.Broadcast()
	return s.pc.Close()
}

// Dial connects to addr on the Socket's own network.
func (s *Socket) Dial(addr string) (net.Conn, error) {
	return s.DialContext(context.Background(), "", addr)
}

// DialTimeout connects to addr, giving up after timeout. A zero timeout means no limit.
func (s *Socket) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	ctx := context.Background()
	if timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return s.DialContext(ctx, "", addr)
}

// DialContext connects to addr. An empty network means the network the Socket is listening on.
func (s *Socket) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network == "" {
		network = s.pc.LocalAddr().Network()
	}
	ua, err := s.resolveAddr(network, addr)
	if err != nil {
		return nil, fmt.Errorf("resolving address: %w", err)
	}
	return s.DialAddrContext(ctx, ua)
}

// DialAddrContext connects to an already resolved address.
func (s *Socket) DialAddrContext(ctx context.Context, addr net.Addr) (net.Conn, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSocketClosed
	}
	s.updateCurrentMS()
	c := s.newOutgoingConn(addr)
	err := c.waitForConnect(ctx)
	if err != nil {
		c.closeLocked()
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Unlock()
	return c, nil
}

// Corresponds to utp_create_socket followed by utp_connect.
func (s *Socket) newOutgoingConn(addr net.Addr) *Conn {
	c := newConn(s, addr)
	// The connection is identified to the peer by a seed we pick: we receive on it and send on
	// the one above it.
	var seed uint16
	for {
		seed = uint16(rand.Uint32())
		if _, ok := s.conns[socketKey{connKey(addr), seed}]; !ok {
			break
		}
	}
	c.initialize(seed, seed+1)
	c.state = csSynSent

	c.retransmitTimeout = 3000
	c.rtoTimeout = s.currentMS + uint64(c.retransmitTimeout)
	c.lastRcvWin = c.rcvWindow()
	// A random initial sequence number, so a blind attacker can't guess where the window is.
	c.seqNr = uint16(rand.Uint32())

	pkt := &outgoingPacket{data: make([]byte, headerSize)}
	h := header{
		Type:    stSyn,
		Version: protocolVersion,
		// SYNs are special: they carry the ID we want to receive on, not the one we send on.
		ConnID:  c.connIDRecv,
		WndSize: uint32(c.lastRcvWin),
		SeqNr:   c.seqNr,
	}
	h.marshalTo(pkt.data)
	c.outbuf.ensureSize(uint32(c.seqNr), uint32(c.curWindowPackets))
	c.outbuf.put(uint32(c.seqNr), pkt)
	c.seqNr++
	c.curWindowPackets++
	c.fastResendSeqNr = c.seqNr
	c.sendPacket(pkt)
	return c
}

func (c *Conn) waitForConnect(ctx context.Context) error {
	if ctx.Done() != nil {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			<-ctx.Done()
			c.broadcast()
		}()
	}
	for {
		switch {
		case c.err != nil:
			return c.err
		case c.closed || c.destroyed:
			return ErrConnClosed
		case c.connected:
			return nil
		case ctx.Err() != nil:
			return ctx.Err()
		}
		c.cond.Wait()
	}
}

// ReadFrom returns the next datagram received on this port that wasn't uTP. It implements
// net.PacketConn. Datagrams are dropped if they aren't read promptly.
func (s *Socket) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		if len(s.nonUtpReads) > 0 {
			p := s.nonUtpReads[0]
			s.nonUtpReads = s.nonUtpReads[1:]
			return copy(b, p.b), p.from, nil
		}
		if s.closed {
			return 0, nil, ErrSocketClosed
		}
		if !s.readDeadline.IsZero() && !time.Now().Before(s.readDeadline) {
			return 0, nil, errDeadlineExceeded
		}
		s.nonUtpReadCond.Wait()
	}
}

// WriteTo sends a datagram on the underlying PacketConn, bypassing uTP entirely. It implements
// net.PacketConn.
func (s *Socket) WriteTo(b []byte, addr net.Addr) (int, error) {
	s.mu.Lock()
	if !s.writeDeadline.IsZero() && !time.Now().Before(s.writeDeadline) {
		s.mu.Unlock()
		return 0, errDeadlineExceeded
	}
	s.mu.Unlock()
	return s.pc.WriteTo(b, addr)
}

// SetReadDeadline sets the deadline for ReadFrom. It has no effect on connections.
func (s *Socket) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readDeadline = t
	if t.IsZero() {
		s.readDeadlineTmr.Stop()
	} else {
		s.readDeadlineTmr.Reset(time.Until(t))
	}
	s.nonUtpReadCond.Broadcast()
	return nil
}

// SetWriteDeadline sets the deadline for WriteTo. It has no effect on connections.
func (s *Socket) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeDeadline = t
	return nil
}

// SetDeadline sets both ReadFrom and WriteTo deadlines.
func (s *Socket) SetDeadline(t time.Time) error {
	s.SetReadDeadline(t)
	return s.SetWriteDeadline(t)
}

// ReadBufferLen is the receive buffer new connections get.
func (s *Socket) ReadBufferLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.optRcvBuf
}

// WriteBufferLen is the send buffer new connections get.
func (s *Socket) WriteBufferLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.optSndBuf
}

// SetWriteBufferLen sets the send buffer new connections get.
func (s *Socket) SetWriteBufferLen(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optSndBuf = n
}

// SetReadBufferLen sets the receive buffer new connections get.
func (s *Socket) SetReadBufferLen(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optRcvBuf = n
}
