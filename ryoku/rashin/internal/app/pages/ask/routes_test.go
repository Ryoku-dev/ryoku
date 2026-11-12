package ask

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

type askCalls struct {
	listener net.Listener
	mu       sync.Mutex
	methods  []string
	models   []string
}

func (server *askCalls) serve() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer connection.Close()
			decoder, encoder := protocol.NewDecoder(connection), protocol.NewEncoder(connection)
			for {
				frame, err := decoder.Next()
				if err != nil {
					return
				}
				if frame.ID == nil {
					continue
				}
				server.mu.Lock()
				server.methods = append(server.methods, frame.Method)
				server.mu.Unlock()
				var result any = protocol.Empty{}
				switch frame.Method {
				case protocol.MHello:
					result = protocol.HelloResult{Daemon: "test", Version: "test", Protocol: protocol.Version, Enabled: true}
				case protocol.MSubscribe:
					result = protocol.Snapshot{}
				case protocol.MSessionNew:
					result = protocol.SessionInfo{ID: "ask-new", Kind: protocol.KindAsk}
				case protocol.MSessionAttach:
					result = protocol.SessionAttachResult{Info: protocol.SessionInfo{ID: "ask-new", Kind: protocol.KindAsk}}
				case protocol.MSessionSetModel:
					var params protocol.SetModelParams
					_ = json.Unmarshal(frame.Params, &params)
					server.mu.Lock()
					server.models = append(server.models, params.Model)
					server.mu.Unlock()
				case protocol.MSessionPrompt:
					result = protocol.PromptResult{}
				}
				payload, _ := json.Marshal(result)
				if encoder.Encode(protocol.Response{ID: *frame.ID, Result: payload}) != nil {
					return
				}
			}
		}()
	}
}

// the order the daemon heard the session's calls in, once the question was asked
func askedWith(t *testing.T, route string) ([]string, []string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "rashind.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &askCalls{listener: listener}
	go server.serve()
	defer listener.Close()

	store := state.New(nil)
	connection := conn.NewOptions(store, client.Options{Socket: socket, Client: protocol.ClientApp})
	defer connection.Close()
	posts := make(chan func(), 64)
	e := &env.Env{
		Store: store, Client: connection, Overlays: new(look.Overlays),
		Post: func(fn func()) { posts <- fn }, Navigate: func(string, string) {}, Notify: func(string) {},
	}
	deadline := time.Now().Add(3 * time.Second)
	for !connection.Connected() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	page := New()
	page.draftRoute = route
	page.start(e, chat.Submission{Text: "What is using my disk?"})
	for time.Now().Before(deadline) {
		select {
		case fn := <-posts:
			fn()
		case <-time.After(5 * time.Millisecond):
		}
		server.mu.Lock()
		asked := slices.Contains(server.methods, protocol.MSessionPrompt)
		methods, models := slices.Clone(server.methods), slices.Clone(server.models)
		server.mu.Unlock()
		if asked {
			return methods, models
		}
	}
	t.Fatal("the question never reached the daemon")
	return nil, nil
}

func TestAPickedModelIsHeldToBeforeTheFirstQuestion(t *testing.T) {
	methods, models := askedWith(t, "openai/gpt")
	set, asked := slices.Index(methods, protocol.MSessionSetModel), slices.Index(methods, protocol.MSessionPrompt)
	if set < 0 || set > asked || !slices.Equal(models, []string{"openai/gpt"}) {
		t.Fatalf("the daemon heard %v with models %v: the pick must be set before the question so it never fails over", methods, models)
	}
	methods, _ = askedWith(t, protocol.RouteAuto)
	if slices.Contains(methods, protocol.MSessionSetModel) {
		t.Fatalf("Auto was pinned: %v", methods)
	}
}
