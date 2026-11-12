package index

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/index/extract"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

var (
	bucketMeta        = []byte("meta")
	bucketFiles       = []byte("files")
	bucketSymbols     = []byte("symbols")
	bucketSymbolsFile = []byte("symbols-file")
	bucketChunks      = []byte("chunks")
	bucketChunksFile  = []byte("chunks-file")
	bucketTerms       = []byte("terms")
	bucketVectors     = []byte("vectors")
	bucketImports     = []byte("imports")
)
var contentBuckets = [][]byte{bucketFiles, bucketSymbols, bucketSymbolsFile, bucketChunks, bucketChunksFile, bucketTerms, bucketVectors, bucketImports}
var fileDerivedBuckets = []struct{ index, records []byte }{{bucketSymbolsFile, bucketSymbols}, {bucketChunksFile, bucketChunks}}

const schemaVersion = "2"

// extractorVersion rises whenever an extractor finds different symbols;
// extractorLanguages names the languages each version changed, so an
// upgrade re-reads only those files and keeps every other file's vectors
const extractorVersion = 1

var extractorLanguages = map[int][]string{1: {"c", "cpp"}}

// staleLanguages lists the languages whose files were extracted by an
// older extractor than this one
func staleLanguages(stored string) map[string]bool {
	from, _ := strconv.Atoi(stored)
	stale := map[string]bool{}
	for version := from + 1; version <= extractorVersion; version++ {
		for _, language := range extractorLanguages[version] {
			stale[language] = true
		}
	}
	return stale
}

