package daemon

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// apiServer is the loopback listener behind the local API. It exists only
// while the setting is on and the daemon runs.
type apiServer struct {
	mu      sync.Mutex
	server  *http.Server
	port    int
	failure string
}

func (a *apiServer) listening() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.server != nil
}

// applyAPI brings the listener in line with the settings: it starts it, moves
// it to another port or closes it, and tells the clients when that changed.
func (d *Daemon) applyAPI() {
	settings := d.Settings().API
	api := &d.api
	api.mu.Lock()
	wasListening, wasPort, wasFailure := api.server != nil, api.port, api.failure
	if api.server != nil && (!settings.Enabled || api.port != settings.Port) {
		_ = api.server.Close()
		api.server = nil
	}
	api.port, api.failure = settings.Port, ""
	var failed error
	if settings.Enabled && api.server == nil {
		if api.server, failed = d.listenAPI(settings.Port); failed != nil {
			api.failure = failed.Error()
		}
	}
	changed := wasListening != (api.server != nil) || wasPort != api.port || wasFailure != api.failure
	api.mu.Unlock()
	if failed != nil {
		d.warn("local API: %v", failed)
	}
	if changed {
		d.Broadcast(protocol.EvtAPIChanged, d.apiInfo())
	}
}

// listenAPI opens both loopback families: a client that resolves localhost to
// ::1 would otherwise find nothing there.
func (d *Daemon) listenAPI(port int) (*http.Server, error) {
	if d.deps.Gateway == nil {
		return nil, errors.New(i18n.T("The model router is not available."))
	}
	four, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if errors.Is(err, syscall.EADDRINUSE) {
		return nil, errors.New(i18n.Tf("Another program is using port %d. Pick another port.", port))
	}
	if err != nil {
		return nil, errors.New(i18n.Tf("Rashin could not listen on port %d: %v", port, err))
	}
	server := &http.Server{
		Handler:           d.deps.Gateway.API(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	serve := func(listener net.Listener) {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.warn("local API: %v", err)
		}
	}
	go serve(four)
	// a machine without IPv6 just has no ::1 to listen on
	if six, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(port))); err == nil {
		go serve(six)
	}
	return server, nil
}

func (d *Daemon) closeAPI() {
	d.api.mu.Lock()
	if d.api.server != nil {
		_ = d.api.server.Close()
		d.api.server = nil
	}
	d.api.mu.Unlock()
}

func (d *Daemon) apiInfo() protocol.APIInfo {
	settings := d.Settings().API
	info := protocol.APIInfo{
		Enabled: settings.Enabled, Port: settings.Port,
		URL: "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(settings.Port)) + "/v1",
	}
	d.api.mu.Lock()
	info.Listening, info.Error = d.api.server != nil, d.api.failure
	d.api.mu.Unlock()
	return info
}
