// Package grievances keeps the reports models make with report_issue about
// tools that did not behave as described (protocol.FeatureGrievances). They
// stay on this computer, in one bbolt file under Rashin's state folder, and
// are never uploaded; the Learning page lists and clears them.
//
// The database opens on the first report, so a user who never turned the
// feature on never has the file; listing and clearing without it open
// nothing.
//
// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/report-tool-issue.ts@c7fff67.
package grievances

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Limits on what is kept.
const (
	// MaxStored caps the reports kept; the oldest go first.
	MaxStored = 5000
	// MaxToolBytes caps a tool name; a longer one is prose the model put in
	// the wrong field, kept at the head of the report instead.
	MaxToolBytes = 128
	// MaxReportBytes caps one report.
	MaxReportBytes = 4096
	// DefaultLimit is the page List returns when asked for none.
	DefaultLimit = 200
	// MaxLimit is the largest page List returns.
	MaxLimit = 1000
)

var bucket = []byte("grievances")

// Store implements core.Grievances and records new reports.
type Store struct {
	path string

	mu sync.Mutex
	db *bbolt.DB
}

var _ core.Grievances = (*Store)(nil)

// Open makes the store for the database at path ("" is
// paths.GrievancesDB()). It opens nothing until the first report.
func Open(path string) *Store {
	if path == "" {
		path = paths.GrievancesDB()
	}
	return &Store{path: path}
}

// handle returns the open database. Without create it opens only a database
// that already exists, and returns nil when there is none.
func (s *Store) handle(create bool) (*bbolt.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db, nil
	}
	if !create {
		if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(s.path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	s.db = db
	return db, nil
}

// Add records one report and returns it with its ID and time. Tool and
// Report are cleaned first: the user's home and login name replaced, sizes
// capped. At is set to now when zero.
func (s *Store) Add(ctx context.Context, g protocol.Grievance) (protocol.Grievance, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Grievance{}, err
	}
	g.Tool, g.Report = Normalize(g.Tool, g.Report)
	if g.Tool == "" || g.Report == "" {
		return protocol.Grievance{}, protocol.Errorf(protocol.CodeBadRequest, "a report needs a tool and a description")
	}
	if g.At.IsZero() {
		g.At = time.Now().UTC()
	}
	db, err := s.handle(true)
	if err != nil {
		return protocol.Grievance{}, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}
		seq, err := b.NextSequence()
		if err != nil {
			return err
		}
		g.ID = int64(seq)
		raw, err := json.Marshal(g)
		if err != nil {
			return err
		}
		if err := b.Put(key(g.ID), raw); err != nil {
			return err
		}
		return trim(b)
	})
	if err != nil {
		return protocol.Grievance{}, err
	}
	return g, nil
}

// trim keeps the newest MaxStored reports, checked every hundredth report.
func trim(b *bbolt.Bucket) error {
	if b.Sequence()%100 != 0 {
		return nil
	}
	count := 0
	c := b.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		count++
	}
	var stale [][]byte
	for k, _ := c.First(); k != nil && count-len(stale) > MaxStored; k, _ = c.Next() {
		stale = append(stale, append([]byte(nil), k...))
	}
	for _, k := range stale {
		if err := b.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// List returns reports newest first, of p.Tool when set, at most p.Limit.
func (s *Store) List(ctx context.Context, p protocol.LearningGrievancesParams) (protocol.LearningGrievancesResult, error) {
	out := protocol.LearningGrievancesResult{Grievances: []protocol.Grievance{}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	db, err := s.handle(false)
	if err != nil || db == nil {
		return out, err
	}
	counts := map[string]int{}
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var g protocol.Grievance
			if json.Unmarshal(v, &g) != nil {
				continue
			}
			out.Total++
			counts[g.Tool]++
			if p.Tool != "" && g.Tool != p.Tool {
				continue
			}
			if len(out.Grievances) < limit {
				out.Grievances = append(out.Grievances, g)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	for name, n := range counts {
		out.Tools = append(out.Tools, protocol.Count{Name: name, Count: n})
	}
	sort.Slice(out.Tools, func(i, j int) bool {
		if out.Tools[i].Count != out.Tools[j].Count {
			return out.Tools[i].Count > out.Tools[j].Count
		}
		return out.Tools[i].Name < out.Tools[j].Name
	})
	return out, nil
}

// Clear deletes the reports in p.IDs, else every report of p.Tool, else all
// of them.
func (s *Store) Clear(ctx context.Context, p protocol.GrievancesClearParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db, err := s.handle(false)
	if err != nil || db == nil {
		return err
	}
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		switch {
		case len(p.IDs) > 0:
			for _, id := range p.IDs {
				if err := b.Delete(key(id)); err != nil {
					return err
				}
			}
			return nil
		case p.Tool != "":
			var stale [][]byte
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var g protocol.Grievance
				if json.Unmarshal(v, &g) == nil && g.Tool == p.Tool {
					stale = append(stale, append([]byte(nil), k...))
				}
			}
			for _, k := range stale {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			return nil
		default:
			// Keep the bucket so IDs keep rising across a clear.
			var stale [][]byte
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				stale = append(stale, append([]byte(nil), k...))
			}
			for _, k := range stale {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			return nil
		}
	})
}

// Close closes the database if it was opened.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func key(id int64) []byte {
	var k [8]byte
	binary.BigEndian.PutUint64(k[:], uint64(id))
	return k[:]
}

// Normalize cleans a report before it is kept: the user's home and login
// name are replaced, and an over-long tool name, which is prose the model
// put in the wrong field, moves to the head of the report.
func Normalize(tool, report string) (string, string) {
	tool = Redact(strings.TrimSpace(tool))
	report = Redact(strings.TrimSpace(report))
	if len(tool) > MaxToolBytes {
		report = tool + "\n" + report
		tool = cutBytes(tool, MaxToolBytes)
	}
	return tool, cutBytes(report, MaxReportBytes)
}

func cutBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
