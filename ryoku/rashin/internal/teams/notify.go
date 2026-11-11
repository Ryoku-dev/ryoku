package teams

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

const (
	notifyName      = "org.freedesktop.Notifications"
	notifyPath      = "/org/freedesktop/Notifications"
	notifyInterface = "org.freedesktop.Notifications"
	// desktopEntry is the desktop file the notification service takes
	// Rashin's name and icon from
	desktopEntry = "dev.ryoku.rashin"
	// notifyListen bounds how long a notification's Open stays answered
	notifyListen = 12 * time.Hour
)

// notification is one desktop notification about a team: Needs marks one
// that waits on the user, which a team set to notify only then still sends.
type notification struct {
	Team, Task, Title, Body string
	Needs                   bool
}

// notify shows a desktop notification while no Rashin window is open, as far
// as the team's notify level and the quiet hours allow.
func (s *Service) notify(n notification) {
	if route, ok := s.notifyRoute(n); ok {
		go s.post(n, route)
	}
}

// notifyRoute says whether a notification reaches the desktop, and where
// its Open action leads: the Inbox for what needs the user and the task for
// a report. Nothing does while a window is open, inside the quiet hours, or
// beyond the team's notify level; what needs the user still waits in the
// Inbox.
func (s *Service) notifyRoute(n notification) (string, bool) {
	if s.sessions == nil || s.sessions.Watched() {
		return "", false
	}
	if quiet := s.settings().Teams.QuietHours; quiet != nil && quiet.Covers(s.now()) {
		return "", false
	}
	team, ok := s.state.team(n.Team)
	if !ok {
		return "", false
	}
	switch team.Notify {
	case protocol.NotifyNone:
		return "", false
	case protocol.NotifyNeeds:
		if !n.Needs {
			return "", false
		}
	}
	if !n.Needs && n.Task != "" {
		return "work/" + n.Task, true
	}
	return "inbox", true
}

// notifyEnd tells the user a task they gave ended: its report, or that it
// stopped short. Pieces a lead handed out go back to the lead instead.
func (s *Service) notifyEnd(t protocol.TeamTask) {
	if t.Parent != "" {
		return
	}
	team, ok := s.state.team(t.Team)
	if !ok {
		return
	}
	switch t.Status {
	case protocol.TaskDone:
		title := t.Title
		body := ""
		if t.Report != nil {
			title, body = t.Report.Title, t.Report.Body
		}
		s.notify(notification{Team: team.ID, Task: t.ID, Title: i18n.Tf("%s: %s", team.Name, title), Body: clipRunes(strings.TrimSpace(body), 200)})
	case protocol.TaskBlocked:
		s.notify(notification{Team: team.ID, Task: t.ID, Title: i18n.Tf("%s is blocked: %s", team.Name, t.Title), Body: clipRunes(t.Reason, 200), Needs: true})
	}
}

// post sends a notification over the session bus and, while it is on
// screen, opens Rashin on route when the user presses it. Every failure is
// ignored: a desktop without a notification service still has the Inbox.
func (s *Service) post(n notification, route string) {
	ctx, cancel := context.WithTimeout(s.background, notifyListen)
	defer cancel()
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return
	}
	defer conn.Close()
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface(notifyInterface)); err != nil {
		return
	}
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(desktopEntry)}
	// -1 leaves how long it stays to the notification service
	call := conn.Object(notifyName, notifyPath).CallWithContext(ctx, notifyInterface+".Notify", 0,
		"Rashin", uint32(0), desktopEntry, n.Title, n.Body, []string{"default", i18n.T("Open")}, hints, int32(-1))
	var id uint32
	if call.Store(&id) != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case signal, open := <-signals:
			if !open {
				return
			}
			if len(signal.Body) == 0 {
				continue
			}
			if posted, ok := signal.Body[0].(uint32); !ok || posted != id {
				continue
			}
			switch signal.Name {
			case notifyInterface + ".ActionInvoked":
				openRoute(route)
				return
			case notifyInterface + ".NotificationClosed":
				return
			}
		}
	}
}

// openRoute shows route in Rashin: rashin open hands it to a running window
// or starts the app on it.
func openRoute(route string) {
	command := rashinCommand()
	if command == "" {
		return
	}
	open := exec.Command(command, "open", "--route", route)
	if open.Start() == nil {
		go func() { _ = open.Wait() }()
	}
}

// rashinCommand is the rashin command installed beside the daemon, or the
// one on PATH.
func rashinCommand() string {
	if program, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(program), "rashin")
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}
	path, _ := exec.LookPath("rashin")
	return path
}
