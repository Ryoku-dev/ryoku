package structures

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Mutation errors support errors.Is. A stale caller must reload before retrying.
var (
	ErrStaleRevision = errors.New("structure revision changed; reload before editing")
	ErrImmutable     = errors.New("shipped structure is immutable; clone it to customize")
	ErrNotFound      = errors.New("structure not found")
	ErrInvalid       = errors.New("invalid structure")
)

const maxStoreBytes = 8 << 20
const maxTemplates = 256

type storedTemplate struct {
	ID       string                      `json:"id"`
	Revision uint64                      `json:"revision"`
	Hidden   bool                        `json:"hidden,omitempty"`
	Deleted  bool                        `json:"deleted,omitempty"`
	Template *protocol.StructureTemplate `json:"template,omitempty"`
}

type storeFile struct {
	Version int              `json:"version"`
	Entries []storedTemplate `json:"entries"`
}

// Store persists custom templates and default visibility in structures.json.
// Every operation reloads while holding both an in-process mutex and a Linux
// advisory file lock; independent daemon/editor handles cannot lose updates.
// Deleted custom snapshots remain as tombstones for explicit restore and CAS.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open opens an owner-private local directory. Malformed or unsupported stored
// data fails closed; it is never silently replaced with defaults.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("structure store directory is required")
	}
	if err := paths.EnsureDir(dir); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	if err := s.withLock(func(entries map[string]storedTemplate) error { return nil }); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) withLock(fn func(map[string]storedTemplate) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := os.OpenFile(filepath.Join(s.dir, "structures.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	entries, err := s.load()
	if err != nil {
		return err
	}
	return fn(entries)
}

func (s *Store) load() (map[string]storedTemplate, error) {
	entries := make(map[string]storedTemplate)
	f, err := os.Open(filepath.Join(s.dir, "structures.json"))
	if errors.Is(err, os.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxStoreBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxStoreBytes {
		return nil, errors.New("structure store exceeds size limit")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file storeFile
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("read structure store: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("structure store has trailing content")
	}
	if file.Version != 1 || len(file.Entries) > maxTemplates {
		return nil, errors.New("unsupported structure store version or entry limit")
	}
	defaults := make(map[string]bool)
	for _, t := range defaultCatalog {
		defaults[t.ID] = true
	}
	for _, rec := range file.Entries {
		if _, exists := entries[rec.ID]; exists || rec.Revision == 0 {
			return nil, errors.New("duplicate or unversioned structure record")
		}
		if defaults[rec.ID] {
			if rec.Template != nil || rec.Deleted {
				return nil, errors.New("stored default may contain visibility only")
			}
		} else {
			if !strings.HasPrefix(rec.ID, "custom:") || rec.Template == nil || rec.Template.ID != rec.ID || rec.Template.Builtin || rec.Template.Revision != rec.Revision || rec.Template.Hidden != rec.Hidden {
				return nil, errors.New("invalid custom structure record")
			}
			if err := Validate(*rec.Template); err != nil {
				return nil, fmt.Errorf("stored %s: %w", rec.ID, err)
			}
		}
		entries[rec.ID] = rec
	}
	return entries, nil
}

func (s *Store) persist(entries map[string]storedTemplate) error {
	if len(entries) > maxTemplates {
		return fmt.Errorf("%w: entry limit reached; restore or reuse an existing custom ID", ErrInvalid)
	}
	file := storeFile{Version: 1, Entries: make([]storedTemplate, 0, len(entries))}
	for _, rec := range entries {
		file.Entries = append(file.Entries, rec)
	}
	sort.Slice(file.Entries, func(i, j int) bool { return file.Entries[i].ID < file.Entries[j].ID })
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxStoreBytes {
		return fmt.Errorf("%w: store exceeds size limit", ErrInvalid)
	}
	f, err := os.CreateTemp(s.dir, ".structures-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), filepath.Join(s.dir, "structures.json")); err != nil {
		return err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// List returns defaults (including Hidden ones) and non-deleted custom templates
// sorted by ID. Hidden entries support restore UI; Suggest excludes them.
func (s *Store) List() ([]protocol.StructureTemplate, error) {
	var out []protocol.StructureTemplate
	err := s.withLock(func(entries map[string]storedTemplate) error {
		out = Builtin()
		for i := range out {
			if rec, ok := entries[out[i].ID]; ok {
				out[i].Revision, out[i].Hidden = rec.Revision, rec.Hidden
			}
		}
		for _, rec := range entries {
			if rec.Template != nil && !rec.Deleted {
				out = append(out, *rec.Template)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return nil
	})
	return out, err
}

// ListDeleted returns retained custom tombstones with their current revisions,
// allowing a restore picker to survive restart without remembering old IDs.
func (s *Store) ListDeleted() ([]protocol.StructureTemplate, error) {
	var out []protocol.StructureTemplate
	err := s.withLock(func(entries map[string]storedTemplate) error {
		for _, rec := range entries {
			if rec.Deleted && rec.Template != nil {
				out = append(out, *rec.Template)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return nil
	})
	return out, err
}

// Catalogue takes one locked snapshot so a concurrent delete/restore cannot
// appear both active and deleted (or disappear from both lists) in one response.
func (s *Store) Catalogue() (protocol.TeamStructuresResult, error) {
	out := protocol.TeamStructuresResult{Templates: Builtin(), Deleted: []protocol.StructureTemplate{}, Spirits: Spirits()}
	err := s.withLock(func(entries map[string]storedTemplate) error {
		for i := range out.Templates {
			if rec, ok := entries[out.Templates[i].ID]; ok {
				out.Templates[i].Revision, out.Templates[i].Hidden = rec.Revision, rec.Hidden
			}
		}
		for _, rec := range entries {
			if rec.Template == nil {
				continue
			}
			if rec.Deleted {
				out.Deleted = append(out.Deleted, *rec.Template)
			} else {
				out.Templates = append(out.Templates, *rec.Template)
			}
		}
		sort.Slice(out.Templates, func(i, j int) bool { return out.Templates[i].ID < out.Templates[j].ID })
		sort.Slice(out.Deleted, func(i, j int) bool { return out.Deleted[i].ID < out.Deleted[j].ID })
		return nil
	})
	return out, err
}

// Save creates custom:<slug> at expectedRevision zero, or replaces an existing
// active custom template using its exact revision. Deleted IDs must be restored,
// not recreated, preventing delete/recreate ABA stale-edit acceptance. Returned
// revision is authoritative; caller-supplied template.Revision must match too.
func (s *Store) Save(t protocol.StructureTemplate, expectedRevision uint64) (protocol.StructureTemplate, error) {
	if !strings.HasPrefix(t.ID, "custom:") || t.Builtin {
		return protocol.StructureTemplate{}, ErrImmutable
	}
	if t.Revision != expectedRevision {
		return protocol.StructureTemplate{}, ErrStaleRevision
	}
	if err := Validate(t); err != nil {
		return protocol.StructureTemplate{}, fmt.Errorf("%w: %s", ErrInvalid, err)
	}
	t = cloneTemplate(t)
	var saved protocol.StructureTemplate
	err := s.withLock(func(entries map[string]storedTemplate) error {
		rec, exists := entries[t.ID]
		if !exists && expectedRevision != 0 {
			return ErrNotFound
		}
		if exists && rec.Revision != expectedRevision {
			return ErrStaleRevision
		}
		if exists && rec.Deleted {
			return ErrNotFound
		}
		if exists && rec.Template != nil {
			old := rec.Template
			if !reflect.DeepEqual(t.Seats, old.Seats) || t.Reporting != old.Reporting || t.Shape != old.Shape ||
				t.Variable != old.Variable || t.MinSeats != old.MinSeats || t.MaxSeats != old.MaxSeats || t.DefaultSeats != old.DefaultSeats {
				// An inherited example describes its source roster, never an
				// edited custom workflow. Do not regenerate removed identities.
				t.Example = protocol.StructureExample{}
			}
		}
		if expectedRevision == math.MaxUint64 {
			return fmt.Errorf("%w: revision exhausted", ErrInvalid)
		}
		t.Revision = expectedRevision + 1
		entries[t.ID] = storedTemplate{ID: t.ID, Revision: t.Revision, Hidden: t.Hidden, Template: &t}
		if err := s.persist(entries); err != nil {
			return err
		}
		saved = t
		return nil
	})
	return saved, err
}

func (s *Store) visibility(id string, expected uint64, operation string) (protocol.StructureTemplate, error) {
	var out protocol.StructureTemplate
	err := s.withLock(func(entries map[string]storedTemplate) error {
		rec, exists := entries[id]
		var t protocol.StructureTemplate
		if builtin, ok := Lookup(id); ok {
			t = builtin
			if !exists {
				rec = storedTemplate{ID: id, Revision: 1}
			}
		} else {
			if !exists || rec.Template == nil {
				return ErrNotFound
			}
			t = *rec.Template
		}
		if rec.Revision != expected {
			return ErrStaleRevision
		}
		if expected == math.MaxUint64 {
			return fmt.Errorf("%w: revision exhausted", ErrInvalid)
		}
		if rec.Deleted && operation != "restore" {
			return ErrNotFound
		}
		rec.Revision++
		rec.Hidden = operation != "restore"
		rec.Deleted = operation == "delete" && !t.Builtin
		t.Revision, t.Hidden = rec.Revision, rec.Hidden
		if !t.Builtin {
			rec.Template = &t
		}
		entries[id] = rec
		if err := s.persist(entries); err != nil {
			return err
		}
		out = t
		return nil
	})
	return out, err
}

// Hide hides a default or custom template without changing its content.
func (s *Store) Hide(id string, expectedRevision uint64) error {
	_, err := s.visibility(id, expectedRevision, "hide")
	return err
}

// Delete hides immutable defaults; custom templates become restorable tombstones.
// The resulting tombstone revision is expectedRevision+1.
func (s *Store) Delete(id string, expectedRevision uint64) error {
	_, err := s.visibility(id, expectedRevision, "delete")
	return err
}

// Restore unhides a default or restores a custom tombstone's last content. The
// expected revision is the current hidden/tombstone revision, not its old edit.
func (s *Store) Restore(id string, expectedRevision uint64) (protocol.StructureTemplate, error) {
	return s.visibility(id, expectedRevision, "restore")
}

// Clone copies a visible or hidden default/custom template into a new custom ID
// and persists it at revision one. Deleted custom sources must first be restored.
func (s *Store) Clone(id, newID, name string) (protocol.StructureTemplate, error) {
	list, err := s.List()
	if err != nil {
		return protocol.StructureTemplate{}, err
	}
	for _, t := range list {
		if t.ID != id {
			continue
		}
		if t.Example.Title != "" {
			t.Example.Title = "Source-template example (" + t.Name + "): " + t.Example.Title
		}
		t.ID, t.Name, t.Builtin, t.Hidden, t.Revision = newID, name, false, false, 0
		return s.Save(t, 0)
	}
	return protocol.StructureTemplate{}, ErrNotFound
}
