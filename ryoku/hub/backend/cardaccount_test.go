package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCardAccountOriginAndScope(t *testing.T) {
	for raw, want := range map[string]string{"https://EXAMPLE.com:443/": "https://example.com", "http://127.0.0.1:8787": "http://127.0.0.1:8787", "https://[::1]:443": "https://[::1]"} {
		got, e := cardAccountOrigin(raw)
		if e != nil || got != want {
			t.Fatalf("origin %q: %q %v", raw, got, e)
		}
	}
	if got := cardAccountScope("http://127.0.0.1:8787"); got != "ryoku-server-b1a61bf29a38ff3642af0dad0785e167" {
		t.Fatal(got)
	}
	for _, raw := range []string{"", "http://example.com", "https://user:secret@example.com", "https://example.com/api", "https://example.com?token=x", "https://example.com#x", "file:///tmp/x", "https://example.com//"} {
		if _, e := cardAccountOrigin(raw); e == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, endpoint := range []string{"https://evil.example/token", "http://issuer.example/token", "https://user@issuer.example/token", "https://issuer.example:444/token", "https://issuer.example/token#x"} {
		if cardAccountEndpoint(endpoint, "https://issuer.example") {
			t.Fatalf("accepted %q", endpoint)
		}
	}
	if !cardAccountEndpoint("https://issuer.example/oauth/token", "https://issuer.example") {
		t.Fatal("same-origin endpoint refused")
	}
}
func accountTestChallenge(t *testing.T, fields []any) cardAccountChallenge {
	t.Helper()
	raw, e := cardAccountJSON(fields)
	if e != nil {
		t.Fatal(e)
	}
	return cardAccountChallenge{ID: "dch_" + strings.Repeat("a", 32), Input: base64.StdEncoding.EncodeToString(append([]byte("ryoku-card-link-v1\x00"), raw...)), Expires: fields[5].(int64), CardID: fields[1].(string), Origin: fields[4].(string)}
}
func TestCardAccountLinkActionBinding(t *testing.T) {
	now := time.Unix(1800000000, 0)
	seed := bytes.Repeat([]byte{7}, 32)
	pub := badgeDevicePublic(seed)
	account := "ra_abcdefgh12345678"
	server := "https://accounts.example"
	fields := []any{account, cardIDText(pub), base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), server, now.Add(4 * time.Minute).UnixMilli()}
	good := accountTestChallenge(t, fields)
	input, e := cardAccountLinkInput(good, server, account, pub, now)
	if e != nil {
		t.Fatal(e)
	}
	key := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(key, input)
	if !ed25519.Verify(pub, input, sig) {
		t.Fatal("valid link signature failed")
	}
	cases := map[string]func(*cardAccountChallenge){
		"action": func(c *cardAccountChallenge) {
			raw, _ := base64.StdEncoding.DecodeString(c.Input)
			c.Input = base64.StdEncoding.EncodeToString(bytes.Replace(raw, []byte("ryoku-card-link-v1"), []byte("ryoku-submit-v1"), 1))
		},
		"origin":         func(c *cardAccountChallenge) { c.Origin = "https://evil.example" },
		"card":           func(c *cardAccountChallenge) { c.CardID = "0000000000000000" },
		"expiry-wrapper": func(c *cardAccountChallenge) { c.Expires++ },
		"challenge-id":   func(c *cardAccountChallenge) { c.ID = "dch_bad" },
		"canonical": func(c *cardAccountChallenge) {
			raw, _ := base64.StdEncoding.DecodeString(c.Input)
			c.Input = base64.StdEncoding.EncodeToString(append(raw, ' '))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := good
			mutate(&c)
			if _, e := cardAccountLinkInput(c, server, account, pub, now); e == nil {
				t.Fatal("accepted mismatched challenge")
			}
		})
	}
	for name, change := range map[string]func([]any){"account": func(v []any) { v[0] = "ra_otheraccount123" }, "key": func(v []any) { v[2] = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)) }, "nonce": func(v []any) { v[3] = "AQ==" }, "expired": func(v []any) { v[5] = now.UnixMilli() }, "future": func(v []any) { v[5] = now.Add(6 * time.Minute).UnixMilli() }, "frame-origin": func(v []any) { v[4] = "https://evil.example" }} {
		t.Run(name, func(t *testing.T) {
			f := append([]any(nil), fields...)
			change(f)
			c := accountTestChallenge(t, f)
			if _, e := cardAccountLinkInput(c, server, account, pub, now); e == nil {
				t.Fatal("accepted changed link fields")
			}
		})
	}
	changed := append([]any(nil), fields...)
	changed[3] = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	other := accountTestChallenge(t, changed)
	otherInput, e := cardAccountLinkInput(other, server, account, pub, now)
	if e != nil {
		t.Fatal(e)
	}
	if ed25519.Verify(pub, otherInput, sig) {
		t.Fatal("signature replayed across nonce")
	}
}
func TestCardAccountSubmissionBindsExactArchive(t *testing.T) {
	p := designPack{files: map[string][]byte{"hero.png": []byte("exact compressed image bytes")}, names: []string{"hero.png"}}
	raw, e := designZip(p)
	if e != nil {
		t.Fatal(e)
	}
	d := cardAccountDraft{Kind: "hero", SHA: cardAccountHash(raw), Files: []cardAccountFile{{len(p.files["hero.png"]), "hero.png", cardAccountHash(p.files["hero.png"])}}, Nonce: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), At: 1800000000, Version: "2026-11-20.private-test.2.account", AttestationVersion: "2026-11-20.private-test.2.cards", Gate: "https://team.example"}
	if e = cardAccountCheckDraftArchive(raw, d); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*cardAccountDraft){"hash": func(x *cardAccountDraft) { x.Files[0].SHA = strings.Repeat("0", 64) }, "bytes": func(x *cardAccountDraft) { x.Files[0].Bytes++ }, "name": func(x *cardAccountDraft) { x.Files[0].Name = "another.png" }, "inventory": func(x *cardAccountDraft) { x.Files = nil }} {
		t.Run(name, func(t *testing.T) {
			x := d
			x.Files = append([]cardAccountFile(nil), d.Files...)
			change(&x)
			if cardAccountCheckDraftArchive(raw, x) == nil {
				t.Fatal("signed inventory mismatch accepted")
			}
		})
	}
	var duplicate bytes.Buffer
	zw := zip.NewWriter(&duplicate)
	for range 2 {
		h := zip.FileHeader{Name: "hero.png", Method: zip.Store}
		h.SetMode(0600)
		w, e := zw.CreateHeader(&h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(p.files["hero.png"]); e != nil {
			t.Fatal(e)
		}
	}
	if e = zw.Close(); e != nil {
		t.Fatal(e)
	}
	if cardAccountCheckDraftArchive(duplicate.Bytes(), d) == nil {
		t.Fatal("duplicate accepted")
	}
	seed := bytes.Repeat([]byte{3}, 32)
	pub := badgeDevicePublic(seed)
	d.Card = cardIDText(pub)
	checks := map[string]bool{"ownOrLicensed": true, "policyAccepted": true, "consentToReview": true}
	payload, e := cardAccountSubmissionManifest(d, pub, checks, time.Unix(d.At, 0))
	if e != nil {
		t.Fatal(e)
	}
	sig := signDomain(seed, "ryoku-submit-v1", payload)
	if !verifyDomain(pub, "ryoku-submit-v1", payload, sig) {
		t.Fatal("manifest signature invalid")
	}
	if verifyDomain(pub, "ryoku-card-link-v1", payload, sig) {
		t.Fatal("cross-action signature accepted")
	}
	var decoded map[string]any
	if json.Unmarshal(payload, &decoded) != nil {
		t.Fatal("bad payload")
	}
	if decoded["audience"] != d.Gate || decoded["originalSha"] != cardAccountHash(raw) {
		t.Fatal("missing Gate/archive binding")
	}
	if decoded["termsVersion"] != d.Version || decoded["attestationVersion"] != d.AttestationVersion || decoded["termsVersion"] == decoded["attestationVersion"] {
		t.Fatal("distinct Account/card consent versions were not bound")
	}
	for name, change := range map[string]func(*cardAccountDraft){"terms": func(x *cardAccountDraft) { x.Version = "" }, "card consent": func(x *cardAccountDraft) { x.AttestationVersion = "" }, "nonce": func(x *cardAccountDraft) { x.Nonce = "bad" }, "expiry": func(x *cardAccountDraft) { x.At -= 601 }, "device": func(x *cardAccountDraft) { x.Card = "0000000000000000" }} {
		t.Run(name, func(t *testing.T) {
			x := d
			change(&x)
			if _, e := cardAccountSubmissionManifest(x, pub, checks, time.Unix(d.At, 0)); e == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
	for _, field := range []string{"ownOrLicensed", "policyAccepted", "consentToReview"} {
		checks[field] = false
		if _, e := cardAccountSubmissionManifest(d, pub, checks, time.Unix(d.At, 0)); e == nil {
			t.Fatalf("missing %s accepted", field)
		}
		checks[field] = true
	}
}

func TestCardAccountScopedConsent(t *testing.T) {
	policy := func(scope string, ids []string) map[string]any {
		documents := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			documents = append(documents, map[string]string{"id": id, "sha256": strings.Repeat("a", 64)})
		}
		return map[string]any{"scope": scope, "version": "private-test.2." + scope, "disclosureSha256": strings.Repeat("b", 64), "documents": documents}
	}
	raw, err := json.Marshal(map[string]any{
		"account": map[string]any{"accepted": true, "acceptedAt": 1800000000000, "policy": policy("account", []string{"terms", "privacy", "refunds"})},
		"cards":   map[string]any{"accepted": true, "acceptedAt": 1800000000001, "policy": policy("cards", []string{"content-review"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	var valid cardAccountConsent
	if err = json.Unmarshal(raw, &valid); err != nil || !valid.cardsReady() {
		t.Fatal("current explicitly scoped receipt was refused", err)
	}
	for name, change := range map[string]func(*cardAccountConsent){
		"account only":       func(c *cardAccountConsent) { c.Cards.Accepted = false },
		"cards only":         func(c *cardAccountConsent) { c.Account.Accepted = false },
		"missing cards":      func(c *cardAccountConsent) { c.Cards = cardAccountConsentStatus{} },
		"wrong scope":        func(c *cardAccountConsent) { c.Cards = c.Account },
		"missing version":    func(c *cardAccountConsent) { c.Cards.Policy.Version = "" },
		"missing disclosure": func(c *cardAccountConsent) { c.Cards.Policy.DisclosureSHA = "" },
		"missing receipt":    func(c *cardAccountConsent) { c.Account.AcceptedAt = 0 },
		"missing documents":  func(c *cardAccountConsent) { c.Account.Policy.Documents = nil },
		"altered digest":     func(c *cardAccountConsent) { c.Cards.Policy.Documents[0].SHA = "invalid" },
		"duplicate documents": func(c *cardAccountConsent) {
			c.Cards.Policy.Documents = append(c.Cards.Policy.Documents, c.Cards.Policy.Documents[0])
		},
	} {
		t.Run(name, func(t *testing.T) {
			var consent cardAccountConsent
			if err := json.Unmarshal(raw, &consent); err != nil {
				t.Fatal(err)
			}
			change(&consent)
			if consent.cardsReady() {
				t.Fatal("incomplete, missing or wrong-scope receipt authorized a card action")
			}
		})
	}
	var legacy cardAccountConsent
	if err := json.Unmarshal([]byte(`{"accepted":true,"policy":{"version":"2026-11-20.private-test.1"}}`), &legacy); err != nil || legacy.cardsReady() {
		t.Fatal("legacy combined receipt inherited scoped authorization", err)
	}
}
func TestCardAccountTransportNeverRedirectsSecrets(t *testing.T) {
	var reached atomic.Bool
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Store(true) }))
	defer sink.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-test-token" {
			t.Error("missing authorization")
		}
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	status, e := cardAccountRequest(context.Background(), "GET", source.URL, "private-test-token", "", nil, nil)
	if e == nil || status != 307 || reached.Load() {
		t.Fatalf("redirect leaked authorization: %d %v %v", status, e, reached.Load())
	}
	if strings.Contains(e.Error(), "private-test-token") {
		t.Fatal("credential leaked in error")
	}
}
func TestCardAccountSelectionAndStateIsolation(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("RYOKU_TEAM_URL", "https://team.example")
	gate, id, e := cardApprovedPublication(context.Background())
	if e != nil || id != "" || gate != "https://team.example" {
		t.Fatalf("empty selection: %q %q %v", gate, id, e)
	}
	good := cardAccountSelection{"https://team.example", "12345678-abcd-1234-abcd-123456789abc"}
	if e = cardAccountWrite("publication.json", good); e != nil {
		t.Fatal(e)
	}
	_, id, e = cardApprovedPublication(context.Background())
	if e != nil || id != good.ID {
		t.Fatal("valid selection refused", e)
	}
	t.Setenv("RYOKU_TEAM_URL", "https://other.example")
	if _, _, e = cardApprovedPublication(context.Background()); e == nil {
		t.Fatal("cross-origin selection accepted")
	}
	dir, e := cardAccountDir()
	if e != nil {
		t.Fatal(e)
	}
	external := filepath.Join(t.TempDir(), "external")
	os.WriteFile(external, []byte(`{"secret":"sentinel"}`), 0600)
	if e = os.Symlink(external, filepath.Join(dir, "link.json")); e != nil {
		t.Fatal(e)
	}
	var out any
	if cardAccountRead("link.json", &out) == nil {
		t.Fatal("symlink read")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	unlock, e := cardAccountLock(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	blocked, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()
	if _, e = cardAccountLock(blocked); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("credential refresh lock did not serialize", e)
	}
}

// The opt-in integration owns its bus, encrypted keyring and daemon lifetimes.
func TestCardAccountRealSecretServiceIsolation(t *testing.T) {
	if os.Getenv("RYOKU_TEST_SECRET_SERVICE") != "1" {
		t.Skip("requires isolated real Secret Service; set RYOKU_TEST_SECRET_SERVICE=1")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	startCardAccountTestKeyring(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	server := "https://native-account-test.invalid"
	token := "native-account-secret-sentinel-" + time.Now().Format("150405.000000000")
	value := cardAccountRecord{Server: server, Access: token, Refresh: token + "refresh"}
	if e := cardAccountSecret(ctx, server, "store", "regression", value, nil); e != nil {
		t.Fatal(e)
	}
	defer cardAccountSecret(context.Background(), server, "clear", "regression", nil, nil)
	var got cardAccountRecord
	if e := cardAccountSecret(ctx, server, "lookup", "regression", nil, &got); e != nil || got.Access != token {
		t.Fatal("real credential retrieval failed", e)
	}
	if e := cardAccountSecret(ctx, "https://another-native-test.invalid", "lookup", "regression", nil, &got); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("credential crossed server scope", e)
	}
	dir, e := cardAccountDir()
	if e != nil {
		t.Fatal(e)
	}
	if e = filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if bytes.Contains(data, []byte(token)) {
				t.Fatalf("credential persisted outside Secret Service: %s", path)
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}

func TestCardAccountStatusDoesNotCreateCardIdentity(t *testing.T) {
	if os.Getenv("RYOKU_TEST_SECRET_SERVICE") != "1" {
		t.Skip("requires isolated real Secret Service; set RYOKU_TEST_SECRET_SERVICE=1")
	}
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	startCardAccountTestKeyring(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const accountID = "ra_abcdefghijklmnop"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/account":
			json.NewEncoder(w).Encode(map[string]any{"account": map[string]string{"id": accountID}})
		case "/v1/consent":
			json.NewEncoder(w).Encode(map[string]any{"account": map[string]bool{"accepted": true}, "cards": map[string]bool{"accepted": false}})
		case "/v1/creator":
			json.NewEncoder(w).Encode(map[string]any{"active": true, "until": time.Now().Add(time.Hour).UnixMilli()})
		case "/v1/card-devices":
			json.NewEncoder(w).Encode(map[string]any{"devices": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("RYOKU_ACCOUNTS_URL", server.URL)
	record := cardAccountRecord{Server: server.URL, Issuer: "https://issuer.example", Endpoint: "https://issuer.example/oauth/token", ClientID: "private-test-client", AccountID: accountID, Access: "private-test-account-access", Expires: time.Now().Add(time.Hour).Unix()}
	if err := cardAccountSecret(ctx, server.URL, "store", "tokens", record, nil); err != nil {
		t.Fatal(err)
	}
	state, err := cardAccountStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status := state.(map[string]any)
	if status["state"] != "signed_in" || status["accountID"] != accountID || status["cardID"] != "" {
		t.Fatal("Account-only status manufactured a card identity", status["state"], status["cardID"])
	}
	seedPath := filepath.Join(data, "ryoku", "badge", "device.key")
	if _, err := os.Lstat(seedPath); !os.IsNotExist(err) {
		t.Fatal("Account status created a machine signing key", err)
	}
	if _, err := cardAccountPrepareLink(ctx); err == nil {
		t.Fatal("missing card consent authorized device linking")
	}
	if _, err := os.Lstat(seedPath); !os.IsNotExist(err) {
		t.Fatal("refused linking created a machine signing key", err)
	}
	seed, err := badgeDeviceSeed()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(seed)
	existingID := cardIDText(badgeDevicePublic(seed))
	state, err = cardAccountStatus(ctx)
	if err != nil || state.(map[string]any)["cardID"] != existingID {
		t.Fatal("status did not preserve the independently created local card identity", err)
	}
}
