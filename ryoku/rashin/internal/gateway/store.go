package gateway

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/protocol"
)

var (
	bucketKeys      = []byte("keys")
	bucketProviders = []byte("providers")
	bucketCooldowns = []byte("cooldowns")
	bucketUsage     = []byte("usage")
	bucketStats     = []byte("stats")
)

type keyRecord struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Label     string    `json:"label,omitempty"`
	Sealed    []byte    `json:"sealed"`
	Status    string    `json:"status"`
	LastError string    `json:"lastError,omitempty"`
	Checked   time.Time `json:"checked,omitempty"`
	Created   time.Time `json:"created"`
}

type cooldownRecord struct {
	Until  time.Time `json:"until"`
	Reason string    `json:"reason"`
}

type usageRecord struct {
	Requests     int   `json:"requests"`
	OK           int   `json:"ok"`
	Failed       int   `json:"failed"`
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	Estimated    bool  `json:"estimated"`
}

type successStat struct {
	LatencyMs int64     `json:"latencyMs"`
	TTFBMs    int64     `json:"ttfbMs"`
	At        time.Time `json:"at"`
}

type store struct {
	db   *bbolt.DB
	aead cipher.AEAD
}

func openStore(stateDir string) (*store, error) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("create gateway state directory: %w", err)
	}
	if err := os.Chmod(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("secure gateway state directory: %w", err)
	}
	master, err := loadOrCreateMasterKey(filepath.Join(stateDir, "master.key"))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(master)
	if err != nil {
		return nil, fmt.Errorf("initialize gateway encryption: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize gateway sealing: %w", err)
	}
	db, err := bbolt.Open(filepath.Join(stateDir, "gateway.db"), 0600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open gateway database: %w", err)
	}
	s := &store{db: db, aead: aead}
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketKeys, bucketProviders, bucketCooldowns, bucketUsage, bucketStats} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize gateway database: %w", err)
	}
	return s, nil
}

func loadOrCreateMasterKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("gateway master key is %d bytes, expected 32", len(key))
		}
		if err := os.Chmod(path, 0600); err != nil {
			return nil, fmt.Errorf("secure gateway master key: %w", err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read gateway master key: %w", err)
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate gateway master key: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateMasterKey(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create gateway master key: %w", err)
	}
	if _, err := f.Write(key); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write gateway master key: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("sync gateway master key: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close gateway master key: %w", err)
	}
	return key, nil
}

func (s *store) seal(secret string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(secret), nil), nil
}

func (s *store) open(sealed []byte) (string, error) {
	if len(sealed) < s.aead.NonceSize()+s.aead.Overhead() {
		return "", errors.New("stored provider key is malformed")
	}
	nonce := sealed[:s.aead.NonceSize()]
	plain, err := s.aead.Open(nil, nonce, sealed[s.aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("stored provider key cannot be opened with master.key")
	}
	return string(plain), nil
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *store) addKey(providerID, label, secret string) (keyRecord, bool, error) {
	sealed, err := s.seal(secret)
	if err != nil {
		return keyRecord{}, false, err
	}
	id, err := randomID()
	if err != nil {
		return keyRecord{}, false, err
	}
	record := keyRecord{ID: id, Provider: providerID, Label: strings.TrimSpace(label), Sealed: sealed, Status: protocol.KeyUnknown, Created: time.Now().UTC()}
	duplicate := false
	err = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		cursor := b.Cursor()
		for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
			var existing keyRecord
			if err := json.Unmarshal(value, &existing); err != nil {
				return err
			}
			if existing.Provider != providerID {
				continue
			}
			plain, err := s.open(existing.Sealed)
			if err != nil {
				return err
			}
			if plain == secret {
				record = existing
				duplicate = true
				return nil
			}
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(record.ID), data)
	})
	return record, duplicate, err
}

func (s *store) listKeys(providerID string) ([]keyRecord, error) {
	var records []keyRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketKeys).ForEach(func(_, value []byte) error {
			var record keyRecord
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			if providerID == "" || record.Provider == providerID {
				records = append(records, record)
			}
			return nil
		})
	})
	sort.Slice(records, func(i, j int) bool { return records[i].Created.Before(records[j].Created) })
	return records, err
}

func (s *store) reveal(id string) (string, keyRecord, error) {
	var record keyRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketKeys).Get([]byte(id))
		if data == nil {
			return errors.New("provider key was not found")
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return "", record, err
	}
	secret, err := s.open(record.Sealed)
	return secret, record, err
}

