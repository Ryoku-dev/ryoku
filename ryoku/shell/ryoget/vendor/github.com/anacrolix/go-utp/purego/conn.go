package purego

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"
)

// Tunables ported from libutp. The comments explain what each one is protecting against; the
// values are the ones the reference implementation has shipped for over a decade, and changing
// them changes how this implementation shares a link with everything else running uTP.
const (
	// Bytes the window may grow by per RTT when every packet in it reported zero queuing delay.
	// Scaled down linearly by how close the measured delay is to the target. TCP grows by one MSS
	// (1500) per RTT for comparison.
	maxCwndIncreaseBytesPerRTT = 3000
	// The window may never drop below this many bytes.
	minWindowSize = 10
	// The nominal packet size used before MTU discovery has an answer, and for the default peer
	// receive window.
	packetSize = 1435
	// Resend the oldest unacked packet once this many duplicate acks have arrived.
	duplicateAcksBeforeResend = 3
	// A non-SYN packet acking more than this far past what we've sent is treated as spoofed.
	ackNrAllowedWindow = duplicateAcksBeforeResend
	// How long the window must have been left alone before it may be decayed again.
	maxWindowDecay = 100 * time.Millisecond
	// The largest reorder distance we'll buffer, and the largest send queue we'll build.
	reorderBufferMaxSize  = 1024
	outgoingBufferMaxSize = 1024
	// The furthest a single delay sample may sit from the average delay baseline. Samples are
	// wrapping distances over a peer supplied field, so without a bound a peer can pick one that
	// no average over it can represent.
	maxAverageDelaySample = math.MaxInt32 / 4
	// The smallest MTU the discovery search will go down to. Less would not pass TCP.
	mtuFloorMin = 576
	// Measured across many home NAT devices as a mapping lifetime that's safe to sit inside.
	keepAliveInterval = 29 * time.Second
	// The default congestion control target: one way queuing delay, in microseconds.
	defaultTargetDelay = 100 * 1000
	// Default send and receive buffers. A megabyte of receive buffer means a peer 200ms away
	// can't send faster than 5MB/s, which is assumed to be plenty.
	defaultBufferSize = 1024 * 1024

	seqNrMask = 0xffff
	ackNrMask = 0xffff
)

// Assumed UDP payload limits, used to seed MTU discovery. libutp is deliberately pessimistic here
// because it can't see the local interface: it budgets for tunnels stacked on top of Ethernet.
const (
	ethernetMTU     = 1500
	ipv4HeaderSize  = 20
	ipv6HeaderSize  = 40
	udpHeaderSize   = 8
	greHeaderSize   = 24
	pppoeHeaderSize = 8
	mppeHeaderSize  = 2
	// Packets have been seen in the wild fragmented with a first fragment payload of 1416, and
	// there are reports of routers with an MTU as low as 1392.
	fudgeHeaderSize = 36
	teredoMTU       = 1280

	udpIPv4MTU   = ethernetMTU - ipv4HeaderSize - udpHeaderSize - greHeaderSize - pppoeHeaderSize - mppeHeaderSize - fudgeHeaderSize
	udpTeredoMTU = teredoMTU - ipv6HeaderSize - udpHeaderSize
)

type connState uint8

const (
	csUninitialized connState = iota
	csIdle
	csSynSent
	csSynRecv
	csConnected
	// Connected, but the congestion window is full so writes can't proceed.
	csConnectedFull
	csReset
	csDestroy
)

func (me connState) String() string {
	switch me {
	case csUninitialized:
		return "uninitialized"
	case csIdle:
		return "idle"
	case csSynSent:
		return "syn-sent"
	case csSynRecv:
		return "syn-recv"
	case csConnected:
		return "connected"
	case csConnectedFull:
		return "connected-full"
	case csReset:
		return "reset"
	case csDestroy:
		return "destroy"
	default:
		return fmt.Sprintf("connState(%d)", uint8(me))
	}
}

// Errors reported through Conn's methods. They correspond to libutp's UTP_ECONNREFUSED,
// UTP_ECONNRESET and UTP_ETIMEDOUT.
var (
	ErrConnRefused = errors.New("connection refused")
	ErrConnReset   = errors.New("connection reset by peer")
	ErrTimedOut    = errors.New("uTP connection timed out")
	// Returned by Conn methods after Close. It wraps net.ErrClosed, so callers written against
	// the standard library's conventions recognize it.
	ErrConnClosed = fmt.Errorf("uTP connection closed: %w", net.ErrClosed)
)

// A packet held in the send queue. data is the whole datagram, header included, and is reused as
// is for retransmissions apart from the ack number and timestamps, which are refreshed each time.
type outgoingPacket struct {
	payload int
	// When the packet was last handed to the socket, in microseconds.
	timeSent      uint64
	transmissions uint32
	// Set when a timeout has written the packet off, so that it isn't counted against the window
	// until it goes out again.
	needResend bool
	data       []byte
}

// A payload held in the reorder buffer, waiting on an earlier packet.
type inboundPacket struct {
	data []byte
}

// Conn is a uTP connection. It implements net.Conn.
type Conn struct {
	s       *Socket
	addr    net.Addr
	addrKey connAddrKey
	// Guarded by s.mu, and broadcast on whenever anything a blocked caller might be waiting for
	// changes.
	cond sync.Cond

	state connState

	retransmitCount uint16
	reorderCount    uint16
	duplicateAck    uint8
	// Packets in the send queue, sent or not. The oldest unacked packet is seqNr-curWindowPackets.
	curWindowPackets uint16
	// Bytes in flight. Packets not yet sent, and packets a timeout has written off, don't count.
	curWindow int
	maxWindow int
	optSndBuf int
	optRcvBuf int
	// Congestion control target, in microseconds.
	targetDelay int

	gotFin         bool
	gotFinReached  bool
	finSent        bool
	finSentAcked   bool
	readShutdown   bool
	closeRequested bool
	fastTimeout    bool

	// The peer's advertised receive window, in bytes.
	maxWindowUser int
	lastRwinDecay uint64
	// Sequence number of the peer's FIN. Only meaningful once gotFin.
	eofPkt uint16
	// Everything up to and including this has been received.
	ackNr uint16
	// The sequence number of the next packet to send.
	seqNr uint16
	// The next sequence number a fast resend is allowed on, so each packet is fast resent at most
	// once.
	fastResendSeqNr uint16

	// The one way delay we last measured, echoed back to the peer in every packet we send.
	replyMicro uint32

	lastGotPacket     uint64
	lastSentPacket    uint64
	lastMeasuredDelay uint64
	// When the window was last full. Used to keep it from growing while we're application limited.
	lastMaxedOutWindow uint64

	// Round trip time, its variance, and the resulting timeout. All in milliseconds.
	rtt     uint32
	rttVar  uint32
	rto     uint32
	rttHist delayHist
	// The current timeout, which backs off on repeated loss, and when it expires.
	retransmitTimeout uint32
	rtoTimeout        uint64
	// When the peer advertises a zero window, when to try again anyway.
	zeroWindowTime uint64

	connIDRecv uint16
	connIDSend uint16
	// The last receive window we advertised.
	lastRcvWin int

	ourHist   delayHist
	theirHist delayHist
	// The extension bits the peer advertised in its SYN. libutp records these and leaves them to
	// the application; nothing in the protocol proper reads them.
	extensions [8]byte

	// MTU discovery. floor is known good, ceiling is known bad (or unprobed), last is what we're
	// currently sending at. Only one probe is ever in flight.
	mtuDiscoverTime uint64
	mtuCeiling      int
	mtuFloor        int
	mtuLast         int
	mtuProbeSeq     uint16
	mtuProbeSize    int

	// The average of the delay samples of the last five seconds, relative to averageDelayBase,
	// and the pieces used to compute it. Their slope over time is the clock drift estimate.
	averageDelay        int32
	currentDelaySum     int64
	currentDelaySamples int
	averageDelayBase    uint32
	averageSampleTime   uint64
	clockDrift          int32

	inbuf  circularBuffer[inboundPacket]
	outbuf circularBuffer[outgoingPacket]

	slowStart bool
	ssthresh  int

	// Application facing state.
	readBuf   readBuffer
	connected bool
	gotEOF    bool
	closed    bool
	destroyed bool
	err       error

	readDeadline       time.Time
	writeDeadline      time.Time
	readDeadlineTimer  *time.Timer
	writeDeadlineTimer *time.Timer

	ackScheduled bool
}

