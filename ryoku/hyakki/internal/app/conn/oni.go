package conn

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

// WatchOni opens a connection of its own, opens the oni event on it as an
// observer and hands every frame to frame, the frames to start from first,
// until ctx ends, which closes the connection, or the daemon drops it. The
// app's own connection never opens the event: a connection that watches
// may not answer a guide, a keep offer or a picture. frame runs on the
// watch's goroutine.
func (c *Client) WatchOni(ctx context.Context, frame func(json.RawMessage)) error {
	options := c.options
	options.Start = false
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	connection, err := client.Dial(dialCtx, options)
	cancel()
	if err != nil {
		return err
	}
	defer connection.Close()
	var opened protocol.OniOpenResult
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = connection.Call(openCtx, protocol.MOniOpen, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver}, &opened)
	cancel()
	if err != nil {
		return err
	}
	for _, raw := range opened.Frames {
		frame(raw)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-connection.Events():
			if !ok {
				if err := connection.Err(); err != nil && !errors.Is(err, io.EOF) && ctx.Err() == nil {
					return err
				}
				return nil
			}
			if event.Event == protocol.EvtOni {
				frame(event.Data)
			}
		}
	}
}
