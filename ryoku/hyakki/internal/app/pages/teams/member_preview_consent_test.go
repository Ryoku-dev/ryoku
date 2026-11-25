package teams

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

func TestHistoricalPreviewCannotReuseEarlierConsent(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(directory, "s"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	requested := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	restored := make(chan protocol.MemberConfigRestoreParams, 1)
	serverErrors := make(chan error, 1)
	go func() {
		socket, err := listener.Accept()
		if err != nil {
			return
		}
		defer socket.Close()
		decoder, encoder := protocol.NewDecoder(socket), protocol.NewEncoder(socket)
		for {
			frame, err := decoder.Next()
			if err != nil {
				return
			}
			if frame.ID == nil {
				continue
			}
			var result any = protocol.Empty{}
			switch frame.Method {
			case protocol.MHello:
				result = protocol.HelloResult{Daemon: "private-consent", Version: "test", Protocol: protocol.Version, Enabled: true}
			case protocol.MSubscribe:
				result = protocol.Snapshot{}
			case protocol.MTeamsMemberConfigGet:
				requested <- struct{}{}
				<-release
				result = protocol.MemberConfigDocument{YAML: "revision B", Revision: 8, CurrentRevision: 9}
			case protocol.MTeamsMemberConfigRestore:
				var params protocol.MemberConfigRestoreParams
				if err := json.Unmarshal(frame.Params, &params); err != nil {
					serverErrors <- err
					return
				}
				restored <- params
				result = protocol.MemberConfigDocument{YAML: "restored B", Revision: 10, CurrentRevision: 10}
			case protocol.MTeamsMemberConfigHistory:
				result = protocol.MemberConfigHistoryResult{RetentionLimit: 32}
			}
			raw, err := json.Marshal(result)
			if err != nil {
				serverErrors <- err
				return
			}
			if err := encoder.Encode(protocol.Response{ID: *frame.ID, Result: raw}); err != nil {
				return
			}
		}
	}()
	store := state.New(nil)
	connection := conn.NewOptions(store, client.Options{Socket: listener.Addr().String(), Client: protocol.ClientApp})
	t.Cleanup(connection.Close)
	deadline := time.Now().Add(5 * time.Second)
	for store.Snapshot().Connection != state.Ready {
		if time.Now().After(deadline) {
			t.Fatal("controlled connection did not subscribe")
		}
		time.Sleep(time.Millisecond)
	}
	posted := make(chan func(), 8)
	view := &env.Env{Store: store, Client: connection, Post: func(f func()) { posted <- f }}
	var editor memberConfigEditor
	editor.reset("team", "member")
	editor.current = protocol.MemberConfigDocument{YAML: "current", Revision: 9, CurrentRevision: 9}
	editor.editor.SetText("current")
	editor.preview = protocol.MemberConfigDocument{YAML: "revision A", Revision: 7, CurrentRevision: 9}
	editor.review.Value = true
	editor.show(view, 8)
	select {
	case <-requested:
	case <-time.After(5 * time.Second):
		t.Fatal("replacement preview was not requested")
	}
	// A click can already have been queued while the old preview was visible.
	editor.review.Value = true
	editor.commit(view, true)
	if editor.busy {
		t.Fatal("an in-flight preview allowed an old review to start restore")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case apply := <-posted:
		apply()
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("replacement preview did not publish")
	}
	if editor.preview.Revision != 8 || editor.review.Value {
		t.Fatal("replacement preview inherited approval of the earlier revision")
	}
	editor.review.Value = true
	editor.commit(view, true)
	select {
	case params := <-restored:
		if params.Revision != 8 || params.ExpectedRevision != 9 {
			t.Fatalf("reviewed restore targeted a different configuration: %+v", params)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("freshly reviewed configuration did not restore")
	}
	select {
	case apply := <-posted:
		apply()
	case <-time.After(5 * time.Second):
		t.Fatal("reviewed restore did not publish")
	}
	if editor.current.Revision != 10 || editor.review.Value {
		t.Fatal("completed restore retained an old approval or failed to advance current configuration")
	}
}