var _ net.Conn = (*Conn)(nil)

// Corresponds to utp_create_socket: everything that doesn't depend on the peer address.
func newConn(s *Socket, addr net.Addr) *Conn {
	c := &Conn{
		s:               s,
		addr:            addr,
		state:           csUninitialized,
		rto:             3000,
		rttVar:          800,
		seqNr:           1,
		ackNr:           0,
		maxWindowUser:   255 * packetSize,
		fastResendSeqNr: 1,
		targetDelay:     s.targetDelay,
		optSndBuf:       s.optSndBuf,
		optRcvBuf:       s.optRcvBuf,
		slowStart:       true,
		ssthresh:        s.optSndBuf,
	}
	c.cond.L = &s.mu
	c.inbuf.init()
	c.outbuf.init()
	c.readDeadlineTimer = time.AfterFunc(math.MaxInt64, c.broadcast)
	c.readDeadlineTimer.Stop()
	c.writeDeadlineTimer = time.AfterFunc(math.MaxInt64, c.broadcast)
	c.writeDeadlineTimer.Stop()
	return c
}

func (c *Conn) broadcast() {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.cond.Broadcast()
}

// Corresponds to utp_initialize_socket: everything that needs the peer address and connection IDs.
func (c *Conn) initialize(connIDRecv, connIDSend uint16) {
	currentMS := c.s.currentMS
	c.state = csIdle
	c.connIDRecv = connIDRecv
	c.connIDSend = connIDSend
	c.lastGotPacket = currentMS
	c.lastSentPacket = currentMS
	// Deliberately far in the future: there is no delay measurement to age until one arrives.
	c.lastMeasuredDelay = currentMS + 0x70000000
	c.averageSampleTime = currentMS + 5000
	c.lastRwinDecay = currentMS - uint64(maxWindowDecay/time.Millisecond)
	c.ourHist.clear(currentMS)
	c.theirHist.clear(currentMS)
	c.rttHist.clear(currentMS)
	c.mtuReset()
	c.mtuLast = c.mtuCeiling
	c.addrKey = connKey(c.addr)
	c.s.conns[socketKey{c.addrKey, connIDRecv}] = c
	// We need to fit one packet in the window to get started.
	c.maxWindow = c.packetSize()
}

// The largest payload we'll put in a packet, given what MTU discovery currently believes.
func (c *Conn) packetSize() int {
	mtu := c.mtuLast
	if mtu == 0 {
		mtu = c.mtuCeiling
	}
	return mtu - headerSize
}

// The receive window to advertise: what's left of the receive buffer once buffered data is
// accounted for.
func (c *Conn) rcvWindow() int {
	n := c.readBuf.len()
	if c.optRcvBuf > n {
		return c.optRcvBuf - n
	}
	return 0
}

func (c *Conn) udpMTU() int {
	if addrIsIPv6(c.addr) {
		// Without knowing the local interface, assume the worst and treat every IPv6 connection
		// as if it were Teredo.
		return udpTeredoMTU
	}
	return udpIPv4MTU
}

// Called whenever the MTU floor or ceiling moves.
func (c *Conn) mtuSearchUpdate() {
	// The floor can end up above the ceiling: a probe that previously got through may be dropped
	// once we're in steady state. That's not worth aborting the search over, so repair the range
	// and search again from half way down rather than from the bottom.
	if c.mtuFloor > c.mtuCeiling {
		c.mtuCeiling = c.mtuFloor
		if c.mtuCeiling > mtuFloorMin {
			c.mtuFloor = (mtuFloorMin + c.mtuCeiling) / 2
		} else {
			c.mtuFloor = c.mtuCeiling
		}
	}
	c.mtuLast = (c.mtuFloor + c.mtuCeiling) / 2
	// Allow a new probe to be sent.
	c.mtuProbeSeq = 0
	c.mtuProbeSize = 0
	// Close enough: settle on the floor, which is the only size we know gets through, and stop.
	if c.mtuCeiling-c.mtuFloor <= 16 {
		c.mtuLast = c.mtuFloor
		c.mtuCeiling = c.mtuFloor
		c.mtuDiscoverTime = c.s.currentMS + 30*60*1000
	}
}

func (c *Conn) mtuReset() {
	c.mtuCeiling = c.udpMTU()
	c.mtuFloor = mtuFloorMin
	// An interface can report an MTU below the smallest size we would otherwise search down to.
	// Follow it down rather than inverting the range.
	if c.mtuFloor > c.mtuCeiling {
		c.mtuFloor = c.mtuCeiling
	}
	c.mtuDiscoverTime = c.s.currentMS + 30*60*1000
}

// Ask the socket to send an ack for this connection once the current batch of reads is done, so
// that a burst of packets produces one ack rather than one per packet.
func (c *Conn) scheduleAck() {
	if !c.ackScheduled {
		c.ackScheduled = true
		c.s.ackConns = append(c.s.ackConns, c)
	}
}

// Stamps and sends a datagram that's already been built. This is the one place packets leave.
func (c *Conn) sendData(b []byte, dontFragment bool) {
	if traceEnabled {
		trace("SEND %d type=%v seq=%d ack=%d len=%d", c.connIDRecv, packetType(b[0]>>4), headerSeqNr(b), binary.BigEndian.Uint16(b[18:20]), len(b))
	}
	stampHeader(b, uint32(nowMicros()), c.replyMicro)
	c.lastSentPacket = c.s.currentMS
	c.s.send(b, c.addr, dontFragment)
	c.unscheduleAck()
}

