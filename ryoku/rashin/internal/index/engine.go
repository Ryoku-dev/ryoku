// Package index provides Rashin's persistent source and system index.
package index

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/index/embed"
	"ryoku-rashin/internal/index/extract"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type indexOperation struct {
	mu         sync.Mutex
	control    sync.Mutex
	active     bool
	generation uint64
	cancel     context.CancelFunc
}

type Engine struct {
	events          core.Broadcaster
	settings        func() protocol.Settings
	opMu            sync.Mutex
	mu              sync.Mutex
	entries         map[string]registryEntry
	stores          map[string]*indexStore
	checked         map[string]time.Time
	operations      map[string]*indexOperation
	busy            atomic.Int32
	jobsMu          sync.Mutex
	jobsCtx         context.Context
	jobsCancel      context.CancelFunc
	jobs            sync.WaitGroup
	embedding       map[string]bool
	embeddingCancel map[string]context.CancelFunc
	embeddingDone   map[string]chan struct{}
	embedTexts      func(context.Context, []string) ([][]float32, error)
	readSource      func(context.Context, string) ([]byte, error)
	stopping        bool
	closed          bool
}

var _ core.Indexer = (*Engine)(nil)

func Open(events core.Broadcaster, settings func() protocol.Settings) (*Engine, error) {
	entries, err := loadRegistry()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	engine := &Engine{events: events, settings: settings, entries: entries, stores: map[string]*indexStore{}, checked: map[string]time.Time{}, operations: map[string]*indexOperation{}, jobsCtx: ctx, jobsCancel: cancel, embedding: map[string]bool{}, embeddingCancel: map[string]context.CancelFunc{}, embeddingDone: map[string]chan struct{}{}}
	if _, ok := entries["system"]; !ok {
		entries["system"] = registryEntry{ID: "system", Name: i18n.T("System"), Kind: "system", Roots: systemRoots(), Created: time.Now()}
		if err := saveRegistry(entries); err != nil {
			return nil, err
		}
	}
	return engine, nil
}
func (e *Engine) currentSettings() protocol.Settings {
	if e.settings == nil {
		return protocol.Settings{}
	}
	return e.settings()
}
func systemRoots() []string {
	return []string{paths.VaultDir(), paths.ConfigHome(), paths.SystemShare(), paths.SystemConfig()}
}
func (e *Engine) broadcast(event string, data any) {
	if e.events != nil {
		e.events.Broadcast(event, data)
	}
}
func (e *Engine) operationFor(id string) *indexOperation {
	e.mu.Lock()
	defer e.mu.Unlock()
	operation := e.operations[id]
	if operation == nil {
		operation = &indexOperation{}
		e.operations[id] = operation
	}
	return operation
}

func (operation *indexOperation) activate(cancel context.CancelFunc) uint64 {
	operation.control.Lock()
	defer operation.control.Unlock()
	operation.generation++
	operation.active = true
	operation.cancel = cancel
	return operation.generation
}
func (operation *indexOperation) tryActivate(cancel context.CancelFunc) (uint64, bool) {
	operation.control.Lock()
	defer operation.control.Unlock()
	if operation.active {
		return 0, false
	}
	operation.generation++
	operation.active = true
	operation.cancel = cancel
	return operation.generation, true
}

func (operation *indexOperation) deactivate(generation uint64) {
	operation.control.Lock()
	defer operation.control.Unlock()
	if operation.generation == generation {
		operation.active = false
		operation.cancel = nil
	}
}

func (operation *indexOperation) cancelActive() {
	operation.control.Lock()
	defer operation.control.Unlock()
	if operation.cancel != nil {
		operation.cancel()
	}
}

func (e *Engine) storeFor(id string) (*indexStore, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, errors.New(i18n.T("The index is closed."))
	}
	if _, registered := e.entries[id]; !registered {
		return nil, fmt.Errorf("%s", i18n.Tf("Index %s is not registered.", id))
	}
	if store := e.stores[id]; store != nil {
		if !store.isDamaged() {
			return store, nil
		}
		delete(e.stores, id)
	}
	if err := paths.EnsureDir(paths.IndexDir(id)); err != nil {
		return nil, err
	}
	store, err := openStore(id)
	if err != nil {
		return nil, err
	}
	e.stores[id] = store
	return store, nil
}

