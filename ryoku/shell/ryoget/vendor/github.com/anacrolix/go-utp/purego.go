package utp

import (
	"net"

	"github.com/anacrolix/go-utp/purego"
)

// PureGo is the pure Go implementation, [github.com/anacrolix/go-utp/purego]. It is available
// whatever the build selected as [Default].
var PureGo Implementation = &implementation{"purego", newPuregoSocket}

// A purego Socket is already the right shape in full, so it needs no adapter: it's returned as
// itself, and a caller that wants the implementation's own API can assert for it.
var _ Socket = (*purego.Socket)(nil)

func newPuregoSocket(pc net.PacketConn, opts ...Option) (Socket, error) {
	o := newOptions(opts)
	var popts []purego.NewSocketOpt
	if o.logger != nil {
		popts = append(popts, purego.WithLogger(o.logger))
	}
	if o.targetDelay != 0 {
		popts = append(popts, purego.WithTargetDelay(o.targetDelay))
	}
	s, err := purego.NewSocketFromPacketConn(pc, popts...)
	if err != nil {
		return nil, err
	}
	if o.sendBuffer != 0 {
		s.SetWriteBufferLen(o.sendBuffer)
	}
	if o.receiveBuffer != 0 {
		s.SetReadBufferLen(o.receiveBuffer)
	}
	return s, nil
}