func (c *Conn) unscheduleAck() {
	if !c.ackScheduled {
		return
	}
	c.ackScheduled = false
	for i, ac := range c.s.ackConns {
		if ac == c {
			c.s.ackConns = append(c.s.ackConns[:i], c.s.ackConns[i+1:]...)
			break
		}
	}
}

// Sends a state packet, carrying a selective ack if anything is sitting in the reorder buffer.
func (c *Conn) sendAck(synack bool) {
	var b [headerSize + 6]byte
	c.lastRcvWin = c.rcvWindow()
	h := header{
		Type:    stState,
		Version: protocolVersion,
		ConnID:  c.connIDSend,
		WndSize: uint32(c.lastRcvWin),
		SeqNr:   c.seqNr,
		AckNr:   c.ackNr,
	}
	n := headerSize
	// There's never a reason to send a selective ack on a connection that's shutting down.
	if c.reorderCount != 0 && !c.gotFinReached {
		h.Extension = extSelectiveAck
		var m uint32
		// The mask starts at ackNr+2, because ackNr+1 is by definition the packet we're missing.
		window := uint32(14 + 16)
		if s := c.inbuf.size(); s < window {
			window = s
		}
		for i := uint32(0); i < window; i++ {
			if c.inbuf.get(uint32(c.ackNr)+i+2) != nil {
				m |= 1 << i
			}
		}
		b[headerSize] = extNone
		b[headerSize+1] = 4
		b[headerSize+2] = byte(m)
		b[headerSize+3] = byte(m >> 8)
		b[headerSize+4] = byte(m >> 16)
		b[headerSize+5] = byte(m >> 24)
		n += 6
	}
	h.marshalTo(b[:])
	c.sendData(b[:n], false)
}

// Sends an ack for one before what we've actually received, to draw out a response from the peer
// and keep any NAT mappings in between alive.
func (c *Conn) sendKeepAlive() {
	c.ackNr--
	c.sendAck(false)
	c.ackNr++
}

func (c *Conn) sendPacket(pkt *outgoingPacket) {
	// A packet only counts against the window the first time it goes out, or when it's going out
	// again after a timeout wrote it off.
	if pkt.transmissions == 0 || pkt.needResend {
		c.curWindow += pkt.payload
	}
	pkt.needResend = false
	setHeaderAckNr(pkt.data, c.ackNr)
	pkt.timeSent = nowMicros()

	if c.mtuDiscoverTime < c.s.currentMS {
		// Time to throw away our MTU assumptions and search again.
		c.mtuReset()
	}
	useAsMTUProbe := false
	// Don't probe with packets larger than the ceiling: those have probably already failed as
	// probes, and now need to be fragmented just to get through. Sequence number 0 is the magic
	// "no probe" value, so a packet with sequence number 1 can't be probed with either.
	if c.mtuFloor < c.mtuCeiling &&
		len(pkt.data) > c.mtuFloor &&
		len(pkt.data) <= c.mtuCeiling &&
		c.mtuProbeSeq == 0 &&
		c.seqNr != 1 &&
		pkt.transmissions == 0 {
		// seqNr has already been incremented past this packet.
		c.mtuProbeSeq = (c.seqNr - 1) & ackNrMask
		c.mtuProbeSize = len(pkt.data)
		useAsMTUProbe = true
		c.s.logf(logMTU, "%v: MTU [PROBE] floor:%d ceiling:%d current:%d", c, c.mtuFloor, c.mtuCeiling, c.mtuProbeSize)
	}
	pkt.transmissions++
	c.sendData(pkt.data, useAsMTUProbe)
}

// Reports whether another bytes bytes may be put in flight. Pass -1 for a full packet.
func (c *Conn) isFull(bytes int) bool {
	ps := c.packetSize()
	if bytes < 0 {
		bytes = ps
	} else if bytes > ps {
		bytes = ps
	}
	maxSend := min(c.maxWindow, c.optSndBuf, c.maxWindowUser)
	// Leave one slot for the FIN.
	if int(c.curWindowPackets) >= outgoingBufferMaxSize-1 {
		c.lastMaxedOutWindow = c.s.currentMS
		return true
	}
	if c.curWindow+bytes > maxSend {
		c.lastMaxedOutWindow = c.s.currentMS
		return true
	}
	return false
}

// Sends whatever is queued and allowed out. Returns whether it stopped because the window is full.
func (c *Conn) flushPackets() bool {
	ps := c.packetSize()
	// i must wrap the same way sequence numbers do.
	for i := c.seqNr - c.curWindowPackets; i != c.seqNr; i++ {
		pkt := c.outbuf.get(uint32(i))
		if pkt == nil || (pkt.transmissions > 0 && !pkt.needResend) {
			continue
		}
		if c.isFull(-1) {
			return true
		}
		// Nagle: hold back a partial packet while there's already one in flight.
		if i != (c.seqNr-1)&ackNrMask || c.curWindowPackets == 1 || pkt.payload >= ps {
			c.sendPacket(pkt)
		}
	}
	return false
}

// Queues payload for sending, appending to the last unsent packet where there's room, and flushes.
// flags is stData or stFin.
func (c *Conn) writeOutgoingPacket(payload []byte, flags packetType) {
	// Start the retransmit timer if this is the first thing in the queue.
	if c.curWindowPackets == 0 {
		c.retransmitTimeout = c.rto
		c.rtoTimeout = c.s.currentMS + uint64(c.retransmitTimeout)
	}
	ps := c.packetSize()
	for {
		added := 0
		var pkt *outgoingPacket
		if c.curWindowPackets > 0 {
			pkt = c.outbuf.get(uint32(c.seqNr - 1))
		}
		append_ := true
		if len(payload) > 0 && pkt != nil && pkt.transmissions == 0 && pkt.payload < ps {
			// There's room in the last packet and it hasn't gone out yet, so fill it first.
			added = min(len(payload)+pkt.payload, max(ps, pkt.payload)) - pkt.payload
			append_ = false
		} else {
			added = len(payload)
			capacity := headerSize + added
			if flags == stData {
				// Leave room for this packet to be filled up by a later write, so that appending
				// to it doesn't reallocate and copy what's already in it.
				capacity = headerSize + max(added, ps)
			}
			pkt = &outgoingPacket{data: make([]byte, headerSize, capacity)}
		}
		if added > 0 {
			pkt.data = append(pkt.data, payload[:added]...)
			payload = payload[added:]
			pkt.payload += added
		}
		c.lastRcvWin = c.rcvWindow()
		h := header{
			Type:      flags,
			Version:   protocolVersion,
			Extension: extNone,
			ConnID:    c.connIDSend,
			WndSize:   uint32(c.lastRcvWin),
			AckNr:     c.ackNr,
		}
		// A packet we filled up rather than created keeps the sequence number it was given when
		// it was queued. Only a new packet takes the next one.
		if append_ {
			h.SeqNr = c.seqNr
		} else {
			h.SeqNr = headerSeqNr(pkt.data)
		}
		h.marshalTo(pkt.data)
		if append_ {
			c.outbuf.ensureSize(uint32(c.seqNr), uint32(c.curWindowPackets))
			c.outbuf.put(uint32(c.seqNr), pkt)
			c.seqNr++
			c.curWindowPackets++
		}
		if len(payload) == 0 {
			break
		}
	}
	c.flushPackets()
}

