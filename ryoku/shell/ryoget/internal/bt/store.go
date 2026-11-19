package bt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// completion is the bolt piece-completion store with one change: while a
// torrent's storage is being opened, a piece stored as complete is not demoted
// to unknown. The file storage demotes every piece of a file still named .part
// because, with an in-memory store, it cannot tell what is there. Our store is
// persistent and only records a piece after its data was flushed, so trusting
// it is what lets a restarted daemon skip rehashing partial files.
type completion struct {
	storage.PieceCompletion

	mu      sync.Mutex
	opening map[metainfo.Hash]bool
}

var (
	_ storage.PieceCompletionCheckpointer = (*completion)(nil)
	_ storage.PieceCompletionPersistenter = (*completion)(nil)
)

func openCompletion(dir string) (*completion, error) {
	pc, err := storage.NewBoltPieceCompletion(dir)
	if err != nil {
		return nil, err
	}
	return &completion{PieceCompletion: pc, opening: map[metainfo.Hash]bool{}}, nil
}

func (c *completion) Set(pk metainfo.PieceKey, v g.Option[bool]) error {
	if !v.Ok {
		c.mu.Lock()
		opening := c.opening[pk.InfoHash]
		c.mu.Unlock()
		if opening {
			if cur, err := c.PieceCompletion.Get(pk); err == nil && cur.Ok && cur.Complete {
				return nil
			}
		}
	}
	return c.PieceCompletion.Set(pk, v)
}

func (c *completion) Checkpoint(keys []metainfo.PieceKey) error {
	if cp, ok := c.PieceCompletion.(storage.PieceCompletionCheckpointer); ok {
		return cp.Checkpoint(keys)
	}
	return nil
}

func (c *completion) Persistent() bool { return true }

func (c *completion) markOpening(ih metainfo.Hash, on bool) {
	c.mu.Lock()
	if on {
		c.opening[ih] = true
	} else {
		delete(c.opening, ih)
	}
	c.mu.Unlock()
}

// forget drops every stored piece of one torrent so the database does not grow
// with each torrent ever added.
func (c *completion) forget(ctx context.Context, ih metainfo.Hash, pieces int) {
	for i := range pieces {
		if ctx.Err() != nil {
			return
		}
		_ = c.PieceCompletion.Set(metainfo.PieceKey{InfoHash: ih, Index: i}, g.None[bool]())
	}
}

// noStorage is the client default. Every torrent gets its own file storage when
// it is opened, so nothing may ever fall back to a client-wide directory.
type noStorage struct{}

var errNoStorage = errors.New("bt: torrent has no storage")

func (noStorage) OpenTorrent(context.Context, *metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	return storage.TorrentImpl{}, errNoStorage
}

// sidecar is what survives a restart besides metainfo and piece completion.
type sidecar struct {
	Src      string  `json:"src"`
	Uploaded int64   `json:"uploaded"`
	Seeded   float64 `json:"seeded"`
}

func (c *Client) torrentsDir() string { return filepath.Join(c.cfg.StateDir, "torrents") }

func (c *Client) metainfoPath(ih metainfo.Hash) string {
	return filepath.Join(c.torrentsDir(), ih.HexString()+".torrent")
}

func (c *Client) sidecarPath(ih metainfo.Hash) string {
	return filepath.Join(c.torrentsDir(), ih.HexString()+".json")
}

func (c *Client) loadMetainfo(ih metainfo.Hash) *metainfo.MetaInfo {
	mi, err := metainfo.LoadFromFile(c.metainfoPath(ih))
	if err != nil {
		return nil
	}
	if spec, err := specFromMetainfo(mi); err != nil || specHash(spec) != ih {
		return nil
	}
	return mi
}

func (c *Client) saveMetainfo(ih metainfo.Hash, mi metainfo.MetaInfo) error {
	var b bytes.Buffer
	if err := mi.Write(&b); err != nil {
		return err
	}
	return writeAtomic(c.metainfoPath(ih), b.Bytes())
}

func (c *Client) loadSidecar(ih metainfo.Hash) sidecar {
	var s sidecar
	if b, err := os.ReadFile(c.sidecarPath(ih)); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func (c *Client) saveSidecar(ih metainfo.Hash, s sidecar) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return writeAtomic(c.sidecarPath(ih), b)
}

// loadSources maps each remembered source to its infohash, so re-adding a
// .torrent URL after a restart does not fetch it again.
func (c *Client) loadSources() map[string]metainfo.Hash {
	out := map[string]metainfo.Hash{}
	entries, _ := os.ReadDir(c.torrentsDir())
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		var ih metainfo.Hash
		if ih.FromHexString(name) != nil {
			continue
		}
		if s := c.loadSidecar(ih); s.Src != "" {
			out[s.Src] = ih
		}
	}
	return out
}

func (c *Client) forgetState(ih metainfo.Hash) {
	_ = os.Remove(c.metainfoPath(ih))
	_ = os.Remove(c.sidecarPath(ih))
}

func writeAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Close()
	} else {
		tmp.Close()
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}
