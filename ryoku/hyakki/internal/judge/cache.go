package judge

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/paths"
)

// CacheName is the answer cache's file in Rashin's cache folder.
const CacheName = "judge.db"

// MaxCached caps the answers kept; past it the oldest tenth goes.
const MaxCached = 2000

var (
	bucketAnswers = []byte("answers")
	bucketOrder   = []byte("order")
)

// cache keeps verdicts by question. It opens its database on first use and
// works without one when that fails: a judge never fails for its cache.
type cache struct {
	path string

	mu     sync.Mutex
	db     *bbolt.DB
	failed bool
}

type cachedVerdict struct {
	JSON        json.RawMessage `json:"json"`
	Probability float64         `json:"p"`
}

func newCache(path string) *cache {
	return &cache{path: path}
}

// cacheKey names one question to one light model: the light pin, the schema
// and the question text with the state it carries.
func cacheKey(pin string, schema json.RawMessage, question string) []byte {
	h := sha256.New()
	h.Write([]byte(pin))
	h.Write([]byte{0})
	h.Write(schema)
	h.Write([]byte{0})
	h.Write([]byte(question))
	sum := h.Sum(nil)
	out := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(out, sum)
	return out
}

func (c *cache) open() *bbolt.DB {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db != nil || c.failed {
		return c.db
	}
	path := c.path
	if path == "" {
		path = filepath.Join(paths.CacheDir(), CacheName)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		c.failed = true
		return nil
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		c.failed = true
		return nil
	}
	c.db = db
	return db
}

func (c *cache) get(key []byte) (engine.Verdict, bool) {
	db := c.open()
	if db == nil {
		return engine.Verdict{}, false
	}
	var v cachedVerdict
	found := false
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAnswers)
		if b == nil {
			return nil
		}
		raw := b.Get(key)
		if raw == nil {
			return nil
		}
		found = json.Unmarshal(raw, &v) == nil
		return nil
	})
	if !found {
		return engine.Verdict{}, false
	}
	return engine.Verdict{JSON: v.JSON, Probability: v.Probability}, true
}

func (c *cache) put(key []byte, v engine.Verdict) {
	db := c.open()
	if db == nil {
		return
	}
	raw, err := json.Marshal(cachedVerdict{JSON: v.JSON, Probability: v.Probability})
	if err != nil {
		return
	}
	_ = db.Update(func(tx *bbolt.Tx) error {
		answers, err := tx.CreateBucketIfNotExists(bucketAnswers)
		if err != nil {
			return err
		}
		order, err := tx.CreateBucketIfNotExists(bucketOrder)
		if err != nil {
			return err
		}
		if answers.Get(key) == nil {
			seq, err := order.NextSequence()
			if err != nil {
				return err
			}
			var k [8]byte
			binary.BigEndian.PutUint64(k[:], seq)
			if err := order.Put(k[:], key); err != nil {
				return err
			}
		}
		if err := answers.Put(key, raw); err != nil {
			return err
		}
		return prune(answers, order)
	})
}

// prune drops the oldest answers once the cache holds more than MaxCached,
// down to nine tenths of it. It counts only every hundredth answer, so a
// put stays cheap.
func prune(answers, order *bbolt.Bucket) error {
	if order.Sequence()%100 != 0 {
		return nil
	}
	count := 0
	cur := order.Cursor()
	for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
		count++
	}
	if count <= MaxCached {
		return nil
	}
	drop := count - MaxCached + MaxCached/10
	var stale [][]byte
	for k, v := cur.First(); k != nil && len(stale) < drop; k, v = cur.Next() {
		stale = append(stale, append([]byte(nil), k...))
		if err := answers.Delete(v); err != nil {
			return err
		}
	}
	for _, k := range stale {
		if err := order.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func (c *cache) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}
