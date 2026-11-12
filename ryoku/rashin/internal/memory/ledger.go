package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

// Ledger buckets. Facts hold every memory's metadata (and the text last seen,
// so a hand edit shows as a change). Pending holds the scopes whose file is
// being or failed to be written, and forgets the scopes an external engine
// still has to forget; the rest are as named.
var (
	bucketFacts      = []byte("facts")
	bucketTombs      = []byte("tombstones")
	bucketChanges    = []byte("changes")
	bucketSuppressed = []byte("suppressed")
	bucketWatermarks = []byte("watermarks")
	bucketNotes      = []byte("notes")
	bucketDays       = []byte("days")
	bucketCard       = []byte("card")
	bucketFiles      = []byte("files")
	bucketScopes     = []byte("scopes")
	bucketPending    = []byte("pending")
	bucketForgets    = []byte("forgets")
	bucketMeta       = []byte("meta")
)

var ledgerBuckets = [][]byte{bucketFacts, bucketTombs, bucketChanges, bucketSuppressed, bucketWatermarks, bucketNotes, bucketDays, bucketCard, bucketFiles, bucketScopes, bucketPending, bucketForgets, bucketMeta}

// cardKey is the one key of the card bucket.
var cardKey = []byte("card")

// vectorsKey holds the version of the facts' and tombstones' vectors;
// vectorsVersion 2 embeds a fact's content words only. Vectors of another
// version are dropped: facts are embedded again when searched, tombstones
// keep their text hash.
var (
	vectorsKey     = []byte("vectors")
	vectorsVersion = []byte("2")
)

// upgradeVectors drops vectors made another way than vectorsVersion says.
func upgradeVectors(tx *bbolt.Tx) error {
	if string(tx.Bucket(bucketMeta).Get(vectorsKey)) == string(vectorsVersion) {
		return nil
	}
	for _, name := range [][]byte{bucketFacts, bucketTombs} {
		bucket := tx.Bucket(name)
		type entry struct{ key, value []byte }
		var changed []entry
		err := bucket.ForEach(func(k, v []byte) error {
			var record map[string]json.RawMessage
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			if _, ok := record["vector"]; !ok {
				return nil
			}
			delete(record, "vector")
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			changed = append(changed, entry{append([]byte(nil), k...), data})
			return nil
		})
		if err != nil {
			return err
		}
		for _, e := range changed {
			if err := bucket.Put(e.key, e.value); err != nil {
				return err
			}
		}
	}
	return tx.Bucket(bucketMeta).Put(vectorsKey, vectorsVersion)
}

// openLedger opens or creates the ledger at path, owner-only.
func openLedger(path string) (*bbolt.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range ledgerBuckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return upgradeVectors(tx)
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// putJSON stores v as JSON under key.
func putJSON(tx *bbolt.Tx, bucket []byte, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return tx.Bucket(bucket).Put([]byte(key), data)
}

// getJSON reads key into v; found is false when the key is absent.
func getJSON(tx *bbolt.Tx, bucket []byte, key string, v any) (bool, error) {
	data := tx.Bucket(bucket).Get([]byte(key))
	if data == nil {
		return false, nil
	}
	return true, json.Unmarshal(data, v)
}
