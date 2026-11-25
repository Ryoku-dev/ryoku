package tts

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/paths"
)

// Bounds on the line cache.
const (
	CacheMaxEntries = 128
	CacheMaxBytes   = 16 << 20
	// CacheMaxRunes is the longest sentence cached; longer ones are
	// rendered every time. Pre-rendered lines (acknowledgements, greetings)
	// are far shorter.
	CacheMaxRunes = 160
)

const cacheExt = ".f32"

// CacheDir is $XDG_CACHE_HOME/hyakki/oni/voice.
func CacheDir() string { return filepath.Join(paths.CacheHome(), "hyakki", "oni", "voice") }

// Cache keeps rendered sentences as files of native-endian float32 PCM at
// SpeechRateHz, keyed by engine voice and text. It holds nothing in memory
// and starts nothing: the folder is made on the first Put, and every Put
// evicts the least recently used files beyond the entry and byte bounds.
type Cache struct {
	dir        string
	maxEntries int
	maxBytes   int64
	mu         sync.Mutex
}

// NewCache returns a cache in dir ("" is CacheDir()) with its bounds (0 is
// the default). It touches nothing on disk.
func NewCache(dir string, maxEntries int, maxBytes int64) *Cache {
	if dir == "" {
		dir = CacheDir()
	}
	if maxEntries <= 0 {
		maxEntries = CacheMaxEntries
	}
	if maxBytes <= 0 {
		maxBytes = CacheMaxBytes
	}
	return &Cache{dir: dir, maxEntries: maxEntries, maxBytes: maxBytes}
}

// Cacheable reports whether a sentence is short enough to cache.
func Cacheable(line string) bool {
	n := utf8.RuneCountInString(line)
	return n > 0 && n <= CacheMaxRunes
}

func (c *Cache) path(e Engine, line string) string {
	h := sha256.Sum256([]byte(e.Name() + "\x00" + e.Voice() + "\x00" + strconv.Itoa(SpeechRateHz()) + "\x00" + line))
	return filepath.Join(c.dir, hex.EncodeToString(h[:16])+cacheExt)
}

// Get returns the cached PCM of line as e renders it. A damaged file is
// removed and read as a miss.
func (c *Cache) Get(e Engine, line string) ([]float32, bool) {
	if !Cacheable(line) {
		return nil, false
	}
	p := c.path(e, line)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	if len(data) == 0 || len(data)%4 != 0 {
		_ = os.Remove(p)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	pcm := make([]float32, len(data)/4)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.NativeEndian.Uint32(data[i*4:]))
	}
	return pcm, true
}

// Put stores pcm for line as e renders it, then trims the cache to its
// bounds. A line too long, or PCM larger than the whole cache, is not
// stored.
func (c *Cache) Put(e Engine, line string, pcm []float32) error {
	if !Cacheable(line) || len(pcm) == 0 || int64(len(pcm))*4 > c.maxBytes {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	data := make([]byte, 0, len(pcm)*4)
	for _, v := range pcm {
		data = binary.NativeEndian.AppendUint32(data, math.Float32bits(v))
	}
	f, err := os.CreateTemp(c.dir, ".line-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), c.path(e, line)); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return c.trim(filepath.Base(c.path(e, line)))
}

// trim removes the least recently used entries, never keep (the entry just
// written), until the cache is within its bounds.
func (c *Cache) trim(keep string) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	type file struct {
		name string
		size int64
		used time.Time
	}
	var files []file
	var total int64
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), cacheExt) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{e.Name(), info.Size(), info.ModTime()})
		total += info.Size()
	}
	slices.SortFunc(files, func(a, b file) int {
		switch {
		case a.name == keep:
			return 1
		case b.name == keep:
			return -1
		}
		return a.used.Compare(b.used)
	})
	for len(files) > 0 && (len(files) > c.maxEntries || total > c.maxBytes) {
		if err := os.Remove(filepath.Join(c.dir, files[0].name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		total -= files[0].size
		files = files[1:]
	}
	return nil
}
