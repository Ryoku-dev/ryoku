package desktop

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// a session bus that knows no service files: asking it for the portal can
// never start the real one, which would open a chooser on the desktop the
// test runs on
const quietBus = `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:dir=%DIR%</listen>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
`

// privateBus starts that bus and points this test at it, so the services it
// fakes are the only ones there.
func privateBus(t *testing.T) *dbus.Conn {
	t.Helper()
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("no dbus-daemon on this machine")
	}
	directory := t.TempDir()
	config := filepath.Join(directory, "bus.conf")
	if err := os.WriteFile(config, []byte(strings.ReplaceAll(quietBus, "%DIR%", directory)), 0o600); err != nil {
		t.Fatal(err)
	}
	bus := exec.Command(daemon, "--config-file="+config, "--nofork", "--print-address=1")
	out, err := bus.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bus.Process.Kill()
		_ = bus.Wait()
	})
	address, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(address))
	service, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func own(t *testing.T, service *dbus.Conn, name string) {
	t.Helper()
	reply, err := service.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("could not own %s on the test bus: %v, %v", name, reply, err)
	}
}
