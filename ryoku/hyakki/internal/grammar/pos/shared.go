package pos

import (
	"container/list"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"sync"
)

//go:embed model/trained_tagger_model.json
var taggerSource []byte

//go:embed model/trained_chunker_model.json
var chunkerSource []byte

//go:embed model/model.mpk
var burnSource []byte

//go:embed model/vocab.json
var vocabSource []byte

var sharedTagger = sync.OnceValue(func() *BrillTagger {
	var b BrillTagger
	if err := json.Unmarshal(taggerSource, &b); err != nil {
		panic(err)
	}
	return &b
})
var sharedBrill = sync.OnceValue(func() *BrillChunker {
	var b BrillChunker
	if err := json.Unmarshal(chunkerSource, &b); err != nil {
		panic(err)
	}
	return &b
})
var sharedBurn = sync.OnceValue(func() *CachedChunker {
	b, e := LoadBurnChunkerCPU(burnSource, vocabSource, 6, 0.3)
	if e != nil {
		panic(e)
	}
	return NewCachedChunker(b, 10000)
})

func SharedBrillTagger() *BrillTagger   { return sharedTagger() }
func SharedBrillChunker() *BrillChunker { return sharedBrill() }
func SharedBurnChunker() *CachedChunker { return sharedBurn() }

// CachedChunker wraps a chunker with a concurrency-safe, bounded LRU cache.
type CachedChunker struct {
	inner    Chunker
	capacity int
	mu       sync.Mutex
	entries  map[string]*list.Element
	lru      list.List
}
type cacheEntry struct {
	key   string
	flags []bool
}

func NewCachedChunker(inner Chunker, capacity int) *CachedChunker {
	if capacity <= 0 {
		panic("pos: cache capacity must be nonzero")
	}
	return &CachedChunker{inner: inner, capacity: capacity, entries: make(map[string]*list.Element)}
}
func chunkKey(words []string, tags []*UPOS) string {
	n := 8 + len(tags)
	for _, w := range words {
		n += 8 + len(w)
	}
	b := make([]byte, 0, n)
	b = binary.LittleEndian.AppendUint64(b, uint64(len(words)))
	for _, w := range words {
		b = binary.LittleEndian.AppendUint64(b, uint64(len(w)))
		b = append(b, w...)
	}
	for _, t := range tags {
		u := byte(0)
		if t != nil {
			u = byte(*t) + 1
		}
		b = append(b, u)
	}
	return string(b)
}
func (c *CachedChunker) ChunkSentence(sentence []string, tags []*UPOS) []bool {
	key := chunkKey(sentence, tags)
	if c.mu.TryLock() {
		if e, ok := c.entries[key]; ok {
			c.lru.MoveToFront(e)
			out := append([]bool{}, e.Value.(cacheEntry).flags...)
			c.mu.Unlock()
			return out
		}
		c.mu.Unlock()
	}
	out := c.inner.ChunkSentence(sentence, tags)
	if c.mu.TryLock() {
		if e, ok := c.entries[key]; ok {
			c.lru.MoveToFront(e)
		} else {
			e := c.lru.PushFront(cacheEntry{key, append([]bool{}, out...)})
			c.entries[key] = e
			if c.lru.Len() > c.capacity {
				last := c.lru.Back()
				delete(c.entries, last.Value.(cacheEntry).key)
				c.lru.Remove(last)
			}
		}
		c.mu.Unlock()
	}
	return out
}
