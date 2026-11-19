//go:build live

package search

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// go test -tags live,nosqlite,purego_utp -run Live -v ./internal/search/
func TestLiveTbh(t *testing.T) {
	dir := t.TempDir()
	e, err := New(Options{StateFile: filepath.Join(dir, "health.json"), CacheDir: dir, Version: "live-test"})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	s := startSession(t, e)
	s.hello()
	report := func(id int64, start time.Time, lines []map[string]any) map[string]map[string]any {
		last := map[string]map[string]any{}
		var out []string
		for _, m := range lines {
			src, _ := m["source"].(string)
			if src == "" || num(m, "id") != id {
				continue
			}
			last[src] = m
			switch m["state"] {
			case "done":
				items := m["items"].([]any)
				first, engines := "", ""
				if len(items) > 0 {
					it := items[0].(map[string]any)
					first = it["title"].(string) + " | " + it["snippet"].(string)
					if eng, ok := it["meta"].(map[string]any)["engines"]; ok {
						engines = fmt.Sprint(eng)
					}
				}
				out = append(out, fmt.Sprintf("%-11s done   %2d rows rank=%v %s %.90q", src, len(items), m["rank"], engines, first))
			case "error", "paused", "waiting":
				out = append(out, fmt.Sprintf("%-11s %-6s %v %v", src, m["state"], m["error"], m["until"]))
			}
		}
		sort.Strings(out)
		t.Logf("query %d after %v:\n%s", id, time.Since(start).Round(time.Millisecond), strings.Join(out, "\n"))
		return last
	}

	start := time.Now()
	s.send(`{"id":1,"q":"what does tbh mean in texting","scope":"all","mode":"suggest"}`)
	suggest := report(1, start, s.until(isFinal(1)))
	if d := suggest["dictionary"]; d == nil || d["state"] != "done" || len(d["items"].([]any)) == 0 {
		t.Errorf("no definition card in suggest mode: %v", d)
	}
	if _, ok := suggest["web"]; ok {
		t.Error("web lane ran in suggest mode")
	}

	start = time.Now()
	s.send(`{"id":2,"q":"what does tbh mean in texting","scope":"web","mode":"full"}`)
	full := report(2, start, s.until(isFinal(2)))
	web := full["web"]
	if web == nil || web["state"] != "done" || len(web["items"].([]any)) == 0 {
		t.Fatalf("no merged web results: %v", web)
	}
	answered := map[string]int{}
	for _, raw := range web["items"].([]any) {
		for _, eng := range raw.(map[string]any)["meta"].(map[string]any)["engines"].([]any) {
			answered[eng.(string)]++
		}
	}
	t.Logf("web rows credited per engine: %v", answered)

	start = time.Now()
	s.send(`{"id":3,"q":"what does tbh mean in texting","scope":"video","mode":"full"}`)
	if yt := report(3, start, s.until(isFinal(3)))["youtube"]; yt == nil || yt["state"] != "done" {
		t.Errorf("youtube: %v", yt)
	}
	for _, info := range e.gov.infos() {
		if info.State != "ok" {
			t.Logf("engine %s %s until %s", info.ID, info.State, info.Until)
		}
	}
	s.finish()
}