// Test whether enough time has passed to decay the window again.
func (c *Conn) canDecayWin(currentMS uint64) bool {
	return currentMS-c.lastRwinDecay >= uint64(maxWindowDecay/time.Millisecond)
}

func (c *Conn) maybeDecayWin(currentMS uint64) {
	if !c.canDecayWin(currentMS) {
		return
	}
	// TCP uses 0.5 here too.
	c.maxWindow /= 2
	c.lastRwinDecay = currentMS
	if c.maxWindow < minWindowSize {
		c.maxWindow = minWindowSize
	}
	c.slowStart = false
	c.ssthresh = c.maxWindow
}

// Acks the packet with the given sequence number. Reports 0 if it was acked now, 1 if it was
// already acked or never sent, and 2 if it hasn't gone out yet.
func (c *Conn) ackPacket(seq uint16) int {
	pkt := c.outbuf.get(uint32(seq))
	if pkt == nil {
		return 1
	}
	if pkt.transmissions == 0 {
		return 2
	}
	c.outbuf.put(uint32(seq), nil)
	// Only packets that went out exactly once produce a usable RTT sample: for anything resent
	// there's no telling which transmission is being acked.
	if pkt.transmissions == 1 {
		ertt := uint32((nowMicros() - pkt.timeSent) / 1000)
		if c.rtt == 0 {
			c.rtt = ertt
			c.rttVar = ertt / 2
		} else {
			delta := int32(c.rtt) - int32(ertt)
			if delta < 0 {
				delta = -delta
			}
			c.rttVar = uint32(int32(c.rttVar) + (delta-int32(c.rttVar))/4)
			c.rtt = c.rtt - c.rtt/8 + ertt/8
			c.rttHist.addSample(ertt, c.s.currentMS)
		}
		c.rto = max(c.rtt+c.rttVar*4, 1000)
	}
	c.retransmitTimeout = c.rto
	c.rtoTimeout = c.s.currentMS + uint64(c.rto)
	// A packet a timeout has written off is no longer counted in the window.
	if !pkt.needResend {
		c.curWindow -= pkt.payload
	}
	c.retransmitCount = 0
	return 0
}

// Counts the bytes acked by a selective ack header, and folds the round trip times of those
// packets into minRTT.
func (c *Conn) selectiveAckBytes(base uint32, mask []byte, minRTT *int64) int {
	if c.curWindowPackets == 0 {
		return 0
	}
	ackedBytes := 0
	now := nowMicros()
	// Unlike libutp, which starts one bit past the end of the mask, we start at the last bit that
	// actually exists. The extra iteration there can read a byte past the header.
	for bits := len(mask)*8 - 1; bits >= -1; bits-- {
		v := base + uint32(bits)
		// Ignore bits for packets outside the send window. See selectiveAck for the reasoning.
		if uint32(c.seqNr-uint16(v)-1)&ackNrMask >= uint32(uint16(c.curWindowPackets-1)) {
			continue
		}
		pkt := c.outbuf.get(v)
		if pkt == nil || pkt.transmissions == 0 {
			continue
		}
		if bits >= 0 && mask[bits>>3]&(1<<(bits&7)) != 0 {
			ackedBytes += pkt.payload
			// Guard against a clock that isn't monotonic.
			if pkt.timeSent < now {
				*minRTT = min(*minRTT, int64(now-pkt.timeSent))
			} else {
				*minRTT = min(*minRTT, 50000)
			}
		}
	}
	return ackedBytes
}

// The largest number of resends one selective ack may schedule.
const maxEack = 128

// Processes a selective ack header: acks what it reports received, and fast resends the gaps that
// have enough acked packets in front of them to look like real loss.
func (c *Conn) selectiveAck(base uint32, mask []byte) {
	if c.curWindowPackets == 0 {
		return
	}
	// A stack of sequence numbers to resend. We walk the bits from high sequence numbers down, so
	// by the end the top of the stack holds the oldest packets, which are the ones worth resending.
	var resends [maxEack]uint32
	nr := 0
	count := 0

	for bits := len(mask)*8 - 1; bits >= -1; bits-- {
		v := base + uint32(bits)
		// Ignore bits for packets we haven't sent, and bits below the acked sequence number,
		// which happens when an ack overtakes a selective ack. Written as one wrapping comparison:
		//
		//     rejected <   accepted   > rejected
		// <============+--------------+============>
		//              ^              ^
		//        (seqNr-window)     seqNr
		if uint32(c.seqNr-uint16(v)-1)&ackNrMask >= uint32(uint16(c.curWindowPackets-1)) {
			continue
		}
		bitSet := bits >= 0 && mask[bits>>3]&(1<<(bits&7)) != 0
		// Every acked bit counts as a duplicate ack, even if we've seen it in an earlier header.
		if bitSet {
			count++
		}
		pkt := c.outbuf.get(v)
		if pkt == nil || pkt.transmissions == 0 {
			continue
		}
		if bitSet {
			c.ackPacket(uint16(v))
			continue
		}
		// A gap. Only resend it if enough packets past it have been acked, and we haven't already
		// fast resent this far.
		if (v-uint32(c.fastResendSeqNr))&ackNrMask <= outgoingBufferMaxSize && count >= duplicateAcksBeforeResend {
			if nr >= maxEack-2 {
				// Full. The bottom half is the least interesting, so throw it away.
				copy(resends[:], resends[maxEack/2:])
				nr -= maxEack / 2
			}
			resends[nr] = v
			nr++
		}
	}
	// If we saw enough duplicate acks to resend at all, the first packet to resend is base-1.
	if (base-1-uint32(c.fastResendSeqNr))&ackNrMask <= outgoingBufferMaxSize && count >= duplicateAcksBeforeResend {
		resends[nr] = (base - 1) & ackNrMask
		nr++
	}

	backOff := false
	for i := 0; nr > 0; {
		nr--
		v := resends[nr]
		pkt := c.outbuf.get(v)
		// This may be an old, reordered header, and some of these may have been acked since.
		if pkt == nil {
			continue
		}
		c.s.logf(logNormal, "%v: packet %d lost, resending", c, v)
		backOff = true
		c.sendPacket(pkt)
		c.fastResendSeqNr = uint16(v+1) & ackNrMask
		i++
		if i >= 4 {
			break
		}
	}
	if backOff {
		c.maybeDecayWin(c.s.currentMS)
	}
	c.duplicateAck = uint8(min(count, math.MaxUint8))
}

