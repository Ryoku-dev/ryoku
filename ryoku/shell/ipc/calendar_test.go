package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarRegionUsesLocaleAndOverride(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_US.UTF-8")
	country, subdivision, valid := resolveCalendarRegion("")
	if !valid || country != "US" || subdivision != "" {
		t.Fatalf("auto region = %q %q %v, want US empty true", country, subdivision, valid)
	}
	country, subdivision, valid = resolveCalendarRegion("us-ca")
	if !valid || country != "US" || subdivision != "US-CA" {
		t.Fatalf("override = %q %q %v, want US US-CA true", country, subdivision, valid)
	}
}

func TestCalendarRegionRejectsMalformedOverride(t *testing.T) {
	if _, _, valid := resolveCalendarRegion("california"); valid {
		t.Fatal("malformed override accepted")
	}
}

func TestCalendarHolidayFiltering(t *testing.T) {
	raw := []nagerHoliday{
		{Date: "2026-01-01", Name: "National", NationalHoliday: true, HolidayTypes: []string{"Public"}},
		{Date: "2026-03-31", Name: "California", SubdivisionCodes: []string{"US-CA"}},
		{Date: "2026-03-31", Name: "Nevada", SubdivisionCodes: []string{"US-NV"}},
	}
	got := filterCalendarHolidays(raw, "US-CA")
	if len(got) != 2 || got[0].Name != "National" || got[1].Name != "California" {
		t.Fatalf("filtered holidays = %#v", got)
	}
}

func TestCalendarYearFetchCachesAndReusesResponse(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"date":"2026-01-01","name":"New Year","countryCode":"US","nationalHoliday":true,"holidayTypes":["Public"]}]`))
	}))
	defer server.Close()

	s := &calendarState{client: server.Client(), baseURL: server.URL, cacheDir: t.TempDir(), now: time.Now}
	first, source, err := s.loadYear("US", "", 2026)
	if err != nil || source != "network" || len(first) != 1 {
		t.Fatalf("first load = %#v %q %v", first, source, err)
	}
	second, source, err := s.loadYear("US", "", 2026)
	if err != nil || source != "cache" || len(second) != 1 || calls != 1 {
		t.Fatalf("cached load = %#v %q %v, calls %d", second, source, err, calls)
	}
}

func TestCalendarFailedRefreshKeepsStaleCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "US", "2026.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `[{"date":"2026-01-01","name":"Cached","countryCode":"US","nationalHoliday":true,"holidayTypes":["Public"]}]`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) }))
	defer server.Close()
	s := &calendarState{client: server.Client(), baseURL: server.URL, cacheDir: dir, now: time.Now}
	got, source, err := s.loadYear("US", "", 2026)
	if err == nil || source != "stale-cache" || len(got) != 1 || got[0].Name != "Cached" {
		t.Fatalf("stale fallback = %#v %q %v", got, source, err)
	}
}

type calendarHarness struct {
	s     *calendarState
	calls atomic.Int32
	mu    sync.Mutex
	now   time.Time
}

func newCalendarHarness(t *testing.T) *calendarHarness {
	t.Helper()
	h := &calendarHarness{now: time.Now()}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"date":"2026-01-01","name":"New Year","countryCode":"US","nationalHoliday":true,"holidayTypes":["Public"]}]`))
	}))
	t.Cleanup(server.Close)
	h.s = &calendarState{
		topic: newStateTopic(), client: server.Client(), baseURL: server.URL, cacheDir: t.TempDir(),
		now: func() time.Time {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.now
		},
	}
	h.s.topic.onDemand(h.s.setActive)
	return h
}

func (h *calendarHarness) advance(d time.Duration) {
	h.mu.Lock()
	h.now = h.now.Add(d)
	h.mu.Unlock()
}

func (h *calendarHarness) settle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.s.mu.Lock()
		done := h.s.loadedGen == h.s.generation
		h.s.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("calendar load did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCalendarSubscriberWithoutConfigureFetchesNothing(t *testing.T) {
	h := newCalendarHarness(t)
	sub := h.s.topic.subscribe()
	h.s.topic.unsubscribe(sub)
	h.advance(48 * time.Hour)
	sub = h.s.topic.subscribe()
	defer h.s.topic.unsubscribe(sub)
	if n := h.calls.Load(); n != 0 {
		t.Fatalf("fetched %d times with no configured region", n)
	}
}

func TestCalendarConfigureLoadsWithoutSubscriber(t *testing.T) {
	h := newCalendarHarness(t)
	h.s.configure("US", []int{2026})
	h.settle(t)
	if n := h.calls.Load(); n != 1 {
		t.Fatalf("explicit configure fetched %d times, want 1", n)
	}
}

func TestCalendarFirstSubscriberRefreshesOnlyWhenStale(t *testing.T) {
	h := newCalendarHarness(t)
	h.s.configure("US", []int{2026})
	h.settle(t)

	h.advance(time.Hour)
	sub := h.s.topic.subscribe()
	h.settle(t)
	h.s.topic.unsubscribe(sub)
	if n := h.calls.Load(); n != 1 {
		t.Fatalf("fresh load refetched: %d calls", n)
	}

	h.advance(calendarRefreshAge)
	sub = h.s.topic.subscribe()
	defer h.s.topic.unsubscribe(sub)
	h.settle(t)
	if n := h.calls.Load(); n != 2 {
		t.Fatalf("stale load on first subscriber: %d calls, want 2", n)
	}
}
