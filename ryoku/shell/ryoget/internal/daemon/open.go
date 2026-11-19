package daemon

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// spawn starts a helper in its own session so it outlives an idle exit.
func spawn(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if devnull, err := os.Open(os.DevNull); err == nil {
		defer devnull.Close()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func openPath(p string) error { return spawn("xdg-open", p) }

// reveal selects the file in the file manager when one answers
// org.freedesktop.FileManager1, else opens the folder.
func reveal(path, dir string) {
	if path != "" {
		if _, err := os.Stat(path); err == nil && showItems(path) == nil {
			return
		}
	}
	_ = spawn("xdg-open", dir)
}

func showItems(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return err
	}
	defer conn.Close()
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return conn.Object("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1").
		CallWithContext(ctx, "org.freedesktop.FileManager1.ShowItems", 0, []string{uri}, "").Err
}