// LEDBAT. Grows the window towards a fixed target queuing delay, so that a uTP transfer yields to
// anything else sharing the bottleneck rather than filling the buffer in front of it.
func (c *Conn) applyCControl(bytesAcked int, minRTT int64) {
	// The delay can never exceed the round trip time it was measured over.
	ourDelay := int32(min(uint64(c.ourHist.value()), uint64(minRTT)))

	target := c.targetDelay
	if target <= 0 {
		target = defaultTargetDelay
	}

	// Compensate for large clock drift, which otherwise hands one end an unfair share of the
	// bandwidth. The unit of clockDrift is microseconds per five seconds; empirically a cut-off
	// around 200000 catches peers whose clock runs slow, deliberately or not, without any risk of
	// false positives.
	if c.clockDrift < -200000 {
		penalty := (-c.clockDrift - 200000) / 7
		ourDelay += penalty
	}

	offTarget := float64(target - int(ourDelay))

	// Scale the maximum increase by the fraction of the window this ack covers and the fraction
	// of the target the current delay leaves, so a full window's worth of acks grows the window
	// by at most maxCwndIncreaseBytesPerRTT. The min and max keep the ratio sane when the window
	// was just shrunk below what's being acked.
	windowFactor := float64(min(bytesAcked, c.maxWindow)) / float64(max(c.maxWindow, bytesAcked))
	delayFactor := offTarget / float64(target)
	scaledGain := maxCwndIncreaseBytesPerRTT * windowFactor * delayFactor

	if scaledGain > 0 && c.s.currentMS-c.lastMaxedOutWindow > 1000 {
		// It's been more than a second since we last filled the window, so we're rate limited by
		// the application rather than the network. Growing the window on that basis would let it
		// grow without bound.
		scaledGain = 0
	}

	ledbatCwnd := minWindowSize
	if float64(c.maxWindow)+scaledGain >= minWindowSize {
		ledbatCwnd = int(float64(c.maxWindow) + scaledGain)
	}

	if c.slowStart {
		ssCwnd := int(float64(c.maxWindow) + windowFactor*float64(c.packetSize()))
		if ssCwnd > c.ssthresh {
			c.slowStart = false
		} else if float64(ourDelay) > float64(target)*0.9 {
			// Even a little under the target is reason enough to stop growing exponentially.
			c.slowStart = false
			c.ssthresh = c.maxWindow
		} else {
			c.maxWindow = max(ssCwnd, ledbatCwnd)
		}
	} else {
		c.maxWindow = ledbatCwnd
	}
	c.maxWindow = min(max(c.maxWindow, minWindowSize), c.optSndBuf)
}

// Runs the retransmit, keep alive and window reset timers. Called on every connection roughly
// every 500ms.
func (c *Conn) checkTimeouts() {
	if c.state != csDestroy {
		c.flushPackets()
	}
	switch c.state {
	case csSynSent, csSynRecv, csConnectedFull, csConnected:
	default:
		return
	}
	currentMS := c.s.currentMS

	// The peer advertised a zero window long enough ago that it's worth probing it again.
	if currentMS >= c.zeroWindowTime && c.maxWindowUser == 0 {
		c.maxWindowUser = packetSize
	}

	if currentMS >= c.rtoTimeout && c.rtoTimeout > 0 {
		ignoreLoss := false
		if c.curWindowPackets == 1 && (c.seqNr-1)&ackNrMask == c.mtuProbeSeq && c.mtuProbeSeq != 0 {
			// The only thing outstanding was the MTU probe. It was most likely dropped for being
			// too big rather than through congestion, so narrow the search, resend immediately,
			// and leave the window alone.
			c.mtuCeiling = c.mtuProbeSize - 1
			c.mtuSearchUpdate()
			ignoreLoss = true
			c.s.logf(logMTU, "%v: MTU [PROBE-TIMEOUT] floor:%d ceiling:%d current:%d", c, c.mtuFloor, c.mtuCeiling, c.mtuLast)
		}
		// The probe is gone either way; clear the fields so a new one can be sent.
		c.mtuProbeSeq = 0
		c.mtuProbeSize = 0

		newTimeout := c.retransmitTimeout
		if !ignoreLoss {
			newTimeout *= 2
		}

		// They opened the connection but never responded. A malicious peer can also spoof the
		// source address of a SYN to get us here, so don't tell the application about it.
		if c.state == csSynRecv {
			c.state = csDestroy
			c.onError(ErrTimedOut)
			return
		}
		// We opened the connection and they didn't respond, or four consecutive transmissions
		// have timed out. Give up after only two if we never connected in the first place.
		if c.retransmitCount >= 4 || (c.state == csSynSent && c.retransmitCount >= 2) {
			if c.closeRequested {
				c.state = csDestroy
			} else {
				c.state = csReset
			}
			c.onError(ErrTimedOut)
			return
		}

		c.retransmitTimeout = newTimeout
		c.rtoTimeout = currentMS + uint64(newTimeout)

		if !ignoreLoss {
			c.duplicateAck = 0
			ps := c.packetSize()
			if c.curWindowPackets == 0 && c.maxWindow > ps {
				// Nothing was in flight even though there could have been, so the connection is
				// just idling. No need to be aggressive: let the window decay by a third, but not
				// below one packet.
				c.maxWindow = max(c.maxWindow*2/3, ps)
			} else {
				// The delay was high enough to shrink the window below a single packet, which
				// stopped us sending anything for a whole timeout. Reset to one packet and start
				// over.
				c.maxWindow = ps
				c.slowStart = true
			}
		}

		// Everything outstanding is written off.
		for i := uint16(0); i < c.curWindowPackets; i++ {
			pkt := c.outbuf.get(uint32(c.seqNr - i - 1))
			if pkt == nil || pkt.transmissions == 0 || pkt.needResend {
				continue
			}
			pkt.needResend = true
			c.curWindow -= pkt.payload
		}

		if c.curWindowPackets > 0 {
			c.retransmitCount++
			c.s.logf(logNormal, "%v: packet timeout, resending seq_nr:%d timeout:%d max_window:%d in_flight:%d",
				c, c.seqNr-c.curWindowPackets, c.retransmitTimeout, c.maxWindow, c.curWindowPackets)
			c.fastTimeout = true
			if pkt := c.outbuf.get(uint32(c.seqNr - c.curWindowPackets)); pkt != nil {
				c.sendPacket(pkt)
			}
		}
	}

	// If the window grew, or in-flight bytes dropped below it, writers can go again.
	if c.state == csConnectedFull && !c.isFull(-1) {
		c.state = csConnected
		c.cond.Broadcast()
	}

	if c.state >= csConnected && !c.finSent {
		if currentMS-c.lastSentPacket >= uint64(keepAliveInterval/time.Millisecond) {
			c.sendKeepAlive()
		}
	}
}

