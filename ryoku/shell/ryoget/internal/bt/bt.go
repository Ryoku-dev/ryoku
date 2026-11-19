// Package bt is ryoget's BitTorrent and magnet engine on anacrolix/torrent.
//
// One Client serves every torrent task. Metainfo and a small sidecar live under
// <StateDir>/torrents, piece completion in a bolt database in StateDir, so a
// restarted daemon re-adds a task and carries on where it stopped. Build with
// `-tags nosqlite,purego_utp` so no lane ever links SQLite or C++ libutp.
package bt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/time/rate"

	"ryoget/internal/proto"
	i18n "ryoku-i18n"
)

// SeedPolicy says how long a finished torrent keeps uploading. Seeding ends
// when either bound is reached; a zero bound is off, and On with both bounds
// off seeds until the task is removed. On false stops at completion.
type SeedPolicy struct {
	On      bool
	Ratio   float64
	Minutes int
}

type Config struct {
	StateDir string
	CacheDir string
	// Port 0 picks a random port.
	Port int
	DHT  bool
	PEX  bool
	// Down and Up are shared with the HTTP engine; the caller changes them live
	// with SetLimit. Their burst must hold a 16 KiB chunk. nil is unlimited.
	Down, Up *rate.Limiter
	Seed     SeedPolicy
	// Trackers are added to every public torrent.
	Trackers []string
	HTTP     *http.Client
	Log      *slog.Logger
}

type Client struct {
	cfg  Config
	cl   *torrent.Client
	pc   *completion
	http *http.Client
	log  *slog.Logger
	trk  *trackerList

	// addPeers lets tests wire clients together without trackers or DHT.
	addPeers func(*torrent.Torrent)

	mu        sync.Mutex
	seed      SeedPolicy
	torrents  map[metainfo.Hash]*Torrent
	resolving map[metainfo.Hash]int
	purging   map[metainfo.Hash]chan struct{}
	sources   map[string]metainfo.Hash
	ticking   bool
	closed    bool
	stop      chan struct{}
	wg        sync.WaitGroup
}

// configure is the test seam over the anacrolix config.
var configure = func(*torrent.ClientConfig) {}

const maxMetainfoBytes = 16 << 20

func New(cfg Config) (*Client, error) {
	if cfg.StateDir == "" {
		return nil, errors.New("bt: no state dir")
	}
	c := &Client{
		cfg:       cfg,
		seed:      cfg.Seed,
		http:      cfg.HTTP,
		log:       cfg.Log,
		torrents:  map[metainfo.Hash]*Torrent{},
		resolving: map[metainfo.Hash]int{},
		purging:   map[metainfo.Hash]chan struct{}{},
		stop:      make(chan struct{}),
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: time.Minute}
	}
	if c.log == nil {
		c.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := os.MkdirAll(c.torrentsDir(), 0o700); err != nil {
		return nil, err
	}
	pc, err := openCompletion(cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("bt: piece completion: %w", err)
	}
	c.pc = pc

	cc := torrent.NewDefaultClientConfig()
	cc.DataDir = cfg.StateDir
	cc.DefaultStorage = noStorage{}
	cc.ListenPort = cfg.Port
	cc.NoDHT = !cfg.DHT
	cc.DisablePEX = !cfg.PEX
	cc.Seed = true
	cc.DownloadRateLimiter = cfg.Down
	if cfg.Up != nil {
		cc.UploadRateLimiter = cfg.Up
	}
	cc.Slogger = c.log
	cc.MetainfoSourcesClient = c.http
	configure(cc)
	cl, err := torrent.NewClient(cc)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("bt: %w", err)
	}
	c.cl = cl
	c.sources = c.loadSources()
	c.trk = newTrackerList(cfg.CacheDir, c.http)
	return c, nil
}

// Port is the port peers reach this client on.
func (c *Client) Port() int { return c.cl.LocalPort() }