type fileRecord struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	MTime    int64  `json:"mtime"`
	Language string `json:"language"`
}
type symbolRecord struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Signature string `json:"signature"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Parent    string `json:"parent"`
	Doc       string `json:"doc"`
	Depth     int    `json:"depth"`
}
type chunkRecord struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
	Symbol   string `json:"symbol"`
	Language string `json:"language"`
	Length   int    `json:"length"`
}
type importRecord struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Line   int    `json:"line"`
}
type fileReplacement struct {
	file   fileRecord
	result extract.Result
}

type indexStore struct {
	db      *bbolt.DB
	id      string
	mu      sync.Mutex
	dbMu    sync.RWMutex
	writeMu sync.Mutex
	damaged bool
}

func openStore(id string) (*indexStore, error) {
	store, err := openStoreOnce(id)
	if err == nil {
		return store, nil
	}
	if !storeCorruption(err) {
		return nil, err
	}
	if removeErr := removeIndexFiles(id); removeErr != nil {
		return nil, removeErr
	}
	store, err = openStoreOnce(id)
	if err != nil {
		return nil, err
	}
	if err = store.setMeta(map[string]string{"state": "error", "error": "The index cache was damaged and will be rebuilt.", "full_refresh": "1"}); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}
func openStoreOnce(id string) (store *indexStore, err error) {
	dir := paths.IndexDir(id)
	if err = paths.EnsureDir(dir); err != nil {
		return nil, err
	}
	var db *bbolt.DB
	defer func() {
		if value := recover(); value != nil {
			if db != nil {
				_ = db.Close()
			}
			store = nil
			err = fmt.Errorf("index store panic: %v", value)
		}
	}()
	db, err = bbolt.Open(filepath.Join(dir, "index.db"), 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	store = &indexStore{db: db, id: id}
	err = store.update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketMeta, bucketFiles, bucketSymbols, bucketSymbolsFile, bucketChunks, bucketChunksFile, bucketTerms, bucketVectors, bucketImports} {
			if _, createErr := tx.CreateBucketIfNotExists(name); createErr != nil {
				return createErr
			}
		}
		meta := tx.Bucket(bucketMeta)
		if state := string(meta.Get([]byte("state"))); state == "refreshing" || state == "embedding" {
			if putErr := meta.Put([]byte("state"), []byte("idle")); putErr != nil {
				return putErr
			}
		}
		storedSchema := string(meta.Get([]byte("schema")))
		if (meta.Get([]byte("count_files")) == nil || (storedSchema != "" && storedSchema != schemaVersion)) && meta.Get([]byte("stat_signature")) != nil {
			if deleteErr := meta.Delete([]byte("stat_signature")); deleteErr != nil {
				return deleteErr
			}
			if putErr := meta.Put([]byte("full_refresh"), []byte("1")); putErr != nil {
				return putErr
			}
		}
		if len(staleLanguages(string(meta.Get([]byte("extractor"))))) > 0 && meta.Get([]byte("stat_signature")) != nil {
			if deleteErr := meta.Delete([]byte("stat_signature")); deleteErr != nil {
				return deleteErr
			}
		}
		return meta.Put([]byte("schema"), []byte(schemaVersion))
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}
func storeCorruption(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, bbolt.ErrInvalid) || errors.Is(err, bbolt.ErrChecksum) || errors.Is(err, bbolt.ErrVersionMismatch) || strings.Contains(err.Error(), "index store panic")
}
func (s *indexStore) isDamaged() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.damaged }
func (s *indexStore) markDamaged() {
	s.mu.Lock()
	if s.damaged {
		s.mu.Unlock()
		return
	}
	s.damaged = true
	s.mu.Unlock()
	s.writeMu.Lock()
	s.dbMu.Lock()
	_ = s.db.Close()
	s.dbMu.Unlock()
	s.writeMu.Unlock()
	_ = removeIndexFiles(s.id)
}
func (s *indexStore) transaction(write bool, fn func(*bbolt.Tx) error) (err error) {
	if s.isDamaged() {
		return errors.New("the index cache is damaged")
	}
	if write {
		s.writeMu.Lock()
	}
	s.dbMu.RLock()
	func() {
		defer func() {
			if value := recover(); value != nil {
				err = fmt.Errorf("index store panic: %v", value)
			}
		}()
		if write {
			err = s.db.Update(fn)
		} else {
			err = s.db.View(fn)
		}
	}()
	s.dbMu.RUnlock()
	if write {
		s.writeMu.Unlock()
	}
	if storeCorruption(err) {
		s.markDamaged()
	}
	return err
}
func (s *indexStore) view(fn func(*bbolt.Tx) error) error   { return s.transaction(false, fn) }
func (s *indexStore) update(fn func(*bbolt.Tx) error) error { return s.transaction(true, fn) }
func (s *indexStore) Close() error {
	if s.isDamaged() {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	return s.db.Close()
}
func (s *indexStore) compact() error {
	path := filepath.Join(paths.IndexDir(s.id), "index.db")
	info, err := os.Stat(path)
	if err != nil || info.Size() < 64<<20 {
		return err
	}
	temporary := path + ".compact"
	_ = os.Remove(temporary)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	destination, err := bbolt.Open(temporary, 0o600, nil)
	if err != nil {
		return err
	}
	s.dbMu.RLock()
	err = bbolt.Compact(destination, s.db, 8<<20)
	s.dbMu.RUnlock()
	closeErr := destination.Close()
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return closeErr
	}
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if err = s.db.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		s.db, _ = bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
		return err
	}
	s.db, err = bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	return err
}
func encode(value any) ([]byte, error)    { return json.Marshal(value) }
func decode(data []byte, value any) error { return json.Unmarshal(data, value) }
func hashBytes(data []byte) string        { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func recordID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}
func pairKey(left, right string) []byte { return []byte(left + "\x00" + right) }

func (s *indexStore) files() ([]fileRecord, error) {
	var out []fileRecord
	err := s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketFiles).ForEach(func(_, value []byte) error {
			var item fileRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}
func (s *indexStore) file(path string) (fileRecord, bool, error) {
	var out fileRecord
	found := false
	err := s.view(func(tx *bbolt.Tx) error {
		value := tx.Bucket(bucketFiles).Get([]byte(path))
		if value == nil {
			return nil
		}
		found = true
		return decode(value, &out)
	})
	return out, found, err
}
func (s *indexStore) meta(key string) (string, error) {
	var value string
	err := s.view(func(tx *bbolt.Tx) error { value = string(tx.Bucket(bucketMeta).Get([]byte(key))); return nil })
	return value, err
}
func (s *indexStore) setMeta(values map[string]string) error {
	return s.update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		for key, value := range values {
			if err := bucket.Put([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *indexStore) resetContent() error {
	return s.update(func(tx *bbolt.Tx) error {
		for _, name := range contentBuckets {
			if err := tx.DeleteBucket(name); err != nil {
				return err
			}
			if _, err := tx.CreateBucket(name); err != nil {
				return err
			}
		}
		return nil
	})
}

func deletePrefix(bucket *bbolt.Bucket, prefix []byte, each func([]byte) error) error {
	cursor := bucket.Cursor()
	for key, value := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = cursor.Next() {
		if each != nil {
			copied := append([]byte(nil), value...)
			if err := each(copied); err != nil {
				return err
			}
		}
		if err := cursor.Delete(); err != nil {
			return err
		}
	}
	return nil
}
func (s *indexStore) replaceFile(file fileRecord, result extract.Result) error {
	return s.replaceFiles([]fileReplacement{{file: file, result: result}})
}
func (s *indexStore) replaceFiles(replacements []fileReplacement) error {
	return s.update(func(tx *bbolt.Tx) error {
		for _, replacement := range replacements {
			if err := replaceFileTx(tx, replacement.file, replacement.result); err != nil {
				return err
			}
		}
		return nil
	})
}
func replaceFileTx(tx *bbolt.Tx, file fileRecord, result extract.Result) error {
	redactedChunks := extract.RedactChunks(result.Chunks)
	keepVectors := make(map[string]bool, len(redactedChunks))
	for _, chunk := range redactedChunks {
		id := recordID(file.Path, fmt.Sprint(chunk.Start), fmt.Sprint(chunk.End), hashBytes([]byte(chunk.Text)))
		keepVectors[id] = true
	}
	if err := deleteFileTx(tx, file.Path, keepVectors); err != nil {
		return err
	}
	encoded, err := encode(file)
	if err != nil {
		return err
	}
	if err = tx.Bucket(bucketFiles).Put([]byte(file.Path), encoded); err != nil {
		return err
	}
	for _, symbol := range result.Symbols {
		id := recordID(file.Path, symbol.Name, fmt.Sprint(symbol.Start), fmt.Sprint(symbol.End))
		record := symbolRecord{ID: id, Path: file.Path, Name: symbol.Name, Kind: symbol.Kind, Signature: redact.Patterns(symbol.Signature, nil), Start: symbol.Start, End: symbol.End, Parent: symbol.Parent, Doc: redact.Patterns(symbol.Doc, nil), Depth: symbol.Depth}
		data, encodeErr := encode(record)
		if encodeErr != nil {
			return encodeErr
		}
		if err = tx.Bucket(bucketSymbols).Put([]byte(id), data); err != nil {
			return err
		}
		if err = tx.Bucket(bucketSymbolsFile).Put(pairKey(file.Path, id), []byte(id)); err != nil {
			return err
		}
	}
	for _, chunk := range redactedChunks {
		text := chunk.Text
		id := recordID(file.Path, fmt.Sprint(chunk.Start), fmt.Sprint(chunk.End), hashBytes([]byte(text)))
		terms := tokenFrequencies(text)
		record := chunkRecord{ID: id, Path: file.Path, Start: chunk.Start, End: chunk.End, Text: text, Symbol: chunk.Symbol, Language: file.Language, Length: termCount(terms)}
		data, encodeErr := encode(record)
		if encodeErr != nil {
			return encodeErr
		}
		if err = tx.Bucket(bucketChunks).Put([]byte(id), data); err != nil {
			return err
		}
		if err = tx.Bucket(bucketChunksFile).Put(pairKey(file.Path, id), []byte(id)); err != nil {
			return err
		}
		for term, frequency := range terms {
			var count [4]byte
			binary.BigEndian.PutUint32(count[:], uint32(frequency))
			if err = tx.Bucket(bucketTerms).Put(pairKey(term, id), count[:]); err != nil {
				return err
			}
		}
	}
	for _, edge := range result.Imports {
		record := importRecord{Source: file.Path, Target: edge.Target, Line: edge.Line}
		data, encodeErr := encode(record)
		if encodeErr != nil {
			return encodeErr
		}
		if err = tx.Bucket(bucketImports).Put(pairKey(file.Path, fmt.Sprintf("%06d:%s", edge.Line, edge.Target)), data); err != nil {
			return err
		}
	}
	return nil
}
func deleteFileTx(tx *bbolt.Tx, path string, keepVectors map[string]bool) error {
	for _, pair := range fileDerivedBuckets {
		indexBucket := tx.Bucket(pair.index)
		derived := tx.Bucket(pair.records)
		if err := deletePrefix(indexBucket, []byte(path+"\x00"), func(id []byte) error {
			if bytes.Equal(pair.records, bucketChunks) {
				chunk := derived.Get(id)
				if chunk != nil {
					var record chunkRecord
					if err := decode(chunk, &record); err != nil {
						return err
					}
					for term := range tokenFrequencies(record.Text) {
						if err := tx.Bucket(bucketTerms).Delete(pairKey(term, record.ID)); err != nil {
							return err
						}
					}
				}
				if !keepVectors[string(id)] {
					if err := tx.Bucket(bucketVectors).Delete(id); err != nil {
						return err
					}
				}
			}
			return derived.Delete(id)
		}); err != nil {
			return err
		}
	}
	if err := deletePrefix(tx.Bucket(bucketImports), []byte(path+"\x00"), nil); err != nil {
		return err
	}
	return tx.Bucket(bucketFiles).Delete([]byte(path))
}
func (s *indexStore) deleteFile(path string) error {
	return s.update(func(tx *bbolt.Tx) error { return deleteFileTx(tx, path, nil) })
}
func (s *indexStore) symbols() ([]symbolRecord, error) {
	var out []symbolRecord
	err := s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketSymbols).ForEach(func(_, value []byte) error {
			var item symbolRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}
func (s *indexStore) chunks() ([]chunkRecord, error) {
	var out []chunkRecord
	err := s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChunks).ForEach(func(_, value []byte) error {
			var item chunkRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}
func (s *indexStore) chunksByID(ids []string) (map[string]chunkRecord, error) {
	out := make(map[string]chunkRecord, len(ids))
	err := s.view(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketChunks)
		for _, id := range ids {
			value := bucket.Get([]byte(id))
			if value == nil {
				continue
			}
			var item chunkRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			out[id] = item
		}
		return nil
	})
	return out, err
}
func (s *indexStore) imports() ([]importRecord, error) {
	var out []importRecord
	err := s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketImports).ForEach(func(_, value []byte) error {
			var item importRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			out = append(out, item)
			return nil
		})
	})
	return out, err
}
func (s *indexStore) counts() (files, chunks, symbols, vectors int, err error) {
	err = s.view(func(tx *bbolt.Tx) error {
		files = tx.Bucket(bucketFiles).Stats().KeyN
		chunks = tx.Bucket(bucketChunks).Stats().KeyN
		symbols = tx.Bucket(bucketSymbols).Stats().KeyN
		vectors = tx.Bucket(bucketVectors).Stats().KeyN
		return nil
	})
	return
}

// languageCounts tallies the indexed files by language, encoded for the
// meta bucket so status reads it without walking the files
func (s *indexStore) languageCounts() (string, error) {
	files, err := s.files()
	if err != nil {
		return "", err
	}
	tally := map[string]int{}
	for _, file := range files {
		if file.Language != "" {
			tally[file.Language]++
		}
	}
	counts := make([]protocol.Count, 0, len(tally))
	for name, count := range tally {
		counts = append(counts, protocol.Count{Name: name, Count: count})
	}
	sortCounts(counts)
	encoded, err := json.Marshal(counts)
	return string(encoded), err
}
func (s *indexStore) storedCounts() (files, chunks, symbols, vectors int, err error) {
	err = s.view(func(tx *bbolt.Tx) error {
		meta := tx.Bucket(bucketMeta)
		files, _ = strconv.Atoi(string(meta.Get([]byte("count_files"))))
		chunks, _ = strconv.Atoi(string(meta.Get([]byte("count_chunks"))))
		symbols, _ = strconv.Atoi(string(meta.Get([]byte("count_symbols"))))
		vectors, _ = strconv.Atoi(string(meta.Get([]byte("count_vectors"))))
		return nil
	})
	return
}
func (s *indexStore) lexicalStats() (documents, totalTerms int, err error) {
	err = s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChunks).ForEach(func(_, value []byte) error {
			var item chunkRecord
			if err := decode(value, &item); err != nil {
				return err
			}
			documents++
			totalTerms += item.Length
			return nil
		})
	})
	return
}
func (s *indexStore) info(entry registryEntry) (protocol.IndexInfo, error) {
	files, chunks, symbols, vectors, err := s.storedCounts()
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	state, err := s.meta("state")
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	if state == "" {
		state = "idle"
	}
	rawUpdated, err := s.meta("updated")
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	var updated *time.Time
	if rawUpdated != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, rawUpdated); parseErr == nil {
			updated = &parsed
		}
	}
	indexError, err := s.meta("error")
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	var languages []protocol.Count
	if raw, metaErr := s.meta("count_languages"); metaErr == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &languages)
	}
	root := ""
	if len(entry.Roots) > 0 {
		root = entry.Roots[0]
	}
	return protocol.IndexInfo{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Root: root, Roots: entry.Roots, State: state, Files: files, Chunks: chunks, Symbols: symbols, Vectors: vectors, Languages: languages, Updated: updated, Error: indexError}, nil
}
func removeIndexFiles(id string) error { return os.RemoveAll(paths.IndexDir(id)) }
