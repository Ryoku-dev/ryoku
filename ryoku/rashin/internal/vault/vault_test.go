package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

// memoryKeeper stands in for the login keyring.
type memoryKeeper struct {
	stored    []byte
	missing   bool
	locked    bool
	dismissed bool
	created   int
}

func (k *memoryKeeper) Key(_ context.Context, create, prompt bool) ([]byte, error) {
	if k.missing {
		return nil, errNoKeyring
	}
	if k.locked {
		if !prompt || k.dismissed {
			return nil, errLocked
		}
		k.locked = false
	}
	if k.stored == nil {
		if !create {
			return nil, errNoKey
		}
		k.stored = make([]byte, keySize)
		if _, err := rand.Read(k.stored); err != nil {
			return nil, err
		}
		k.created++
	}
	return bytes.Clone(k.stored), nil
}

func openTest(t *testing.T, keys Keeper) (*Vault, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "vault.db")
	v := OpenWith(path, keys)
	t.Cleanup(func() { _ = v.Close() })
	return v, path
}

const githubToken = "ghp_0123456789abcdefghijklmnopqrstuv"

func addGitHub(t *testing.T, v *Vault) protocol.Credential {
	t.Helper()
	c, err := v.Add(protocol.VaultAddParams{
		Name: "GITHUB_TOKEN", Description: "Reads the organisation's pull requests", Use: protocol.CredentialHeader,
		Prefix: "Bearer ", Hosts: []string{"API.github.com"}, Teams: []string{"team-1"}, Value: githubToken,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	return c
}

func TestAddSealsTheValueAndRevealOpensIt(t *testing.T) {
	keys := &memoryKeeper{}
	v, path := openTest(t, keys)
	if state, _, keeper := v.State(); state != protocol.VaultOpen || keeper == "" {
		t.Fatalf("an empty vault with a keyring is %q kept by %q, want open", state, keeper)
	}
	c := addGitHub(t, v)
	if keys.created != 1 {
		t.Fatalf("the first value made %d keys, want 1", keys.created)
	}
	if c.Version != 1 || c.Header != "Authorization" || c.Hosts[0] != "api.github.com" || c.Fingerprint != fingerprint(fingerprintKey(keys.stored), githubToken) || len(c.Fingerprint) != 8 {
		t.Fatalf("unexpected credential %+v", c)
	}
	raw, err := json.Marshal(v.List())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), githubToken) {
		t.Fatal("the credential list holds the value")
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(file, []byte(githubToken)) {
		t.Fatal("the vault file holds the value in the clear")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("the vault file is %v, want 0600", info.Mode().Perm())
	}
	value, err := v.Reveal(c.ID)
	if err != nil || value != githubToken {
		t.Fatalf("reveal = %q, %v", value, err)
	}
}

func TestSealedValueIsBoundToItsCredentialAndVersion(t *testing.T) {
	key := make([]byte, keySize)
	aead, err := newCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := seal(aead, "credential-1", 1, "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unseal(aead, "credential-2", 1, sealed); err == nil {
		t.Fatal("a value opened under another credential")
	}
	if _, err := unseal(aead, "credential-1", 2, sealed); err == nil {
		t.Fatal("a value opened under another version")
	}
	if value, err := unseal(aead, "credential-1", 1, sealed); err != nil || value != "secret-value" {
		t.Fatalf("unseal = %q, %v", value, err)
	}
}

func TestReplacingAValueKeepsOnePreviousVersion(t *testing.T) {
	keys := &memoryKeeper{}
	v, _ := openTest(t, keys)
	c := addGitHub(t, v)
	second := "ghp_second_value_0000000000000000"
	updated, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Value: second})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Rotated == nil || updated.Fingerprint != fingerprint(fingerprintKey(keys.stored), second) {
		t.Fatalf("unexpected rotation %+v", updated)
	}
	if value, _ := v.Reveal(c.ID); value != second {
		t.Fatal("reveal does not give the new value")
	}
	if !hasValue(t, v, c.ID, 1) {
		t.Fatal("the previous version is gone after one replacement")
	}
	if _, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Value: "ghp_third_value_00000000000000000"}); err != nil {
		t.Fatal(err)
	}
	if hasValue(t, v, c.ID, 1) || !hasValue(t, v, c.ID, 2) || !hasValue(t, v, c.ID, 3) {
		t.Fatal("a second replacement must keep versions 2 and 3 only")
	}
}

