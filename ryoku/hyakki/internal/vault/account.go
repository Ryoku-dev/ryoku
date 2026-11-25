package vault

import (
	"encoding/json"
	"slices"
	"strings"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

var accountKey = []byte("account")

func loadAccount(tx *bbolt.Tx) *protocol.MailAccount {
	raw := tx.Bucket(bucketMail).Get(accountKey)
	if raw == nil {
		return nil
	}
	var account protocol.MailAccount
	if json.Unmarshal(raw, &account) != nil {
		return nil
	}
	return &account
}

func putAccount(tx *bbolt.Tx, account *protocol.MailAccount) error {
	raw, err := json.Marshal(account)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketMail).Put(accountKey, raw)
}

// Mail returns the mail account teams send from, or nil when none is set.
func (v *Vault) Mail() *protocol.MailAccount {
	if v.db == nil {
		return nil
	}
	var account *protocol.MailAccount
	_ = v.db.View(func(tx *bbolt.Tx) error {
		account = loadAccount(tx)
		return nil
	})
	return account
}

// SetMail sets the mail account, or removes it when account is nil. Its
// password must be a mail credential in the vault.
func (v *Vault) SetMail(account *protocol.MailAccount) error {
	if v.db == nil {
		return v.closedError()
	}
	return v.db.Update(func(tx *bbolt.Tx) error {
		if account == nil {
			return tx.Bucket(bucketMail).Delete(accountKey)
		}
		credentials := map[string]protocol.Credential{}
		_ = tx.Bucket(bucketCredentials).ForEach(func(id, raw []byte) error {
			var c protocol.Credential
			if json.Unmarshal(raw, &c) == nil {
				credentials[string(id)] = c
			}
			return nil
		})
		next := *account
		next.Trusted = slices.Clone(account.Trusted)
		if err := checkAccount(&next, credentials); err != nil {
			return err
		}
		return putAccount(tx, &next)
	})
}

// TrustMail lets one team mail one recipient without asking.
func (v *Vault) TrustMail(team, recipient string) error {
	if v.db == nil {
		return v.closedError()
	}
	return v.db.Update(func(tx *bbolt.Tx) error {
		account := loadAccount(tx)
		if account == nil {
			return nil
		}
		trust := protocol.MailTrust{Team: team, Recipient: strings.ToLower(strings.TrimSpace(recipient))}
		if slices.Contains(account.Trusted, trust) {
			return nil
		}
		account.Trusted = append(account.Trusted, trust)
		return putAccount(tx, account)
	})
}
