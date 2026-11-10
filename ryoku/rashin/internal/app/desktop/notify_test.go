package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type postedNotice struct {
	app      string
	replaces uint32
	title    string
	body     string
	actions  []string
	entry    string
}

// fakeNotifications is the desktop's notification service as far as a client sees it.
type fakeNotifications struct {
	posted chan postedNotice
	next   uint32
}

func (f *fakeNotifications) Notify(
	app string, replaces uint32, icon, title, body string, actions []string, hints map[string]dbus.Variant, expire int32,
) (uint32, *dbus.Error) {
	entry, _ := hints["desktop-entry"].Value().(string)
	f.posted <- postedNotice{app: app, replaces: replaces, title: title, body: body, actions: actions, entry: entry}
	if replaces != 0 {
		return replaces, nil
	}
	f.next++
	return f.next, nil
}

func TestANoticeReachesTheDesktopAndAPressComesBack(t *testing.T) {
	service := privateBus(t)
	fake := &fakeNotifications{posted: make(chan postedNotice, 4), next: 6}
	if err := service.Export(fake, notifyPath, notifyInterface); err != nil {
		t.Fatal(err)
	}
	own(t, service, notifyName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pressed := make(chan string, 1)
	notifier, err := NewNotifier(ctx, func(key string) { pressed <- key })
	if err != nil {
		t.Fatal(err)
	}
	defer notifier.Close()

	if err := notifier.Post(ctx, Notice{Key: "session-1", Title: "Add the panel needs you", Body: "Run the tests?"}); err != nil {
		t.Fatal(err)
	}
	first := <-fake.posted
	if first.app != "Rashin" || first.title != "Add the panel needs you" || first.body != "Run the tests?" || first.replaces != 0 {
		t.Fatalf("the desktop was sent %+v", first)
	}
	if first.entry != desktopEntry || len(first.actions) != 2 || first.actions[0] != "default" {
		t.Fatalf("the notice does not name the app or cannot be pressed: %+v", first)
	}

	// what the same session says next takes the place of the first notice
	if err := notifier.Post(ctx, Notice{Key: "session-1", Title: "Add the panel finished"}); err != nil {
		t.Fatal(err)
	}
	if second := <-fake.posted; second.replaces != 7 {
		t.Fatalf("the second notice about one session replaces %d, want the first one's id 7", second.replaces)
	}

	if err := service.Emit(notifyPath, notifyInterface+".ActionInvoked", uint32(7), "default"); err != nil {
		t.Fatal(err)
	}
	select {
	case key := <-pressed:
		if key != "session-1" {
			t.Fatalf("pressing the notice reported %q", key)
		}
	case <-ctx.Done():
		t.Fatal("pressing the notice was never reported")
	}
}
