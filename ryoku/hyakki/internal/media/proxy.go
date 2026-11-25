package media

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/safehttp"
)

type videoProxy struct {
	url       string
	server    *http.Server
	transport *http.Transport
	cancel    context.CancelFunc
	once      sync.Once
	mu        sync.Mutex
	closed    bool
	tunnels   map[net.Conn]struct{}
}

func startVideoProxy(ctx context.Context, raw string) (*videoProxy, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("%s", i18n.T("Use a public http or https video link."))
	}
	checkCtx, cancelCheck := context.WithTimeout(ctx, 15*time.Second)
	err = safehttp.CheckHost(checkCtx, u.Hostname())
	cancelCheck()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("The video link must point to a public internet address"), err)
	}
	return newVideoProxy(ctx)
}

func newVideoProxy(parent context.Context) (*videoProxy, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	proxy := &videoProxy{
		url: "http://" + listener.Addr().String(), cancel: cancel,
		transport: &http.Transport{
			DialContext: safehttp.DialContext, TLSHandshakeTimeout: 10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, MaxIdleConns: 4,
		},
		tunnels: make(map[net.Conn]struct{}),
	}
	proxy.server = &http.Server{
		Handler: proxy, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 64 << 10, BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() { _ = proxy.server.Serve(listener) }()
	go func() {
		<-ctx.Done()
		proxy.Close()
	}()
	return proxy, nil
}

func (p *videoProxy) Close() {
	p.once.Do(func() {
		p.cancel()
		_ = p.server.Close()
		p.transport.CloseIdleConnections()
		p.mu.Lock()
		p.closed = true
		for conn := range p.tunnels {
			_ = conn.Close()
		}
		p.mu.Unlock()
	})
}

func (p *videoProxy) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		p.connect(w, request)
		return
	}
	if request.URL.Scheme != "http" || request.URL.Host == "" || request.URL.User != nil {
		http.Error(w, "Use a public HTTP destination", http.StatusBadRequest)
		return
	}
	out := request.Clone(request.Context())
	out.RequestURI = ""
	out.Host = out.URL.Host
	stripProxyHeaders(out.Header)
	response, err := p.transport.RoundTrip(out)
	if err != nil {
		http.Error(w, "The proxy refused the destination: "+err.Error(), http.StatusForbidden)
		return
	}
	defer response.Body.Close()
	stripProxyHeaders(response.Header)
	for key, values := range response.Header {
		w.Header()[key] = values
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (p *videoProxy) connect(w http.ResponseWriter, request *http.Request) {
	if _, _, err := net.SplitHostPort(request.Host); err != nil {
		http.Error(w, "Use a host and port", http.StatusBadRequest)
		return
	}
	upstream, err := safehttp.DialContext(request.Context(), "tcp", request.Host)
	if err != nil {
		http.Error(w, "The proxy refused the destination: "+err.Error(), http.StatusForbidden)
		return
	}
	defer upstream.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Tunnelling is unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.tunnels[client], p.tunnels[upstream] = struct{}{}, struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.tunnels, client)
		delete(p.tunnels, upstream)
		p.mu.Unlock()
	}()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	copied := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, buffered)
		_ = upstream.Close()
		_ = client.Close()
		close(copied)
	}()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
	_ = upstream.Close()
	<-copied
}

func stripProxyHeaders(header http.Header) {
	for _, connection := range header.Values("Connection") {
		for _, key := range strings.Split(connection, ",") {
			header.Del(strings.TrimSpace(key))
		}
	}
	for _, key := range []string{"Connection", "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(key)
	}
}

func (p *videoProxy) run(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	guarded := []string{"--ignore-config", "--proxy", p.url, "--geo-verification-proxy", p.url, "--downloader", "native"}
	return run(ctx, timeout, "yt-dlp", append(guarded, args...)...)
}
