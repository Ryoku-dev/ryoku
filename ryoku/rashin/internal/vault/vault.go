// Package vault keeps the credentials Rashin's teams use without ever seeing
// them. Values are sealed with AES-256-GCM in a bbolt file; the key lives
// only in the login keyring and in the daemon's memory while it runs. No
// method hands a value to a client: Reveal serves the daemon itself, which
// adds a value to a request or a mail login on a team's behalf.
package vault

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

var (
	bucketCredentials = []byte("credentials")
	bucketValues      = []byte("values")
	bucketEvents      = []byte("events")
	bucketMail        = []byte("mail")
)

// How long the vault waits on the keyring: briefly when it only looks for the
// key, longer when the user may be typing the keyring's password.
const (
	lookupWait = 10 * time.Second
	promptWait = 2 * time.Minute
)

// why the vault is not open, translated when asked so a language change
// reaches it
type why int

const (
	whyNone why = iota
	whyNoKeyring
	whyLocked
	whyNoKey
	whyFile
)

// Vault is the credential store. It never fails to open: a vault without a
// keyring or a readable file reports why through State and refuses what
// needs it.
type Vault struct {
	keys Keeper
	now  func() time.Time
	db   *bbolt.DB

	// unlocking serialises trips to the keyring, which may wait on the user
	unlocking sync.Mutex

	mu    sync.Mutex
	aead  cipher.AEAD
	state string
	why   why

	// mark is the key fingerprints are made with, derived from the vault key
	// and guarded by mu like aead
	mark []byte
}

// Open opens the vault file at path and looks for its key in the login
// keyring without asking the user anything.
func Open(path string) *Vault { return OpenWith(path, secretService{}) }

// OpenWith opens the vault file at path with its key kept by keys.
func OpenWith(path string, keys Keeper) *Vault {
	v := &Vault{keys: keys, now: time.Now, state: protocol.VaultUnavailable, why: whyFile}
	db, err := openFile(path)
	if err != nil {
		return v
	}
	v.db = db
	ctx, cancel := context.WithTimeout(context.Background(), lookupWait)
	defer cancel()
	_ = v.unlock(ctx, false, false)
	return v
}

func openFile(path string) (*bbolt.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketCredentials, bucketValues, bucketEvents, bucketMail} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// State says whether the vault is open, why not, and what keeps its key.
func (v *Vault) State() (state, reason, keeper string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state != protocol.VaultUnavailable {
		keeper = i18n.T("your login keyring")
	}
	return v.state, v.reasonLocked(), keeper
}

func (v *Vault) reasonLocked() string {
	switch v.why {
	case whyNoKeyring:
		return i18n.T("There is no login keyring, so the vault stays closed.")
	case whyLocked:
		return i18n.T("Your login keyring is locked.")
	case whyNoKey:
		return i18n.T("The vault's key is missing from your login keyring, so its values cannot be read.")
	case whyFile:
		return i18n.T("The vault's file cannot be opened.")
	}
	return ""
}

// closedError refuses what needs the key while the vault is not open.
func (v *Vault) closedError() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	reason := v.reasonLocked()
	if reason == "" {
		reason = i18n.T("The vault is closed.")
	}
	return protocol.Errorf(protocol.CodeUnavailable, "%s", reason)
}

func (v *Vault) cipher() cipher.AEAD {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.aead
}

// fingerprintOf is a value's fingerprint under the vault's key.
func (v *Vault) fingerprintOf(value string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return fingerprint(v.mark, value)
}

// unlock fetches the key from the keeper once; after that it stays in memory
// until Close. create makes a key when the keyring has none, prompt lets the
// keyring ask the user to unlock.
func (v *Vault) unlock(ctx context.Context, create, prompt bool) error {
	v.unlocking.Lock()
	defer v.unlocking.Unlock()
	if v.cipher() != nil {
		return nil
	}
	// a new key would leave every value sealed under the missing one unreadable
	// for good, so a key is made only while the vault holds no value
	key, err := v.keys.Key(ctx, create && !v.holdsValues(), prompt)
	var aead cipher.AEAD
	var mark []byte
	if err == nil {
		aead, err = newCipher(key)
		mark = fingerprintKey(key)
		clear(key)
	}
	v.mu.Lock()
	switch {
	case err == nil:
		v.aead, v.mark, v.state, v.why = aead, mark, protocol.VaultOpen, whyNone
	case errors.Is(err, errNoKey):
		// with nothing sealed yet the vault is open: the first value makes the key
		v.state, v.why = protocol.VaultOpen, whyNone
		if v.holdsValues() {
			v.state, v.why = protocol.VaultClosed, whyNoKey
		}
	case errors.Is(err, errLocked):
		v.state, v.why = protocol.VaultClosed, whyLocked
	default:
		v.state, v.why = protocol.VaultUnavailable, whyNoKeyring
	}
	v.mu.Unlock()
	if err == nil {
		v.refingerprint(aead, mark)
	}
	return err
}

