package vault

import (
	"net"
	"net/mail"
	"net/textproto"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

// Limits on what a credential holds.
const (
	maxValue       = 16 << 10
	maxDescription = 500
	maxPrefix      = 64
	maxRecipients  = 200
)

var (
	nameRule  = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
	tokenRule = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]{1,64}$")
)

// headersOwnedByRashin frame the request itself, so no credential may set
// them.
var headersOwnedByRashin = []string{"Host", "Content-Length", "Transfer-Encoding", "Connection"}

func badRequest(text string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", text)
}

// CheckName refuses a credential name agents could not write as an
// environment-style name, such as GITHUB_TOKEN.
func CheckName(name string) error {
	if !nameRule.MatchString(name) {
		return badRequest(i18n.Tf("%q is not a credential name: use capitals, digits and _, starting with a capital, such as GITHUB_TOKEN.", name))
	}
	return nil
}

// CheckAddress takes one plain mail address, such as ana@example.org, and
// returns it without surrounding space.
func CheckAddress(raw string) (string, error) {
	address := strings.TrimSpace(raw)
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || parsed.Name != "" {
		return "", badRequest(i18n.Tf("%q is not a mail address.", raw))
	}
	return address, nil
}

// checkValue keeps values a header or a login can carry. Its errors never
// hold the value.
func checkValue(value string) error {
	if value == "" {
		return badRequest(i18n.T("The value is empty."))
	}
	if len(value) > maxValue {
		return badRequest(i18n.T("The value is longer than 16 KiB."))
	}
	if !utf8.ValidString(value) || strings.ContainsFunc(value, isControl) {
		return badRequest(i18n.T("The value holds a line break or another control character, which a request cannot carry."))
	}
	return nil
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// pasted drops the line break a paste leaves at a value's end; one inside a
// value is refused by checkValue.
func pasted(value string) string { return strings.TrimRight(value, "\r\n") }

// checkShape validates and normalises a credential's metadata for its use.
func checkShape(c *protocol.Credential) error {
	c.Description = strings.TrimSpace(c.Description)
	if utf8.RuneCountInString(c.Description) > maxDescription || strings.ContainsFunc(c.Description, isControl) {
		return badRequest(i18n.Tf("The description must be one line of at most %d characters.", maxDescription))
	}
	hosts, err := hostrule.Check(c.Hosts)
	if err != nil {
		return err
	}
	c.Hosts = hosts
	switch c.Use {
	case protocol.CredentialHeader:
		if c.Header == "" {
			c.Header = "Authorization"
		}
		if !tokenRule.MatchString(c.Header) {
			return badRequest(i18n.Tf("%q is not a header name.", c.Header))
		}
		c.Header = textproto.CanonicalMIMEHeaderKey(c.Header)
		if slices.Contains(headersOwnedByRashin, c.Header) {
			return badRequest(i18n.Tf("Rashin sets %s itself, so a credential cannot.", c.Header))
		}
		if len(c.Prefix) > maxPrefix || strings.ContainsFunc(c.Prefix, isControl) {
			return badRequest(i18n.Tf("The prefix must be one line of at most %d characters.", maxPrefix))
		}
		if len(c.Hosts) == 0 {
			return badRequest(i18n.T("Say where the credential may go: at least one host."))
		}
	case protocol.CredentialMail:
		if c.Header != "" || c.Prefix != "" || len(c.Hosts) != 0 {
			return badRequest(i18n.T("A mail password goes only to the mail account's server, so it takes no header, prefix or hosts."))
		}
		if len(c.Trusted) != 0 {
			return badRequest(i18n.T("A mail password is trusted through the mail account's recipients, not through hosts."))
		}
	default:
		return badRequest(i18n.Tf("%q is not a way to send a credential.", c.Use))
	}
	var teams []string
	for _, team := range c.Teams {
		if team != "" && !slices.Contains(teams, team) {
			teams = append(teams, team)
		}
	}
	c.Teams = teams
	// a trust outlives neither its team's grant nor its host
	var trusted []protocol.CredentialTrust
	for _, trust := range c.Trusted {
		trust.Host = strings.ToLower(strings.TrimSpace(trust.Host))
		if slices.Contains(c.Teams, trust.Team) && hostrule.Allows(c.Hosts, trust.Host) && !slices.Contains(trusted, trust) {
			trusted = append(trusted, trust)
		}
	}
	c.Trusted = trusted
	return nil
}

// checkAccount validates and normalises the mail account against the
// credentials it may name.
func checkAccount(account *protocol.MailAccount, credentials map[string]protocol.Credential) error {
	address, err := CheckAddress(account.Address)
	if err != nil {
		return err
	}
	account.Address = address
	account.Name = strings.TrimSpace(account.Name)
	if utf8.RuneCountInString(account.Name) > 100 || strings.ContainsFunc(account.Name, isControl) {
		return badRequest(i18n.T("The sender's name must be one line of at most 100 characters."))
	}
	account.Host = strings.ToLower(strings.TrimSpace(account.Host))
	if net.ParseIP(account.Host) == nil && (strings.Contains(account.Host, ":") || strings.HasPrefix(account.Host, "*") || !hostrule.Valid(account.Host)) {
		return badRequest(i18n.Tf("%q is not a mail server name.", account.Host))
	}
	if account.Port < 1 || account.Port > 65535 {
		return badRequest(i18n.T("The port must be between 1 and 65535."))
	}
	switch account.Security {
	case protocol.MailStartTLS, protocol.MailTLS:
	case protocol.MailPlain:
		if !hostrule.IsLocal(account.Host) {
			return badRequest(i18n.T("Only a mail server on this computer may be reached without encryption."))
		}
	default:
		return badRequest(i18n.Tf("%q is not a mail security mode.", account.Security))
	}
	account.Username = strings.TrimSpace(account.Username)
	if len(account.Username) > 256 || strings.ContainsFunc(account.Username, isControl) {
		return badRequest(i18n.T("The user name must be one line of at most 256 characters."))
	}
	if account.Credential != "" {
		credential, ok := credentials[account.Credential]
		if !ok {
			return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The mail account's password credential is not in the vault."))
		}
		if credential.Use != protocol.CredentialMail {
			return badRequest(i18n.Tf("%s is not a mail password.", credential.Name))
		}
	}
	if account.Username != "" && account.Credential == "" {
		return badRequest(i18n.T("A mail account with a user name needs its password credential."))
	}
	var trusted []protocol.MailTrust
	for _, trust := range account.Trusted {
		recipient, err := CheckAddress(trust.Recipient)
		if err != nil {
			return err
		}
		trust.Recipient = strings.ToLower(recipient)
		if trust.Team != "" && !slices.Contains(trusted, trust) {
			trusted = append(trusted, trust)
		}
	}
	if len(trusted) > maxRecipients {
		return badRequest(i18n.Tf("The mail account may trust at most %d recipients.", maxRecipients))
	}
	account.Trusted = trusted
	return nil
}
