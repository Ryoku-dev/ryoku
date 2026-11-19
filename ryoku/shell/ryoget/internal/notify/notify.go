// Package notify posts ryoget's desktop notifications over
// org.freedesktop.Notifications and hears which action the reader pressed.
// The session bus is only dialled when the first notice is posted.
package notify

import (
	"context"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	i18n "ryoku-i18n"
)

const (
	busName   = "org.freedesktop.Notifications"
	busPath   = "/org/freedesktop/Notifications"
	busIface  = "org.freedesktop.Notifications"
	appName   = "Downloads"
	appIcon   = "folder-download"
	ActOpen   = "default"
	ActFolder = "folder"
	// how long a notice may keep the daemon up waiting for a press
	PendingFor = 120 * time.Second
)

type Notice struct {
	Title, Body string
	// the finished file and its folder; a notice with a path offers Open and Show in folder
	Path, Dir string
}

type posted struct {
	notice Notice
	timer  *time.Timer
}

type Notifier struct {
	onAction func(Notice, string)
	onChange func()

	mu      sync.Mutex
	conn    *dbus.Conn
	signals chan *dbus.Signal
	pending map[uint32]*posted
}

// New returns a notifier; onAction hears a press, onChange hears whenever the
// pending set changes. Both run on the notifier's own goroutines.
func New(onAction func(Notice, string), onChange func()) *Notifier {
	return &Notifier{onAction: onAction, onChange: onChange, pending: map[uint32]*posted{}}
}

func (n *Notifier) dial(ctx context.Context) (*dbus.Conn, error) {
	if n.conn != nil {
		return n.conn, nil
	}
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(context.Background()))
	if err != nil {
		return nil, err
	}
	n.signals = make(chan *dbus.Signal, 16)
	conn.Signal(n.signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface(busIface), dbus.WithMatchObjectPath(busPath)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	n.conn = conn
	go n.listen(n.signals)
	return conn, nil
}

func (n *Notifier) Post(ctx context.Context, notice Notice) error {
	n.mu.Lock()
	conn, err := n.dial(ctx)
	n.mu.Unlock()
	if err != nil {
		return err
	}
	var actions []string
	if notice.Path != "" {
		actions = []string{ActOpen, i18n.T("Open"), ActFolder, i18n.T("Show in folder")}
	}
	hints := map[string]dbus.Variant{}
	call := conn.Object(busName, busPath).CallWithContext(ctx, busIface+".Notify", 0,
		appName, uint32(0), appIcon, notice.Title, notice.Body, actions, hints, int32(-1))
	var id uint32
	if err := call.Store(&id); err != nil {
		return err
	}
	if len(actions) == 0 {
		return nil
	}
	p := &posted{notice: notice}
	n.mu.Lock()
	n.pending[id] = p
	p.timer = time.AfterFunc(PendingFor, func() { n.forget(id) })
	n.mu.Unlock()
	n.changed()
	return nil
}

func (n *Notifier) changed() {
	if n.onChange != nil {
		n.onChange()
	}
}

func (n *Notifier) forget(id uint32) (Notice, bool) {
	n.mu.Lock()
	p, ok := n.pending[id]
	if ok {
		delete(n.pending, id)
		p.timer.Stop()
	}
	n.mu.Unlock()
	if !ok {
		return Notice{}, false
	}
	n.changed()
	return p.notice, true
}

// Pending reports a notice still waiting for a press.
func (n *Notifier) Pending() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.pending) > 0
}

func (n *Notifier) listen(signals <-chan *dbus.Signal) {
	for sig := range signals {
		if len(sig.Body) == 0 {
			continue
		}
		id, ok := sig.Body[0].(uint32)
		if !ok {
			continue
		}
		switch sig.Name {
		case busIface + ".ActionInvoked":
			action := ""
			if len(sig.Body) > 1 {
				action, _ = sig.Body[1].(string)
			}
			if notice, known := n.forget(id); known && n.onAction != nil {
				n.onAction(notice, action)
			}
		case busIface + ".NotificationClosed":
			n.forget(id)
		}
	}
}

// Close leaves notices on screen; their actions no longer reach anyone.
func (n *Notifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for id, p := range n.pending {
		p.timer.Stop()
		delete(n.pending, id)
	}
	if n.conn != nil {
		_ = n.conn.Close()
		n.conn = nil
	}
}
