package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var cardAccountIDPattern = regexp.MustCompile(`^ra_[a-z0-9]{8,64}$`)
var cardAccountChallengePattern = regexp.MustCompile(`^dch_[a-z2-9]{32}$`)
var cardAccountPolicyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)
var cardAccountDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type cardAccountChallenge struct {
	ID      string `json:"challengeID"`
	Input   string `json:"signingInput"`
	Expires int64  `json:"expiresAt"`
	CardID  string `json:"cardID"`
	Origin  string `json:"origin"`
}
type cardAccountLink struct {
	Server    string
	AccountID string
	Challenge cardAccountChallenge
}
type cardAccountConsentStatus struct {
	Accepted   bool  `json:"accepted"`
	AcceptedAt int64 `json:"acceptedAt"`
	Policy     struct {
		Scope         string `json:"scope"`
		Version       string `json:"version"`
		DisclosureSHA string `json:"disclosureSha256"`
		Documents     []struct {
			ID  string `json:"id"`
			SHA string `json:"sha256"`
		} `json:"documents"`
	} `json:"policy"`
}
type cardAccountConsent struct {
	Account cardAccountConsentStatus `json:"account"`
	Cards   cardAccountConsentStatus `json:"cards"`
}

func (c *cardAccountConsentStatus) current(scope string) bool {
	if !c.Accepted || c.AcceptedAt <= 0 || c.Policy.Scope != scope || !cardAccountPolicyVersionPattern.MatchString(c.Policy.Version) || !cardAccountDigestPattern.MatchString(c.Policy.DisclosureSHA) {
		return false
	}
	expected := 7
	if scope == "cards" {
		expected = 8
	} else if scope != "account" {
		return false
	}
	seen := 0
	for _, doc := range c.Policy.Documents {
		bit := 0
		switch doc.ID {
		case "terms":
			bit = 1
		case "privacy":
			bit = 2
		case "refunds":
			bit = 4
		case "content-review":
			bit = 8
		}
		if bit == 0 || bit&expected == 0 || seen&bit != 0 || !cardAccountDigestPattern.MatchString(doc.SHA) {
			return false
		}
		seen |= bit
	}
	return seen == expected
}
func (c *cardAccountConsent) cardsReady() bool {
	return c.Account.current("account") && c.Cards.current("cards")
}

