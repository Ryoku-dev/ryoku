package conn

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

// privateSocket returns a socket path in a 0700 directory, which the client
// requires before it dials.
func privateSocket(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "rashind.sock")
}

func TestReconnectAttachesFromModelLastSeq(t *testing.T) {
	socket := privateSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	info := protocol.SessionInfo{ID: "ask-resume", Kind: protocol.KindAsk, Title: "Resume", Created: time.Now(), Updated: time.Now()}
	store := state.New(nil)
	store.ApplyAttach(0, protocol.SessionAttachResult{Info: info, Events: []protocol.SessionEvent{{Seq: 17, Kind: protocol.EvTitle, Data: mustJSON(t, protocol.TitleData{Title: "Resume"})}}})

	attached := make(chan int64, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		decoder := protocol.NewDecoder(connection)
		encoder := protocol.NewEncoder(connection)
		for {
			frame, readErr := decoder.Next()
			if readErr != nil {
				return
			}
			var result any
			switch frame.Method {
			case protocol.MHello:
				result = protocol.HelloResult{Daemon: "test", Version: "test", Protocol: protocol.Version, Enabled: true}
			case protocol.MSubscribe:
				result = protocol.Snapshot{Sessions: []protocol.SessionInfo{info}}
			case protocol.MSessionAttach:
				var params protocol.SessionAttachParams
				if json.Unmarshal(frame.Params, &params) != nil {
					return
				}
				attached <- params.Since
				result = protocol.SessionAttachResult{Info: info}
			default:
				result = protocol.Empty{}
			}
			payload, marshalErr := json.Marshal(result)
			if marshalErr != nil || encoder.Encode(protocol.Response{ID: *frame.ID, Result: payload}) != nil {
				return
			}
		}
	}()

	connection := NewOptions(store, client.Options{Socket: socket, Client: protocol.ClientApp})
	select {
	case since := <-attached:
		if since != 17 {
			t.Fatalf("session.attach since = %d, want 17", since)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("session.attach was not sent")
	}
	connection.Close()
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("test server did not stop")
	}
}

func TestProjectWatchRefcountAndReconnectResend(t *testing.T) {
	socket := privateSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	watches := make(chan protocol.WatchParams, 8)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for connectionIndex := 0; connectionIndex < 2; connectionIndex++ {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			decoder := protocol.NewDecoder(connection)
			encoder := protocol.NewEncoder(connection)
			for {
				frame, readErr := decoder.Next()
				if readErr != nil {
					_ = connection.Close()
					break
				}
				var result any = protocol.Empty{}
				switch frame.Method {
				case protocol.MHello:
					result = protocol.HelloResult{Daemon: "test", Version: "test", Protocol: protocol.Version, Enabled: true}
				case protocol.MSubscribe:
					result = protocol.Snapshot{}
				case protocol.MProjectsWatch:
					var params protocol.WatchParams
					if json.Unmarshal(frame.Params, &params) != nil {
						return
					}
					watches <- params
				}
				payload, marshalErr := json.Marshal(result)
				if marshalErr != nil || encoder.Encode(protocol.Response{ID: *frame.ID, Result: payload}) != nil {
					_ = connection.Close()
					break
				}
			}
		}
	}()
	store := state.New(nil)
	connection := NewOptions(store, client.Options{Socket: socket, Client: protocol.ClientApp})
	defer connection.Close()
	deadline := time.Now().Add(2 * time.Second)
	for store.Snapshot().Connection != state.Ready && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	connection.WatchProject("hyperion")
	connection.WatchProject("hyperion")
	first := waitWatch(t, watches)
	if first.Project != "hyperion" || !first.Watch {
		t.Fatalf("first watch = %#v", first)
	}
	connection.UnwatchProject("hyperion")
	select {
	case unexpected := <-watches:
		t.Fatalf("refcount release sent %#v", unexpected)
	case <-time.After(40 * time.Millisecond):
	}
	connection.Reconnect()
	replayed := waitWatch(t, watches)
	if replayed.Project != "hyperion" || !replayed.Watch {
		t.Fatalf("replayed watch = %#v", replayed)
	}
	connection.UnwatchProject("hyperion")
	last := waitWatch(t, watches)
	if last.Project != "hyperion" || last.Watch {
		t.Fatalf("last watch = %#v", last)
	}
}

func waitWatch(t *testing.T, watches <-chan protocol.WatchParams) protocol.WatchParams {
	t.Helper()
	select {
	case params := <-watches:
		return params
	case <-time.After(3 * time.Second):
		t.Fatal("projects.watch was not sent")
		return protocol.WatchParams{}
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
