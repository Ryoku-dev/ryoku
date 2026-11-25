package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"

	"go.etcd.io/bbolt"
)

// Hosted is a signed-in Ryoku Account as the router needs it: the account's
// models and a fresh bearer token for its server. The account service in
// internal/account implements it; the gateway never sees how sign-in works.
type Hosted interface {
	// Models are the hosted models, by their Ryoku names; none while signed
	// out, so the provider disappears.
	Models() []protocol.AccountModel
	// Endpoint is the server's OpenAI-compatible base, a token that is valid
	// now, and a context derived from ctx that ends when this account signs
	// out or another signs in, so a late answer never reaches the next one.
	// A nonempty expectedAccount must match while the request context is bound.
	Endpoint(ctx context.Context, expectedAccount string) (base, token string, bound context.Context, err error)
	// Rejected says the server refused the token, so it is refreshed or the
	// account signed out.
	Rejected()
	// Used says a hosted request ended, so the balance is read again.
	Used()
	// OnChange is called when the account signs in, out or changes models.
	OnChange(func())
}

var (
	bucketAccount   = []byte("account")
	accountTokenKey = []byte("tokens")
)

// hostedContext is the window the hosted models take; the server bounds
// each request's output itself.
const hostedContext = 131072

type hostedSlot struct {
	mu     sync.RWMutex
	hosted Hosted
}

// SetHosted hands the gateway the Ryoku Account; nil removes the hosted
// provider.
func (g *Gateway) SetHosted(h Hosted) {
	g.hosted.mu.Lock()
	g.hosted.hosted = h
	g.hosted.mu.Unlock()
	if h != nil {
		h.OnChange(func() {
			if g.ctx.Err() == nil {
				g.notifyChange()
			}
		})
	}
}

func (g *Gateway) account() Hosted {
	g.hosted.mu.RLock()
	defer g.hosted.mu.RUnlock()
	return g.hosted.hosted
}

// hostedProvider is Ryoku Ember while an account is signed in. It is paid, so
// Auto, Fast and Smart never pick it: only a request naming one of its models
// reaches it.
func (g *Gateway) hostedProvider() (provider, bool) {
	h := g.account()
	if h == nil {
		return provider{}, false
	}
	models := h.Models()
	if len(models) == 0 {
		return provider{}, false
	}
	p := provider{
		ID: protocol.ProviderEmber, Name: "Ryoku Ember", Tier: protocol.TierPaid, Auth: "ryoku", Keyless: true,
		Note: i18n.T("Uses Ember credits. Conversation messages and included context are sent to Ryoku Ember and its inference provider."),
	}
	for _, m := range models {
		p.Models = append(p.Models, model{ID: m.ID, Name: m.Name, Logo: m.Logo, Tier: protocol.TierPaid, Context: hostedContext, Tools: true})
	}
	return p, true
}

func (g *Gateway) hostedEndpoint(ctx context.Context) (string, string, context.Context, error) {
	h := g.account()
	if h == nil {
		return "", "", nil, errors.New(i18n.T("Sign in to your Ryoku Account to use its models."))
	}
	return h.Endpoint(ctx, "")
}

// AccountRecord is what is kept of a signed-in Ryoku Account: where it was
// signed in and its OAuth tokens. It is sealed with master.key like the
// provider keys, never logged and never sent to the app.
type AccountRecord struct {
	Server             string    `json:"server"`
	Issuer             string    `json:"issuer"`
	ClientID           string    `json:"clientID"`
	TokenEndpoint      string    `json:"tokenEndpoint"`
	RevocationEndpoint string    `json:"revocationEndpoint,omitempty"`
	Access             string    `json:"access"`
	Refresh            string    `json:"refresh,omitempty"`
	Expires            time.Time `json:"expires"`
}

// String and GoString keep the tokens out of any message that prints a
// record, whatever the verb.
func (r AccountRecord) String() string {
	return "AccountRecord{" + r.Server + ", tokens redacted}"
}

func (r AccountRecord) GoString() string { return r.String() }

// AccountTokens returns the kept account record, nil when there is none or
// it cannot be opened with master.key.
func (g *Gateway) AccountTokens() (*AccountRecord, error) {
	return g.store.accountRecord()
}

func (s *store) accountRecord() (*AccountRecord, error) {
	var record *AccountRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		sealed := tx.Bucket(bucketAccount).Get(accountTokenKey)
		if sealed == nil {
			return nil
		}
		text, err := s.open(sealed)
		if err != nil {
			return nil
		}
		var r AccountRecord
		if json.Unmarshal([]byte(text), &r) == nil && r.Access != "" {
			record = &r
		}
		return nil
	})
	return record, err
}

// SetAccountTokens seals and keeps the account record; nil forgets it. A
// token the record no longer holds stays registered with the scrubber until
// the daemon ends, so it cannot show in text a server echoed back.
func (g *Gateway) SetAccountTokens(record *AccountRecord) error {
	old, _ := g.store.accountRecord()
	if record != nil {
		// hidden even when the write fails and only memory holds the pair
		g.store.retired.add(record.Access, record.Refresh)
	}
	err := g.store.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAccount)
		if record == nil {
			return bucket.Delete(accountTokenKey)
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		sealed, err := g.store.seal(string(data))
		if err != nil {
			return err
		}
		return bucket.Put(accountTokenKey, sealed)
	})
	if old != nil {
		g.store.retired.add(old.Access, old.Refresh)
	}
	return err
}

func pinKey(server string) []byte { return []byte("issuer:" + server) }

// AccountIssuer is the identity provider a server named when an account
// first signed in at it, "" before that. It is kept sealed beside the
// tokens and is not forgotten when the account signs out.
func (g *Gateway) AccountIssuer(server string) (string, error) {
	issuer := ""
	err := g.store.db.View(func(tx *bbolt.Tx) error {
		sealed := tx.Bucket(bucketAccount).Get(pinKey(server))
		if sealed == nil {
			return nil
		}
		text, err := g.store.open(sealed)
		if err != nil {
			return nil
		}
		issuer = text
		return nil
	})
	return issuer, err
}

// SetAccountIssuer keeps the identity provider of a server; "" forgets it.
func (g *Gateway) SetAccountIssuer(server, issuer string) error {
	return g.store.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAccount)
		if issuer == "" {
			return bucket.Delete(pinKey(server))
		}
		sealed, err := g.store.seal(issuer)
		if err != nil {
			return err
		}
		return bucket.Put(pinKey(server), sealed)
	})
}

// retiredTokens are the account tokens the store no longer holds.
type retiredTokens struct {
	mu   sync.Mutex
	list []string
}

// retiredMax bounds what a long-running daemon keeps of past tokens.
const retiredMax = 512

func (r *retiredTokens) add(values ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range values {
		if v != "" && !slices.Contains(r.list, v) {
			r.list = append(r.list, v)
		}
	}
	if len(r.list) > retiredMax {
		r.list = r.list[len(r.list)-retiredMax:]
	}
}

func (r *retiredTokens) into(out map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.list {
		out[v] = "RYOKU_ACCOUNT_TOKEN"
	}
}

// noRedirects is c that stops at a redirect and hands it back as the answer,
// so a hosted request never follows the server to another place.
func noRedirects(c *http.Client) *http.Client {
	next := *c
	next.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &next
}

func errHostedProvider() error {
	return errors.New(i18n.T("The Ryoku models are paid from your account and are managed there, not as a provider."))
}
