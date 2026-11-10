package daemon

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// servingGateway is a gateway whose local API answers "served".
type servingGateway struct{ core.Gateway }

func (g *servingGateway) API() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("served"))
	})
}

func (g *servingGateway) Close() error { return nil }

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func apiDaemon(t *testing.T, gateway core.Gateway) (*Daemon, func(enabled bool, port int)) {
	t.Helper()
	isolateDaemon(t)
	d := New(Deps{Gateway: gateway, Version: "test"})
	t.Cleanup(d.closeAPI)
	set := func(enabled bool, port int) {
		settings := config.Defaults()
		settings.Enabled = true
		settings.API = protocol.APISettings{Enabled: enabled, Port: port}
		d.mu.Lock()
		d.settings = settings
		d.mu.Unlock()
		d.applyAPI()
	}
	return d, set
}

func askPort(port int) (string, error) {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/v1/models")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return string(body), err
}

func TestLocalAPIListensOnlyWhileItIsOn(t *testing.T) {
	d, set := apiDaemon(t, &servingGateway{})
	first, second := freePort(t), freePort(t)

	set(false, first)
	if _, err := askPort(first); err == nil {
		t.Fatal("the port answered while the local API was off")
	}
	if info := d.apiInfo(); info.Listening || info.Enabled {
		t.Fatalf("info while off = %#v", info)
	}

	set(true, first)
	if body, err := askPort(first); err != nil || body != "served" {
		t.Fatalf("on: %q, %v", body, err)
	}
	if info := d.apiInfo(); !info.Listening || info.URL != "http://127.0.0.1:"+strconv.Itoa(first)+"/v1" {
		t.Fatalf("info while on = %#v", info)
	}

	set(true, second)
	if _, err := askPort(first); err == nil {
		t.Fatal("the old port still answered after the port changed")
	}
	if body, err := askPort(second); err != nil || body != "served" {
		t.Fatalf("on the new port: %q, %v", body, err)
	}

	set(false, second)
	if _, err := askPort(second); err == nil {
		t.Fatal("the port still answered after the local API was turned off")
	}
}

func TestLocalAPISaysWhenItsPortIsTaken(t *testing.T) {
	taken, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	d, set := apiDaemon(t, &servingGateway{})
	set(true, taken.Addr().(*net.TCPAddr).Port)
	if info := d.apiInfo(); info.Listening || info.Error == "" {
		t.Fatalf("info with the port taken = %#v", info)
	}
}

func stopped(d *Daemon) bool {
	select {
	case <-d.stop:
		return true
	default:
		return false
	}
}

func TestTheDaemonStaysUpWhileTheLocalAPIListens(t *testing.T) {
	d, set := apiDaemon(t, &servingGateway{})
	port := freePort(t)
	set(true, port)
	d.onIdleDeadline()
	if stopped(d) {
		t.Fatal("an idle daemon exited while its local API was on")
	}
	if body, err := askPort(port); err != nil || body != "served" {
		t.Fatalf("after the idle deadline: %q, %v", body, err)
	}
	set(false, port)
	d.onIdleDeadline()
	if !stopped(d) {
		t.Fatal("an idle daemon stayed up after its local API was turned off")
	}
}

func TestAPortThatIsTakenDoesNotHoldTheDaemonUp(t *testing.T) {
	taken, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	d, set := apiDaemon(t, &servingGateway{})
	set(true, taken.Addr().(*net.TCPAddr).Port)
	d.onIdleDeadline()
	if !stopped(d) {
		t.Fatal("a daemon that serves nothing stayed up")
	}
}

func TestShutdownClosesTheLocalAPI(t *testing.T) {
	d, set := apiDaemon(t, &servingGateway{})
	port := freePort(t)
	set(true, port)
	d.Shutdown()
	if _, err := askPort(port); err == nil {
		t.Fatal("the port still answered after shutdown")
	}
}