func (c *Client) SetSeed(p SeedPolicy) {
	c.mu.Lock()
	c.seed = p
	c.mu.Unlock()
	c.kick()
}

func (c *Client) seedPolicy() SeedPolicy {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seed
}

// Close saves every torrent's counters and stops the client. Torrent handles
// are unusable afterwards.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.stop)
	list := make([]*Torrent, 0, len(c.torrents))
	for _, t := range c.torrents {
		list = append(list, t)
	}
	c.mu.Unlock()
	for _, t := range list {
		t.persist()
	}
	errs := c.cl.Close()
	c.wg.Wait()
	c.trk.close()
	errs = append(errs, c.pc.Close())
	return errors.Join(errs...)
}

// Add starts (or returns) the torrent behind src: a magnet URI, an http(s)
// .torrent URL or a local .torrent path. selected holds file indices; nil
// means every file. It returns before a magnet's metadata arrives.
func (c *Client) Add(ctx context.Context, src, dir string, selected []int) (*Torrent, error) {
	if selected != nil && len(selected) == 0 {
		return nil, invalid("Select at least one file.")
	}
	spec, magnet, err := c.load(ctx, src)
	if err != nil {
		return nil, err
	}
	ih := specHash(spec)
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, errClosed
		}
		if t, ok := c.torrents[ih]; ok {
			c.mu.Unlock()
			if selected != nil {
				if err := t.Select(selected); err != nil {
					return nil, err
				}
			}
			return t, nil
		}
		wait, purging := c.purging[ih]
		if !purging {
			break
		}
		c.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	side := c.loadSidecar(ih)
	side.Src = src
	t := &Torrent{
		c:        c,
		ih:       ih,
		dir:      dir,
		magnet:   magnet,
		trackers: spec.Trackers,
		display:  spec.DisplayName,
		want:     selected,
		side:     side,
		meta:     make(chan struct{}),
		done:     make(chan struct{}),
		fin:      make(chan struct{}),
	}
	c.torrents[ih] = t
	c.sources[src] = ih
	c.mu.Unlock()
	if spec.InfoBytes != nil {
		if mi := c.loadMetainfo(ih); mi == nil {
			if err := c.saveMetainfo(ih, specMetainfo(spec)); err != nil {
				c.log.Warn("saving metainfo", "err", err)
			}
		}
	}
	if err := c.saveSidecar(ih, side); err != nil {
		c.log.Warn("saving torrent state", "err", err)
	}
	t.mu.Lock()
	err = t.startLocked(spec)
	t.mu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.torrents, ih)
		c.mu.Unlock()
		return nil, err
	}
	return t, nil
}

// Resolve reads a torrent's name and files without starting it. A magnet
// waits for its metadata from peers until ctx ends; the metadata is kept so a
// following Add starts at once.
func (c *Client) Resolve(ctx context.Context, src string) (string, []proto.File, error) {
	spec, magnet, err := c.load(ctx, src)
	if err != nil {
		return "", nil, err
	}
	ih := specHash(spec)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return "", nil, errClosed
	}
	if t, ok := c.torrents[ih]; ok {
		c.mu.Unlock()
		select {
		case <-t.Metadata():
			return t.Name(), t.Files(), nil
		case <-ctx.Done():
			return "", nil, unavailable()
		}
	}
	if spec.InfoBytes != nil {
		c.mu.Unlock()
		info, err := specInfo(spec)
		if err != nil {
			return "", nil, invalid("This is not a torrent or magnet link.")
		}
		return info.BestName(), infoFiles(info), nil
	}
	c.resolving[ih]++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.resolving[ih]--
		if c.resolving[ih] == 0 {
			delete(c.resolving, ih)
			if _, owned := c.torrents[ih]; !owned {
				if tor, ok := c.cl.Torrent(ih); ok {
					tor.Drop()
				}
			}
		}
		c.mu.Unlock()
	}()
	tor, err := c.addSpec(spec, magnet)
	if err != nil {
		return "", nil, err
	}
	select {
	case <-tor.GotInfo():
	case <-tor.Closed():
		return "", nil, errClosed
	case <-ctx.Done():
		return "", nil, unavailable()
	}
	if err := c.saveMetainfo(ih, tor.Metainfo()); err != nil {
		c.log.Warn("saving metainfo", "err", err)
	}
	info := tor.Info()
	return info.BestName(), infoFiles(info), nil
}