// refingerprint makes every fingerprint again under mark where it differs,
// as for a credential stored before fingerprints were keyed.
func (v *Vault) refingerprint(aead cipher.AEAD, mark []byte) {
	stale := map[string]protocol.Credential{}
	_ = v.db.View(func(tx *bbolt.Tx) error {
		sealed := tx.Bucket(bucketValues)
		return tx.Bucket(bucketCredentials).ForEach(func(_, raw []byte) error {
			var c protocol.Credential
			if json.Unmarshal(raw, &c) != nil {
				return nil
			}
			value, err := unseal(aead, c.ID, c.Version, sealed.Get(valueKey(c.ID, c.Version)))
			if err != nil {
				return nil
			}
			if made := fingerprint(mark, value); made != c.Fingerprint {
				c.Fingerprint = made
				stale[c.ID] = c
			}
			return nil
		})
	})
	if len(stale) == 0 {
		return
	}
	_ = v.db.Update(func(tx *bbolt.Tx) error {
		for id, made := range stale {
			c, err := loadCredential(tx, id)
			if err != nil || c.Version != made.Version {
				continue
			}
			c.Fingerprint = made.Fingerprint
			if err := putCredential(tx, c); err != nil {
				return err
			}
		}
		return nil
	})
}

func (v *Vault) holdsValues() bool {
	held := false
	_ = v.db.View(func(tx *bbolt.Tx) error {
		key, _ := tx.Bucket(bucketValues).Cursor().First()
		held = key != nil
		return nil
	})
	return held
}

// ready returns the cipher, asking the keyring for the key, and the user to
// unlock it, when the vault does not hold it yet.
func (v *Vault) ready(create bool) (cipher.AEAD, error) {
	if v.db == nil {
		return nil, v.closedError()
	}
	if aead := v.cipher(); aead != nil {
		return aead, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), promptWait)
	defer cancel()
	if v.unlock(ctx, create, true) != nil {
		return nil, v.closedError()
	}
	return v.cipher(), nil
}

// Unlock makes sure the vault holds its key, asking the keyring, and the
// user to unlock it, when it does not. An empty vault without a key yet is
// open and needs none.
func (v *Vault) Unlock() error {
	if _, err := v.ready(false); err != nil {
		if state, _, _ := v.State(); state != protocol.VaultOpen {
			return err
		}
	}
	return nil
}

// List returns every credential by name, without any value.
func (v *Vault) List() []protocol.Credential {
	list := []protocol.Credential{}
	if v.db == nil {
		return list
	}
	_ = v.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketCredentials).ForEach(func(_, raw []byte) error {
			var c protocol.Credential
			if json.Unmarshal(raw, &c) == nil {
				list = append(list, c)
			}
			return nil
		})
	})
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// Find returns the credential an agent names.
func (v *Vault) Find(name string) (protocol.Credential, bool) {
	for _, c := range v.List() {
		if c.Name == name {
			return c, true
		}
	}
	return protocol.Credential{}, false
}

// Get returns one credential by id.
func (v *Vault) Get(id string) (protocol.Credential, bool) {
	if v.db == nil {
		return protocol.Credential{}, false
	}
	var c protocol.Credential
	found := false
	_ = v.db.View(func(tx *bbolt.Tx) error {
		var err error
		c, err = loadCredential(tx, id)
		found = err == nil
		return nil
	})
	return c, found
}

func loadCredential(tx *bbolt.Tx, id string) (protocol.Credential, error) {
	var c protocol.Credential
	raw := tx.Bucket(bucketCredentials).Get([]byte(id))
	if raw == nil {
		return c, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That credential is not in the vault."))
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}

func putCredential(tx *bbolt.Tx, c protocol.Credential) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketCredentials).Put([]byte(c.ID), raw)
}

func nameTaken(tx *bbolt.Tx, name string) bool {
	taken := false
	_ = tx.Bucket(bucketCredentials).ForEach(func(_, raw []byte) error {
		var c protocol.Credential
		if json.Unmarshal(raw, &c) == nil && c.Name == name {
			taken = true
		}
		return nil
	})
	return taken
}

func takenError(name string) error {
	return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The vault already holds a credential named %s.", name))
}

