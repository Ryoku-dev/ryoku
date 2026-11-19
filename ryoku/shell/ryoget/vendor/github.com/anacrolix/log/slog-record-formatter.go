package log

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
)

var (
	gstbhLocker                 sync.Mutex
	globalSlogTextBufferHandler = slogTextBufferHandler{}
)

func init() {
	globalSlogTextBufferHandler.init()
}

func (me *slogTextBufferHandler) handleAppend(b []byte, r slog.Record) []byte {
	me.buf.Reset()
	err := me.handler.Handle(context.Background(), r)
	if err != nil {
		panic(err)
	}
	return append(b, me.buf.Bytes()...)
}

type slogTextBufferHandler struct {
	buf     bytes.Buffer
	handler *slog.TextHandler
}

func (me *slogTextBufferHandler) init() {
	me.handler = slog.NewTextHandler(&me.buf, &slog.HandlerOptions{
		AddSource: false,
		Level:     toSlogMinLevel(NotSet),
		ReplaceAttr: func(groups []string, a slog.Attr) (ret slog.Attr) {
			if len(groups) == 0 {
				switch a.Key {
				case slog.TimeKey, slog.LevelKey:
					return
				}
			}
			ret = a
			return
		},
	})
}