func TestPatchEditsMetadataAndDropsStaleTrust(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	c := addGitHub(t, v)
	if err := v.Trust(c.ID, "team-1", "api.github.com"); err != nil {
		t.Fatal(err)
	}
	if err := v.Trust(c.ID, "team-2", "api.github.com"); err == nil {
		t.Fatal("a team without the credential was trusted")
	}
	if err := v.Trust(c.ID, "team-1", "evil.example.org"); err == nil {
		t.Fatal("a host outside the credential's hosts was trusted")
	}
	updated, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"description":"Reviews","teams":["team-2"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "Reviews" || len(updated.Trusted) != 0 || updated.Version != 1 {
		t.Fatalf("unexpected update %+v", updated)
	}
	if _, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"name":"OTHER"}`)}); err == nil {
		t.Fatal("a patch renamed the credential")
	}
	if _, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"hosts":null}`)}); err == nil {
		t.Fatal("a header credential lost every host")
	}
}

func TestRemoveDeletesEveryVersion(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	c := addGitHub(t, v)
	if _, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Value: "ghp_second_value_0000000000000000"}); err != nil {
		t.Fatal(err)
	}
	if err := v.Remove(c.ID); err != nil {
		t.Fatal(err)
	}
	if hasValue(t, v, c.ID, 1) || hasValue(t, v, c.ID, 2) {
		t.Fatal("a version survived removal")
	}
	if _, err := v.Reveal(c.ID); err == nil {
		t.Fatal("a removed credential revealed a value")
	}
	if len(v.List()) != 0 {
		t.Fatal("the removed credential is still listed")
	}
}

func TestTheMailPasswordStaysWhileTheAccountNamesIt(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	password, err := v.Add(protocol.VaultAddParams{Name: "SMTP_PASSWORD", Use: protocol.CredentialMail, Value: "correct horse battery"})
	if err != nil {
		t.Fatal(err)
	}
	account := &protocol.MailAccount{Address: "me@example.org", Host: "smtp.example.org", Port: 587, Security: protocol.MailStartTLS, Username: "me", Credential: password.ID}
	if err := v.SetMail(account); err != nil {
		t.Fatal(err)
	}
	if err := v.Remove(password.ID); err == nil {
		t.Fatal("the mail account's password was removed")
	}
	plain := *account
	plain.Security = protocol.MailPlain
	if err := v.SetMail(&plain); err == nil {
		t.Fatal("a remote server was allowed without encryption")
	}
	plain.Host = "localhost"
	if err := v.SetMail(&plain); err != nil {
		t.Fatalf("a local server without encryption was refused: %v", err)
	}
	if err := v.TrustMail("team-1", "Ana@Example.org"); err != nil {
		t.Fatal(err)
	}
	if err := v.DropTeam("team-1"); err != nil {
		t.Fatal(err)
	}
	if got := v.Mail(); got == nil || len(got.Trusted) != 0 {
		t.Fatalf("the dropped team still trusts a recipient: %+v", got)
	}
}

