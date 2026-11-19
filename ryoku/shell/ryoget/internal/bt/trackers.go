package bt

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// publicTrackersURL is ngosang/trackerslist's "best" list, the one Motrix
// syncs by default.
var publicTrackersURL = "https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best.txt"

const (
	trackersFile     = "trackers_best.txt"
	trackersMaxAge   = 24 * time.Hour
	trackersRetry    = time.Hour
	trackersMaxBytes = 256 << 10
	// A magnet with fewer trackers than this gets the public list too.
	fewTrackers = 5
)

type trackerList struct {
	path string
	http *http.Client

	mu       sync.Mutex
	list     []string
	fetched  time.Time
	tried    time.Time
	fetching bool
	waiters  []func([]string)
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func newTrackerList(cacheDir string, hc *http.Client) *trackerList {
	tl := &trackerList{http: hc}
	tl.ctx, tl.cancel = context.WithCancel(context.Background())
	if cacheDir == "" {
		return tl
	}
	tl.path = filepath.Join(cacheDir, trackersFile)
	if b, err := os.ReadFile(tl.path); err == nil {
		tl.list = parseTrackers(b)
		if fi, err := os.Stat(tl.path); err == nil {
			tl.fetched = fi.ModTime()
		}
	}
	return tl
}

// with hands fn the list: now when it is fresh, otherwise once a refresh has
// settled (with the stale list if the refresh failed). Failures stay silent.
func (tl *trackerList) with(fn func([]string)) {
	tl.mu.Lock()
	now := time.Now()
	if now.Sub(tl.fetched) < trackersMaxAge || publicTrackersURL == "" ||
		(!tl.fetching && now.Sub(tl.tried) < trackersRetry) {
		list := tl.list
		tl.mu.Unlock()
		if len(list) > 0 {
			fn(list)
		}
		return
	}
	tl.waiters = append(tl.waiters, fn)
	if !tl.fetching {
		tl.fetching = true
		tl.tried = now
		tl.wg.Add(1)
		go tl.refresh()
	}
	tl.mu.Unlock()
}

func (tl *trackerList) refresh() {
	defer tl.wg.Done()
	list, err := tl.fetch()
	tl.mu.Lock()
	if err == nil && len(list) > 0 {
		tl.list = list
		tl.fetched = time.Now()
	}
	list = tl.list
	waiters := tl.waiters
	tl.waiters = nil
	tl.fetching = false
	tl.mu.Unlock()
	if len(list) == 0 {
		return
	}
	for _, fn := range waiters {
		fn(list)
	}
}

func (tl *trackerList) fetch() ([]string, error) {
	ctx, cancel := context.WithTimeout(tl.ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicTrackersURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := tl.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errStatus(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, trackersMaxBytes))
	if err != nil {
		return nil, err
	}
	list := parseTrackers(b)
	if len(list) > 0 && tl.path != "" {
		if os.MkdirAll(filepath.Dir(tl.path), 0o700) == nil {
			_ = writeAtomic(tl.path, []byte(strings.Join(list, "\n")+"\n"))
		}
	}
	return list, nil
}

func parseTrackers(b []byte) []string {
	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if seen[line] {
			continue
		}
		for _, scheme := range []string{"udp://", "http://", "https://", "wss://"} {
			if strings.HasPrefix(line, scheme) {
				out = append(out, line)
				seen[line] = true
				break
			}
		}
	}
	return out
}

func (tl *trackerList) close() {
	tl.cancel()
	tl.wg.Wait()
}
