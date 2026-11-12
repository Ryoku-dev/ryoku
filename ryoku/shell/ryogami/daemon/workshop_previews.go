package main

import (
	"os"
	"path/filepath"
	"sync"
)

func init() {
	// Clips are skipped while previews are off, so turning them on backfills the library.
	watchSetting("videoPreview.enabled", func(d *daemon, _ string, v interface{}) {
		if on, ok := v.(bool); ok && on {
			go d.rescan(false)
		}
	})
}

func weVideoSource(cacheDir string, e Entry) (scanned, bool) {
	if e.WeType != "video" || e.VideoFile == "" {
		return scanned{}, false
	}
	st, err := os.Stat(e.VideoFile)
	if err != nil || !st.Mode().IsRegular() {
		return scanned{}, false
	}
	return scanned{
		key:       e.Key,
		name:      e.WeID,
		wpType:    "video",
		src:       e.VideoFile,
		videoFile: e.VideoFile,
		mtime:     st.ModTime().Unix(),
		filesize:  st.Size(),
		thumbPath: filepath.Join(cacheDir, "wallpaper", "video-thumbs", "we--"+e.WeID+".webp"),
	}, true
}

// on the thumbnail job pool, so a large workshop library never transcodes everything at once
func buildWeVideoPreviews(cacheDir string, rows map[string]Entry, onItem func(Entry)) {
	var todo []scanned
	for _, e := range rows {
		if it, ok := weVideoSource(cacheDir, e); ok {
			todo = append(todo, it)
		}
	}
	var mu sync.Mutex
	forEachThumbJob(todo, func(it scanned) {
		prev := ensureVideoPreview(it)
		mu.Lock()
		defer mu.Unlock()
		e := rows[it.key]
		if e.VideoPrev == prev {
			return
		}
		e.VideoPrev = prev
		rows[it.key] = e
		if prev != "" && onItem != nil {
			onItem(e)
		}
	})
}
