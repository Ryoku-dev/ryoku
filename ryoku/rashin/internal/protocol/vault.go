package protocol

import (
	"encoding/json"
	"time"
)

// Credential is a secret the vault keeps for teams. Its value never leaves
// the daemon and no method returns it: Rashin adds it to a request or a mail
// on a team's behalf, only where the credential may go.
type Credential struct {
	ID string `json:"id"`
	// Name is how agents name it, such as GITHUB_TOKEN.
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Use is CredentialHeader (sent as an HTTP header to Hosts) or
	// CredentialMail (the mail account's password).
	Use string `json:"use"`
	// Header and Prefix shape a header credential: Header defaults to
	// Authorization and Prefix, such as "Bearer ", stands before the value.
	Header string   `json:"header,omitempty"`
	Prefix string   `json:"prefix,omitempty"`
	Hosts  []string `json:"hosts,omitempty"`
	// Teams may use it; a team's first use on a host asks the user unless
	// Trusted already holds that team and host.
	Teams   []string          `json:"teams,omitempty"`
	Trusted []CredentialTrust `json:"trusted,omitempty"`
	// Fingerprint is the first eight hex digits of the value's HMAC-SHA-256
	// under a key derived from the vault key, to tell two values apart
	// without showing either or confirming a guess.
	Fingerprint string     `json:"fingerprint"`
	Version     int        `json:"version"`
	Created     time.Time  `json:"created"`
	Rotated     *time.Time `json:"rotated,omitempty"`
	LastUsed    *time.Time `json:"lastUsed,omitempty"`
	Uses        int        `json:"uses"`
}

// CredentialTrust lets one team send a credential to one host without asking.
type CredentialTrust struct {
	Team string `json:"team"`
	Host string `json:"host"`
}

// Credential uses.
const (
	CredentialHeader = "header"
	CredentialMail   = "mail"
)

// Vault states: open while the key is at hand, closed while the login keyring
// is locked, unavailable when there is no login keyring to keep the key.
const (
	VaultOpen        = "open"
	VaultClosed      = "closed"
	VaultUnavailable = "unavailable"
)

// VaultInfo is the vault without any value: its state, what keeps its key,
// its credentials and the mail account.
type VaultInfo struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Keeper names what guards the vault's key, such as the login keyring.
	Keeper      string       `json:"keeper,omitempty"`
	Credentials []Credential `json:"credentials"`
	Mail        *MailAccount `json:"mail,omitempty"`
}

// Mail security modes. MailPlain is accepted only for a server on this
// computer.
const (
	MailStartTLS = "starttls"
	MailTLS      = "tls"
	MailPlain    = "none"
)

// MailAccount is where teams send mail from. Its password is a
// CredentialMail credential.
type MailAccount struct {
	Address    string `json:"address"`
	Name       string `json:"name,omitempty"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Security   string `json:"security"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
	// Trusted lets a team mail one recipient without asking each time.
	Trusted []MailTrust `json:"trusted,omitempty"`
}

// MailTrust lets one team mail one recipient without asking.
type MailTrust struct {
	Team      string `json:"team"`
	Recipient string `json:"recipient"`
}

// VaultAddParams stores a credential. Value is sealed at once and never
// returned.
type VaultAddParams struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Use         string   `json:"use"`
	Header      string   `json:"header,omitempty"`
	Prefix      string   `json:"prefix,omitempty"`
	Hosts       []string `json:"hosts,omitempty"`
	Teams       []string `json:"teams,omitempty"`
	Value       string   `json:"value"`
}

// VaultUpdateParams applies a JSON merge patch to a credential (description,
// header, prefix, hosts, teams, trusted) and, with Value, replaces its value.
type VaultUpdateParams struct {
	ID    string          `json:"id"`
	Patch json.RawMessage `json:"patch,omitempty"`
	Value string          `json:"value,omitempty"`
}

// VaultRef names a credential.
type VaultRef struct {
	ID string `json:"id"`
}

// MailAccountParams sets the mail account, or removes it when Account is nil.
type MailAccountParams struct {
	Account *MailAccount `json:"account"`
}

// MailTestResult says whether the mail account's server took the login.
type MailTestResult struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}

// Vault event outcomes.
const (
	VaultUsed    = "used"
	VaultAsked   = "asked"
	VaultDenied  = "denied"
	VaultRefused = "refused"
	VaultFailed  = "failed"
	VaultAdded   = "added"
	VaultRotated = "rotated"
	VaultRemoved = "removed"
)

// VaultEvent records one thing that happened to a credential. It never holds
// a value.
type VaultEvent struct {
	Time       time.Time `json:"time"`
	Credential string    `json:"credential"`
	Name       string    `json:"name"`
	Team       string    `json:"team,omitempty"`
	Task       string    `json:"task,omitempty"`
	Host       string    `json:"host,omitempty"`
	Outcome    string    `json:"outcome"`
	Detail     string    `json:"detail,omitempty"`
	Unwatched  bool      `json:"unwatched,omitempty"`
}

// VaultEventsParams pages the vault's record, newest first.
type VaultEventsParams struct {
	Credential string `json:"credential,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// VaultEventsResult lists vault events, newest first.
type VaultEventsResult struct {
	Events []VaultEvent `json:"events"`
}
