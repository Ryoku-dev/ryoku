package protocol

import (
	"fmt"
	"math/bits"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ProviderEmber is Ryoku Ember, the hosted models a signed-in Ryoku Account
// pays for. It appears only while an account is signed in, and only a request
// that names one of its models uses it.
const ProviderEmber = "ember"

// AccountServer cleans a Ryoku Accounts server address: an https origin, or
// an http one on this computer for a server run locally. Any path, query or
// credentials make it unusable, as does any other scheme.
func AccountServer(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" || u.Hostname() == "" {
		return "", false
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !LoopbackHost(u.Hostname()) {
			return "", false
		}
	default:
		return "", false
	}
	host := u.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" {
		host = net.JoinHostPort(u.Hostname(), port)
	}
	return u.Scheme + "://" + strings.ToLower(host), true
}

// Ryoku Account states.
const (
	// AccountUnconfigured is no Ryoku Accounts server set.
	AccountUnconfigured = "unconfigured"
	AccountSignedOut    = "signed_out"
	// AccountSigningIn waits for the user to approve the code in the browser.
	AccountSigningIn = "signing_in"
	AccountSignedIn  = "signed_in"
)

// AccountState is everything the account popover shows. The daemon holds the
// tokens; none of them is ever part of this state.
type AccountState struct {
	State string `json:"state"`
	// Server is the Ryoku Accounts server's address.
	Server string `json:"server,omitempty"`
	// Busy is a request to the server in flight.
	Busy bool `json:"busy,omitempty"`
	// Error is the last failure, in words a user can act on.
	Error   string           `json:"error,omitempty"`
	Code    *AccountCode     `json:"code,omitempty"`
	Account *AccountIdentity `json:"account,omitempty"`
	Balance *AccountBalance  `json:"balance,omitempty"`
	Models  []AccountModel   `json:"models,omitempty"`
	// UnitsPerCredit is how many balance units make one credit.
	UnitsPerCredit int64 `json:"unitsPerCredit,omitempty"`
	// Notice is what hosted models do with what the user writes.
	Notice string `json:"notice,omitempty"`
	// Manage is the website page where the account is managed (AccountManage
	// of what the server names), "" when it names none that can be opened.
	Manage string `json:"manage,omitempty"`
}

// AccountCode is what the user approves in the browser: the short code and
// the page that asks for it. The device code itself never leaves the daemon.
type AccountCode struct {
	UserCode                string    `json:"userCode"`
	VerificationURI         string    `json:"verificationURI"`
	VerificationURIComplete string    `json:"verificationURIComplete,omitempty"`
	ExpiresAt               time.Time `json:"expiresAt"`
}

// AccountIdentity is who is signed in, as the server knows them.
type AccountIdentity struct {
	ID      string `json:"id"`
	Display string `json:"display,omitempty"`
}

// AccountBalance is the server's balance in units (UnitsPerCredit to a
// credit), as last read. Only the display rounds it.
type AccountBalance struct {
	Available int64     `json:"available"`
	Held      int64     `json:"held"`
	Read      time.Time `json:"read"`
}

// AccountModel is a hosted model: the server's alias, the real model's name,
// and the mark that names its maker ("" leaves the app's fallback).
type AccountModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
}

// IsHostedModel is whether a model id names a hosted model of the Ryoku
// Account.
func IsHostedModel(id string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(id)), ProviderEmber+"/")
}

// AccountManage cleans the address of the website page where the account is
// managed: https, or http on this computer for a site run locally, with no
// credentials. "" for anything else, and then nothing is opened.
func AccountManage(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Opaque != "" || u.Hostname() == "" {
		return ""
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !LoopbackHost(u.Hostname()) {
			return ""
		}
	default:
		return ""
	}
	return u.String()
}

// Parts of the account page an app opens, as fragments of Manage.
const (
	AccountPagePlans = "plans"
	AccountPageTerms = "consent"
	AccountPageKeys  = "ember-api-keys"
)

// AccountManageAt is the account page at one of its parts: the address
// AccountManage keeps, with only the fragment changed. "" when the page
// cannot be opened.
func AccountManageAt(raw, part string) string {
	page := AccountManage(raw)
	if page == "" {
		return ""
	}
	u, err := url.Parse(page)
	if err != nil {
		return ""
	}
	u.Fragment, u.RawFragment = part, ""
	return u.String()
}

// Ryoku Ember refusals the account page answers. They are Error codes, so the
// app can offer the page beside the words.
const (
	// CodeAccountTerms is a request refused until the account's terms are accepted.
	CodeAccountTerms = "account_consent_required"
	// CodeEmberCredits is a request the account's Ember credits do not cover.
	CodeEmberCredits = "insufficient_credits"
)

// LoopbackHost is exactly localhost or a loopback address: no other name
// is trusted to stay on this computer.
func LoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// ServerTextMax is how long a word from a server is when a user reads it.
const ServerTextMax = 160

// ServerText makes a string a server or identity provider sent fit for the
// user: control and format characters (bidi marks included) are gone, white
// space is one blank, and it ends at max runes.
func ServerText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cf, r):
			return -1
		case unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max-1]) + "…"
	}
	return s
}

// AccountCredits shows a balance in credits with two decimals. It rounds
// down, so the display never shows credits the account does not have; the
// balance itself keeps every unit. It cannot overflow, whatever the server
// sends.
func AccountCredits(units, perCredit int64) string {
	if perCredit <= 0 {
		perCredit = 1
	}
	sign := ""
	magnitude := uint64(units)
	if units < 0 {
		sign, magnitude = "-", -magnitude
	}
	per := uint64(perCredit)
	hi, lo := bits.Mul64(magnitude%per, 100)
	cents, _ := bits.Div64(hi, lo, per)
	return sign + strconv.FormatUint(magnitude/per, 10) + "." + fmt.Sprintf("%02d", cents)
}