// Handles one incoming packet for this connection. syn is set for the SYN that created it, which
// stops parsing once the header has been taken in. Returns the number of payload bytes consumed.
func (c *Conn) processIncoming(p *packet, syn bool) {
	currentMS := c.s.currentMS
	pkSeqNr := p.h.SeqNr
	pkAckNr := p.h.AckNr

	// Mark receipt time before doing anything that could take a while.
	now := nowMicros()
	if traceEnabled {
		trace("RECV %d type=%v seq=%d ack=%d payload=%d syn=%v state=%v myAck=%d mySeq=%d cwp=%d selack=%v", c.connIDRecv, p.h.Type, pkSeqNr, pkAckNr, len(p.payload), syn, c.state, c.ackNr, c.seqNr, c.curWindowPackets, p.selAck)
	}

	// The range of ack numbers we're willing to believe. Anything outside it implies a spoofed
	// source address or a peer attacking the implementation, since it acks something we can't
	// have sent. SYNs are exempt: there are no previous packets to bound them with.
	currWindow := max(c.curWindowPackets+ackNrAllowedWindow, ackNrAllowedWindow)
	if p.h.Type != stSyn || c.state != csSynRecv {
		if wrappingCompareLess(uint32(c.seqNr-1), uint32(pkAckNr), ackNrMask) ||
			wrappingCompareLess(uint32(pkAckNr), uint32(c.seqNr-1-currWindow), ackNrMask) {
			return
		}
	}

	if p.hasExtBits {
		c.extensions = p.extBits
	}

	if c.state == csSynSent {
		// This is the syn-ack, so take our ack number from the sequence number it carries.
		c.ackNr = (pkSeqNr - 1) & seqNrMask
	}

	c.lastGotPacket = currentMS

	if syn {
		return
	}

	// How far past the next expected packet this one is. Zero means it's the one we want.
	seqNr := uint32(pkSeqNr-c.ackNr-1) & seqNrMask
	if seqNr >= reorderBufferMaxSize {
		// Ancient or absurd. If it's just behind us, the peer probably lost our ack, so send
		// another one.
		if seqNr >= (seqNrMask+1)-reorderBufferMaxSize && p.h.Type != stState {
			c.scheduleAck()
		}
		return
	}

	// How many of our packets this acks.
	acks := int(uint16(pkAckNr-(c.seqNr-1-c.curWindowPackets)) & ackNrMask)
	// An old ack number, arriving late.
	if acks > int(c.curWindowPackets) {
		acks = 0
	}

	// Count duplicate acks, but only in state packets. Any other packet was most likely sent
	// because the peer had data of its own to send, not in response to ours. Without this, a
	// connection carrying payload in both directions sees three duplicate acks almost every time
	// it sends something, which would defeat the fast resend logic entirely. BSD 4.4's TCP does
	// the same.
	if c.curWindowPackets > 0 {
		if pkAckNr == (c.seqNr-c.curWindowPackets-1)&ackNrMask && p.h.Type == stState {
			c.duplicateAck++
			if c.duplicateAck == duplicateAcksBeforeResend && c.mtuProbeSeq != 0 {
				// The probe was probably rejected for its size, and the ICMP report hasn't caught
				// up with us yet.
				if pkAckNr == (c.mtuProbeSeq-1)&ackNrMask {
					c.mtuCeiling = c.mtuProbeSize - 1
					c.mtuSearchUpdate()
					c.s.logf(logMTU, "%v: MTU [DUPACK] floor:%d ceiling:%d current:%d", c, c.mtuFloor, c.mtuCeiling, c.mtuLast)
				} else {
					// Something that wasn't the probe was dropped before it. That says nothing
					// about the MTU, so allow a new probe.
					c.mtuProbeSeq = 0
					c.mtuProbeSize = 0
				}
			}
		} else {
			c.duplicateAck = 0
		}
	}

	ackedBytes := 0
	// The smallest round trip time across everything this packet acked. It's the upper bound on
	// the delay the peer can report back to us.
	minRTT := int64(math.MaxInt64)

	for i := 0; i < acks; i++ {
		seq := (c.seqNr - c.curWindowPackets + uint16(i)) & ackNrMask
		pkt := c.outbuf.get(uint32(seq))
		if pkt == nil || pkt.transmissions == 0 {
			continue
		}
		ackedBytes += pkt.payload
		if c.mtuProbeSeq != 0 && seq == c.mtuProbeSeq {
			c.mtuFloor = c.mtuProbeSize
			c.mtuSearchUpdate()
			c.s.logf(logMTU, "%v: MTU [ACK] floor:%d ceiling:%d current:%d", c, c.mtuFloor, c.mtuCeiling, c.mtuLast)
		}
		if pkt.timeSent < now {
			minRTT = min(minRTT, int64(now-pkt.timeSent))
		} else {
			minRTT = min(minRTT, 50000)
		}
	}
	if p.selAck != nil {
		ackedBytes += c.selectiveAckBytes((uint32(pkAckNr)+2)&ackNrMask, p.selAck, &minRTT)
	}

	c.lastMeasuredDelay = currentMS

	// The delay in each direction. Ours is what the peer measured and echoed back; theirs is what
	// we measure now and will echo back to them.
	theirDelay := uint32(0)
	if p.h.TimestampMicros != 0 {
		theirDelay = uint32(now) - p.h.TimestampMicros
	}
	c.replyMicro = theirDelay
	prevDelayBase := c.theirHist.delayBase
	if theirDelay != 0 {
		c.theirHist.addSample(theirDelay, currentMS)
	}
	// If their delay base dropped, our clock is running fast relative to theirs, so shift our own
	// base the other way to compensate.
	if prevDelayBase != 0 && wrappingCompareLess(c.theirHist.delayBase, prevDelayBase, math.MaxUint32) {
		// Never adjust by more than 10 milliseconds at a time.
		if prevDelayBase-c.theirHist.delayBase <= 10000 {
			c.ourHist.shift(prevDelayBase - c.theirHist.delayBase)
		}
	}

	actualDelay := p.h.TimestampDiffMicros
	if actualDelay == math.MaxInt32 {
		actualDelay = 0
	}
	// A zero means the peer hasn't measured us yet and doesn't know, which isn't a sample.
	if actualDelay != 0 {
		c.ourHist.addSample(actualDelay, currentMS)
		c.updateClockDrift(actualDelay, currentMS)
	}

	// If the delay estimate exceeds the round trip time it was measured over, it's wrong; move
	// the base to make it fit.
	if int64(c.ourHist.value()) > minRTT {
		c.ourHist.shift(uint32(int64(c.ourHist.value()) - minRTT))
	}

	// Congestion control only runs on acks, and only when there's a delay measurement to run it on.
	if actualDelay != 0 && ackedBytes >= 1 {
		c.applyCControl(ackedBytes, minRTT)
	}

	// The peer should never ack past what we've sent.
	if acks <= int(c.curWindowPackets) {
		c.maxWindowUser = int(p.h.WndSize)
		// A zero window stalls us completely, so arrange to probe it again in 15 seconds.
		if c.maxWindowUser == 0 {
			c.zeroWindowTime = currentMS + 15000
		}

		// An incoming connection is established by the first data packet on it. Writes were
		// refused until now, so wake up anyone waiting to retry.
		if p.h.Type == stData && c.state == csSynRecv {
			c.state = csConnected
			c.cond.Broadcast()
		}

		if p.h.Type == stState && c.state == csSynSent {
			// Outgoing connection established.
			c.state = csConnected
			c.connected = true
			c.cond.Broadcast()
			// The peer holds an accepted connection in syn-recv until our first data packet
			// arrives, and refuses writes on it until then. A dialer that has nothing to say
			// would stall the connection in that state indefinitely, so send an empty data
			// packet to complete the handshake.
			c.writeOutgoingPacket(nil, stData)
		} else if c.finSent && int(c.curWindowPackets) == acks {
			// Everything we sent has been acked, including our FIN.
			c.finSentAcked = true
			if c.closeRequested {
				c.state = csDestroy
			}
		}

		if wrappingCompareLess(uint32(c.fastResendSeqNr), uint32(pkAckNr+1)&ackNrMask, ackNrMask) {
			c.fastResendSeqNr = (pkAckNr + 1) & ackNrMask
		}

		for i := 0; i < acks; i++ {
			status := c.ackPacket(c.seqNr - c.curWindowPackets)
			// A packet that hasn't been sent yet can't be acked. This can happen when the ack
			// number covers packets we've queued but not transmitted, and there's nothing past it
			// worth looking at.
			if status == 2 {
				break
			}
			c.curWindowPackets--
		}

		// Packets in front of this one may have been acked by a selective ack, so keep going
		// until we find one that's still waiting.
		for c.curWindowPackets > 0 && c.outbuf.get(uint32(c.seqNr-c.curWindowPackets)) == nil {
			c.curWindowPackets--
		}

		// Flush Nagle.
		if c.curWindowPackets == 1 {
			pkt := c.outbuf.get(uint32(c.seqNr - 1))
			if pkt != nil && pkt.transmissions == 0 {
				c.sendPacket(pkt)
			}
		}

		if c.fastTimeout {
			// If the fast resend pointer has moved past the oldest outstanding packet, we've
			// already resent what timed out, so leave fast timeout mode.
			if (c.seqNr-c.curWindowPackets)&ackNrMask != c.fastResendSeqNr {
				c.fastTimeout = false
			} else if pkt := c.outbuf.get(uint32(c.seqNr - c.curWindowPackets)); pkt != nil && pkt.transmissions > 0 {
				// Resend the oldest packet, and step the pointer so it isn't fast resent again.
				c.fastResendSeqNr++
				c.sendPacket(pkt)
			}
		}
	}

	if p.selAck != nil {
		c.selectiveAck((uint32(pkAckNr)+2)&ackNrMask, p.selAck)
	}

	// Acks free up window space, so send whatever the queue has been holding. libutp leaves this
	// to the next application write or to the 500ms timer, which stalls a sender that has queued
	// more than the window and then stopped writing.
	if acks > 0 || p.selAck != nil {
		c.flushPackets()
	}

	// If the ack dropped in-flight bytes below the window, writers can go again.
	if c.state == csConnectedFull && !c.isFull(-1) {
		c.state = csConnected
		c.cond.Broadcast()
	}

	if p.h.Type == stState {
		// Nothing but an ack.
		return
	}
	if c.state != csConnected && c.state != csConnectedFull {
		return
	}

	if p.h.Type == stFin && !c.gotFin {
		c.gotFin = true
		c.eofPkt = pkSeqNr
		// The peer may have sent packets with higher sequence numbers than this, which leaves our
		// reorder count out of sync. That's dealt with by ignoring anything past the FIN when we
		// reach it.
	}

	if seqNr == 0 {
		// The packet we were waiting for.
		if len(p.payload) > 0 && !c.readShutdown {
			c.readBuf.write(p.payload)
			c.cond.Broadcast()
		}
		c.ackNr++
		// Deliver anything the reorder buffer was holding behind it.
		for {
			if !c.gotFinReached && c.gotFin && c.eofPkt == c.ackNr {
				c.gotFinReached = true
				c.rtoTimeout = currentMS + uint64(min(c.rto*3, 60))
				c.gotEOF = true
				c.cond.Broadcast()
				// The peer wants to close, so ack it right away.
				c.sendAck(false)
				// Anything left in the reorder buffer is past the FIN, so drop it.
				c.reorderCount = 0
				break
			}
			if c.reorderCount == 0 {
				break
			}
			ip := c.inbuf.get(uint32(c.ackNr + 1))
			if ip == nil {
				break
			}
			c.inbuf.put(uint32(c.ackNr+1), nil)
			if len(ip.data) > 0 && !c.readShutdown {
				c.readBuf.writeOwned(ip.data)
				c.cond.Broadcast()
			}
			c.ackNr++
			c.reorderCount--
		}
		c.scheduleAck()
	} else {
		// Out of order. Remember it for later.

		// A packet past the FIN can't be real.
		if c.gotFin && wrappingCompareLess(uint32(c.eofPkt), uint32(pkSeqNr), seqNrMask) {
			return
		}
		// Don't size the reorder buffer from untrusted input alone.
		if seqNr > 0x3ff {
			return
		}
		// Grow before the duplicate check, so we don't look at an older packet that the smaller
		// buffer had aliased onto this slot.
		c.inbuf.ensureSize(uint32(pkSeqNr)+1, seqNr+1)
		if c.inbuf.get(uint32(pkSeqNr)) != nil {
			// Duplicate.
			return
		}
		c.inbuf.put(uint32(pkSeqNr), &inboundPacket{data: append([]byte(nil), p.payload...)})
		c.reorderCount++
		c.scheduleAck()
	}
}