func TestRedactReplacesValuesLongestFirst(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	addGitHub(t, v)
	if _, err := v.Add(protocol.VaultAddParams{Name: "SHORT_PIN", Use: protocol.CredentialHeader, Hosts: []string{"example.org"}, Value: "1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add(protocol.VaultAddParams{Name: "TOKEN_PART", Use: protocol.CredentialHeader, Hosts: []string{"example.org"}, Value: "0123456789abcdef"}); err != nil {
		t.Fatal(err)
	}
	got := v.Redact("sent Bearer " + githubToken + " and 0123456789abcdef with pin 1234")
	want := "sent Bearer ‹GITHUB_TOKEN› and ‹TOKEN_PART› with pin 1234"
	if got != want {
		t.Fatalf("redact = %q, want %q", got, want)
	}
}

func TestValidationNeverEchoesTheValue(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	addGitHub(t, v)
	secret := "line-one\nline-two-secret"
	cases := []protocol.VaultAddParams{
		{Name: "github_token", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"},
		{Name: "G", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"},
		{Name: "GITHUB_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"},
		{Name: "NO_HOSTS", Use: protocol.CredentialHeader, Value: "value-123"},
		{Name: "BAD_HOST", Use: protocol.CredentialHeader, Hosts: []string{"https://a.org/path"}, Value: "value-123"},
		{Name: "WIDE_HOST", Use: protocol.CredentialHeader, Hosts: []string{"*.org"}, Value: "value-123"},
		{Name: "MAIL_HOSTS", Use: protocol.CredentialMail, Hosts: []string{"a.org"}, Value: "value-123"},
		{Name: "BAD_HEADER", Use: protocol.CredentialHeader, Header: "Content-Length", Hosts: []string{"a.org"}, Value: "value-123"},
		{Name: "EMPTY_VALUE", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}},
		{Name: "LONG_VALUE", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: strings.Repeat("x", maxValue+1)},
		{Name: "BROKEN_VALUE", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: secret},
	}
	for _, p := range cases {
		_, err := v.Add(p)
		if err == nil {
			t.Fatalf("%s was accepted", p.Name)
		}
		if p.Value != "" && strings.Contains(err.Error(), p.Value) {
			t.Fatalf("the error for %s holds the value", p.Name)
		}
	}
	if _, err := v.Add(protocol.VaultAddParams{Name: "PORTED", Use: protocol.CredentialHeader, Header: "x-api-key", Hosts: []string{"localhost:8025", "*.example.org"}, Value: "value-123"}); err != nil {
		t.Fatalf("a valid credential was refused: %v", err)
	}
}

func TestStatesFollowTheKeyring(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{missing: true})
	if state, reason, _ := v.State(); state != protocol.VaultUnavailable || reason == "" {
		t.Fatalf("no keyring gives %q (%q), want unavailable", state, reason)
	}
	if _, err := v.Add(protocol.VaultAddParams{Name: "SOME_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"}); err == nil {
		t.Fatal("a vault without a keyring took a value")
	}

	keys := &memoryKeeper{stored: make([]byte, keySize), locked: true, dismissed: true}
	locked, _ := openTest(t, keys)
	if state, _, _ := locked.State(); state != protocol.VaultClosed {
		t.Fatalf("a locked keyring gives %q, want closed", state)
	}
	if _, err := locked.Add(protocol.VaultAddParams{Name: "SOME_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"}); err == nil {
		t.Fatal("a dismissed unlock prompt still took a value")
	}
	keys.dismissed = false
	if _, err := locked.Add(protocol.VaultAddParams{Name: "SOME_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"}); err != nil {
		t.Fatalf("an unlocked keyring refused a value: %v", err)
	}
	if state, _, _ := locked.State(); state != protocol.VaultOpen {
		t.Fatalf("after unlocking the vault is %q, want open", state)
	}
}

func TestAVaultWhoseKeyIsGoneStaysClosed(t *testing.T) {
	keys := &memoryKeeper{}
	path := filepath.Join(t.TempDir(), "vault.db")
	first := OpenWith(path, keys)
	c := addGitHub(t, first)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	keys.stored = nil
	second := OpenWith(path, keys)
	defer second.Close()
	if state, _, _ := second.State(); state != protocol.VaultClosed {
		t.Fatalf("values without their key give %q, want closed", state)
	}
	_, reason, _ := second.State()
	_, err := second.Add(protocol.VaultAddParams{Name: "NEW_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"})
	if err == nil || keys.created != 1 || keys.stored != nil {
		t.Fatalf("adding over values whose key is gone gave %v and made %d keys, want a refusal and no new key", err, keys.created)
	}
	if !strings.Contains(err.Error(), reason) {
		t.Fatalf("the refusal says %q, want the vault's own reason %q", err, reason)
	}
	if err := second.Remove(c.ID); err != nil {
		t.Fatal(err)
	}
	if state, _, _ := second.State(); state != protocol.VaultOpen {
		t.Fatalf("once nothing sealed is left the vault is %q, want open", state)
	}
	if _, err := second.Add(protocol.VaultAddParams{Name: "NEW_TOKEN", Use: protocol.CredentialHeader, Hosts: []string{"a.org"}, Value: "value-123"}); err != nil || keys.created != 2 {
		t.Fatalf("an emptied vault gave %v and made %d keys, want a new key for the new value", err, keys.created)
	}
}

// bareFingerprint is how fingerprints were made before they were keyed.
func bareFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

func TestFingerprintsAreKeyedByTheVaultKey(t *testing.T) {
	first, _ := openTest(t, &memoryKeeper{})
	second, _ := openTest(t, &memoryKeeper{})
	a, b := addGitHub(t, first), addGitHub(t, second)
	if a.Fingerprint == b.Fingerprint {
		t.Fatal("one value has the same fingerprint under two vault keys")
	}
	if a.Fingerprint == bareFingerprint(githubToken) {
		t.Fatal("the fingerprint is the value's bare SHA-256, which confirms a guessed value")
	}
}

func TestOpeningMakesOldFingerprintsAgainUnderTheKey(t *testing.T) {
	keys := &memoryKeeper{}
	path := filepath.Join(t.TempDir(), "vault.db")
	first := OpenWith(path, keys)
	c := addGitHub(t, first)
	c.Fingerprint = bareFingerprint(githubToken)
	if err := first.db.Update(func(tx *bbolt.Tx) error { return putCredential(tx, c) }); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := OpenWith(path, keys)
	defer second.Close()
	if got, _ := second.Get(c.ID); got.Fingerprint != fingerprint(fingerprintKey(keys.stored), githubToken) {
		t.Fatalf("an old fingerprint stayed %q after opening", got.Fingerprint)
	}
}

func TestScrubTakesOutEveryFormOfAValue(t *testing.T) {
	value := "ghp_a/b+c=d&e<f>"
	raw := []byte(value)
	quoted, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	forms := []string{
		value,
		base64.StdEncoding.EncodeToString(raw), base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw), base64.RawURLEncoding.EncodeToString(raw),
		url.QueryEscape(value), url.PathEscape(value), string(quoted),
	}
	got := Scrub(strings.Join(forms, " | "), value, "API_TOKEN")
	want := strings.TrimSuffix(strings.Repeat("‹API_TOKEN› | ", len(forms)-1), " | ") + ` | "‹API_TOKEN›"`
	if got != want {
		t.Fatalf("scrub = %q, want %q", got, want)
	}

	v, _ := openTest(t, &memoryKeeper{})
	addGitHub(t, v)
	encoded := base64.StdEncoding.EncodeToString([]byte(githubToken))
	if got := v.Redact("echo " + encoded + " and " + url.QueryEscape(githubToken)); got != "echo ‹GITHUB_TOKEN› and ‹GITHUB_TOKEN›" {
		t.Fatalf("redact = %q, want every form named", got)
	}
}