func (e *Engine) List(ctx context.Context) ([]protocol.IndexInfo, error) {
	e.mu.Lock()
	entries := make([]registryEntry, 0, len(e.entries))
	for _, entry := range e.entries {
		if entry.ID != "system" || e.currentSettings().Index.System {
			entries = append(entries, entry)
		}
	}
	e.mu.Unlock()
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Name < entries[j].Name
	})
	out := make([]protocol.IndexInfo, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		store, err := e.storeFor(entry.ID)
		if err == nil {
			var info protocol.IndexInfo
			info, err = store.info(entry)
			if err == nil {
				out = append(out, info)
				continue
			}
		}
		root := ""
		if len(entry.Roots) > 0 {
			root = entry.Roots[0]
		}
		out = append(out, protocol.IndexInfo{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Root: root, Roots: entry.Roots, State: "error", Error: err.Error()})
	}
	return out, nil
}
func validateProjectRoot(root string) error {
	if root == string(filepath.Separator) {
		return errors.New(i18n.T("The filesystem root cannot be indexed."))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return nil
	}
	if within(root, home) {
		return errors.New(i18n.T("The home directory and its ancestors cannot be indexed."))
	}
	return nil
}

func (e *Engine) Add(ctx context.Context, p protocol.IndexAddParams) (protocol.IndexInfo, error) {
	if !filepath.IsAbs(p.Path) {
		return protocol.IndexInfo{}, errors.New(i18n.T("Index paths must be absolute."))
	}
	root, err := filepath.EvalSymlinks(filepath.Clean(p.Path))
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	if err = validateProjectRoot(root); err != nil {
		return protocol.IndexInfo{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	if !info.IsDir() {
		return protocol.IndexInfo{}, fmt.Errorf("%s", i18n.Tf("%s is not a directory.", root))
	}
	name := p.Name
	if name == "" {
		name = filepath.Base(root)
	}
	entry := registryEntry{ID: projectID(root), Name: name, Kind: "project", Roots: []string{root}, Created: time.Now()}
	e.opMu.Lock()
	e.mu.Lock()
	e.entries[entry.ID] = entry
	err = saveRegistry(e.entries)
	e.mu.Unlock()
	e.opMu.Unlock()
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	return e.Refresh(ctx, protocol.IndexRef{ID: entry.ID})
}
func (e *Engine) Remove(ctx context.Context, ref protocol.IndexRef) error {
	entry, err := e.resolve(ctx, ref, false)
	if err != nil {
		return err
	}
	if entry.ID == "system" {
		return errors.New(i18n.T("Turn off the system index instead of removing it."))
	}
	operation := e.operationFor(entry.ID)
	operation.cancelActive()
	operation.mu.Lock()
	defer operation.mu.Unlock()
	e.stopIndexEmbedding(entry.ID)
	e.opMu.Lock()
	defer e.opMu.Unlock()
	e.mu.Lock()
	if store := e.stores[entry.ID]; store != nil {
		store.Close()
		delete(e.stores, entry.ID)
	}
	delete(e.entries, entry.ID)
	delete(e.checked, entry.ID)
	err = saveRegistry(e.entries)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return removeIndexFiles(entry.ID)
}
func (e *Engine) Status(ctx context.Context, ref protocol.IndexRef) (protocol.IndexInfo, error) {
	entry, err := e.resolve(ctx, ref, true)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	store, err := e.storeFor(entry.ID)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	return store.info(entry)
}

func (e *Engine) resolve(ctx context.Context, ref protocol.IndexRef, register bool) (registryEntry, error) {
	if ref.ID != "" {
		e.mu.Lock()
		entry, ok := e.entries[ref.ID]
		e.mu.Unlock()
		if !ok {
			return registryEntry{}, fmt.Errorf("%s", i18n.Tf("Index %s is not registered.", ref.ID))
		}
		if entry.ID == "system" && !e.currentSettings().Index.System {
			return registryEntry{}, errors.New(i18n.T("The system index is off."))
		}
		return entry, nil
	}
	cwd := ref.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	e.mu.Lock()
	var selected registryEntry
	best := -1
	for _, entry := range e.entries {
		if entry.ID == "system" {
			continue
		}
		if entryContains(entry, cwd) && len(entry.Roots[0]) > best {
			selected = entry
			best = len(entry.Roots[0])
		}
	}
	e.mu.Unlock()
	if best >= 0 {
		return selected, nil
	}
	if !register {
		return registryEntry{}, errors.New(i18n.T("No index matches the current directory."))
	}
	root, err := gitRoot(cwd)
	if err != nil {
		return registryEntry{}, err
	}
	info, err := e.Add(ctx, protocol.IndexAddParams{Path: root})
	if err != nil {
		return registryEntry{}, err
	}
	e.mu.Lock()
	entry := e.entries[info.ID]
	e.mu.Unlock()
	return entry, nil
}

func (e *Engine) Refresh(ctx context.Context, ref protocol.IndexRef) (protocol.IndexInfo, error) {
	entry, err := e.resolve(ctx, ref, true)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	return e.runRefresh(ctx, entry)
}

func (e *Engine) runRefresh(ctx context.Context, entry registryEntry) (protocol.IndexInfo, error) {
	e.jobsMu.Lock()
	stopping := e.stopping || e.closed
	e.jobsMu.Unlock()
	if stopping {
		return protocol.IndexInfo{}, context.Canceled
	}
	operation := e.operationFor(entry.ID)
	operation.mu.Lock()
	defer operation.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return protocol.IndexInfo{}, err
	}
	operation.cancelActive()
	refreshCtx, cancel := context.WithCancel(ctx)
	generation := operation.activate(cancel)
	defer cancel()
	defer operation.deactivate(generation)
	e.busy.Add(1)
	defer e.busy.Add(-1)
	return e.refreshEntry(refreshCtx, entry)
}

func (e *Engine) refreshEntry(ctx context.Context, entry registryEntry) (protocol.IndexInfo, error) {
	store, err := e.storeFor(entry.ID)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	if err = store.setMeta(map[string]string{"state": "refreshing", "error": ""}); err != nil {
		return e.refreshError(store, entry, err)
	}
	e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "walk"})
	files, err := e.walkEntry(ctx, entry)
	if err != nil {
		return e.refreshError(store, entry, err)
	}
	fullRefresh, err := store.meta("full_refresh")
	if err != nil {
		return e.refreshError(store, entry, err)
	}
	if fullRefresh == "1" {
		if err = store.resetContent(); err != nil {
			return e.refreshError(store, entry, err)
		}
	}
	existing, err := store.files()
	if err != nil {
		return e.refreshError(store, entry, err)
	}
	extractor, err := store.meta("extractor")
	if err != nil {
		return e.refreshError(store, entry, err)
	}
	stale := staleLanguages(extractor)
	known := map[string]fileRecord{}
	for _, file := range existing {
		if !stale[file.Language] {
			known[file.Path] = file
		}
	}
	current := map[string]bool{}
	const replacementBatchSize = 64
	const replacementBatchBytes = 8 << 20
	replacements := make([]fileReplacement, 0, replacementBatchSize)
	batchBytes := 0
	changed := 0
	flush := func() error {
		if len(replacements) == 0 {
			return nil
		}
		if err := store.replaceFiles(replacements); err != nil {
			return err
		}
		changed += len(replacements)
		replacements = make([]fileReplacement, 0, replacementBatchSize)
		batchBytes = 0
		e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "extract", Done: changed, Total: len(files)})
		return nil
	}
	readSource := e.readSource
	if readSource == nil {
		readSource = readIndexSource
	}
	for _, file := range files {
		if err = ctx.Err(); err != nil {
			return e.refreshError(store, entry, err)
		}
		data, readErr := readSource(ctx, file.Full)
		if readErr != nil {
			if skippableReadError(readErr) {
				continue
			}
			return e.refreshError(store, entry, readErr)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		current[file.Path] = true
		if previous, ok := known[file.Path]; ok && previous.Hash == hash {
			continue
		}
		estimatedBytes := len(data) * 3
		if len(replacements) > 0 && batchBytes+estimatedBytes > replacementBatchBytes {
			if err = flush(); err != nil {
				return e.refreshError(store, entry, err)
			}
		}
		result, parseErr := extract.Extract(file.Path, data)
		if parseErr != nil && result.Language == "" {
			return e.refreshError(store, entry, parseErr)
		}
		record := fileRecord{Path: file.Path, Source: file.Full, Hash: hash, Size: file.Size, MTime: file.MTime, Language: cmp.Or(result.Language, file.Language)}
		replacements = append(replacements, fileReplacement{file: record, result: result})
		batchBytes += estimatedBytes
		if len(replacements) == replacementBatchSize {
			if err = flush(); err != nil {
				return e.refreshError(store, entry, err)
			}
		}
	}
	if err = flush(); err != nil {
		return e.refreshError(store, entry, err)
	}
	for _, file := range existing {
		if !current[file.Path] {
			if err = store.deleteFile(file.Path); err != nil {
				return e.refreshError(store, entry, err)
			}
		}
	}
	documents, totalTerms, statsErr := store.lexicalStats()
	if statsErr != nil {
		return e.refreshError(store, entry, statsErr)
	}
	fileCount, chunkCount, symbolCount, vectorCount, countErr := store.counts()
	if countErr != nil {
		return e.refreshError(store, entry, countErr)
	}
	languages, countErr := store.languageCounts()
	if countErr != nil {
		return e.refreshError(store, entry, countErr)
	}
	now := time.Now()
	signature := statSignature(files)
	if err = store.setMeta(map[string]string{"count_languages": languages, "schema": schemaVersion, "extractor": strconv.Itoa(extractorVersion), "model": embed.ModelName, "state": "idle", "updated": now.Format(time.RFC3339Nano), "stat_signature": signature, "bm25_documents": strconv.Itoa(documents), "bm25_total_terms": strconv.Itoa(totalTerms), "count_files": strconv.Itoa(fileCount), "count_chunks": strconv.Itoa(chunkCount), "count_symbols": strconv.Itoa(symbolCount), "count_vectors": strconv.Itoa(vectorCount), "error": "", "full_refresh": ""}); err != nil {
		return e.refreshError(store, entry, err)
	}
	e.mu.Lock()
	e.checked[entry.ID] = now
	e.mu.Unlock()
	info, err := store.info(entry)
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "done", Done: len(files), Total: len(files)})
	e.broadcast(protocol.EvtIndexChanged, info)
	if e.currentSettings().Index.Semantic {
		e.startEmbedding(entry, store)
	}
	return info, nil
}
func (e *Engine) refreshError(store *indexStore, entry registryEntry, cause error) (protocol.IndexInfo, error) {
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		_ = store.setMeta(map[string]string{"state": "idle"})
		info, _ := store.info(entry)
		return info, cause
	}
	_ = store.setMeta(map[string]string{"state": "error", "error": cause.Error()})
	info, _ := store.info(entry)
	e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "error", Message: cause.Error()})
	e.broadcast(protocol.EvtIndexChanged, info)
	return info, cause
}
func (e *Engine) walkEntry(ctx context.Context, entry registryEntry) ([]walkedFile, error) {
	var out []walkedFile
	for index, root := range entry.Roots {
		prefix := ""
		policy := walkPolicy{MaxSize: 1 << 20}
		if entry.Kind == "system" {
			policy.System = true
			policy.MaxSize = 512 << 10
			switch index {
			case 0:
				prefix = "vault"
			case 1:
				prefix = "config"
			case 2:
				prefix = "share"
			case 3:
				prefix = "etc"
			}
		}
		files, err := walkRoot(ctx, root, prefix, policy)
		if err != nil {
			return nil, err
		}
		out = append(out, files...)
	}
	if entry.Kind == "system" {
		var err error
		out, err = deduplicateSystemFiles(ctx, out)
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func deduplicateSystemFiles(ctx context.Context, files []walkedFile) ([]walkedFile, error) {
	configFiles := make(map[string]walkedFile)
	for _, file := range files {
		if strings.HasPrefix(file.Path, "config/") {
			configFiles[strings.TrimPrefix(file.Path, "config/")] = file
		}
	}
	leftBuffer := make([]byte, 32<<10)
	rightBuffer := make([]byte, len(leftBuffer))
	out := files[:0]
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		relative, shipped := strings.CutPrefix(file.Path, "share/config/")
		configFile, matched := configFiles[relative]
		if shipped && matched && configFile.Size == file.Size {
			identical, err := identicalFileContents(configFile.Full, file.Full, configFile.Size, leftBuffer, rightBuffer)
			if err == nil && identical {
				continue
			}
		}
		out = append(out, file)
	}
	return out, nil
}

func identicalFileContents(leftPath, rightPath string, size int64, leftBuffer, rightBuffer []byte) (bool, error) {
	left, err := os.Open(leftPath)
	if err != nil {
		return false, err
	}
	defer left.Close()
	right, err := os.Open(rightPath)
	if err != nil {
		return false, err
	}
	defer right.Close()
	for remaining := size; remaining > 0; {
		count := min(int64(len(leftBuffer)), remaining)
		if _, err = io.ReadFull(left, leftBuffer[:count]); err != nil {
			return false, err
		}
		if _, err = io.ReadFull(right, rightBuffer[:count]); err != nil {
			return false, err
		}
		if !bytes.Equal(leftBuffer[:count], rightBuffer[:count]) {
			return false, nil
		}
		remaining -= count
	}
	return true, nil
}

func readIndexSource(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func skippableReadError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission)
}
func statSignature(files []walkedFile) string {
	hash := sha256.New()
	for _, file := range files {
		hash.Write([]byte(file.Path))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.FormatInt(file.Size, 10)))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.FormatInt(file.MTime, 10)))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (e *Engine) refreshChanged(ctx context.Context, entry registryEntry) error {
	files, err := e.walkEntry(ctx, entry)
	if err != nil {
		return err
	}
	store, err := e.storeFor(entry.ID)
	if err != nil {
		return err
	}
	signature, err := store.meta("stat_signature")
	if err != nil {
		return err
	}
	if statSignature(files) != signature {
		_, err = e.refreshEntry(ctx, entry)
		return err
	}
	e.mu.Lock()
	e.checked[entry.ID] = time.Now()
	e.mu.Unlock()
	return nil
}

