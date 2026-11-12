package gateway

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Every stored provider key and the local API's key leave a text by name,
// in the forms a value comes back in, and nothing else in it changes.
func TestRedactNamesEveryStoredKey(t *testing.T) {
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
	if want := "sent ‹OPENROUTER_API_KEY› and ‹OPENROUTER_API_KEY› with ‹RASHIN_API_KEY›, abc stays"; got != want {
		t.Fatalf("Redact = %q, want %q", got, want)
	}
	if strings.Contains(g.Redact(""), "‹") {
		t.Fatal("an empty text gained a name")
	}
}
