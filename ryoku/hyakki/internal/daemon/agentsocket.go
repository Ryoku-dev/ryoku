package daemon

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"

	"ryoku-rashin/internal/paths"
)

// agentSockets are the sockets team sessions reach the daemon through from
// inside their sandboxes: one per running session, bound to it.
type agentSockets struct {
	mu    sync.Mutex
	items map[string]net.Listener
}

// openAgentSocket listens on a team session's own socket, unless it already
// does, and returns the dir that holds it.
func (d *Daemon) openAgentSocket(id string) (string, error) {
	socket := paths.AgentSocket(id)
	dir := filepath.Dir(socket)
	d.agents.mu.Lock()
	defer d.agents.mu.Unlock()
	if d.agents.items[id] != nil {
		return dir, nil
	}
	if err := secureDir(paths.AgentSocketsDir()); err != nil {
		return "", err
	}
	_ = os.RemoveAll(dir)
	if err := secureDir(dir); err != nil {
		return "", err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = listener.Close()
		return "", err
	}
	if d.agents.items == nil {
		d.agents.items = make(map[string]net.Listener)
	}
	d.agents.items[id] = listener
	go d.serveAgent(id, listener)
	return dir, nil
}

// serveAgent takes connections on a session's own socket, each bound to that
// session and never taken for the app.
func (d *Daemon) serveAgent(id string, listener net.Listener) {
	for {
		nc, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			d.warn("accept on the agent socket of %s: %v", id, err)
			continue
		}
		if _, err := verifyPeer(nc); err != nil {
			d.warn("agent client credentials: %v", err)
			_ = nc.Close()
			continue
		}
		conn := newConnection(d, nc)
		conn.session = id
		// added only while the socket is open, so closing it closes every
		// connection it took
		d.agents.mu.Lock()
		open := d.agents.items[id] == listener
		if open {
			d.mu.Lock()
			d.clients[conn] = struct{}{}
			d.mu.Unlock()
		}
		d.agents.mu.Unlock()
		if !open {
			conn.close()
			return
		}
		d.stateChanged()
		go conn.serve()
	}
}

// closeAgentSocket stops a session's own socket, removes it and ends the
// connections it took.
func (d *Daemon) closeAgentSocket(id string) {
	d.agents.mu.Lock()
	listener := d.agents.items[id]
	delete(d.agents.items, id)
	d.agents.mu.Unlock()
	if listener == nil {
		return
	}
	_ = listener.Close()
	_ = os.RemoveAll(filepath.Dir(paths.AgentSocket(id)))
	var bound []*connection
	d.mu.RLock()
	for c := range d.clients {
		if c.session == id {
			bound = append(bound, c)
		}
	}
	d.mu.RUnlock()
	for _, c := range bound {
		c.close()
	}
}

// closeAgentSockets stops every session's own socket.
func (d *Daemon) closeAgentSockets() {
	d.agents.mu.Lock()
	ids := make([]string, 0, len(d.agents.items))
	for id := range d.agents.items {
		ids = append(ids, id)
	}
	d.agents.mu.Unlock()
	for _, id := range ids {
		d.closeAgentSocket(id)
	}
}