// Keeps a five second average of the delay samples, and estimates the clock drift between the two
// peers from how that average moves. The samples are held relative to averageDelayBase so that
// the wrapping 32 bit field they come from can be averaged at all.
func (c *Conn) updateClockDrift(actualDelay uint32, currentMS uint64) {
	if c.averageDelayBase == 0 {
		c.averageDelayBase = actualDelay
	}
	var averageDelaySample int64
	distDown := c.averageDelayBase - actualDelay
	distUp := actualDelay - c.averageDelayBase
	// Both distances derive from a field the peer picks freely, so clamp the sample to the range
	// this average can represent. A one way delay that far from the baseline is garbage whatever
	// the peer intended by it.
	if distDown > distUp {
		averageDelaySample = int64(min(distUp, maxAverageDelaySample))
	} else {
		averageDelaySample = -int64(min(distDown, maxAverageDelaySample))
	}
	c.currentDelaySum += averageDelaySample
	c.currentDelaySamples++

	if currentMS <= c.averageSampleTime {
		return
	}
	prevAverageDelay := c.averageDelay
	c.averageDelay = int32(c.currentDelaySum / int64(c.currentDelaySamples))
	// Each slot is five seconds.
	c.averageSampleTime += 5000
	c.currentDelaySum = 0
	c.currentDelaySamples = 0

	// Only the slope of the averages matters, so keep them near zero to stay well away from
	// wrapping.
	minSample := min(prevAverageDelay, c.averageDelay)
	maxSample := max(prevAverageDelay, c.averageDelay)
	adjust := int32(0)
	if minSample > 0 {
		adjust = -minSample
	} else if maxSample < 0 {
		adjust = -maxSample
	}
	if adjust != 0 {
		c.averageDelayBase -= uint32(adjust)
		c.averageDelay += adjust
		prevAverageDelay += adjust
	}

	// The drift is just the average difference between consecutive slots. Since each slot is five
	// seconds and the samples are microseconds, this is the average slope across our history, in
	// microseconds per five seconds. A consistent trend shows up here.
	drift := c.averageDelay - prevAverageDelay
	c.clockDrift = int32((int64(c.clockDrift)*7 + int64(drift)) / 8)
}