func (e *Engine) queueRefreshCheck(entry registryEntry) {
	operation := e.operationFor(entry.ID)
	e.jobsMu.Lock()
	if e.closed || e.stopping {
		e.jobsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(e.jobsCtx)
	generation, ok := operation.tryActivate(cancel)
	if !ok {
		cancel()
		e.jobsMu.Unlock()
		return
	}
	e.jobs.Add(1)
	e.jobsMu.Unlock()
	e.busy.Add(1)
	go func() {
		defer e.jobs.Done()
		defer e.busy.Add(-1)
		defer cancel()
		defer operation.deactivate(generation)
		operation.mu.Lock()
		defer operation.mu.Unlock()
		_ = e.refreshChanged(ctx, entry)
	}()
}

func (e *Engine) ensureFresh(ctx context.Context, entry registryEntry) error {
	e.mu.Lock()
	checked := e.checked[entry.ID]
	e.mu.Unlock()
	if time.Since(checked) < 2*time.Second {
		return nil
	}
	files, err := e.walkEntry(ctx, entry)
	if err != nil {
		return err
	}
	store, err := e.storeFor(entry.ID)
	if err != nil {
		return err
	}
	signature, err := store.meta("stat_signature")
	if err != nil {
		return err
	}
	if statSignature(files) != signature {
		_, err = e.Refresh(ctx, protocol.IndexRef{ID: entry.ID})
		return err
	}
	e.mu.Lock()
	e.checked[entry.ID] = time.Now()
	e.mu.Unlock()
	return nil
}
func (e *Engine) Busy() bool { return e.busy.Load() > 0 }
func (e *Engine) stopJobs() {
	e.jobsMu.Lock()
	e.stopping = true
	e.jobsCancel()
	e.jobsMu.Unlock()
	e.jobs.Wait()
}
func (e *Engine) resumeJobs() {
	e.jobsMu.Lock()
	if !e.closed {
		e.jobsCtx, e.jobsCancel = context.WithCancel(context.Background())
		e.stopping = false
	}
	e.jobsMu.Unlock()
}
func (e *Engine) Release() {
	e.stopJobs()
	e.mu.Lock()
	operations := make([]*indexOperation, 0, len(e.operations))
	for _, operation := range e.operations {
		operations = append(operations, operation)
	}
	e.mu.Unlock()
	for _, operation := range operations {
		operation.cancelActive()
	}
	for _, operation := range operations {
		operation.mu.Lock()
	}
	e.opMu.Lock()
	e.mu.Lock()
	for id, store := range e.stores {
		store.Close()
		delete(e.stores, id)
	}
	e.mu.Unlock()
	e.opMu.Unlock()
	for index := len(operations) - 1; index >= 0; index-- {
		operations[index].mu.Unlock()
	}
	embed.Release()
	e.resumeJobs()
}
func (e *Engine) Close() error {
	e.jobsMu.Lock()
	e.closed = true
	e.jobsMu.Unlock()
	e.Release()
	return nil
}
