package mcpservers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"syscall"

	"ryoku-rashin/internal/protocol"
)

const rpcLimit = 8 << 20

// client owns one short-lived MCP connection. A stdio server exits after the
// list or call; no server is left running while an agent is idle.
type client struct {
	ctx                        context.Context
	info                       protocol.MCPServerInfo
	headers                    map[string]string
	command                    *exec.Cmd
	input                      io.WriteCloser
	output                     *bufio.Scanner
	stream                     io.ReadCloser
	endpoint, session, version string
	http                       *http.Client
	next                       int
}

func openClient(ctx context.Context, info protocol.MCPServerInfo, headers map[string]string) (_ *client, err error) {
	c := &client{ctx: ctx, info: info, headers: headers, endpoint: info.URL, version: "2025-03-26", http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if info.Type == "stdio" {
		cmd := exec.CommandContext(ctx, info.Command, info.Args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process != nil {
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil
		}
		if c.input, err = cmd.StdinPipe(); err != nil {
			return nil, err
		}
		stdout, pipeErr := cmd.StdoutPipe()
		if pipeErr != nil {
			c.input.Close()
			return nil, pipeErr
		}
		if err = cmd.Start(); err != nil {
			c.input.Close()
			return nil, err
		}
		c.command = cmd
		c.output = scanner(stdout)
	} else if info.Type == "sse" {
		response, requestErr := c.request(http.MethodGet, info.URL, nil)
		if requestErr != nil {
			return nil, requestErr
		}
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			response.Body.Close()
			return nil, errSignIn
		}
		if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
			response.Body.Close()
			return nil, errors.New("MCP stream refused")
		}
		c.stream = response.Body
		c.output = scanner(io.LimitReader(response.Body, rpcLimit))
		if c.endpoint, err = readEndpoint(c.output, info.URL); err != nil {
			c.Close()
			return nil, err
		}
	} else if info.Type != "http" {
		return nil, errors.New("unsupported MCP transport")
	}
	if err = c.initialize(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func scanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1<<20)
	return s
}

func readEndpoint(s *bufio.Scanner, base string) (string, error) {
	event := ""
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if event == "endpoint" && strings.HasPrefix(line, "data:") {
			origin, _ := url.Parse(base)
			next, err := url.Parse(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			if err != nil {
				return "", err
			}
			target := origin.ResolveReference(next)
			if target.Scheme != origin.Scheme || target.Host != origin.Host || target.User != nil || target.Fragment != "" {
				return "", errors.New("MCP endpoint changed origin")
			}
			return target.String(), nil
		}
		if line == "" {
			event = ""
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", errors.New("MCP endpoint missing")
}

func (c *client) initialize() error {
	raw, err := c.call("initialize", map[string]any{"protocolVersion": c.version, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "Hyakki", "version": "2"}})
	if err != nil {
		return err
	}
	var initialized struct {
		Version string `json:"protocolVersion"`
	}
	if json.Unmarshal(raw, &initialized) != nil || initialized.Version == "" {
		return errors.New("MCP version missing")
	}
	c.version = initialized.Version
	_, err = c.call("notifications/initialized", map[string]any{})
	return err
}

func (c *client) list() ([]remoteTool, error) {
	tools := make([]remoteTool, 0)
	cursor := ""
	for range 100 {
		params := map[string]string{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.call("tools/list", params)
		if err != nil {
			return nil, err
		}
		var result struct {
			Tools []remoteTool `json:"tools"`
			Next  string       `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		tools = append(tools, result.Tools...)
		if result.Next == "" {
			return tools, nil
		}
		if result.Next == cursor {
			return nil, errors.New("repeated MCP cursor")
		}
		cursor = result.Next
	}
	return nil, errors.New("too many MCP pages")
}

func (c *client) call(method string, params any) (json.RawMessage, error) {
	id := 0
	if method != "notifications/initialized" {
		c.next++
		id = c.next
	}
	body := rpcBody(id, method, params)
	if c.command != nil {
		if _, err := c.input.Write(body); err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, nil
		}
		return readReply(c.output, id, false)
	}
	response, err := c.request(http.MethodPost, c.endpoint, body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, errSignIn
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP request refused (%d)", response.StatusCode)
	}
	if session := response.Header.Get("Mcp-Session-Id"); session != "" {
		c.session = session
	}
	if id == 0 {
		return nil, nil
	}
	if c.stream != nil {
		return readReply(c.output, id, true)
	}
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return readReply(scanner(io.LimitReader(response.Body, rpcLimit)), id, true)
	}
	var reply rpcReply
	if err := json.NewDecoder(io.LimitReader(response.Body, rpcLimit)).Decode(&reply); err != nil {
		return nil, err
	}
	return reply.result(id)
}

func (c *client) request(method, address string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.ctx, method, address, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for name, value := range c.headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", c.version)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	return c.http.Do(req)
}

func (c *client) Close() {
	if c.stream != nil {
		c.stream.Close()
	}
	if c.input != nil {
		c.input.Close()
	}
	if c.command != nil {
		if c.command.Process != nil {
			_ = syscall.Kill(-c.command.Process.Pid, syscall.SIGKILL)
		}
		_ = c.command.Wait()
	}
	c.http.CloseIdleConnections()
}
