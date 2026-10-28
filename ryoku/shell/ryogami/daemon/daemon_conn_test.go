package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The picker opens and closes on events that share its request connection. A request
// still running (here a settings watcher that has not finished) must not hold them back.
func TestEventsFlowWhileRequestRuns(t *testing.T) {
	d := newSettingsDaemon(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	saved := settingWatchers
	t.Cleanup(func() {
		unblock()
		settingWatchers = saved
	})
	watchSetting("selector.livePreview", func(*daemon, string, interface{}) {
		close(entered)
		<-release
	})

	server, client := net.Pipe()
	t.Cleanup(func() { client.Close() })
	go func() {
		defer server.Close()
		d.handle(server)
	}()
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(client)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	send := func(line string) {
		t.Helper()
		if _, err := client.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	next := func(what string) string {
		t.Helper()
		select {
		case l := <-lines:
			return l
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
			return ""
		}
	}

	send(`{"id":1,"method":"subscribe"}`)
	if l := next("the subscribe reply"); !strings.Contains(l, `"subscribed":true`) {
		t.Fatalf("subscribe reply = %s", l)
	}
	send(`{"id":2,"method":"settings.set","params":{"values":{"selector.livePreview":false}}}`)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the settings watcher never ran")
	}

	d.broadcast("ryogami.wall.toggle", map[string]interface{}{})
	for {
		l := next("the toggle event")
		if strings.Contains(l, "ryogami.wall.toggle") {
			break
		}
		if strings.Contains(l, `"id":2`) {
			t.Fatalf("settings.set answered before its watcher finished: %s", l)
		}
	}

	unblock()
	for {
		if l := next("the settings.set reply"); strings.Contains(l, `"id":2`) {
			break
		}
	}
}

func TestSlowSourceSearchDoesNotBlockStatusOnSameConnection(t *testing.T) {
	d, sources := newTestSources(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	sources.searchHook = func(ctx context.Context, _ string, _ map[string]interface{}) (sourcePage, error) {
		close(entered)
		select {
		case <-release:
			return sourcePage{}, nil
		case <-ctx.Done():
			return sourcePage{}, ctx.Err()
		}
	}

	server, client := net.Pipe()
	t.Cleanup(func() {
		unblock()
		_ = client.Close()
	})
	go func() {
		defer server.Close()
		d.handle(server)
	}()
	lines := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(client)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	if _, err := client.Write([]byte("{\"id\":1,\"method\":\"source.search\",\"params\":{\"source\":\"wallhaven\",\"generation\":1}}\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("source.search did not start")
	}
	if _, err := client.Write([]byte("{\"id\":2,\"method\":\"status\"}\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":2`) {
			t.Fatalf("slow search replied before status: %s", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("status was blocked behind source.search")
	}
	unblock()
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":1`) {
			t.Fatalf("search reply = %s", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("source.search did not reply")
	}
}

func TestSupersedingSourceSearchCancelsOlderGeneration(t *testing.T) {
	_, sources := newTestSources(t)
	const connectionID = 41
	sources.openSearchConnection(connectionID)
	t.Cleanup(func() { sources.dropSearchConnection(connectionID) })
	firstStarted := make(chan struct{})
	firstCancelled := make(chan struct{})
	sources.searchHook = func(ctx context.Context, _ string, p map[string]interface{}) (sourcePage, error) {
		if intParam(p, "generation", 0) == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstCancelled)
			return sourcePage{}, ctx.Err()
		}
		return sourcePage{results: []sourceResult{{id: "generation-2"}}}, nil
	}

	older := make(chan response, 1)
	go func() {
		older <- sources.search(1, connectionID, map[string]interface{}{"source": "wallhaven", "generation": float64(1)})
	}()
	select {
	case <-firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("older search did not start")
	}
	newer := sources.search(2, connectionID, map[string]interface{}{"source": "wallhaven", "generation": float64(2)})
	if newer.Error != nil {
		t.Fatalf("newer search failed: %+v", newer.Error)
	}
	result, ok := newer.Result.(map[string]interface{})
	if !ok || result["generation"] != int64(2) {
		t.Fatalf("newer search result = %#v", newer.Result)
	}
	items, ok := result["results"].([]sourceItem)
	if !ok || len(items) != 1 || items[0].ID != "generation-2" {
		t.Fatalf("newer search items = %#v", result["results"])
	}
	select {
	case <-firstCancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("older search context was not cancelled")
	}
	select {
	case resp := <-older:
		if resp.Error == nil || resp.Error.Message != "search superseded" {
			t.Fatalf("older search reply = %+v", resp)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("older search did not return after cancellation")
	}
}

func TestSearchGenerationsAreScopedPerConnectionAndEmitFreshThumbs(t *testing.T) {
	d, sources := newTestSources(t)
	const thumbURL = "https://w.wallhaven.cc/full/test.jpg"
	sources.searchHook = func(_ context.Context, _ string, p map[string]interface{}) (sourcePage, error) {
		id := "old"
		if intParam(p, "generation", 0) == 1 {
			id = "fresh"
		}
		dest := sources.remoteThumbPath("wallhaven", id, thumbURL)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return sourcePage{}, err
		}
		if err := os.WriteFile(dest, []byte("cached"), 0o644); err != nil {
			return sourcePage{}, err
		}
		return sourcePage{results: []sourceResult{{id: id, thumbURL: thumbURL}}}, nil
	}
	events := d.events.subscribe()
	defer d.events.unsubscribe(events)
	waitThumb := func(want string) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case line := <-events:
				var ev capturedEvent
				if json.Unmarshal([]byte(line), &ev) != nil || ev.Event != evRemoteThumb {
					continue
				}
				var data map[string]interface{}
				if json.Unmarshal(ev.Data, &data) == nil && data["id"] == want {
					return
				}
			case <-deadline:
				t.Fatalf("thumbnail %q was not emitted", want)
			}
		}
	}

	const firstConnection = 71
	sources.openSearchConnection(firstConnection)
	if resp := sources.search(1, firstConnection, map[string]interface{}{"source": "wallhaven", "generation": float64(7)}); resp.Error != nil {
		t.Fatalf("first connection generation 7 failed: %+v", resp.Error)
	}
	waitThumb("old")
	sources.dropSearchConnection(firstConnection)

	const freshConnection = 72
	sources.openSearchConnection(freshConnection)
	defer sources.dropSearchConnection(freshConnection)
	if resp := sources.search(2, freshConnection, map[string]interface{}{"source": "wallhaven", "generation": float64(1)}); resp.Error != nil {
		t.Fatalf("fresh connection generation 1 failed: %+v", resp.Error)
	}
	waitThumb("fresh")
}

