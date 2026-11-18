package mcpservers

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ryoku-i18n"
)

type proxyEndpoint struct{ server, address string }

func (s *Store) startProxyLocked() error {
	if s.proxy != nil {
		return nil
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.proxyToken = randomID() + randomID()
	s.proxyURL = "http://" + listener.Addr().String()
	s.proxyEndpoints = make(map[string]proxyEndpoint)
	s.proxyCancel = cancel
	s.proxyClient = &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	server := &http.Server{Handler: http.HandlerFunc(s.proxyRequest), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, BaseContext: func(net.Listener) context.Context { return ctx }}
	s.proxy = server
	go func() { _ = server.Serve(listener) }()
	return nil
}

func (s *Store) stopProxyLocked() {
	if s.proxy == nil {
		return
	}
	s.proxyCancel()
	_ = s.proxy.Close()
	s.proxyClient.CloseIdleConnections()
	s.proxyClient = nil
	s.proxy, s.proxyURL, s.proxyToken, s.proxyEndpoints, s.proxyCancel = nil, "", "", nil, nil
}

func (s *Store) proxyAddressLocked(id string) string {
	return s.proxyURL + "/" + s.proxyToken + "/" + id
}

func (s *Store) proxyRequest(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if req.Method != http.MethodGet && req.Method != http.MethodPost && req.Method != http.MethodDelete {
		http.Error(w, i18n.T("Method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
	s.mu.Lock()
	base, err := url.Parse(s.proxyURL)
	if err != nil || base.Host != req.Host || len(parts) < 2 || s.proxyToken == "" || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(s.proxyToken)) != 1 || req.Header.Get("Origin") != "" {
		s.mu.Unlock()
		http.NotFound(w, req)
		return
	}
	var selected *record
	for index := range s.records {
		if s.records[index].Info.ID == parts[1] {
			selected = &s.records[index]
			break
		}
	}
	if selected == nil || !selected.Info.Enabled || selected.Info.State != "ready" || (selected.Info.Type != "http" && selected.Info.Type != "sse") {
		s.mu.Unlock()
		http.NotFound(w, req)
		return
	}
	address := selected.Info.URL
	if len(parts) != 2 {
		if len(parts) != 4 || parts[2] != "endpoint" {
			s.mu.Unlock()
			http.NotFound(w, req)
			return
		}
		endpoint, ok := s.proxyEndpoints[parts[3]]
		if !ok || endpoint.server != selected.Info.ID {
			s.mu.Unlock()
			http.NotFound(w, req)
			return
		}
		address = endpoint.address
	}
	serverID, transport := selected.Info.ID, selected.Info.Type
	headers, err := s.headers(req.Context(), selected)
	if err != nil {
		selected.Info.State = "needs-signin"
		_ = s.persist()
		s.mu.Unlock()
		http.Error(w, i18n.T("Sign in or unlock the credential vault in Rashin"), http.StatusUnauthorized)
		return
	}
	proxyBase := s.proxyAddressLocked(serverID)
	client := s.proxyClient
	s.mu.Unlock()
	target, err := url.Parse(address)
	if err != nil {
		http.Error(w, i18n.T("Invalid MCP endpoint"), http.StatusBadGateway)
		return
	}
	if req.URL.RawQuery != "" {
		query := target.Query()
		for name, values := range req.URL.Query() {
			query[name] = values
		}
		target.RawQuery = query.Encode()
	}
	var body io.Reader
	if req.Method == http.MethodPost {
		body = http.MaxBytesReader(w, req.Body, 8<<20)
	}
	forward, err := http.NewRequestWithContext(req.Context(), req.Method, target.String(), body)
	if err != nil {
		http.Error(w, i18n.T("MCP request failed"), http.StatusBadGateway)
		return
	}
	for _, name := range []string{"Accept", "Content-Type", "Mcp-Session-Id", "MCP-Protocol-Version", "Last-Event-ID"} {
		if value := req.Header.Get(name); value != "" {
			forward.Header.Set(name, value)
		}
	}
	for name, value := range headers {
		forward.Header.Set(name, value)
	}
	response, err := client.Do(forward)
	if err != nil {
		http.Error(w, i18n.T("MCP server could not be reached"), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		http.Error(w, i18n.T("MCP server changed its address"), http.StatusBadGateway)
		return
	}
	for _, name := range []string{"Content-Type", "Mcp-Session-Id", "MCP-Protocol-Version", "Retry-After"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if transport == "sse" && req.Method == http.MethodGet && strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		s.proxySSE(req.Context(), w, response.Body, serverID, address, proxyBase)
		return
	}
	buffer := make([]byte, 32<<10)
	for {
		count, err := response.Body.Read(buffer)
		if count > 0 {
			if _, writeErr := w.Write(buffer[:count]); writeErr != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Store) proxySSE(ctx context.Context, w http.ResponseWriter, input io.Reader, serverID, upstream, proxyBase string) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	event := ""
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if event == "endpoint" && strings.HasPrefix(line, "data:") {
			address, err := sameOriginEndpoint(upstream, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			if err != nil {
				return
			}
			s.mu.Lock()
			if s.proxy == nil {
				s.mu.Unlock()
				return
			}
			if len(s.proxyEndpoints) >= 1024 {
				s.mu.Unlock()
				return
			}
			id := randomID()
			s.proxyEndpoints[id] = proxyEndpoint{server: serverID, address: address}
			s.mu.Unlock()
			line = "data: " + proxyBase + "/endpoint/" + id
		}
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return
		}
		if line == "" {
			event = ""
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func sameOriginEndpoint(base, next string) (string, error) {
	origin, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	relative, err := url.Parse(next)
	if err != nil {
		return "", err
	}
	target := origin.ResolveReference(relative)
	if target.Scheme != origin.Scheme || target.Host != origin.Host || target.User != nil || target.Fragment != "" {
		return "", errors.New("MCP endpoint changed origin")
	}
	return target.String(), nil
}
