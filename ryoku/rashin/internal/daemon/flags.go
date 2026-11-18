package daemon

import (
	"context"

	"ryoku-rashin/internal/protocol"
)

func (d *Daemon) sessionSetFlags(ctx context.Context, c *connection, p protocol.SessionFlagsParams) (protocol.SessionInfo, error) {
	if !d.feature("history") {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeDisabled, "Turn on Star and archive in Settings first.")
	}
	s, err := d.manager.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	info := s.log.Info()
	if info.Kind != protocol.KindAsk || info.Temporary {
		s.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "Only saved Ask conversations can be starred or archived.")
	}
	if err := s.log.SetFlags(p.Starred, p.Archived); err != nil {
		s.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	info = s.log.Info()
	d.manager.emitInfoLocked(s)
	s.mu.Unlock()
	d.manager.changed()
	return info, nil
}
