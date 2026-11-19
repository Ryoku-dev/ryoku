package mcpservers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"ryoku-rashin/internal/protocol"
)

var errSignIn = errors.New("MCP server requires sign-in")

type rpcReply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (r rpcReply) result(id int) (json.RawMessage, error) {
	if r.ID != id {
		return nil, errors.New("MCP response ID mismatch")
	}
	if len(r.Error) > 0 && string(r.Error) != "null" {
		var detail struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(r.Error, &detail) == nil && detail.Message != "" {
			return nil, errors.New(detail.Message)
		}
		return nil, errors.New("MCP server returned an error")
	}
	if len(r.Result) == 0 {
		return nil, errors.New("MCP response missing result")
	}
	return r.Result, nil
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
		return reply.result(id)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.ErrUnexpectedEOF
}

func probe(ctx context.Context, info protocol.MCPServerInfo, headers map[string]string) ([]remoteTool, error) {
	client, err := openClient(ctx, info, headers)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.list()
}