func TestClosingConnectionClearsSearchRegistry(t *testing.T) {
	d, sources := newTestSources(t)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	sources.searchHook = func(ctx context.Context, _ string, _ map[string]interface{}) (sourcePage, error) {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		return sourcePage{}, ctx.Err()
	}

	server, client := net.Pipe()
	go func() {
		defer server.Close()
		d.handle(server)
	}()
	if _, err := client.Write([]byte("{\"id\":1,\"method\":\"source.search\",\"params\":{\"source\":\"wallhaven\",\"generation\":5}}\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("search did not start")
	}
	_ = client.Close()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("closing the connection did not cancel its search")
	}

	sources.searchMu.Lock()
	defer sources.searchMu.Unlock()
	if len(sources.searchConnections) != 0 || len(sources.searches) != 0 || len(sources.searchLatest) != 0 || len(sources.thumbLatest) != 0 {
		t.Fatalf("connection search state leaked: active=%v searches=%v latest=%v thumbs=%v",
			sources.searchConnections, sources.searches, sources.searchLatest, sources.thumbLatest)
	}
}

func TestPickerOpenDuringLaunchIsDeliveredAfterSubscribe(t *testing.T) {
	d := &daemon{
		events: newEventHub(),
		ui: &managedProcess{
			cmd: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}},
		},
	}
	if d.ui.ensure() {
		t.Fatal("picker without a subscriber reported ready")
	}

	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	go func() {
		defer server.Close()
		d.handle(server)
	}()
	lines := make(chan string, 4)
	go func() {
		scanner := bufio.NewScanner(client)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	if _, err := client.Write([]byte("{\"id\":1,\"method\":\"subscribe\"}\n")); err != nil {
		t.Fatal(err)
	}
	next := func(what string) string {
		t.Helper()
		select {
		case line := <-lines:
			return line
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
			return ""
		}
	}
	if reply := next("subscribe reply"); !strings.Contains(reply, `"subscribed":true`) {
		t.Fatalf("subscribe reply = %s", reply)
	}
	var ev capturedEvent
	if line := next("retained picker open"); json.Unmarshal([]byte(line), &ev) != nil || ev.Event != "ryogami.wall.toggle" {
		t.Fatalf("retained event = %s", line)
	}
	if !d.ui.ensure() {
		t.Fatal("subscribed picker did not report ready")
	}
}

func TestPickerToggleSurvivesFullEventQueue(t *testing.T) {
	d := &daemon{events: newEventHub()}
	events := d.events.subscribe()
	d.events.setControl(events)
	defer d.events.unsubscribe(events)

	for i := range cap(events) {
		d.broadcast("ryogami.wall.cached", map[string]interface{}{"index": i})
	}
	d.broadcast("ryogami.wall.toggle", map[string]interface{}{})

	found := false
	for range cap(events) {
		var ev capturedEvent
		if line := <-events; json.Unmarshal([]byte(line), &ev) != nil {
			t.Fatalf("invalid event: %s", line)
		} else if ev.Event == "ryogami.wall.toggle" {
			found = true
		}
	}
	if !found {
		t.Fatal("picker toggle was dropped behind a full subscriber queue")
	}
}
