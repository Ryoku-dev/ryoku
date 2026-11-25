package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/mcpserve"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "mcp", Group: "tools", Summary: "Serve Oni's tools to an MCP client over stdin and stdout; off until turned on and the client approved in Settings.", Run: runMCP})
}

// runMCP serves one MCP client on stdin and stdout until stdin ends. The
// log on stderr holds codes only.
func runMCP(args []string) int {
	if len(args) != 0 {
		return usageError("hyakki mcp")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveMCP(ctx, os.Stdin, os.Stdout, os.Stderr, func(ctx context.Context, name string) (mcpserve.Daemon, error) {
		return client.Dial(ctx, client.Options{Client: protocol.ClientMCP, Name: name, Start: true})
	})
}

func serveMCP(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, dial mcpserve.Dial) int {
	err := mcpserve.Serve(ctx, stdin, stdout, mcpserve.Options{
		Dial:    dial,
		Version: version,
		Log:     func(code string) { fmt.Fprintln(stderr, "hyakki mcp: "+code) },
	})
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(stderr, i18n.Tf("hyakki mcp: %v", err))
		return 1
	}
	return 0
}
