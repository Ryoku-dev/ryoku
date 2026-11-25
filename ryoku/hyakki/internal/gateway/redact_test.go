package gateway

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Provider secrets and the local API credential must not escape in response
// text, including encoded provider keys. Very short values are not redacted.
func TestRedactProtectsStoredSecrets(t *testing.T) {
	s, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	const provider = "or-test-key-0123456789"
	if _, _, err := s.addKey("openrouter", "main", provider); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.addKey("tiny", "", "abc"); err != nil {
		t.Fatal(err)
	}
	api, err := s.apiKey(false)
	if err != nil {
		t.Fatal(err)
	}
	g := &Gateway{store: s}
	text := "sent " + provider + " and " + base64.StdEncoding.EncodeToString([]byte(provider)) + " with " + api + ", abc stays"
	got := g.Redact(text)
	for _, secret := range []string{provider, base64.StdEncoding.EncodeToString([]byte(provider)), api} {
		if strings.Contains(got, secret) {
			t.Fatalf("Redact retained a stored secret: %q", got)
		}
	}
	if got := g.Redact("abc"); got != "abc" {
		t.Fatalf("Redact changed unrelated short text: %q", got)
	}
}
