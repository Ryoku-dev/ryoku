package vault

import (
	"encoding/binary"
	"encoding/json"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

// keepEvents is how much of its record the vault keeps.
const keepEvents = 2000

// Record appends one event to the vault's record, keeping the newest 2000.
// A use also counts on the credential. Its detail passes through Redact, so
// a value that slipped into it never reaches the file.
func (v *Vault) Record(event protocol.VaultEvent) error {
	if v.db == nil {
		return v.closedError()
	}
	if event.Time.IsZero() {
		event.Time = v.now().UTC()
	}
	event.Detail = v.Redact(event.Detail)
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return v.db.Update(func(tx *bbolt.Tx) error {
		events := tx.Bucket(bucketEvents)
		seq, err := events.NextSequence()
		if err != nil {
			return err
		}
		if err := events.Put(eventKey(seq), raw); err != nil {
			return err
		}
		if seq > keepEvents {
			var old [][]byte
			cursor := events.Cursor()
			for key, _ := cursor.First(); key != nil && binary.BigEndian.Uint64(key) <= seq-keepEvents; key, _ = cursor.Next() {
				old = append(old, append([]byte(nil), key...))
			}
			for _, key := range old {
				if err := events.Delete(key); err != nil {
					return err
				}
			}
		}
		if event.Outcome != protocol.VaultUsed || event.Credential == "" {
			return nil
		}
		c, err := loadCredential(tx, event.Credential)
		if err != nil {
			return nil
		}
		used := event.Time
		c.LastUsed = &used
		c.Uses++
		return putCredential(tx, c)
	})
}

func eventKey(seq uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, seq)
	return key
}

// Events lists the record newest first, of one credential when the params
// name it.
func (v *Vault) Events(p protocol.VaultEventsParams) ([]protocol.VaultEvent, error) {
	list := []protocol.VaultEvent{}
	if v.db == nil {
		return list, nil
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, keepEvents)
	err := v.db.View(func(tx *bbolt.Tx) error {
		cursor := tx.Bucket(bucketEvents).Cursor()
		for key, raw := cursor.Last(); key != nil && len(list) < limit; key, raw = cursor.Prev() {
			var event protocol.VaultEvent
			if json.Unmarshal(raw, &event) != nil {
				continue
			}
			if p.Credential == "" || event.Credential == p.Credential {
				list = append(list, event)
			}
		}
		return nil
	})
	return list, err
}
