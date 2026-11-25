package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type asked struct {
	title   string
	options map[string]dbus.Variant
}

// fakePortal answers OpenFile the way the desktop portal does: with a request
// path at once, and what was chosen in a Response signal on that path.
type fakePortal struct {
	service  *dbus.Conn
	asked    chan asked
	response uint32
	uris     []string
}

func (f *fakePortal) OpenFile(sender dbus.Sender, parent, title string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	f.asked <- asked{title: title, options: options}
	token, _ := options["handle_token"].Value().(string)
	name := []byte(string(sender)[1:])
	for index, character := range name {
		if character == '.' {
			name[index] = '_'
		}
	}
	handle := dbus.ObjectPath(portalPath + "/request/" + string(name) + "/" + token)
	go func() {
		results := map[string]dbus.Variant{"uris": dbus.MakeVariant(f.uris)}
		_ = f.service.Emit(handle, portalRequest+".Response", f.response, results)
	}()
	return handle, nil
}

func portal(t *testing.T, response uint32, uris ...string) *fakePortal {
	t.Helper()
	service := privateBus(t)
	fake := &fakePortal{service: service, asked: make(chan asked, 1), response: response, uris: uris}
	if err := service.Export(fake, portalPath, "org.freedesktop.portal.FileChooser"); err != nil {
		t.Fatal(err)
	}
	own(t, service, portalName)
	return fake
}

func TestChooseAsksThePortalForAFolderAndReturnsItsPath(t *testing.T) {
	fake := portal(t, 0, "file:///home/someone/My%20Work/repo")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paths, err := Choose(ctx, ChooseRequest{Title: "Add a project", Folder: true, Start: "/home/someone"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/home/someone/My Work/repo" {
		t.Fatalf("chosen = %q", paths)
	}
	request := <-fake.asked
	directory, _ := request.options["directory"].Value().(bool)
	multiple, _ := request.options["multiple"].Value().(bool)
	if request.title != "Add a project" || !directory || multiple {
		t.Fatalf("the portal was asked %q with %v", request.title, request.options)
	}
}

func TestChooseReturnsNothingWhenTheReaderBacksOut(t *testing.T) {
	portal(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paths, err := Choose(ctx, ChooseRequest{Title: "Attach files", Multiple: true})
	if err != nil || len(paths) != 0 {
		t.Fatalf("a chooser that was closed gave %q, %v", paths, err)
	}
}

func TestChooseSaysSoWhenNoChooserAnswers(t *testing.T) {
	privateBus(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Choose(ctx, ChooseRequest{Title: "Attach files"}); !errors.Is(err, ErrNoChooser) {
		t.Fatalf("with no portal on the bus Choose returned %v, want ErrNoChooser", err)
	}
}
