package index

import (
	"bytes"
	"context"
	"errors"
	"runtime/debug"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/index/embed"
	"ryoku-rashin/internal/protocol"
)

func (s *indexStore) chunksWithoutVectors(after []byte, limit int) (out []chunkRecord, last []byte, done bool, err error) {
	done = true
	err = s.view(func(tx *bbolt.Tx) error {
		chunks := tx.Bucket(bucketChunks)
		vectors := tx.Bucket(bucketVectors)
		cursor := chunks.Cursor()
		key, value := cursor.First()
		if len(after) > 0 {
			key, value = cursor.Seek(after)
			if bytes.Equal(key, after) {
				key, value = cursor.Next()
			}
		}
		for ; key != nil; key, value = cursor.Next() {
			last = append(last[:0], key...)
			if vectors.Get(key) != nil {
				continue
			}
			var chunk chunkRecord
			if decodeErr := decode(value, &chunk); decodeErr != nil {
				return decodeErr
			}
			out = append(out, chunk)
			if len(out) == limit {
				done = false
				break
			}
		}
		return nil
	})
	return
}

func (e *Engine) stopIndexEmbedding(id string) {
	e.jobsMu.Lock()
	cancel := e.embeddingCancel[id]
	done := e.embeddingDone[id]
	e.jobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (e *Engine) startEmbedding(entry registryEntry, store *indexStore) {
	e.jobsMu.Lock()
	if e.closed || e.stopping || e.embedding[entry.ID] {
		e.jobsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(e.jobsCtx)
	done := make(chan struct{})
	e.embedding[entry.ID] = true
	e.embeddingCancel[entry.ID] = cancel
	e.embeddingDone[entry.ID] = done
	e.jobs.Add(1)
	e.jobsMu.Unlock()
	e.busy.Add(1)
	go func() {
		var embedTexts func(context.Context, []string) ([][]float32, error)
		defer e.jobs.Done()
		defer e.busy.Add(-1)
		defer func() {
			e.jobsMu.Lock()
			delete(e.embedding, entry.ID)
			delete(e.embeddingCancel, entry.ID)
			delete(e.embeddingDone, entry.ID)
			releaseModel := len(e.embedding) == 0
			close(done)
			e.jobsMu.Unlock()
			embedTexts = nil
			if releaseModel {
				embed.Release()
				time.AfterFunc(100*time.Millisecond, debug.FreeOSMemory)
				time.AfterFunc(500*time.Millisecond, debug.FreeOSMemory)
			}
			cancel()
		}()
		embedTexts = e.embedTexts
		if embedTexts == nil {
			model, err := embed.Load()
			if err != nil {
				e.embeddingError(entry, store, err)
				return
			}
			embedTexts = model.Embed
		}
		const batchSize = 32
		_, chunkCount, _, vectorCount, err := store.storedCounts()
		if err != nil {
			e.embeddingError(entry, store, err)
			return
		}
		total := vectorCount
		embedded := false
		var after []byte
		for {
			if !e.currentSettings().Index.Semantic {
				store.setMeta(map[string]string{"state": "idle"})
				return
			}
			pending, last, done, batchErr := store.chunksWithoutVectors(after, batchSize)
			if batchErr != nil {
				e.embeddingError(entry, store, batchErr)
				return
			}
			if len(pending) == 0 {
				if !done {
					after = last
					continue
				}
				probe, _, _, probeErr := store.chunksWithoutVectors(nil, 1)
				if probeErr != nil {
					e.embeddingError(entry, store, probeErr)
					return
				}
				if len(probe) > 0 {
					after = nil
					continue
				}
				if embedded {
					if compactErr := store.compact(); compactErr != nil {
						e.embeddingError(entry, store, compactErr)
						return
					}
				}
				store.setMeta(map[string]string{"state": "idle", "error": "", "count_vectors": strconv.Itoa(total)})
				e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "done", Done: total, Total: chunkCount})
				if info, infoErr := store.info(entry); infoErr == nil {
					e.broadcast(protocol.EvtIndexChanged, info)
				}
				return
			}
			if err := ctx.Err(); err != nil {
				store.setMeta(map[string]string{"state": "idle"})
				return
			}
			store.setMeta(map[string]string{"state": "embedding"})
			texts := make([]string, len(pending))
			ids := make([]string, len(pending))
			for index := range pending {
				texts[index] = pending[index].Text
				ids[index] = pending[index].ID
			}
			vectors, embedErr := embedTexts(ctx, texts)
			if embedErr != nil {
				e.embeddingError(entry, store, embedErr)
				return
			}
			if err := store.putVectors(ids, vectors); err != nil {
				e.embeddingError(entry, store, err)
				return
			}
			embedded = true
			total += len(vectors)
			store.setMeta(map[string]string{"count_vectors": strconv.Itoa(total)})
			e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "embed", Done: total, Total: chunkCount})
			after = last
			pending = nil
			texts = nil
			ids = nil
			vectors = nil
		}
	}()
}

func (e *Engine) embeddingError(entry registryEntry, store *indexStore, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	store.setMeta(map[string]string{"state": "error", "error": err.Error()})
	e.broadcast(protocol.EvtIndexProgress, protocol.IndexProgress{ID: entry.ID, Phase: "error", Message: err.Error()})
	if info, infoErr := store.info(entry); infoErr == nil {
		e.broadcast(protocol.EvtIndexChanged, info)
	}
}
