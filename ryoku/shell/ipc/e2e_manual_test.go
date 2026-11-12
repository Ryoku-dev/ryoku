package main

import (
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

const e2eLabel = "ryoku-e2e-island"

// Throwaway manual driver: triggers a real CreateCollection prompt against the
// running dev shell's island and waits for the user (wtype) to answer it. Proves
// the QML island -> control socket -> daemon -> gnome-keyring path end to end. A
// non-blank password it accepts (no "store unencrypted" confirm) means the secret
// crossed the socket intact.
func TestE2ECreateViaIsland(t *testing.T) {
	if os.Getenv("RYOKU_KEYRING_E2E") != "1" {
		t.Skip("set RYOKU_KEYRING_E2E=1 to run the manual island E2E against the live session")
	}
	useLiveSession(t)
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	secrets := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	var coll, prompt dbus.ObjectPath
	props := map[string]dbus.Variant{
		"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant(e2eLabel),
	}
	if err := secrets.Call("org.freedesktop.Secret.Service.CreateCollection", 0, props, "").Store(&coll, &prompt); err != nil {
		t.Fatal(err)
	}
	if coll == "/" {
		res, dismissed := runSecretPrompt(t, conn, prompt)
		if dismissed {
			t.Fatal("island dismissed the prompt")
		}
		coll = res.Value().(dbus.ObjectPath)
	}
	if coll == "/" || coll == "" {
		t.Fatal("no collection created via island")
	}
	t.Logf("collection created via island: %s", coll)
}

// TestE2ECleanup removes the throwaway collection, auto-answering any prompt.
// Run after the deployed shell is restored so it can own the prompter name.
func TestE2ECleanup(t *testing.T) {
	if os.Getenv("RYOKU_KEYRING_E2E") != "1" {
		t.Skip("set RYOKU_KEYRING_E2E=1 to run the manual island E2E against the live session")
	}
	useLiveSession(t)
	pConn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer pConn.Close()
	p := &prompter{conn: pConn, prompts: map[dbus.ObjectPath]*promptSession{}}
	p.onShow = func(id int, ptype string, props map[string]interface{}) {
		go p.respond(id, "continue", false, "E2EPW")
	}
	if err := pConn.Export(p, dbus.ObjectPath(prompterPath), prompterIface); err != nil {
		t.Fatal(err)
	}
	if reply, _ := pConn.RequestName(prompterName, dbus.NameFlagReplaceExisting|dbus.NameFlagDoNotQueue); reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skip("prompter name owned elsewhere; skip cleanup")
	}

	cConn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer cConn.Close()
	secrets := cConn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	v, err := secrets.GetProperty("org.freedesktop.Secret.Service.Collections")
	if err != nil {
		t.Fatal(err)
	}
	colls, _ := v.Value().([]dbus.ObjectPath)
	for _, c := range colls {
		lv, err := cConn.Object("org.freedesktop.secrets", c).GetProperty("org.freedesktop.Secret.Collection.Label")
		if err != nil {
			continue
		}
		if s, _ := lv.Value().(string); s == e2eLabel {
			t.Logf("deleting %s", c)
			deleteCollection(t, cConn, c)
		}
	}
}
