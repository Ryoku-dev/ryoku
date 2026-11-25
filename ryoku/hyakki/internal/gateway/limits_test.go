package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/protocol"
)

// openGatewayAt opens a gateway on dir with local providers that refuse at
// once, so a test can close it and open the same state again.
func openGatewayAt(t *testing.T, dir string) *Gateway {
	t.Helper()
	local := map[string]string{protocol.ProviderOllamaLocal: "http://127.0.0.1:1/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"}
	gateway, err := Open(dir, func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Local: local}}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	return gateway
}

func providerLimitsOf(t *testing.T, gateway *Gateway, id string) *protocol.ProviderLimits {
	t.Helper()
	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		if p.ID == id {
			return p.Limits
		}
	}
	t.Fatalf("no provider %s", id)
	return nil
}

func findWindow(limits *protocol.ProviderLimits, span, unit, source string) (protocol.LimitWindow, bool) {
	for _, w := range limits.Windows {
		if w.Span == span && w.Unit == unit && w.Source == source {
			return w, true
		}
	}
	return protocol.LimitWindow{}, false
}

// A provider's rate-limit headers are kept and outlive a restart, the
// documented limits fill the windows no header named, and Rashin's own count
// of what it sent today is beside them.
func TestRateLimitHeadersSurviveARestart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-requests", "1000")
		w.Header().Set("x-ratelimit-remaining-requests", "998")
		w.Header().Set("x-ratelimit-reset-requests", "2m")
		w.Header().Set("x-ratelimit-limit-tokens", "6000")
		w.Header().Set("x-ratelimit-remaining-tokens", "5985")
		w.Header().Set("x-ratelimit-reset-tokens", "150ms")
		sse(w, `{"choices":[{"delta":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, `[DONE]`)
	}))
	defer server.Close()
	dir := t.TempDir()
	first := openGatewayAt(t, dir)
	setTestProviders(first, testProvider("groq", server.URL, 80, true))
	if text, _ := answer(t, first, question()); text != "hi" {
		t.Fatalf("answered %q", text)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := openGatewayAt(t, dir)
	// a minute later, past the token window's reset
	second.now = func() time.Time { return time.Now().Add(time.Minute) }
	setTestProviders(second, testProvider("groq", server.URL, 80, true))
	limits := providerLimitsOf(t, second, "groq")
	day, ok := findWindow(limits, protocol.LimitSpanDay, protocol.LimitUnitRequests, protocol.LimitFromHeaders)
	if !ok || day.Limit != 1000 || day.Remaining == nil || *day.Remaining != 998 {
		t.Fatalf("Groq's daily requests after a restart: %+v", limits.Windows)
	}
	// the token window refilled while the gateway was closed
	minute, ok := findWindow(limits, protocol.LimitSpanMinute, protocol.LimitUnitTokens, protocol.LimitFromHeaders)
	if !ok || minute.Remaining == nil || *minute.Remaining != 6000 || minute.Reset != nil {
		t.Fatalf("a minute window past its reset is not full again: %+v", minute)
	}
	if _, ok := findWindow(limits, protocol.LimitSpanMinute, protocol.LimitUnitRequests, protocol.LimitFromDocumented); !ok {
		t.Fatalf("Groq's documented 30 requests a minute is missing: %+v", limits.Windows)
	}
	if _, ok := findWindow(limits, protocol.LimitSpanDay, protocol.LimitUnitRequests, protocol.LimitFromDocumented); ok {
		t.Fatal("a documented window shows beside the live one for the same span")
	}
	if limits.Today.Requests != 1 || limits.Today.Tokens != 15 || limits.Month.Requests != 1 {
		t.Fatalf("Rashin's own count: today %+v, month %+v", limits.Today, limits.Month)
	}
}

// testClock is a clock a test moves while the gateway reads it from its
// background reads.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func keyedOpenRouter(baseURL string) provider {
	p := testProvider("openrouter", baseURL, 80, true)
	p.Keyless, p.Auth, p.Tier = false, "bearer", protocol.TierCredits
	return p
}

func awaitChange(t *testing.T, changes chan struct{}) {
	t.Helper()
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Fatal("the gateway never reported the read")
	}
}

// An account endpoint is read at most every ten minutes, however often the
// app asks and across a restart; Check reads it whatever the pacing says.
func TestAccountReadsArePacedToTenMinutes(t *testing.T) {
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/key":
			reads.Add(1)
			_, _ = io.WriteString(w, openRouterKeySample)
		case "/api/v1/credits":
			// an ordinary key may not read the balance
			w.WriteHeader(http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	clock := &testClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	open := func() (*Gateway, chan struct{}) {
		gateway := openGatewayAt(t, dir)
		gateway.now = clock.Now
		setTestProviders(gateway, keyedOpenRouter(server.URL+"/api/v1"))
		changes := make(chan struct{}, 8)
		gateway.OnChange(func() { changes <- struct{}{} })
		return gateway, changes
	}
	gateway, changes := open()
	if _, _, err := gateway.store.addKey("openrouter", "main", "sk-or-v1-pacing"); err != nil {
		t.Fatal(err)
	}
	refresh := func(want int32) {
		t.Helper()
		if err := gateway.RefreshLimits(context.Background(), ""); err != nil {
			t.Fatal(err)
		}
		awaitChange(t, changes)
		if got := reads.Load(); got != want {
			t.Fatalf("the account was read %d times, want %d", got, want)
		}
	}
	refresh(1)
	limits := providerLimitsOf(t, gateway, "openrouter")
	if w, ok := findWindow(limits, protocol.LimitSpanDay, protocol.LimitUnitRequests, protocol.LimitFromAccount); !ok || *w.Remaining != 964 {
		t.Fatalf("the account's free daily window: %+v", limits.Windows)
	}
	if limits.Credits == nil || *limits.Credits.Remaining != 4.21 {
		t.Fatalf("a refused /credits dropped the key's cap: %+v", limits.Credits)
	}
	clock.Add(5 * time.Minute)
	refresh(1)
	refresh(1)
	clock.Add(6 * time.Minute)
	refresh(2)
	p, err := gateway.findProvider(context.Background(), "openrouter", false)
	if err != nil {
		t.Fatal(err)
	}
	gateway.checkAccount(p)
	if got := reads.Load(); got != 3 {
		t.Fatalf("Check read the account %d times in all, want 3", got)
	}
	if err := gateway.Close(); err != nil {
		t.Fatal(err)
	}
	clock.Add(time.Minute)
	gateway, changes = open()
	refresh(3)
	clock.Add(10 * time.Minute)
	refresh(4)
}

// Nothing the limits keep, report or log carries a key, though the provider
// quotes it back in its answers.
func TestLimitsNeverKeepOrLogAKey(t *testing.T) {
	const secret = "sk-or-v1-0123456789abcdef-never-shown"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/key":
			// the key's label is a preview of the key itself
			_, _ = io.WriteString(w, strings.Replace(openRouterKeySample, "sk-or-v1-abc...123", secret, 1))
		case "/api/v1/credits":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"bad key `+secret+`"}}`)
		case "/api/v1/chat/completions":
			w.Header().Set("X-RateLimit-Limit", "20")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "30")
			w.Header().Set("X-Echo", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"rate limited for `+secret+`"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var logged bytes.Buffer
	previous := slog.Default()
	log.SetOutput(&logged)
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		log.SetOutput(io.Discard)
		slog.SetDefault(previous)
	})

	gateway := openGatewayAt(t, t.TempDir())
	setTestProviders(gateway, keyedOpenRouter(server.URL+"/api/v1"))
	changes := make(chan struct{}, 8)
	gateway.OnChange(func() { changes <- struct{}{} })
	if _, _, err := gateway.store.addKey("openrouter", "main", secret); err != nil {
		t.Fatal(err)
	}
	if stream, err := gateway.Stream(context.Background(), question()); err == nil {
		_, _ = collect(t, stream)
	}
	if err := gateway.RefreshLimits(context.Background(), "openrouter"); err != nil {
		t.Fatal(err)
	}
	awaitChange(t, changes)

	limits := providerLimitsOf(t, gateway, "openrouter")
	if _, ok := findWindow(limits, protocol.LimitSpanMinute, protocol.LimitUnitRequests, protocol.LimitFromHeaders); !ok {
		t.Fatalf("the refusal's headers were not kept, so this proves nothing: %+v", limits.Windows)
	}
	if _, ok := findWindow(limits, protocol.LimitSpanDay, protocol.LimitUnitRequests, protocol.LimitFromAccount); !ok {
		t.Fatalf("the account was not read, so this proves nothing: %+v", limits.Windows)
	}
	reported, err := json.Marshal(limits)
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := gateway.store.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketLimits).ForEach(func(k, v []byte) error {
			stored = append(append(stored, k...), v...)
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string][]byte{"reported": reported, "stored": stored, "logged": logged.Bytes()} {
		if bytes.Contains(text, []byte(secret)) || bytes.Contains(text, []byte(secret[len(secret)-12:])) {
			t.Fatalf("the %s limits carry the key: %s", name, text)
		}
	}
}