func (s *store) updateKey(record keyRecord) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		if b.Get([]byte(record.ID)) == nil {
			return errors.New("provider key was removed")
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(record.ID), data)
	})
}

func (s *store) removeKey(id, providerID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		data := b.Get([]byte(id))
		if data == nil {
			return errors.New("provider key was not found")
		}
		var record keyRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		if record.Provider != providerID {
			return errors.New("provider key does not belong to this provider")
		}
		if err := b.Delete([]byte(id)); err != nil {
			return err
		}
		return tx.Bucket(bucketCooldowns).Delete([]byte(keyScope(id)))
	})
}

func (s *store) providerEnabled(id string) (bool, error) {
	enabled := true
	err := s.db.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket(bucketProviders).Get([]byte(id))
		if value != nil {
			enabled = string(value) == "1"
		}
		return nil
	})
	return enabled, err
}

func (s *store) setProviderEnabled(id string, enabled bool) error {
	value := []byte("0")
	if enabled {
		value = []byte("1")
	}
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketProviders).Put([]byte(id), value) })
}

func (s *store) setCooldown(scope string, until time.Time, reason string) error {
	record := cooldownRecord{Until: until.UTC(), Reason: reason}
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketCooldowns).Put([]byte(scope), data)
	})
}

func (s *store) cooldown(scope string, now time.Time) (time.Time, bool, error) {
	var record cooldownRecord
	found := false
	err := s.db.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket(bucketCooldowns).Get([]byte(scope))
		if value == nil {
			return nil
		}
		found = true
		return json.Unmarshal(value, &record)
	})
	if err != nil || !found || !record.Until.After(now) {
		return time.Time{}, false, err
	}
	return record.Until, true, nil
}

func usageKey(day, providerID, modelID string) []byte {
	return []byte(day + "\x00" + providerID + "\x00" + modelID)
}

func (s *store) recordUsage(at time.Time, providerID, modelID string, ok bool, usage protocol.UsageRow) error {
	day := at.UTC().Format("2006-01-02")
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsage)
		key := usageKey(day, providerID, modelID)
		var record usageRecord
		if value := b.Get(key); value != nil {
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
		}
		record.Requests++
		if ok {
			record.OK++
		} else {
			record.Failed++
		}
		record.InputTokens += usage.InputTokens
		record.OutputTokens += usage.OutputTokens
		record.Estimated = record.Estimated || usage.Estimated
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (s *store) usage(days int, now time.Time) ([]protocol.UsageRow, error) {
	if days <= 0 {
		days = 7
	}
	cutoff := now.UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	var rows []protocol.UsageRow
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketUsage).ForEach(func(key, value []byte) error {
			parts := strings.Split(string(key), "\x00")
			if len(parts) != 3 || parts[0] < cutoff {
				return nil
			}
			var record usageRecord
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			rows = append(rows, protocol.UsageRow{Day: parts[0], Provider: parts[1], Model: parts[2], Requests: record.Requests, OK: record.OK, Failed: record.Failed, InputTokens: record.InputTokens, OutputTokens: record.OutputTokens, Estimated: record.Estimated})
			return nil
		})
	})
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Day != rows[j].Day {
			return rows[i].Day > rows[j].Day
		}
		if rows[i].Provider != rows[j].Provider {
			return rows[i].Provider < rows[j].Provider
		}
		return rows[i].Model < rows[j].Model
	})
	return rows, err
}

func (s *store) recordSuccess(providerID, modelID string, latency, ttfb time.Duration) error {
	record := successStat{LatencyMs: latency.Milliseconds(), TTFBMs: ttfb.Milliseconds(), At: time.Now().UTC()}
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketStats).Put([]byte(providerID+"\x00"+modelID), data)
	})
}

func (s *store) stat(providerID, modelID string) (successStat, bool, error) {
	var record successStat
	found := false
	err := s.db.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket(bucketStats).Get([]byte(providerID + "\x00" + modelID))
		if value == nil {
			return nil
		}
		found = true
		return json.Unmarshal(value, &record)
	})
	return record, found, err
}

func maskSecret(secret string) string {
	if len(secret) < 5 {
		return "****"
	}
	if len(secret) <= 8 {
		return "****" + secret[len(secret)-2:]
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}