// addSpec puts a torrent into the anacrolix client with its storage held back
// until we open it, and gives public magnets more trackers.
func (c *Client) addSpec(spec *torrent.TorrentSpec, magnet bool) (*torrent.Torrent, error) {
	s := *spec
	s.DeferStorageOpen = true
	s.Storage = nil
	if s.InfoBytes == nil || !specPrivate(&s) {
		s.Trackers = appendTier(s.Trackers, c.cfg.Trackers)
	}
	tor, _, err := c.cl.AddTorrentSpec(&s)
	if err != nil {
		return nil, invalid("This is not a torrent or magnet link.")
	}
	if magnet && trackerCount(spec.Trackers) < fewTrackers {
		c.trk.with(func(list []string) {
			select {
			case <-tor.Closed():
			default:
				if info := tor.Info(); info == nil || !isPrivate(info) {
					tor.AddTrackers([][]string{list})
				}
			}
		})
	}
	if c.addPeers != nil {
		c.addPeers(tor)
	}
	return tor, nil
}

// load turns src into a spec, preferring metainfo this client already keeps.
func (c *Client) load(ctx context.Context, src string) (spec *torrent.TorrentSpec, magnet bool, err error) {
	src = strings.TrimSpace(src)
	switch {
	case strings.HasPrefix(strings.ToLower(src), "magnet:"):
		spec, err = torrent.TorrentSpecFromMagnetUri(src)
		if err != nil || spec.InfoHash.IsZero() && !spec.InfoHashV2.Ok {
			return nil, false, invalid("This is not a torrent or magnet link.")
		}
		if mi := c.loadMetainfo(specHash(spec)); mi != nil {
			known, err := specFromMetainfo(mi)
			if err == nil {
				known.Trackers = appendTier(known.Trackers, flatten(spec.Trackers))
				if known.DisplayName == "" {
					known.DisplayName = spec.DisplayName
				}
				return known, true, nil
			}
		}
		return spec, true, nil
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		c.mu.Lock()
		ih, ok := c.sources[src]
		c.mu.Unlock()
		if ok {
			if mi := c.loadMetainfo(ih); mi != nil {
				spec, err = specFromMetainfo(mi)
				if err == nil {
					return spec, false, nil
				}
			}
		}
		mi, err := c.fetchMetainfo(ctx, src)
		if err != nil {
			return nil, false, err
		}
		spec, err = specFromMetainfo(mi)
		if err != nil {
			return nil, false, invalid("This is not a torrent or magnet link.")
		}
		return spec, false, nil
	default:
		path := src
		if u, err := url.Parse(src); err == nil && u.Scheme == "file" {
			path = u.Path
		}
		mi, err := loadMetainfoFile(path)
		if err != nil {
			return nil, false, err
		}
		spec, err = specFromMetainfo(mi)
		if err != nil {
			return nil, false, invalid("This is not a torrent or magnet link.")
		}
		return spec, false, nil
	}
}

func loadMetainfoFile(path string) (*metainfo.MetaInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &proto.Error{Code: proto.CodeNotFound, Message: i18n.T("The torrent file is missing.")}
		}
		return nil, &proto.Error{Code: proto.CodeInvalid, Message: err.Error()}
	}
	defer f.Close()
	mi, err := metainfo.Load(io.LimitReader(f, maxMetainfoBytes))
	if err != nil {
		return nil, invalid("This is not a torrent or magnet link.")
	}
	return mi, nil
}

