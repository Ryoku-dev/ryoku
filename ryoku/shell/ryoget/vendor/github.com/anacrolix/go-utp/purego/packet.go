package purego

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// The only protocol version this package speaks. Version 0 was uTorrent's original wire
	// format and hasn't been seen in the wild for a very long time.
	protocolVersion = 1
	// Size of the fixed part of a version 1 header.
	headerSize = 20
)

type packetType uint8

const (
	stData  packetType = 0
	stFin   packetType = 1
	stState packetType = 2
	stReset packetType = 3
	stSyn   packetType = 4

	// One past the last valid type. Packets with a type at or above this are not uTP.
	stNumStates packetType = 5
)

func (me packetType) String() string {
	switch me {
	case stData:
		return "ST_DATA"
	case stFin:
		return "ST_FIN"
	case stState:
		return "ST_STATE"
	case stReset:
		return "ST_RESET"
	case stSyn:
		return "ST_SYN"
	default:
		return fmt.Sprintf("packetType(%d)", uint8(me))
	}
}

// Extension header types. The chain is terminated by extNone.
const (
	extNone         = 0
	extSelectiveAck = 1
	extBits         = 2
)

// The fixed part of a version 1 uTP header.
type header struct {
	Type    packetType
	Version uint8
	// The first extension in the chain, or extNone.
	Extension uint8
	ConnID    uint16
	// Microseconds since some arbitrary epoch of the sender, when the packet was sent.
	TimestampMicros uint32
	// The sender's most recent one way delay measurement, in microseconds.
	TimestampDiffMicros uint32
	// Bytes of receive buffer the sender still has free.
	WndSize uint32
	SeqNr   uint16
	AckNr   uint16
}

func (h *header) marshalTo(b []byte) {
	_ = b[headerSize-1]
	b[0] = byte(h.Type)<<4 | h.Version&0xf
	b[1] = h.Extension
	binary.BigEndian.PutUint16(b[2:4], h.ConnID)
	binary.BigEndian.PutUint32(b[4:8], h.TimestampMicros)
	binary.BigEndian.PutUint32(b[8:12], h.TimestampDiffMicros)
	binary.BigEndian.PutUint32(b[12:16], h.WndSize)
	binary.BigEndian.PutUint16(b[16:18], h.SeqNr)
	binary.BigEndian.PutUint16(b[18:20], h.AckNr)
}

func (h *header) unmarshal(b []byte) {
	_ = b[headerSize-1]
	h.Type = packetType(b[0] >> 4)
	h.Version = b[0] & 0xf
	h.Extension = b[1]
	h.ConnID = binary.BigEndian.Uint16(b[2:4])
	h.TimestampMicros = binary.BigEndian.Uint32(b[4:8])
	h.TimestampDiffMicros = binary.BigEndian.Uint32(b[8:12])
	h.WndSize = binary.BigEndian.Uint32(b[12:16])
	h.SeqNr = binary.BigEndian.Uint16(b[16:18])
	h.AckNr = binary.BigEndian.Uint16(b[18:20])
}

// Sets the fields that change between transmissions of the same buffer, in place. libutp stamps
// these immediately before handing the packet to the socket so the timestamp is as fresh as
// possible.
func stampHeader(b []byte, timestampMicros, timestampDiffMicros uint32) {
	binary.BigEndian.PutUint32(b[4:8], timestampMicros)
	binary.BigEndian.PutUint32(b[8:12], timestampDiffMicros)
}

func setHeaderAckNr(b []byte, ackNr uint16) {
	binary.BigEndian.PutUint16(b[18:20], ackNr)
}

func headerSeqNr(b []byte) uint16 {
	return binary.BigEndian.Uint16(b[16:18])
}

// A parsed incoming packet. The payload and selective ack alias the buffer they were parsed from,
// so neither outlives the read that produced them.
type packet struct {
	h header
	// The selective ack bitmask, if the packet carried one. Not copied.
	selAck []byte
	// Set from an extension bits header, if the packet carried one.
	extBits    [8]byte
	hasExtBits bool
	payload    []byte
}

var (
	errPacketTooSmall     = errors.New("packet smaller than header")
	errUnsupportedVersion = errors.New("unsupported protocol version")
	errBadPacketType      = errors.New("unrecognized packet type")
	errBadExtensions      = errors.New("malformed extension headers")
)

// Parses a version 1 uTP packet. Anything this rejects is treated as not being uTP at all, which
// is what lets a Socket share a port with other protocols.
func parsePacket(b []byte) (p packet, err error) {
	if len(b) < headerSize {
		err = errPacketTooSmall
		return
	}
	p.h.unmarshal(b)
	if p.h.Type >= stNumStates {
		err = errBadPacketType
		return
	}
	// libutp only considers a packet's version meaningful if the type and first extension are
	// plausible, so that garbage doesn't get mistaken for a version it does support.
	if p.h.Version != protocolVersion || p.h.Extension > extBits {
		err = errUnsupportedVersion
		return
	}
	// Walk the extension chain. Each header is [next type][length][length bytes of payload].
	i := headerSize
	for ext := p.h.Extension; ext != extNone; {
		if i+2 > len(b) {
			err = errBadExtensions
			return
		}
		next := b[i]
		n := int(b[i+1])
		i += 2
		if i+n > len(b) {
			err = errBadExtensions
			return
		}
		switch ext {
		case extSelectiveAck:
			p.selAck = b[i : i+n]
		case extBits:
			if n != 8 {
				err = errBadExtensions
				return
			}
			copy(p.extBits[:], b[i:i+n])
			p.hasExtBits = true
		}
		i += n
		ext = next
	}
	p.payload = b[i:]
	return
}
