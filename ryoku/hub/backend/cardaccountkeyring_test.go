package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func startCardAccountTestKeyring(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", root)
	t.Setenv("GNOME_KEYRING_CONTROL", filepath.Join(root, "keyring"))
	t.Setenv("GNOME_KEYRING_PID", "")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	start := func(cmd *exec.Cmd) {
		t.Helper()
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
		if err := cmd.Start(); err != nil {
			t.Fatal("isolated keyring dependency could not start:", err)
		}
		t.Cleanup(func() {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			cmd.Wait()
		})
	}
	bus := exec.CommandContext(ctx, "/usr/bin/dbus-daemon", "--session", "--nofork", "--print-address=1", "--address=unix:path="+filepath.Join(root, "bus"))
	stdout, err := bus.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	bus.Stderr = &mediaDiagnostic{}
	start(bus)
	addresses := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		addresses <- strings.TrimSpace(line)
	}()
	var address string
	select {
	case address = <-addresses:
	case <-time.After(5 * time.Second):
		t.Fatal("private session bus did not become ready:", bus.Stderr)
	}
	if !strings.HasPrefix(address, "unix:path="+filepath.Join(root, "bus")) {
		t.Fatal("unexpected private bus address:", bus.Stderr)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err = conn.Auth(nil); err != nil {
		t.Fatal(err)
	}
	if err = conn.Hello(); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	if err = conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, "org.freedesktop.secrets")); err != nil {
		t.Fatal(err)
	}
	keyring := exec.CommandContext(ctx, "/usr/bin/gnome-keyring-daemon", "--foreground", "--components=secrets", "--unlock", "--control-directory="+filepath.Join(root, "keyring"))
	keyring.Stdin = strings.NewReader("isolated-test-keyring-password")
	keyring.Stderr = &mediaDiagnostic{}
	keyring.Stdout = &mediaDiagnostic{}
	start(keyring)
	ready := func() bool {
		var owned bool
		return conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.secrets").Store(&owned) == nil && owned
	}
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for !ready() {
		select {
		case <-signals:
		case <-timeout.C:
			t.Fatal("isolated real Secret Service did not become ready:", keyring.Stderr, keyring.Stdout)
		}
	}
	bin := filepath.Join(root, "bin")
	if err = os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("/usr/bin/secret-tool", filepath.Join(bin, "secret-tool")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