func (c *Conn) onError(err error) {
	if c.err == nil {
		c.err = err
	}
	c.cond.Broadcast()
}

// Tears the connection down once its state machine is finished with it.
func (c *Conn) destroy() {
	if c.destroyed {
		return
	}
	c.destroyed = true
	c.unscheduleAck()
	delete(c.s.conns, socketKey{c.addrKey, c.connIDRecv})
	c.readDeadlineTimer.Stop()
	c.writeDeadlineTimer.Stop()
	c.cond.Broadcast()
}

// Reaps the connection if its state machine has finished with it. Called after anything that can
// move the state.
func (c *Conn) maybeDestroy() {
	if c.state == csDestroy {
		c.destroy()
	}
}

func (c *Conn) String() string {
	return fmt.Sprintf("uTP %v/%d", c.addr, c.connIDRecv)
}

// Called with the socket lock held, after the read buffer has been drained, so the peer learns
// the window opened up.
func (c *Conn) readDrained() {
	rcvWin := c.rcvWindow()
	if rcvWin <= c.lastRcvWin {
		return
	}
	if c.lastRcvWin == 0 {
		// We had the peer completely stalled, so don't wait to tell it otherwise.
		c.sendAck(false)
	} else {
		c.scheduleAck()
	}
}

// The error a blocked reader or writer should see, or nil to keep waiting.
func (c *Conn) statusErrLocked(deadline time.Time) error {
	switch {
	case c.err != nil:
		return c.err
	case c.closed:
		return ErrConnClosed
	case c.destroyed:
		return ErrConnClosed
	case !deadline.IsZero() && !time.Now().Before(deadline):
		return errDeadlineExceeded
	default:
		return nil
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	for {
		n = c.readBuf.read(b)
		if n != 0 {
			c.readDrained()
			return
		}
		if len(b) == 0 {
			return
		}
		if c.gotEOF && c.readBuf.len() == 0 {
			return 0, io.EOF
		}
		if err = c.statusErrLocked(c.readDeadline); err != nil {
			return
		}
		c.cond.Wait()
	}
}

// Queues as much of b as the window allows, and reports how much that was. Mirrors utp_writev.
func (c *Conn) writeNoWait(b []byte) int {
	if c.state != csConnected || c.finSent {
		return 0
	}
	sent := 0
	ps := c.packetSize()
	numToSend := min(len(b), ps)
	for !c.isFull(numToSend) {
		c.writeOutgoingPacket(b[:numToSend], stData)
		b = b[numToSend:]
		sent += numToSend
		numToSend = min(len(b), ps)
		if numToSend == 0 {
			return sent
		}
	}
	if c.isFull(-1) {
		// Stop accepting writes until the window opens up again.
		c.state = csConnectedFull
	}
	return sent
}

func (c *Conn) Write(b []byte) (n int, err error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	for len(b) > 0 {
		if err = c.statusErrLocked(c.writeDeadline); err != nil {
			break
		}
		n1 := c.writeNoWait(b)
		b = b[n1:]
		n += n1
		if n1 != 0 {
			continue
		}
		c.cond.Wait()
	}
	return
}

// Close shuts the connection down. Data already written keeps being retransmitted until the peer
// acknowledges it or the connection times out.
func (c *Conn) Close() error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.closeLocked()
	return nil
}

func (c *Conn) closeLocked() {
	if c.closed {
		return
	}
	c.closed = true
	switch c.state {
	case csConnected, csConnectedFull:
		c.readShutdown = true
		c.closeRequested = true
		if !c.finSent {
			c.finSent = true
			c.writeOutgoingPacket(nil, stFin)
		} else if c.finSentAcked {
			c.state = csDestroy
		}
	case csSynSent:
		c.rtoTimeout = c.s.currentMS + uint64(min(c.rto*2, 60))
		c.state = csDestroy
	default:
		c.state = csDestroy
	}
	c.maybeDestroy()
	c.cond.Broadcast()
}

// CloseWrite sends a FIN, so the peer sees end of file, while leaving this end able to read.
func (c *Conn) CloseWrite() error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	switch c.state {
	case csConnected, csConnectedFull:
		if !c.finSent {
			c.finSent = true
			c.writeOutgoingPacket(nil, stFin)
		}
	case csSynSent:
		c.rtoTimeout = c.s.currentMS + uint64(min(c.rto*2, 60))
	}
	c.cond.Broadcast()
	return nil
}

// PeerExtensionBits returns the eight extension bytes the peer sent in its SYN, or zero if it
// sent none. Nothing in µTP itself is negotiated with them; they're here for protocols layered on
// top that want to.
func (c *Conn) PeerExtensionBits() [8]byte {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	return c.extensions
}

func (c *Conn) LocalAddr() net.Addr  { return c.s.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr { return c.addr }

func (c *Conn) SetDeadline(t time.Time) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.setReadDeadlineLocked(t)
	c.setWriteDeadlineLocked(t)
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.setReadDeadlineLocked(t)
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.setWriteDeadlineLocked(t)
	return nil
}

func (c *Conn) setReadDeadlineLocked(t time.Time) {
	c.readDeadline = t
	if t.IsZero() {
		c.readDeadlineTimer.Stop()
	} else {
		c.readDeadlineTimer.Reset(time.Until(t))
	}
	c.cond.Broadcast()
}

func (c *Conn) setWriteDeadlineLocked(t time.Time) {
	c.writeDeadline = t
	if t.IsZero() {
		c.writeDeadlineTimer.Stop()
	} else {
		c.writeDeadlineTimer.Reset(time.Until(t))
	}
	c.cond.Broadcast()
}

// SetWriteBufferLen sets the send buffer, which also caps the congestion window.
func (c *Conn) SetWriteBufferLen(n int) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.optSndBuf = n
}

// SetReadBufferLen sets the receive buffer, which is what the advertised receive window is
// computed from.
func (c *Conn) SetReadBufferLen(n int) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.optRcvBuf = n
}

// SetTargetDelay sets the one way queuing delay the congestion controller aims for. Lower means
// yielding sooner to competing traffic.
func (c *Conn) SetTargetDelay(d time.Duration) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.targetDelay = int(d / time.Microsecond)
}
