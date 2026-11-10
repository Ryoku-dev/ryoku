package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"go.etcd.io/bbolt"
)

const apiKeyPrefix = "rashin-"

var apiKeyName = []byte("key")

// APIKey returns the key other programs present to the local API and makes
// one on first use. rotate replaces it, which cuts off every program that
// holds the old one.
func (g *Gateway) APIKey(_ context.Context, rotate bool) (string, error) {
	g.apiMu.Lock()
	defer g.apiMu.Unlock()
	if g.apiKey != "" && !rotate {
		return g.apiKey, nil
	}
	key, err := g.store.apiKey(rotate)
	if err == nil {
		g.apiKey = key
	}
	return key, err
}

func (s *store) apiKey(rotate bool) (string, error) {
	key := ""
	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketAPI)
		if sealed := bucket.Get(apiKeyName); sealed != nil && !rotate {
			// a key sealed under a master.key that is gone can't be read back, so a new one replaces it
			if plain, err := s.open(sealed); err == nil {
				key = plain
				return nil
			}
		}
		raw := make([]byte, 24)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		key = apiKeyPrefix + hex.EncodeToString(raw)
		sealed, err := s.seal(key)
		if err != nil {
			return err
		}
		return bucket.Put(apiKeyName, sealed)
	})
	return key, err
}
