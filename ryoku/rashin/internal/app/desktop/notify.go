package desktop

import (
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	notifyName      = "org.freedesktop.Notifications"
	notifyPath      = "/org/freedesktop/Notifications"
	notifyInterface = "org.freedesktop.Notifications"
	// the desktop file the notification service takes the app's name and icon from
	desktopEntry = "dev.ryoku.rashin"
)

type Notice struct {
	// a later notice with the same key replaces it on screen, and a press reports it
	Key         string
	Title, Body string
}

type Notifier struct {
	conn    *dbus.Conn
	signals chan *dbus.Signal

	mu     sync.Mutex
	posted map[string]uint32
}

// pressed hears the key of a notice the reader pressed, on the notifier's own goroutine
func NewNotifier(ctx context.Context, pressed func(key string)) (*Notifier, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	n := &Notifier{conn: conn, signals: make(chan *dbus.Signal, 16), posted: make(map[string]uint32)}
	conn.Signal(n.signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface(notifyInterface)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go n.listen(pressed)
	return n, nil
}

func (n *Notifier) Post(ctx context.Context, notice Notice) error {
	n.mu.Lock()
	replaces := n.posted[notice.Key]
	n.mu.Unlock()
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(desktopEntry)}
	// -1 leaves how long it stays to the notification service
	call := n.conn.Object(notifyName, notifyPath).CallWithContext(ctx, notifyInterface+".Notify", 0,
		"Rashin", replaces, desktopEntry, notice.Title, notice.Body, []string{"default", "Open"}, hints, int32(-1))
	var id uint32
	if err := call.Store(&id); err != nil {
		return err
	}
	n.mu.Lock()
	n.posted[notice.Key] = id
	n.mu.Unlock()
	return nil
}

// notices already on screen stay until the reader dismisses them
func (n *Notifier) Close() { _ = n.conn.Close() }

func (n *Notifier) listen(pressed func(key string)) {
	for signal := range n.signals {
		if len(signal.Body) == 0 {
			continue
		}
		id, ok := signal.Body[0].(uint32)
		if !ok {
			continue
		}
		switch signal.Name {
		case notifyInterface + ".ActionInvoked":
			if key, known := n.keyOf(id, false); known && pressed != nil {
				pressed(key)
			}
		case notifyInterface + ".NotificationClosed":
			n.keyOf(id, true)
		}
	}
}

func (n *Notifier) keyOf(id uint32, forget bool) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for key, posted := range n.posted {
		if posted == id {
			if forget {
				delete(n.posted, key)
			}
			return key, true
		}
	}
	return "", false
}
