package demo

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

var (
	credentialPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
	hostPattern       = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*(:[0-9]{1,5})?$`)
)

// the largest value the vault keeps
const maxCredentialValue = 16 * 1024

// tells two values apart without showing either, as the vault does
func valueFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

func (s *server) credentialNamed(name string) int {
	for index, credential := range s.data.teams.vault.Credentials {
		if credential.Name == name {
			return index
		}
	}
	return -1
}

func (s *server) credentialName(id string) string {
	if index := s.data.teams.credentialIndex(id); index >= 0 {
		return s.data.teams.vault.Credentials[index].Name
	}
	return id
}

func (s *server) useCredential(index int, at time.Time) {
	credential := &s.data.teams.vault.Credentials[index]
	credential.Uses++
	credential.LastUsed = &at
}

// lower-case hosts, one each, refusing what the vault would not send to
func cleanHosts(hosts []string) ([]string, error) {
	clean := make([]string, 0, len(hosts))
	seen := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" || seen[host] {
			continue
		}
		if !hostPattern.MatchString(host) {
			return nil, badRequest(i18n.Tf("%s is not a host name.", host))
		}
		seen[host] = true
		clean = append(clean, host)
	}
	return clean, nil
}

func checkValue(value string) error {
	if value == "" || len(value) > maxCredentialValue {
		return badRequest(i18n.T("A value is between 1 byte and 16 KiB."))
	}
	return nil
}

// checks a credential the way the vault does before it is kept
func checkCredential(credential *protocol.Credential) error {
	hosts, err := cleanHosts(credential.Hosts)
	if err != nil {
		return err
	}
	credential.Hosts = hosts
	switch credential.Use {
	case protocol.CredentialHeader:
		if len(credential.Hosts) == 0 {
			return badRequest(i18n.T("Say where it may go: at least one host."))
		}
		if strings.TrimSpace(credential.Header) == "" {
			credential.Header = "Authorization"
		}
	case protocol.CredentialMail:
		credential.Header, credential.Prefix, credential.Hosts = "", "", nil
	default:
		return badRequest(i18n.T("Say how it is sent."))
	}
	return nil
}

func (s *server) addCredential(p protocol.VaultAddParams) (protocol.Credential, error) {
	t := &s.data.teams
	name := strings.TrimSpace(p.Name)
	if !credentialPattern.MatchString(name) {
		return protocol.Credential{}, badRequest(i18n.T("A name is capital letters, digits and underscores, starting with a letter, such as GITHUB_TOKEN."))
	}
	if s.credentialNamed(name) >= 0 {
		return protocol.Credential{}, badRequest(i18n.Tf("%s is already in the vault.", name))
	}
	if err := checkValue(p.Value); err != nil {
		return protocol.Credential{}, err
	}
	credential := protocol.Credential{
		ID: t.newID("credential"), Name: name, Description: strings.TrimSpace(p.Description), Use: p.Use,
		Header: p.Header, Prefix: p.Prefix, Hosts: p.Hosts, Teams: append([]string(nil), p.Teams...),
		Fingerprint: valueFingerprint(p.Value), Version: 1, Created: time.Now(),
	}
	if err := checkCredential(&credential); err != nil {
		return protocol.Credential{}, err
	}
	t.vault.Credentials = append(append([]protocol.Credential(nil), t.vault.Credentials...), credential)
	s.recordVault(protocol.VaultEvent{Credential: credential.ID, Name: name, Outcome: protocol.VaultAdded})
	s.record(protocol.Activity{Action: "credential.add", Target: name, OK: true})
	s.emitVault()
	return credential, nil
}

func (s *server) updateCredential(p protocol.VaultUpdateParams) (protocol.Credential, error) {
	t := &s.data.teams
	index := t.credentialIndex(p.ID)
	if index < 0 {
		return protocol.Credential{}, notFound("credential")
	}
	before := t.vault.Credentials[index]
	credential := before
	if len(p.Patch) > 0 {
		if err := mergeInto(p.Patch, &credential); err != nil {
			return protocol.Credential{}, err
		}
	}
	// a patch changes only what the user may change; the value changes only through Value
	credential.ID, credential.Name, credential.Use, credential.Fingerprint, credential.Version = before.ID, before.Name, before.Use, before.Fingerprint, before.Version
	credential.Created, credential.Rotated, credential.LastUsed, credential.Uses = before.Created, before.Rotated, before.LastUsed, before.Uses
	if err := checkCredential(&credential); err != nil {
		return protocol.Credential{}, err
	}
	if p.Value != "" {
		if err := checkValue(p.Value); err != nil {
			return protocol.Credential{}, err
		}
		now := time.Now()
		credential.Fingerprint, credential.Version, credential.Rotated = valueFingerprint(p.Value), before.Version+1, &now
		s.recordVault(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Outcome: protocol.VaultRotated})
		s.record(protocol.Activity{Action: "credential.rotate", Target: credential.Name, OK: true})
	}
	t.vault.Credentials[index] = credential
	s.emitVault()
	return credential, nil
}

func (s *server) removeCredential(id string) error {
	t := &s.data.teams
	index := t.credentialIndex(id)
	if index < 0 {
		return notFound("credential")
	}
	credential := t.vault.Credentials[index]
	t.vault.Credentials = append(t.vault.Credentials[:index:index], t.vault.Credentials[index+1:]...)
	s.recordVault(protocol.VaultEvent{Credential: id, Name: credential.Name, Outcome: protocol.VaultRemoved})
	s.record(protocol.Activity{Action: "credential.remove", Target: credential.Name, OK: true})
	s.emitVault()
	return nil
}

func (s *server) vaultEvents(p protocol.VaultEventsParams) protocol.VaultEventsResult {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	events := make([]protocol.VaultEvent, 0)
	for _, entry := range s.data.teams.vaultLog {
		if p.Credential != "" && entry.Credential != p.Credential {
			continue
		}
		events = append(events, entry)
		if len(events) == limit {
			break
		}
	}
	return protocol.VaultEventsResult{Events: events}
}

func localMailHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// sets the mail account the way the vault checks it, or removes it
func (s *server) setMailAccount(account *protocol.MailAccount) (protocol.VaultInfo, error) {
	t := &s.data.teams
	if account != nil {
		mail := *account
		mail.Host = strings.ToLower(strings.TrimSpace(mail.Host))
		mail.Address = strings.TrimSpace(mail.Address)
		if mail.Host == "" || !strings.Contains(mail.Address, "@") {
			return protocol.VaultInfo{}, badRequest(i18n.T("A mail account needs an address and a server."))
		}
		if mail.Port < 1 || mail.Port > 65535 {
			return protocol.VaultInfo{}, badRequest(i18n.T("A port is between 1 and 65535."))
		}
		switch mail.Security {
		case protocol.MailTLS, protocol.MailStartTLS:
		case protocol.MailPlain:
			if !localMailHost(mail.Host) {
				return protocol.VaultInfo{}, badRequest(i18n.T("Only a server on this computer may go without encryption."))
			}
		default:
			return protocol.VaultInfo{}, badRequest(i18n.T("Choose how the connection is secured."))
		}
		if index := t.credentialIndex(mail.Credential); index < 0 || t.vault.Credentials[index].Use != protocol.CredentialMail {
			return protocol.VaultInfo{}, badRequest(i18n.T("Choose a mail password from the vault."))
		}
		if t.vault.Mail != nil && mail.Trusted == nil {
			mail.Trusted = t.vault.Mail.Trusted
		}
		account = &mail
	}
	t.vault.Mail = account
	s.emitVault()
	return s.vaultInfo(), nil
}

// the demo has no server to log in to, so a test succeeds whenever the account is set
func (s *server) testMailAccount() protocol.MailTestResult {
	mail := s.data.teams.vault.Mail
	if mail == nil {
		return protocol.MailTestResult{Text: i18n.T("Set up a mail account first.")}
	}
	return protocol.MailTestResult{OK: true, Text: i18n.Tf("%s took the login for %s.", mail.Host, mail.Username)}
}

// a deleted team keeps no grant, trust or mail recipient
func (s *server) dropGrants(team string) {
	vault := &s.data.teams.vault
	credentials := make([]protocol.Credential, len(vault.Credentials))
	for index, credential := range vault.Credentials {
		teams := make([]string, 0, len(credential.Teams))
		for _, granted := range credential.Teams {
			if granted != team {
				teams = append(teams, granted)
			}
		}
		trusted := make([]protocol.CredentialTrust, 0, len(credential.Trusted))
		for _, trust := range credential.Trusted {
			if trust.Team != team {
				trusted = append(trusted, trust)
			}
		}
		credential.Teams, credential.Trusted = teams, trusted
		credentials[index] = credential
	}
	vault.Credentials = credentials
	if vault.Mail != nil {
		mail := *vault.Mail
		mail.Trusted = nil
		for _, trust := range vault.Mail.Trusted {
			if trust.Team != team {
				mail.Trusted = append(mail.Trusted, trust)
			}
		}
		vault.Mail = &mail
	}
}
