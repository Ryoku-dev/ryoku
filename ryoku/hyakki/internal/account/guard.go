package account

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/protocol"
)

// coalescer runs one job at a time, off the caller's goroutine, and never
// starts it twice within gap. Calls that arrive while it runs or waits ask
// for one more run, however many they are.
type coalescer struct {
	mu      sync.Mutex
	running bool
	again   bool
	last    time.Time
	gap     time.Duration
}

func (c *coalescer) run(job func()) {
	c.mu.Lock()
	if c.running {
		c.again = true
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	go func() {
		for {
			c.mu.Lock()
			wait := c.gap - time.Since(c.last)
			c.mu.Unlock()
			if wait > 0 {
				time.Sleep(wait)
			}
			c.mu.Lock()
			c.again = false
			c.last = time.Now()
			c.mu.Unlock()
			job()
			c.mu.Lock()
			if !c.again {
				c.running = false
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}()
}

// noRedirects is c, stopping at a redirect and handing it back as the
// answer: nothing a server sends takes a request to another place.
func noRedirects(c *http.Client) *http.Client {
	next := *c
	next.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &next
}

// internalHost is a host that names this computer or its own network: a
// loopback, private or link-local address, or a name that stays local.
func internalHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// serverIsLoopback is whether the Ryoku Accounts server runs on this
// computer, which is when its identity provider may be here too.
func serverIsLoopback(server string) bool {
	u, err := url.Parse(server)
	return err == nil && protocol.LoopbackHost(u.Hostname())
}

// displayHost is where an address points, as a user can read it.
func displayHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return protocol.ServerText(u.Host, protocol.ServerTextMax)
	}
	return protocol.ServerText(raw, protocol.ServerTextMax)
}

// issuerKey is an identity provider's address as it is compared and kept.
func issuerKey(issuer string) string { return strings.TrimRight(strings.TrimSpace(issuer), "/") }

// cleanVerification is an approval page the browser may be sent to: https,
// no credentials, in the form the browser is handed.
func cleanVerification(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return "", false
	}
	return u.String(), true
}

// tokenTTL is how long a token the identity provider vouched for is trusted
// to last, whatever it said: not under a minute, not over a day.
func tokenTTL(seconds int) time.Duration {
	return time.Duration(min(max(seconds, 60), 24*60*60)) * time.Second
}

func sameHost(endpoint, issuer string) bool {
	a, err1 := url.Parse(endpoint)
	b, err2 := url.Parse(issuer)
	return err1 == nil && err2 == nil && a.Scheme == "https" && a.Host == b.Host && a.User == nil
}
