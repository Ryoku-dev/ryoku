// Package desktop asks the desktop around the app for what the app should not
// build itself: the system's file chooser, what the clipboard holds besides
// text, and a notification.
package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"net/url"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	portalName    = "org.freedesktop.portal.Desktop"
	portalPath    = "/org/freedesktop/portal/desktop"
	portalOpen    = "org.freedesktop.portal.FileChooser.OpenFile"
	portalRequest = "org.freedesktop.portal.Request"
)

// no session bus, no portal, or a portal without a file chooser behind it
var ErrNoChooser = errors.New("no file chooser answered")

type ChooseRequest struct {
	Title    string
	Folder   bool
	Multiple bool
	// the folder the chooser opens in
	Start string
}

// the chooser has no window of ours to stand over, so it gets a place the way a launched program does
var chooserFront = image.Pt(920, 640)

// a chooser the reader backed out of returns no paths and no error
func Choose(ctx context.Context, request ChooseRequest) ([]string, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, ErrNoChooser
	}
	defer conn.Close()
	return choose(ctx, conn, request, seenWindows(ctx))
}

// seen is nil when the chooser's window is left where the desktop puts it
func choose(ctx context.Context, conn *dbus.Conn, request ChooseRequest, seen *windows) ([]string, error) {
	token, err := requestToken()
	if err != nil {
		return nil, err
	}
	// the portal answers on a path made of our bus name and this token, so listen before asking
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	handle := dbus.ObjectPath(portalPath + "/request/" + sender + "/" + token)
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface(portalRequest), dbus.WithMatchMember("Response")); err != nil {
		return nil, ErrNoChooser
	}

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"modal":        dbus.MakeVariant(true),
		"multiple":     dbus.MakeVariant(request.Multiple),
		"directory":    dbus.MakeVariant(request.Folder),
	}
	if request.Start != "" {
		options["current_folder"] = dbus.MakeVariant(append([]byte(request.Start), 0))
	}
	var answered dbus.ObjectPath
	call := conn.Object(portalName, portalPath).CallWithContext(ctx, portalOpen, 0, "", request.Title, options)
	if err := call.Store(&answered); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrNoChooser
	}
	if seen != nil {
		go seen.front(ctx, chooserFront)
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case signal, open := <-signals:
			if !open {
				return nil, ErrNoChooser
			}
			if signal.Path != handle && signal.Path != answered {
				continue
			}
			return chosenPaths(signal.Body)
		}
	}
}

func requestToken() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "rashin" + hex.EncodeToString(raw), nil
}

func chosenPaths(body []any) ([]string, error) {
	var response uint32
	var results map[string]dbus.Variant
	if err := dbus.Store(body, &response, &results); err != nil {
		return nil, ErrNoChooser
	}
	if response != 0 {
		return nil, nil
	}
	uris, _ := results["uris"].Value().([]string)
	paths := make([]string, 0, len(uris))
	for _, raw := range uris {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "file" || parsed.Path == "" {
			continue
		}
		paths = append(paths, parsed.Path)
	}
	return paths, nil
}