func (c *Client) fetchMetainfo(ctx context.Context, src string) (*metainfo.MetaInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, invalid("This is not a torrent or magnet link.")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &proto.Error{Code: proto.CodeNetwork, Message: i18n.T("Could not fetch the torrent file.")}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		code := proto.CodeUnavailable
		if resp.StatusCode == http.StatusTooManyRequests {
			code = proto.CodeRateLimited
		}
		return nil, &proto.Error{Code: code, Message: i18n.T("Could not fetch the torrent file.")}
	}
	mi, err := metainfo.Load(io.LimitReader(resp.Body, maxMetainfoBytes))
	if err != nil {
		return nil, invalid("This is not a torrent or magnet link.")
	}
	return mi, nil
}

// kick starts the supervisor if it is not running. It stops by itself once no
// torrent is downloading or seeding, so an idle client has no ticker.
func (c *Client) kick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ticking || c.closed {
		return
	}
	c.ticking = true
	c.wg.Add(1)
	go c.supervise()
}

func (c *Client) supervise() {
	defer c.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-c.stop:
			return
		case now := <-tick.C:
			c.mu.Lock()
			list := make([]*Torrent, 0, len(c.torrents))
			for _, t := range c.torrents {
				list = append(list, t)
			}
			c.mu.Unlock()
			policy := c.seedPolicy()
			busy := false
			for _, t := range list {
				if t.tick(now, policy) {
					busy = true
				}
			}
			if !busy {
				c.mu.Lock()
				c.ticking = false
				c.mu.Unlock()
				return
			}
		}
	}
}

// Forget removes what a Client keeps for one torrent (metainfo, counters and
// piece completion) without starting one. key is an infohash in hex, a magnet
// or the source the torrent was added from. Downloaded files are left alone.
// Call it only while no Client is open on stateDir: the completion database
// allows one opener.
func Forget(stateDir, key string) error {
	c := &Client{cfg: Config{StateDir: stateDir}}
	ih, ok, err := keyHash(key, c.loadSources())
	if err != nil || !ok {
		return err
	}
	pieces := c.forgetKept(ih)
	if pieces == 0 {
		return nil
	}
	pc, err := openCompletion(stateDir)
	if err != nil {
		return err
	}
	pc.forget(context.Background(), ih, pieces)
	return pc.Close()
}

// Forget is the package Forget through this client's open store, for a
// torrent it does not hold (a paused one from an earlier client). A torrent it
// holds is left alone: its own Drop forgets the same state.
func (c *Client) Forget(key string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errClosed
	}
	ih, ok, err := keyHash(key, c.sources)
	if err != nil || !ok {
		c.mu.Unlock()
		return err
	}
	if _, held := c.torrents[ih]; held {
		c.mu.Unlock()
		return nil
	}
	for src, h := range c.sources {
		if h == ih {
			delete(c.sources, src)
		}
	}
	c.mu.Unlock()
	c.purge(ih, c.forgetKept(ih))
	return nil
}

// forgetKept removes a torrent's metainfo and sidecar and returns its piece
// count, so the caller can clear the completion store.
func (c *Client) forgetKept(ih metainfo.Hash) (pieces int) {
	if mi := c.loadMetainfo(ih); mi != nil {
		if info, err := mi.UnmarshalInfo(); err == nil {
			pieces = info.NumPieces()
		}
	}
	c.forgetState(ih)
	return
}

// keyHash reads an infohash in hex, a magnet, or a source this state knows.
func keyHash(key string, sources map[string]metainfo.Hash) (metainfo.Hash, bool, error) {
	var ih metainfo.Hash
	if ih.FromHexString(key) == nil {
		return ih, true, nil
	}
	if strings.HasPrefix(strings.ToLower(key), "magnet:") {
		spec, err := torrent.TorrentSpecFromMagnetUri(key)
		if err != nil || spec.InfoHash.IsZero() && !spec.InfoHashV2.Ok {
			return ih, false, invalid("This is not a torrent or magnet link.")
		}
		return specHash(spec), true, nil
	}
	ih, ok := sources[key]
	return ih, ok, nil
}

