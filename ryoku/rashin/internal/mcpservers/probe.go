package mcpservers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"syscall"

	"ryoku-rashin/internal/protocol"
)

var errSignIn = errors.New("MCP server requires sign-in")

type rpcReply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type exchangeRPC func(int, string, any) (json.RawMessage, error)

func handshake(call exchangeRPC) (int, error) {
	if _, err := call(1, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "Rashin", "version": "2"}}); err != nil {
		return 0, err
	}
	if _, err := call(0, "notifications/initialized", map[string]any{}); err != nil {
		return 0, err
	}
	count, cursor := 0, ""
	for page := range 100 {
		params := map[string]string{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := call(2+page, "tools/list", params)
		if err != nil {
			return 0, err
		}
		var result struct {
			Tools []json.RawMessage `json:"tools"`
			Next  string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return 0, err
		}
		count += len(result.Tools)
		if result.Next == "" {
			return count, nil
		}
		if result.Next == cursor {
			return 0, errors.New("repeated MCP cursor")
		}
		cursor = result.Next
	}
	return 0, errors.New("too many MCP pages")
}

func rpcBody(id int, method string, params any) []byte {
	message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		message["id"] = id
	}
	raw, _ := json.Marshal(message)
	return append(raw, '\n')
}

func readReply(scanner *bufio.Scanner, id int, sse bool) (json.RawMessage, error) {
	for scanner.Scan() {
		line := scanner.Bytes()
		if sse {
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		}
		var reply rpcReply
		if json.Unmarshal(line, &reply) != nil || reply.ID != id {
			continue
		}
		if len(reply.Error) > 0 && string(reply.Error) != "null" {
			return nil, errors.New("MCP server returned an error")
		}
		return reply.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.ErrUnexpectedEOF
}

func probe(ctx context.Context, info protocol.MCPServerInfo, headers map[string]string) (int, error) {
	if info.Type == "stdio" {
		cmd := exec.CommandContext(ctx, info.Command, info.Args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process != nil {
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil
		}
		input, err := cmd.StdinPipe()
		if err != nil {
			return 0, err
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			return 0, err
		}
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		defer func() { _ = cmd.Cancel(); _ = cmd.Wait() }()
		scanner := bufio.NewScanner(io.LimitReader(output, 8<<20))
		scanner.Buffer(make([]byte, 4096), 1<<20)
		return handshake(func(id int, method string, params any) (json.RawMessage, error) {
			if _, err := input.Write(rpcBody(id, method, params)); err != nil {
				return nil, err
			}
			if id == 0 {
				return nil, nil
			}
			return readReply(scanner, id, false)
		})
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var stream *http.Response
	var scanner *bufio.Scanner
	endpoint, session := info.URL, ""
	version := "2025-03-26"
	request := func(method, address string, body []byte) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", version)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		return client.Do(req)
	}
	if info.Type == "sse" {
		var err error
		stream, err = request(http.MethodGet, info.URL, nil)
		if err != nil {
			return 0, err
		}
		defer stream.Body.Close()
		if stream.StatusCode == http.StatusUnauthorized || stream.StatusCode == http.StatusForbidden {
			return 0, errSignIn
		}
		if stream.StatusCode != http.StatusOK {
			return 0, errors.New("MCP stream refused")
		}
		scanner = bufio.NewScanner(io.LimitReader(stream.Body, 8<<20))
		scanner.Buffer(make([]byte, 4096), 1<<20)
		event, found := "", false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event:") {
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			}
			if event == "endpoint" && strings.HasPrefix(line, "data:") {
				base, _ := url.Parse(info.URL)
				next, err := url.Parse(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				if err != nil {
					return 0, err
				}
				target := base.ResolveReference(next)
				if target.Scheme != base.Scheme || target.Host != base.Host || target.User != nil {
					return 0, errors.New("MCP endpoint changed origin")
				}
				endpoint, found = target.String(), true
				break
			}
		}
		if !found {
			return 0, errors.New("MCP endpoint missing")
		}
	}
	return handshake(func(id int, method string, params any) (json.RawMessage, error) {
		resp, err := request(http.MethodPost, endpoint, rpcBody(id, method, params))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, errSignIn
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.New("MCP request refused")
		}
		if value := resp.Header.Get("Mcp-Session-Id"); value != "" {
			session = value
		}
		if id == 0 {
			return nil, nil
		}
		var raw json.RawMessage
		if scanner != nil {
			raw, err = readReply(scanner, id, true)
		} else if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			var reply rpcReply
			if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&reply); err != nil {
				return nil, err
			}
			if reply.ID != id || (len(reply.Error) > 0 && string(reply.Error) != "null") {
				return nil, errors.New("MCP response failed")
			}
			raw = reply.Result
		} else {
			input := bufio.NewScanner(io.LimitReader(resp.Body, 8<<20))
			input.Buffer(make([]byte, 4096), 1<<20)
			raw, err = readReply(input, id, true)
		}
		if err != nil {
			return nil, err
		}
		if method == "initialize" {
			var initialized struct {
				Version string `json:"protocolVersion"`
			}
			if json.Unmarshal(raw, &initialized) != nil || initialized.Version == "" {
				return nil, errors.New("MCP version missing")
			}
			version = initialized.Version
		}
		return raw, nil
	})
}
