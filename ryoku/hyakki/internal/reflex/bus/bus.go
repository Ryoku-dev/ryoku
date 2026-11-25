// Package bus is the one place reflex executors talk to the session bus.
// A connection is opened for one action and closed after it, so nothing is
// held while no action runs, and every call is checked against the calling
// executor's allowlist before it leaves.
package bus

import (
	"context"
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// ErrNotAllowed means a call is not on the executor's allowlist. Nothing
// was sent.
var ErrNotAllowed = errors.New("reflex: bus call not allowed")

// Conn is one session-bus connection. method is "interface.Member"; reply
// receives the method's return values in order.
type Conn interface {
	Call(ctx context.Context, dest, path, method string, args []any, reply ...any) error
	Close() error
}

// Dial opens a connection. Tests hand in a fake; Session is the real one.
type Dial func(ctx context.Context) (Conn, error)

// Allow reports whether one call is allowed. args are the call's
// arguments, so an allowlist can pin them too.
type Allow func(dest, path, method string, args []any) bool

// Guard wraps dial so every call on the connections it opens is checked
// against allow first.
func Guard(allow Allow, dial Dial) Dial {
	return func(ctx context.Context) (Conn, error) {
		if dial == nil {
			return nil, errors.New("reflex: no session bus")
		}
		c, err := dial(ctx)
		if err != nil {
			return nil, err
		}
		return guarded{c, allow}, nil
	}
}

type guarded struct {
	Conn
	allow Allow
}

func (g guarded) Call(ctx context.Context, dest, path, method string, args []any, reply ...any) error {
	if !g.allow(dest, path, method, args) {
		return fmt.Errorf("%w: %s %s %s", ErrNotAllowed, dest, path, method)
	}
	return g.Conn.Call(ctx, dest, path, method, args, reply...)
}

// Session dials a private connection to the user's session bus.
func Session(ctx context.Context) (Conn, error) {
	c, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return session{c}, nil
}

type session struct{ c *dbus.Conn }

func (s session) Call(ctx context.Context, dest, path, method string, args []any, reply ...any) error {
	call := s.c.Object(dest, dbus.ObjectPath(path)).CallWithContext(ctx, method, 0, args...)
	if call.Err != nil {
		return call.Err
	}
	if len(reply) == 0 {
		return nil
	}
	return call.Store(reply...)
}

func (s session) Close() error { return s.c.Close() }