func (c *Client) remove(t *Torrent) {
	c.mu.Lock()
	if c.torrents[t.ih] == t {
		delete(c.torrents, t.ih)
	}
	for src, ih := range c.sources {
		if ih == t.ih {
			delete(c.sources, src)
		}
	}
	c.mu.Unlock()
}

// purge forgets a dropped torrent's piece completion in the background; an
// Add of the same torrent waits for it.
func (c *Client) purge(ih metainfo.Hash, pieces int) {
	if pieces == 0 {
		return
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if _, busy := c.purging[ih]; busy {
		c.mu.Unlock()
		return
	}
	done := make(chan struct{})
	c.purging[ih] = done
	c.wg.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-c.stop:
				cancel()
			case <-done:
			}
		}()
		c.pc.forget(ctx, ih, pieces)
		cancel()
		c.mu.Lock()
		delete(c.purging, ih)
		c.mu.Unlock()
		close(done)
	}()
}

var errClosed = errors.New("bt: client closed")

type errStatus int

func (e errStatus) Error() string { return fmt.Sprintf("http status %d", int(e)) }

func invalid(msg string) error {
	return &proto.Error{Code: proto.CodeInvalid, Message: i18n.T(msg)}
}

func unavailable() error {
	return &proto.Error{Code: proto.CodeUnavailable, Message: i18n.T("No peers sent this torrent's file list yet.")}
}

func specFromMetainfo(mi *metainfo.MetaInfo) (*torrent.TorrentSpec, error) {
	spec, err := torrent.TorrentSpecFromMetaInfoErr(mi)
	if err != nil {
		return nil, err
	}
	if spec.InfoHash.IsZero() && !spec.InfoHashV2.Ok {
		return nil, errors.New("bt: metainfo has no infohash")
	}
	return spec, nil
}

// specHash is the key a torrent is known by: its v1 infohash, or the
// truncated v2 one for v2-only torrents, as anacrolix keys piece completion.
func specHash(spec *torrent.TorrentSpec) metainfo.Hash {
	if !spec.InfoHash.IsZero() {
		return spec.InfoHash
	}
	return *spec.InfoHashV2.Value.ToShort()
}

func specMetainfo(spec *torrent.TorrentSpec) metainfo.MetaInfo {
	mi := metainfo.MetaInfo{InfoBytes: spec.InfoBytes, AnnounceList: spec.Trackers, PieceLayers: spec.PieceLayers}
	if len(spec.Trackers) > 0 && len(spec.Trackers[0]) > 0 {
		mi.Announce = spec.Trackers[0][0]
	}
	return mi
}

func trackerCount(tiers [][]string) (n int) {
	for _, tier := range tiers {
		n += len(tier)
	}
	return
}

func flatten(tiers [][]string) (out []string) {
	for _, tier := range tiers {
		out = append(out, tier...)
	}
	return
}

// appendTier adds the urls missing from tiers as one more tier.
func appendTier(tiers [][]string, urls []string) [][]string {
	have := map[string]bool{}
	for _, tier := range tiers {
		for _, u := range tier {
			have[u] = true
		}
	}
	var tier []string
	for _, u := range urls {
		if u != "" && !have[u] {
			have[u] = true
			tier = append(tier, u)
		}
	}
	if len(tier) == 0 {
		return tiers
	}
	return append(append([][]string(nil), tiers...), tier)
}

func isPrivate(info *metainfo.Info) bool { return info.Private != nil && *info.Private }

func specPrivate(spec *torrent.TorrentSpec) bool {
	info, err := specInfo(spec)
	return err == nil && isPrivate(info)
}
