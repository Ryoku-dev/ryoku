// Package notify posts one desktop notification through the session bus's
// notification service. It is how a reflex or a reminder still reaches the
// user while Oni is hidden. One connection is opened per notice and closed
// after it; nothing listens for presses.
package notify

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/godbus/dbus/v5"

	"ryoku-rashin/internal/reflex/bus"
)

const (
	dest   = "org.freedesktop.Notifications"
	path   = "/org/freedesktop/Notifications"
	method = "org.freedesktop.Notifications.Notify"

	// AppName is the sender the service shows.
	AppName = "Hyakki"
	// DesktopEntry is the desktop file the service takes the icon from.
	DesktopEntry = "dev.ryoku.Hyakki"

	// MaxSummary and MaxBody bound a notice, in runes.
	MaxSummary = 120
	MaxBody    = 600
)

// ErrBadNotice means a summary is empty or a summary or body is over its bound; nothing was sent.
var ErrBadNotice = errors.New("notify: empty or overlong notice")

// AllowedCall reports whether one bus call is the single Notify this
// package makes: from AppName, replacing nothing, no icon, no actions, only
// the desktop-entry hint, the service's own timeout, and a summary and body
// within their bounds.
func AllowedCall(d, p, m string, args []any) bool {
	if d != dest || p != path || m != method || len(args) != 8 {
		return false
	}
	summary, ok1 := args[3].(string)
	body, ok2 := args[4].(string)
	actions, ok3 := args[5].([]string)
	hints, ok4 := args[6].(map[string]dbus.Variant)
	timeout, ok5 := args[7].(int32)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return false
	}
	entry, _ := hints["desktop-entry"].Value().(string)
	return args[0] == AppName && args[1] == uint32(0) && args[2] == "" &&
		summary != "" && utf8.RuneCountInString(summary) <= MaxSummary &&
		utf8.RuneCountInString(body) <= MaxBody &&
		len(actions) == 0 && len(hints) == 1 && entry == DesktopEntry && timeout == -1
}

// Notifier posts notices.
type Notifier struct{ dial bus.Dial }

// New returns a Notifier that opens a connection with dial for each notice
// (bus.Session for the real one). Every call is checked against
// AllowedCall.
func New(dial bus.Dial) *Notifier { return &Notifier{dial: bus.Guard(AllowedCall, dial)} }

// Post shows one notice and returns the service's id for it.
func (n *Notifier) Post(ctx context.Context, summary, body string) (uint32, error) {
	if summary == "" || utf8.RuneCountInString(summary) > MaxSummary || utf8.RuneCountInString(body) > MaxBody {
		return 0, ErrBadNotice
	}
	conn, err := n.dial(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(DesktopEntry)}
	var id uint32
	err = conn.Call(ctx, dest, path, method,
		[]any{AppName, uint32(0), "", summary, body, []string{}, hints, int32(-1)}, &id)
	return id, err
}