func cardAccountB64(text string, size int) ([]byte, error) {
	b, e := base64.StdEncoding.DecodeString(text)
	if e != nil || (size >= 0 && len(b) != size) || base64.StdEncoding.EncodeToString(b) != text {
		return nil, errors.New("Invalid canonical base64")
	}
	return b, nil
}
func cardAccountLinkInput(c cardAccountChallenge, server, account string, pub []byte, now time.Time) ([]byte, error) {
	if !cardAccountChallengePattern.MatchString(c.ID) || len(c.Input) > 3000 || !cardAccountIDPattern.MatchString(account) {
		return nil, errors.New("Invalid Account link challenge")
	}
	input, e := cardAccountB64(c.Input, -1)
	if e != nil {
		return nil, e
	}
	domain := []byte("ryoku-card-link-v1\x00")
	if !bytes.HasPrefix(input, domain) {
		return nil, errors.New("Refusing to sign another action")
	}
	var fields []json.RawMessage
	if json.Unmarshal(input[len(domain):], &fields) != nil || len(fields) != 6 {
		return nil, errors.New("Invalid link frame")
	}
	var accountID, card, key, nonce, origin string
	var expires int64
	for i, dst := range []any{&accountID, &card, &key, &nonce, &origin, &expires} {
		if json.Unmarshal(fields[i], dst) != nil {
			return nil, errors.New("Invalid link field")
		}
	}
	exact, e := cardAccountJSON([]any{accountID, card, key, nonce, origin, expires})
	if e != nil || !bytes.Equal(exact, input[len(domain):]) {
		return nil, errors.New("Noncanonical link frame")
	}
	if _, e = cardAccountB64(nonce, 32); e != nil {
		return nil, e
	}
	if accountID != account || card != cardIDText(pub) || key != base64.StdEncoding.EncodeToString(pub) || origin != server || c.Origin != server || c.CardID != card || c.Expires != expires || expires <= now.UnixMilli() || expires > now.Add(5*time.Minute).UnixMilli() {
		return nil, errors.New("Link challenge does not match this account, device, origin or expiry")
	}
	return input, nil
}
func cardAccountStart(ctx context.Context) (any, error) {
	server, e := cardAccountServer()
	if e != nil {
		return nil, e
	}
	unlock, e := cardAccountLock(ctx)
	if e != nil {
		return nil, e
	}
	defer unlock()
	cfg, meta, e := cardAccountDiscover(ctx, server)
	if e != nil {
		return nil, e
	}
	// Check durable keyring availability before asking the user to authorize.
	if e = cardAccountSecret(ctx, server, "store", "availability", map[string]bool{"available": true}, nil); e != nil {
		return nil, e
	}
	if e = cardAccountSecret(ctx, server, "clear", "availability", nil, nil); e != nil {
		return nil, e
	}
	scopes := []string{}
	for _, scope := range strings.Fields(cfg.Auth.Scopes) {
		if !strings.HasPrefix(scope, "ryoku-server-") {
			scopes = append(scopes, scope)
		}
	}
	scopes = append(scopes, cardAccountScope(server))
	var answer struct {
		Device   string `json:"device_code"`
		Code     string `json:"user_code"`
		URI      string `json:"verification_uri"`
		Complete string `json:"verification_uri_complete"`
		Expires  int    `json:"expires_in"`
		Interval int    `json:"interval"`
		Error    string `json:"error"`
	}
	_, e = cardAccountForm(ctx, meta.Device, url.Values{"client_id": {cfg.Auth.ClientID}, "scope": {strings.Join(scopes, " ")}}, &answer)
	if answer.Error == "invalid_scope" {
		return nil, errors.New("This server is not registered for Ryoku sign-in; its server scope must be enabled in Clerk")
	}
	if e != nil {
		return nil, errors.New("The identity provider refused device sign-in")
	}
	uri := answer.Complete
	if uri == "" {
		uri = answer.URI
	}
	u, ue := url.Parse(uri)
	if ue != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || answer.Device == "" || len(answer.Device) > 4096 || answer.Code == "" || len(answer.Code) > 80 || strings.ContainsAny(answer.Code, "\x00\r\n") || answer.Expires < 1 || answer.Expires > 3600 {
		return nil, errors.New("The identity provider returned an invalid device code")
	}
	if answer.Interval < 1 {
		answer.Interval = 5
	}
	if answer.Interval > 60 {
		return nil, errors.New("The identity provider returned an invalid polling interval")
	}
	now := time.Now().Unix()
	p := cardAccountPending{Server: server, Issuer: meta.Issuer, ClientID: cfg.Auth.ClientID, Endpoint: meta.Token, Revocation: meta.Revocation, Device: answer.Device, UserCode: answer.Code, Verification: uri, Expires: now + int64(answer.Expires), Next: now + int64(answer.Interval), Interval: int64(answer.Interval)}
	if e = cardAccountSecret(ctx, server, "store", "pending", p, nil); e != nil {
		return nil, e
	}
	if e = cardAccountOpen(ctx, uri); e != nil {
		_ = cardAccountSecret(ctx, server, "clear", "pending", nil, nil)
		return nil, e
	}
	return cardAccountFlow(p), nil
}
func cardAccountFlow(p cardAccountPending) any {
	return map[string]any{"state": "pending", "userCode": p.UserCode, "verification": p.Verification, "expiresAt": p.Expires, "nextPollAt": p.Next}
}
func cardAccountPoll(ctx context.Context) (any, error) {
	server, e := cardAccountServer()
	if e != nil {
		return nil, e
	}
	unlock, e := cardAccountLock(ctx)
	if e != nil {
		return nil, e
	}
	defer unlock()
	var p cardAccountPending
	if e = cardAccountSecret(ctx, server, "lookup", "pending", nil, &p); e != nil {
		return nil, errors.New("No Account sign-in is waiting")
	}
	if p.Server != server || !cardAccountEndpoint(p.Endpoint, p.Issuer) || p.Interval < 1 || p.Interval > 60 {
		return nil, errors.New("Invalid pending Account sign-in")
	}
	now := time.Now().Unix()
	if now >= p.Expires {
		_ = cardAccountSecret(ctx, server, "clear", "pending", nil, nil)
		return nil, errors.New("The sign-in code expired; sign in again")
	}
	if now < p.Next {
		return cardAccountFlow(p), nil
	}
	// Persist next time before making the request, including when it fails.
	p.Next = now + p.Interval
	if e = cardAccountSecret(ctx, server, "store", "pending", p, nil); e != nil {
		return nil, e
	}
	var answer cardAccountToken
	status, e := cardAccountForm(ctx, p.Endpoint, url.Values{"client_id": {p.ClientID}, "grant_type": {cardAccountGrant}, "device_code": {p.Device}}, &answer)
	switch answer.Error {
	case "authorization_pending":
		return cardAccountFlow(p), nil
	case "slow_down":
		p.Interval += 5
		if p.Interval > 60 {
			p.Interval = 60
		}
		p.Next = now + p.Interval
		if e = cardAccountSecret(ctx, server, "store", "pending", p, nil); e != nil {
			return nil, e
		}
		return cardAccountFlow(p), nil
	case "access_denied", "expired_token":
		_ = cardAccountSecret(ctx, server, "clear", "pending", nil, nil)
		return nil, errors.New("Account sign-in was denied or expired")
	case "":
	default:
		_ = cardAccountSecret(ctx, server, "clear", "pending", nil, nil)
		return nil, errors.New("Account sign-in was refused")
	}
	if e != nil || status != 200 {
		return nil, errors.New("Account sign-in could not be checked; retry before the code expires")
	}
	if answer.Access == "" || cardAccountTTL(answer.Expires) == 0 {
		return nil, errors.New("Account sign-in returned invalid credentials")
	}
	var identity cardAccountIdentity
	if _, e = cardAccountRequest(ctx, "GET", server+"/v1/account", answer.Access, "", nil, &identity); e != nil {
		return nil, e
	}
	if !cardAccountIDPattern.MatchString(identity.Account.ID) {
		return nil, errors.New("Account service returned an invalid identity")
	}
	record := cardAccountRecord{Server: server, Issuer: p.Issuer, ClientID: p.ClientID, Endpoint: p.Endpoint, Revocation: p.Revocation, Access: answer.Access, Refresh: answer.Refresh, Expires: cardAccountTTL(answer.Expires), AccountID: identity.Account.ID}
	if e = cardAccountSecret(ctx, server, "store", "tokens", record, nil); e != nil {
		return nil, e
	}
	if e = cardAccountSecret(ctx, server, "clear", "pending", nil, nil); e != nil {
		return nil, e
	}
	return map[string]any{"state": "signed_in", "accountID": identity.Account.ID}, nil
}
func cardAccountStatus(ctx context.Context) (any, error) {
	server, e := cardAccountServer()
	if e != nil {
		return map[string]any{"state": "unconfigured", "message": e.Error()}, nil
	}
	var pending cardAccountPending
	if e = cardAccountSecret(ctx, server, "lookup", "pending", nil, &pending); e == nil && pending.Server == server && pending.Expires > time.Now().Unix() {
		return cardAccountFlow(pending), nil
	} else if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return map[string]any{"state": "signed_out", "server": server, "message": e.Error()}, nil
	}
	var identity cardAccountIdentity
	var consent cardAccountConsent
	var creator any
	var devices any
	for _, q := range []struct {
		path string
		out  any
	}{{"/v1/account", &identity}, {"/v1/consent", &consent}, {"/v1/creator", &creator}, {"/v1/card-devices", &devices}} {
		if e = cardAccountAPI(ctx, server, token, "GET", q.path, nil, q.out); e != nil {
			return nil, e
		}
	}
	localCardID := ""
	seed, e := badgeExistingDeviceSeed()
	if e == nil {
		localCardID = cardIDText(badgeDevicePublic(seed))
		clear(seed)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	var selection cardAccountSelection
	_ = cardAccountRead("publication.json", &selection)
	var job cardAccountJob
	_ = cardAccountRead("submission.json", &job)
	return map[string]any{"state": "signed_in", "server": server, "accountID": identity.Account.ID, "cardID": localCardID, "consent": consent, "creator": creator, "devices": devices, "selection": selection.ID, "job": job}, nil
}
func cardAccountPrepareLink(ctx context.Context) (any, error) {
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return nil, e
	}
	var identity cardAccountIdentity
	var consent cardAccountConsent
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity); e != nil {
		return nil, e
	}
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/consent", nil, &consent); e != nil {
		return nil, e
	}
	if !consent.cardsReady() {
		return nil, errors.New("Accept current Account terms and separate optional online-card consent on the Account website first")
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return nil, e
	}
	defer clear(seed)
	pub := badgeDevicePublic(seed)
	var challenge cardAccountChallenge
	if e = cardAccountAPI(ctx, server, token, "POST", "/v1/card-devices/challenge", map[string]string{"publicKey": base64.StdEncoding.EncodeToString(pub)}, &challenge); e != nil {
		return nil, e
	}
	if _, e = cardAccountLinkInput(challenge, server, identity.Account.ID, pub, time.Now()); e != nil {
		return nil, e
	}
	if e = cardAccountWrite("link.json", cardAccountLink{server, identity.Account.ID, challenge}); e != nil {
		return nil, e
	}
	return map[string]any{"action": "link", "challengeID": challenge.ID, "accountID": identity.Account.ID, "cardID": challenge.CardID, "server": server, "expiresAt": challenge.Expires, "confirmation": "LINK " + challenge.CardID, "disclosure": "This proves possession of this device key, not legal identity or authorship. The private seed stays here. It does not authorize submissions, payments or future signatures."}, nil
}
func cardAccountConfirmLink(ctx context.Context, challengeID, confirmation string) (any, error) {
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return nil, e
	}
	var link cardAccountLink
	if e = cardAccountRead("link.json", &link); e != nil {
		return nil, e
	}
	if link.Server != server || challengeID != link.Challenge.ID || confirmation != "LINK "+link.Challenge.CardID {
		return nil, errors.New("The disclosed link request changed; prepare and confirm it again")
	}
	var identity cardAccountIdentity
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity); e != nil {
		return nil, e
	}
	if identity.Account.ID != link.AccountID {
		return nil, errors.New("Account changed since the linking disclosure")
	}
	var consent cardAccountConsent
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/consent", nil, &consent); e != nil {
		return nil, e
	}
	if !consent.cardsReady() {
		return nil, errors.New("Current Account terms and separate online-card consent are required")
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return nil, e
	}
	defer clear(seed)
	input, e := cardAccountLinkInput(link.Challenge, server, identity.Account.ID, badgeDevicePublic(seed), time.Now())
	if e != nil {
		return nil, e
	}
	key := ed25519.NewKeyFromSeed(seed)
	defer clear(key)
	var result any
	e = cardAccountAPI(ctx, server, token, "POST", "/v1/card-devices/proof", map[string]string{"challengeID": link.Challenge.ID, "signature": base64.StdEncoding.EncodeToString(ed25519.Sign(key, input))}, &result)
	if e != nil {
		return nil, e
	}
	if e = cardAccountRemove("link.json"); e != nil {
		return nil, e
	}
	return result, nil
}
func runCardAccount(args []string) error {
	if len(args) == 0 {
		return errors.New("account needs status|signin|poll|cancel|signout|website|link|unlink|submission|review")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	var out any
	var e error
	switch args[0] {
	case "status":
		out, e = cardAccountStatus(ctx)
	case "signin":
		out, e = cardAccountStart(ctx)
	case "poll":
		out, e = cardAccountPoll(ctx)
	case "cancel", "signout":
		var server string
		server, e = cardAccountServer()
		if e != nil {
			break
		}
		var unlock func()
		unlock, e = cardAccountLock(ctx)
		if e != nil {
			break
		}
		defer unlock()
		e = cardAccountSecret(ctx, server, "clear", "pending", nil, nil)
		if args[0] == "signout" {
			var r cardAccountRecord
			if cardAccountSecret(ctx, server, "lookup", "tokens", nil, &r) == nil && r.Refresh != "" && cardAccountEndpoint(r.Revocation, r.Issuer) {
				revokeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				_, _ = cardAccountForm(revokeCtx, r.Revocation, url.Values{"token": {r.Refresh}, "client_id": {r.ClientID}, "token_type_hint": {"refresh_token"}}, nil)
				stop()
			}
			e = cardAccountSecret(ctx, server, "clear", "tokens", nil, nil)
			_ = cardAccountRemove("link.json")
		}
		out = map[string]any{"state": "signed_out"}
	case "website":
		var server string
		server, e = cardAccountServer()
		if e != nil {
			break
		}
		var cfg cardAccountConfig
		e = cardAccountAPI(ctx, server, "", "GET", "/v1/config", nil, &cfg)
		if e == nil {
			e = cardAccountOpen(ctx, cfg.AccountURL)
		}
		out = map[string]bool{"opened": e == nil}
	case "link":
		if len(args) == 1 {
			out, e = cardAccountPrepareLink(ctx)
		} else if len(args) == 3 {
			out, e = cardAccountConfirmLink(ctx, args[1], args[2])
		} else {
			e = errors.New("link needs challengeID and the exact disclosed LINK confirmation")
		}
	case "unlink":
		if len(args) != 3 {
			e = errors.New("unlink needs the disclosed account ID and card ID")
			break
		}
		var server, token string
		server, token, e = cardAccountAccess(ctx)
		if e != nil {
			break
		}
		var identity cardAccountIdentity
		e = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity)
		if e != nil {
			break
		}
		var seed []byte
		seed, e = badgeDeviceSeed()
		if e != nil {
			break
		}
		id := cardIDText(badgeDevicePublic(seed))
		clear(seed)
		if identity.Account.ID != args[1] || id != args[2] {
			e = errors.New("The disclosed unlink account or device changed")
			break
		}
		e = cardAccountAPI(ctx, server, token, "DELETE", "/v1/card-devices", map[string]string{"cardID": id}, &out)
	case "submission":
		out, e = cardAccountSubmission(ctx, args[1:])
	case "review":
		out, e = cardAccountReview(ctx, args[1:])
	default:
		e = errors.New("Unknown Account command")
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
