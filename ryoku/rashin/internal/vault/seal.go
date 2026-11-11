package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
)

// errSealed is a sealed value the key cannot open: damaged, or sealed under
// another credential or version. It never carries the value.
var errSealed = errors.New("vault: a sealed value cannot be opened")

// newCipher makes the AES-256-GCM cipher values are sealed with.
func newCipher(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// valueKey is where one version of a credential's value is stored. It is
// also the sealed value's additional data, so a value moved to another
// credential or version no longer opens.
func valueKey(id string, version int) []byte {
	return []byte(id + "/" + strconv.Itoa(version))
}

// seal encrypts a value under a fresh random nonce, which leads the result.
func seal(aead cipher.AEAD, id string, version int, value string) ([]byte, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(value), valueKey(id, version)), nil
}

// unseal opens what seal made for the same credential and version.
func unseal(aead cipher.AEAD, id string, version int, sealed []byte) (string, error) {
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return "", errSealed
	}
	nonce := sealed[:aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, sealed[aead.NonceSize():], valueKey(id, version))
	if err != nil {
		return "", errSealed
	}
	return string(plain), nil
}

// fingerprintLabel sets the fingerprint key apart from every other use of
// the vault key.
const fingerprintLabel = "rashin vault fingerprint v1"

// fingerprintKey derives the key fingerprints are made with from the vault
// key.
func fingerprintKey(key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(fingerprintLabel))
	return mac.Sum(nil)
}

// fingerprint tells two values apart without showing either: the first
// eight hex digits of the value's HMAC-SHA-256 under the fingerprint key.
// Keyed, it cannot confirm a guessed value to anyone without the vault key.
func fingerprint(key []byte, value string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))[:8]
}
