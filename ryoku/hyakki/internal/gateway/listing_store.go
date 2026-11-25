package gateway

import (
	"encoding/json"

	"go.etcd.io/bbolt"
)

// listingRecords returns every provider listing the store keeps. A record
// that cannot be read is left out, to be read from the provider again.
func (s *store) listingRecords() (map[string]listingRecord, error) {
	records := make(map[string]listingRecord)
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketListings).ForEach(func(key, value []byte) error {
			var record listingRecord
			if json.Unmarshal(value, &record) == nil {
				records[string(key)] = record
			}
			return nil
		})
	})
	return records, err
}

// putListing keeps a provider's listing.
func (s *store) putListing(providerID string, record listingRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketListings).Put([]byte(providerID), data) })
}
