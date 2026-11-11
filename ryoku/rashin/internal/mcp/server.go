// Package mcp serves tools over the Model Context Protocol on stdio, one
// JSON-RPC message per line: the transport every ACP agent accepts for the
// servers its client names in session/new.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"sync"
)

// Versions are the protocol revisions this server speaks, newest first.
var Versions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Tool is one callable tool and its JSON schema.
type Tool struct {
	Name        string
	Title       string
	Description string
	Input       map[string]any
	// ReadOnly tells the client the tool changes nothing.
	ReadOnly bool
	Call     func(ctx context.Context, arguments json.RawMessage) Result
}

// Content is one block of a tool result.
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// Result is what a tool call returns; a failed call is a result with
// IsError set, so the model reads why.
type Result struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Text is a result of one text block.
func Text(text string) Result { return Result{Content: []Content{{Type: "text", Text: text}}} }

// Failure is a failed result that says why.
func Failure(err error) Result {
	return Result{Content: []Content{{Type: "text", Text: err.Error()}}, IsError: true}
}

// Server answers one client until its input ends.
type Server struct {
	Name         string
	Version      string
	Instructions string
	Tools        []Tool
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type reply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *replyError     `json:"error,omitempty"`
}

type replyError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParse    = -32700
	codeMethod   = -32601
	codeParams   = -32602
	codeInternal = -32603
)

// Serve reads requests from in and writes replies to out. Tool calls run
// side by side, a cancelled request stops its call, and the end of in
// cancels whatever still runs.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		writeMu sync.Mutex
		running sync.WaitGroup
		callsMu sync.Mutex
		calls   = map[string]context.CancelFunc{}
	)
	encoder := json.NewEncoder(out)
	send := func(value reply) {
		value.JSONRPC = "2.0"
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = encoder.Encode(value)
	}
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			s.dispatch(ctx, line, send, &running, &callsMu, calls)
		}
		if err != nil {
			cancel()
			running.Wait()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (s *Server) dispatch(ctx context.Context, line []byte, send func(reply), running *sync.WaitGroup, callsMu *sync.Mutex, calls map[string]context.CancelFunc) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var request message
	if err := json.Unmarshal(line, &request); err != nil {
		send(reply{ID: json.RawMessage("null"), Error: &replyError{Code: codeParse, Message: "parse error"}})
		return
	}
	notification := len(request.ID) == 0
	switch request.Method {
	case "notifications/cancelled":
		var params struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		if json.Unmarshal(request.Params, &params) == nil {
			callsMu.Lock()
			if stop := calls[string(params.RequestID)]; stop != nil {
				stop()
			}
			callsMu.Unlock()
		}
		return
	case "":
		return
	}
	if notification {
		return
	}
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(request.Params, &params)
		version := Versions[0]
		if slices.Contains(Versions, params.ProtocolVersion) {
			version = params.ProtocolVersion
		}
		send(reply{ID: request.ID, Result: map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.Instructions,
		}})
	case "ping":
		send(reply{ID: request.ID, Result: map[string]any{}})
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, tool := range s.Tools {
			entry := map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": tool.Input}
			if tool.Title != "" {
				entry["title"] = tool.Title
			}
			if tool.ReadOnly {
				entry["annotations"] = map[string]any{"readOnlyHint": true}
			}
			tools = append(tools, entry)
		}
		send(reply{ID: request.ID, Result: map[string]any{"tools": tools}})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal(request.Params, &params) != nil {
			send(reply{ID: request.ID, Error: &replyError{Code: codeParams, Message: "invalid params"}})
			return
		}
		index := slices.IndexFunc(s.Tools, func(tool Tool) bool { return tool.Name == params.Name })
		if index < 0 {
			send(reply{ID: request.ID, Error: &replyError{Code: codeParams, Message: "unknown tool: " + params.Name}})
			return
		}
		if len(params.Arguments) == 0 || string(params.Arguments) == "null" {
			params.Arguments = json.RawMessage("{}")
		}
		callCtx, stop := context.WithCancel(ctx)
		key := string(request.ID)
		callsMu.Lock()
		calls[key] = stop
		callsMu.Unlock()
		running.Add(1)
		go func() {
			defer running.Done()
			defer func() {
				callsMu.Lock()
				delete(calls, key)
				callsMu.Unlock()
				stop()
			}()
			result, panicked := safeCall(callCtx, s.Tools[index], params.Arguments)
			if panicked != nil {
				send(reply{ID: request.ID, Error: &replyError{Code: codeInternal, Message: panicked.Error()}})
				return
			}
			if callCtx.Err() != nil && ctx.Err() == nil {
				return
			}
			send(reply{ID: request.ID, Result: result})
		}()
	default:
		send(reply{ID: request.ID, Error: &replyError{Code: codeMethod, Message: "method not found: " + request.Method}})
	}
}

func safeCall(ctx context.Context, tool Tool, arguments json.RawMessage) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("tool failed")
		}
	}()
	result = tool.Call(ctx, arguments)
	if result.Content == nil {
		result.Content = []Content{}
	}
	return result, nil
}
