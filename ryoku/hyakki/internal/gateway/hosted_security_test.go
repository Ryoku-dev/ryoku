package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/protocol"
)

// M2: the hosted provider is no provider to test, switch or strip: a test
// would spend the account's credits with no pick by the user.
func TestHostedProviderCannotBeTestedDisabledOrStripped(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"hosted"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)
	ctx := context.Background()

	if _, err := gateway.Test(ctx, protocol.ProviderEmber); err == nil {
		t.Fatal("testing the hosted provider was allowed")
	}
	if _, err := gateway.EnableProvider(ctx, protocol.EnableProviderParams{Provider: protocol.ProviderEmber, Enabled: false}); err == nil {
		t.Fatal("switching the hosted provider off was allowed")
	}
	if _, err := gateway.RemoveKey(ctx, protocol.RemoveKeyParams{Provider: protocol.ProviderEmber, Key: "any"}); err == nil {
		t.Fatal("removing a key of the hosted provider was allowed")
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 || hosted.used.Load() != 0 {
		t.Fatalf("a refused call reached the server: %d hits, %d endpoints, %d used", hits.Load(), hosted.endpoints.Load(), hosted.used.Load())
	}
	providers, err := gateway.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		if p.ID == protocol.ProviderEmber && !p.Enabled {
			t.Fatalf("the hosted provider was switched off: %#v", p)
		}
	}
}

// M3: the local API key, which every program of the user can read, is not a
// pick of a hosted model.
func TestAPINeitherListsNorServesHostedModels(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"hosted"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway, api := newAPIHarness(t)
	gateway.SetHosted(hosted)

	_, listing := api.do(http.MethodGet, "/v1/models", "", map[string]string{"Authorization": "Bearer " + api.key})
	if strings.Contains(listing, protocol.ProviderEmber+"/") {
		t.Fatalf("the API lists a hosted model: %s", listing)
	}
	for _, model := range []string{hostedModel, strings.ToUpper(hostedModel), " " + hostedModel} {
		response, body := api.chat(fmt.Sprintf(`{"model": %q, "messages": [{"role": "user", "content": "hello"}]}`, model))
		if response.StatusCode != http.StatusForbidden || !strings.Contains(body, "model_not_allowed") {
			t.Fatalf("model %q answered %d: %s", model, response.StatusCode, body)
		}
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 {
		t.Fatalf("the API reached the hosted server: %d hits, %d endpoints", hits.Load(), hosted.endpoints.Load())
	}
}

// L1: the hosted request goes where the account says and nowhere else.
func TestHostedRedirectIsNotFollowed(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"elsewhere"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/chat/completions", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
	if err == nil {
		_, err = collect(t, stream)
	}
	if err == nil || hits.Load() != 0 {
		t.Fatalf("a redirect gave %v with %d hits at the target", err, hits.Load())
	}
}

// L2: a frozen account's words are the server's, shown whole but clean and
// short; neither that nor an identity check that is down signs anyone out.
func TestHostedFailuresShowCleanServerWordsAndNeverSignOut(t *testing.T) {
	hostile := "\u202eYour account is frozen\u202c.\x07 Open evil.example now. " + strings.Repeat("more ", 300)
	for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"error":{"message":%q}}`, hostile)
		}))
		hosted := newFakeHosted(server.URL, true)
		gateway := hostedGateway(t, hosted)
		stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
		if err != nil {
			t.Fatal(err)
		}
		_, err = collect(t, stream)
		if err == nil {
			t.Fatalf("%d ended without an error", status)
		}
		text := err.Error()
		if !strings.Contains(text, "Your account is frozen") || utf8.RuneCountInString(text) > 400 {
			t.Errorf("%d error is %d runes: %q", status, utf8.RuneCountInString(text), text)
		}
		for _, r := range text {
			if r == '\u202e' || r == '\u202c' || r < ' ' && r != '\n' || r == 0x7f {
				t.Errorf("%d error keeps %U: %q", status, r, text)
			}
		}
		if hosted.rejected.Load() != 0 {
			t.Errorf("%d signed the account out: Rejected called %d times", status, hosted.rejected.Load())
		}
		assertNoHostedCooldown(t, gateway)
		server.Close()
	}
}

// L10: a record never prints its tokens, and a token the store dropped
// stays hidden for as long as the daemon runs.
func TestAccountRecordRedactsItselfAndRetiredTokensStayScrubbed(t *testing.T) {
	record := AccountRecord{Server: "https://accounts.example.com", Access: "access-secret-1", Refresh: "refresh-secret-1"}
	for _, text := range []string{fmt.Sprintf("%v", record), fmt.Sprintf("%+v", record), fmt.Sprintf("%#v", record), fmt.Sprintf("%s", &record), fmt.Sprintf("%v", []*AccountRecord{&record}), fmt.Sprint(map[string]AccountRecord{"a": record})} {
		if strings.Contains(text, "secret") {
			t.Fatalf("a printed record carries its tokens: %s", text)
		}
	}

	gateway := newTestGateway(t)
	if err := gateway.SetAccountTokens(&record); err != nil {
		t.Fatal(err)
	}
	rotated := record
	rotated.Access, rotated.Refresh = "access-secret-2", "refresh-secret-2"
	if err := gateway.SetAccountTokens(&rotated); err != nil {
		t.Fatal(err)
	}
	scrub := gateway.Scrubber()
	for _, token := range []string{"access-secret-1", "refresh-secret-1", "access-secret-2", "refresh-secret-2"} {
		if got := scrub("the server said " + token); strings.Contains(got, token) {
			t.Fatalf("scrubber lets %s through: %s", token, got)
		}
	}
	if err := gateway.SetAccountTokens(nil); err != nil {
		t.Fatal(err)
	}
	if got := gateway.Scrubber()("echo refresh-secret-2"); strings.Contains(got, "refresh-secret-2") {
		t.Fatalf("a signed-out token is no longer hidden: %s", got)
	}
}

// M1: the issuer pin is sealed, per server, and is not part of the tokens a
// sign-out wipes.
func TestAccountIssuerPinIsPerServerAndOutlivesSignOut(t *testing.T) {
	gateway := newTestGateway(t)
	if got, err := gateway.AccountIssuer("https://a.example.com"); err != nil || got != "" {
		t.Fatalf("unset pin = %q, %v", got, err)
	}
	if err := gateway.SetAccountIssuer("https://a.example.com", "https://id.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := gateway.SetAccountTokens(&AccountRecord{Server: "https://a.example.com", Access: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := gateway.SetAccountTokens(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := gateway.AccountIssuer("https://a.example.com"); got != "https://id.example.com" {
		t.Fatalf("sign-out dropped the pin: %q", got)
	}
	if got, _ := gateway.AccountIssuer("https://b.example.com"); got != "" {
		t.Fatalf("another server inherited the pin: %q", got)
	}
	var raw []byte
	_ = gateway.store.db.View(func(tx *bbolt.Tx) error {
		raw = append([]byte(nil), tx.Bucket(bucketAccount).Get(pinKey("https://a.example.com"))...)
		return nil
	})
	if len(raw) == 0 || strings.Contains(string(raw), "id.example.com") {
		t.Fatalf("the pin is not sealed: %q", raw)
	}
	if err := gateway.SetAccountIssuer("https://a.example.com", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := gateway.AccountIssuer("https://a.example.com"); got != "" {
		t.Fatalf("clearing left %q", got)
	}
}