// Add seals a new credential. Its value is never returned. While the vault
// holds values whose key is gone from the keyring it refuses, as closed,
// rather than make a new key: the user removes those credentials first.
func (v *Vault) Add(p protocol.VaultAddParams) (protocol.Credential, error) {
	c := protocol.Credential{
		Name: strings.TrimSpace(p.Name), Description: p.Description, Use: p.Use,
		Header: strings.TrimSpace(p.Header), Prefix: p.Prefix, Hosts: p.Hosts, Teams: p.Teams, Version: 1,
	}
	if err := CheckName(c.Name); err != nil {
		return protocol.Credential{}, err
	}
	if err := checkShape(&c); err != nil {
		return protocol.Credential{}, err
	}
	p.Value = pasted(p.Value)
	if err := checkValue(p.Value); err != nil {
		return protocol.Credential{}, err
	}
	if _, taken := v.Find(c.Name); taken {
		return protocol.Credential{}, takenError(c.Name)
	}
	aead, err := v.ready(true)
	if err != nil {
		return protocol.Credential{}, err
	}
	c.ID = newID()
	c.Fingerprint = v.fingerprintOf(p.Value)
	c.Created = v.now().UTC()
	sealed, err := seal(aead, c.ID, c.Version, p.Value)
	if err != nil {
		return protocol.Credential{}, err
	}
	err = v.db.Update(func(tx *bbolt.Tx) error {
		if nameTaken(tx, c.Name) {
			return takenError(c.Name)
		}
		if err := tx.Bucket(bucketValues).Put(valueKey(c.ID, c.Version), sealed); err != nil {
			return err
		}
		return putCredential(tx, c)
	})
	if err != nil {
		return protocol.Credential{}, err
	}
	return c, nil
}

// Update changes a credential's description, header, prefix, hosts, teams
// or trusted pairs through a JSON merge patch and, with a value, replaces
// its value. The replaced value stays as the previous version until the
// next replacement.
func (v *Vault) Update(p protocol.VaultUpdateParams) (protocol.Credential, error) {
	current, ok := v.Get(p.ID)
	if !ok {
		return protocol.Credential{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That credential is not in the vault."))
	}
	// check the change before the keyring may ask the user anything
	if err := applyPatch(&current, p.Patch); err != nil {
		return protocol.Credential{}, err
	}
	if err := checkShape(&current); err != nil {
		return protocol.Credential{}, err
	}
	var aead cipher.AEAD
	var made string
	p.Value = pasted(p.Value)
	if p.Value != "" {
		if err := checkValue(p.Value); err != nil {
			return protocol.Credential{}, err
		}
		var err error
		if aead, err = v.ready(false); err != nil {
			return protocol.Credential{}, err
		}
		made = v.fingerprintOf(p.Value)
	}
	var updated protocol.Credential
	err := v.db.Update(func(tx *bbolt.Tx) error {
		c, err := loadCredential(tx, p.ID)
		if err != nil {
			return err
		}
		if err := applyPatch(&c, p.Patch); err != nil {
			return err
		}
		if err := checkShape(&c); err != nil {
			return err
		}
		if aead != nil {
			c.Version++
			sealed, err := seal(aead, c.ID, c.Version, p.Value)
			if err != nil {
				return err
			}
			values := tx.Bucket(bucketValues)
			if err := values.Put(valueKey(c.ID, c.Version), sealed); err != nil {
				return err
			}
			if c.Version > 2 {
				if err := values.Delete(valueKey(c.ID, c.Version-2)); err != nil {
					return err
				}
			}
			now := v.now().UTC()
			c.Fingerprint, c.Rotated = made, &now
		}
		updated = c
		return putCredential(tx, c)
	})
	if err != nil {
		return protocol.Credential{}, err
	}
	return updated, nil
}

// editable names what a patch may change and the JSON a null resets it to.
var editable = map[string]string{
	"description": `""`, "header": `""`, "prefix": `""`,
	"hosts": `[]`, "teams": `[]`, "trusted": `[]`,
}

// applyPatch applies a JSON merge patch to the fields a user may edit.
func applyPatch(c *protocol.Credential, raw json.RawMessage) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(raw, &patch); err != nil {
		return badRequest(i18n.T("The change is not a JSON object."))
	}
	for key, value := range patch {
		reset, ok := editable[key]
		if !ok {
			return badRequest(i18n.Tf("%q cannot be changed.", key))
		}
		if string(bytes.TrimSpace(value)) == "null" {
			value = json.RawMessage(reset)
		}
		var target any
		switch key {
		case "description":
			target = &c.Description
		case "header":
			target = &c.Header
		case "prefix":
			target = &c.Prefix
		case "hosts":
			target = &c.Hosts
		case "teams":
			target = &c.Teams
		case "trusted":
			target = &c.Trusted
		}
		if err := json.Unmarshal(value, target); err != nil {
			return badRequest(i18n.Tf("%q has the wrong shape.", key))
		}
	}
	return nil
}

