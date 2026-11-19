// Package purego implements the Micro Transport Protocol (µTP, BEP 29) in pure Go.
//
// µTP is a reliable, ordered, stream oriented transport that runs over UDP. What distinguishes it
// from TCP is its congestion control: LEDBAT measures the one way queuing delay towards the peer
// and grows the window towards a fixed delay target rather than until something is dropped. A
// bulk transfer over µTP therefore backs off as soon as it starts filling the buffer in front of
// the bottleneck, leaving the link responsive for interactive traffic sharing it.
//
// The protocol logic here is a port of BitTorrent's libutp, which is the reference implementation
// and what the rest of the network is running. The state machine, the LEDBAT controller, the
// selective acknowledgement and fast resend rules, the retransmission timers and the MTU search
// all follow it closely, including the constants they're tuned with, so that a connection between
// this package and any other µTP implementation behaves the way both ends expect. The interop
// tests in the repository's interop package check exactly that against libutp itself.
//
// # Using it
//
// A [Socket] owns a UDP port and multiplexes connections over it. It implements both
// [net.Listener] and [net.PacketConn], and the connections it hands out implement [net.Conn]:
//
//	s, err := purego.NewSocket("udp", ":4242")
//	if err != nil {
//		return err
//	}
//	defer s.Close()
//	c, err := s.DialContext(ctx, "", "example.com:4242")
//
// Datagrams arriving on the port that aren't µTP are not dropped: they're delivered through
// [Socket.ReadFrom], so one port can carry µTP connections and another protocol, a DHT say, at
// the same time. [NewSocketFromPacketConn] takes that further and runs µTP over a
// [net.PacketConn] obtained elsewhere; if that PacketConn doesn't use UDP addresses, pass
// [WithAddrResolver] so Dial knows how to turn an address string into one it accepts.
//
// Nothing outside the standard library is imported, logging included: a Socket logs to a
// [log/slog.Logger], its own if [WithLogger] gave it one and otherwise slog's default. The package
// builds for every platform Go does.
//
// # Differences from libutp
//
// Two of libutp's inputs aren't available portably from Go, and their absence only affects how
// quickly the MTU search converges, not correctness:
//
//   - ICMP fragmentation-needed and unreachable reports are not fed back in, so a path MTU below
//     what we're sending is discovered from the resulting timeouts and duplicate acks instead of
//     being reported outright.
//   - The don't-fragment bit is not set on MTU probes, so an oversized probe is fragmented along
//     the way rather than dropped. The search still runs; it just tends to settle high.
//
// Everything else follows libutp, with four deliberate departures where following it would be
// worse:
//
//   - A repeated SYN for a connection still in the handshake gets the syn-ack sent again. Nothing
//     else retransmits it, so a single lost syn-ack otherwise strands the accepted connection
//     until the dialer gives up.
//   - The sender flushes its queue when acks free up window space, rather than waiting for the
//     application's next write or for the 500ms timer. A sender that queued more than the window
//     and then stopped writing otherwise trickles out at one window per 500ms.
//   - A reset arriving on a connection still being dialled is reported as a refused connection.
//     libutp overwrites the state before testing it, so it always reports a reset one.
//   - A selective acknowledgement is read one bit short of where libutp reads it, which is one
//     byte past the end of the header.
package purego
