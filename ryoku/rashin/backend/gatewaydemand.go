package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gatewayIdle is how long the gateway outlives the last thing Rashin did with it
var gatewayIdle = 15 * time.Minute

var (
	gatewayUseMu   sync.Mutex
	gatewayLastUse time.Time
)

func noteGatewayUse() {
	gatewayUseMu.Lock()
	gatewayLastUse = gatewayNow()
	gatewayUseMu.Unlock()
}

// useGateway starts Prowl's gateway when it is down and waits until it answers.
// It reports whether the gateway is up, false as well when Prowl is not installed.
func useGateway(wait time.Duration) bool {
	if _, installed := findProwl(); !installed {
		return false
	}
	noteGatewayUse()
	if prowlGatewayRunning() {
		return true
	}
	if startGateway() != nil {
		return false
	}
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(150 * time.Millisecond) {
		if prowlGatewayRunning() {
			return true
		}
	}
	return false
}

// useGatewayFor starts the gateway for a harness Prowl routes, since it can't reach a model without it
func useGatewayFor(cfg Config, harness string) {
	if cfg.HasProwlHarness(harness) {
		useGateway(10 * time.Second)
	}
}

// keptGateway starts the gateway only when the user keeps it on, everything else starts it on use
func keptGateway() error {
	if !LoadConfig().KeepProwl {
		return nil
	}
	return startGateway()
}

// retireIdleGateway stops the gateway unless the user keeps it on or something still needs it.
// force skips the idle wait, for the moment the user turns keep off.
func retireIdleGateway(force bool) {
	cfg := LoadConfig()
	if cfg.KeepProwl {
		return
	}
	gatewayUseMu.Lock()
	last := gatewayLastUse
	gatewayUseMu.Unlock()
	if !force && gatewayNow().Sub(last) < gatewayIdle {
		return
	}
	if routedAgentLive(cfg) || !prowlGatewayRunning() {
		return
	}
	// our own keep-alive connections would count as a client forever
	prowlGatewayHTTPClient.CloseIdleConnections()
	if gatewayClients(prowlGatewayPort()) > 0 {
		return
	}
	_ = stopGateway()
}

// a harness outside Rashin keeps a connection open while it streams, so this
// counts established connections to the gateway's port
func gatewayClients(port int) int {
	want := fmt.Sprintf(":%04X", port)
	n := 0
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scan := bufio.NewScanner(f)
		scan.Scan()
		for scan.Scan() {
			fields := strings.Fields(scan.Text())
			if len(fields) > 3 && fields[3] == "01" && strings.HasSuffix(fields[1], want) {
				n++
			}
		}
		f.Close()
	}
	return n
}

func cmdProwl(args []string) error {
	if len(args) != 2 || args[0] != "keep" || (args[1] != "on" && args[1] != "off") {
		return errors.New("usage: ryoku-rashin prowl keep on|off")
	}
	on := args[1] == "on"
	if err := updateConfig(func(cfg *Config) error {
		cfg.KeepProwl = on
		return nil
	}); err != nil {
		return err
	}
	if on {
		return startGateway()
	}
	releaseGateway()
	return nil
}

// releaseGateway asks a running daemon to stop the gateway now, since only the
// daemon knows whether an agent still needs it
func releaseGateway() {
	cfg := LoadConfig()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post("http://127.0.0.1:"+strconv.Itoa(cfg.Port)+"/api/gateway/release", "application/json", nil)
	if err != nil {
		_ = stopGateway()
		return
	}
	resp.Body.Close()
}