// Remove deletes a credential and every version of its value. The mail
// account's password stays until the account names another.
func (v *Vault) Remove(id string) error {
	if v.db == nil {
		return v.closedError()
	}
	err := v.db.Update(func(tx *bbolt.Tx) error {
		c, err := loadCredential(tx, id)
		if err != nil {
			return err
		}
		if account := loadAccount(tx); account != nil && account.Credential == id {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s is the mail account's password. Change the mail account first.", c.Name))
		}
		values := tx.Bucket(bucketValues)
		var keys [][]byte
		prefix := []byte(id + "/")
		cursor := values.Cursor()
		for key, _ := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, _ = cursor.Next() {
			keys = append(keys, slices.Clone(key))
		}
		for _, key := range keys {
			if err := values.Delete(key); err != nil {
				return err
			}
		}
		return tx.Bucket(bucketCredentials).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	v.settle()
	return nil
}

// settle opens a vault that was closed only for a missing key once nothing
// sealed under the old key is left.
func (v *Vault) settle() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.why == whyNoKey && !v.holdsValues() {
		v.state, v.why = protocol.VaultOpen, whyNone
	}
}

// Reveal opens a credential's current value for the daemon to use. It is
// the only way to a value and never reaches a client.
func (v *Vault) Reveal(id string) (string, error) {
	c, ok := v.Get(id)
	if !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That credential is not in the vault."))
	}
	aead, err := v.ready(false)
	if err != nil {
		return "", err
	}
	var sealed []byte
	_ = v.db.View(func(tx *bbolt.Tx) error {
		sealed = slices.Clone(tx.Bucket(bucketValues).Get(valueKey(c.ID, c.Version)))
		return nil
	})
	value, err := unseal(aead, c.ID, c.Version, sealed)
	if err != nil {
		return "", protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The value of %s cannot be read.", c.Name))
	}
	return value, nil
}

// Trust lets one team send a credential to one host without asking. The
// team must be allowed the credential and the host must be one it may go to.
func (v *Vault) Trust(id, team, host string) error {
	if v.db == nil {
		return v.closedError()
	}
	return v.db.Update(func(tx *bbolt.Tx) error {
		c, err := loadCredential(tx, id)
		if err != nil {
			return err
		}
		trust := protocol.CredentialTrust{Team: team, Host: strings.ToLower(host)}
		if !slices.Contains(c.Teams, team) || !hostrule.Allows(c.Hosts, trust.Host) {
			return badRequest(i18n.Tf("%s may not go to %s for this team.", c.Name, host))
		}
		if slices.Contains(c.Trusted, trust) {
			return nil
		}
		c.Trusted = append(c.Trusted, trust)
		return putCredential(tx, c)
	})
}

// DropTeam takes a team out of every credential's teams and trusted pairs
// and out of the mail account's trusted recipients.
func (v *Vault) DropTeam(team string) error {
	if v.db == nil {
		return nil
	}
	return v.db.Update(func(tx *bbolt.Tx) error {
		var changed []protocol.Credential
		_ = tx.Bucket(bucketCredentials).ForEach(func(_, raw []byte) error {
			var c protocol.Credential
			if json.Unmarshal(raw, &c) != nil {
				return nil
			}
			teams := slices.DeleteFunc(slices.Clone(c.Teams), func(t string) bool { return t == team })
			trusted := slices.DeleteFunc(slices.Clone(c.Trusted), func(t protocol.CredentialTrust) bool { return t.Team == team })
			if len(teams) != len(c.Teams) || len(trusted) != len(c.Trusted) {
				c.Teams, c.Trusted = teams, trusted
				changed = append(changed, c)
			}
			return nil
		})
		for _, c := range changed {
			if err := putCredential(tx, c); err != nil {
				return err
			}
		}
		account := loadAccount(tx)
		if account == nil {
			return nil
		}
		before := len(account.Trusted)
		account.Trusted = slices.DeleteFunc(account.Trusted, func(t protocol.MailTrust) bool { return t.Team == team })
		if len(account.Trusted) == before {
			return nil
		}
		return putAccount(tx, account)
	})
}

// Close forgets the key and closes the file.
func (v *Vault) Close() error {
	v.mu.Lock()
	v.aead, v.mark = nil, nil
	v.mu.Unlock()
	if v.db == nil {
		return nil
	}
	return v.db.Close()
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "credential-" + hex.EncodeToString(b)
}