func TestRecordKeepsTheNewestAndCountsUses(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	c := addGitHub(t, v)
	for range keepEvents + 5 {
		if err := v.Record(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Outcome: protocol.VaultUsed, Detail: "sent " + githubToken}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := v.Events(protocol.VaultEventsParams{Limit: keepEvents + 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != keepEvents {
		t.Fatalf("the record holds %d events, want %d", len(events), keepEvents)
	}
	if strings.Contains(events[0].Detail, githubToken) {
		t.Fatal("an event holds the value")
	}
	got, _ := v.Get(c.ID)
	if got.Uses != keepEvents+5 || got.LastUsed == nil {
		t.Fatalf("uses = %d, want %d", got.Uses, keepEvents+5)
	}
	other, err := v.Events(protocol.VaultEventsParams{Credential: "credential-other"})
	if err != nil || len(other) != 0 {
		t.Fatalf("events of another credential: %d, %v", len(other), err)
	}
}

func TestKeeperErrorsMapToStates(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{errNoKeyring, protocol.VaultUnavailable}, {errLocked, protocol.VaultClosed}, {errors.New("bus gone"), protocol.VaultUnavailable}} {
		v, _ := openTest(t, failingKeeper{tc.err})
		if state, _, _ := v.State(); state != tc.want {
			t.Errorf("%v gives %q, want %q", tc.err, state, tc.want)
		}
	}
}

type failingKeeper struct{ err error }

func (k failingKeeper) Key(context.Context, bool, bool) ([]byte, error) { return nil, k.err }

func hasValue(t *testing.T, v *Vault, id string, version int) bool {
	t.Helper()
	found := false
	if err := v.db.View(func(tx *bbolt.Tx) error {
		found = tx.Bucket(bucketValues).Get(valueKey(id, version)) != nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return found
}
