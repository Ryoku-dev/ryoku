package log

import (
	"io"
	"log/slog"
)

// Deprecated: Use SlogHandlerAsHandler instead.
type JsonHandler struct {
	// This is used to output JSON as it provides a more modern way and probably more efficient way
	// to modify log records. You can alter this in place after initing JsonHandler and before
	// logging to it. I don't know why SlogHandlerAsHandler wasn't just used.
	SlogHandler slog.Handler
}

// Deprecated: Use SlogHandlerAsHandler instead.
func NewJsonHandler(w io.Writer, minLevel Level) *JsonHandler {
	return &JsonHandler{
		SlogHandler: slog.NewJSONHandler(w, &slog.HandlerOptions{
			AddSource:   false,
			Level:       toSlogMinLevel(minLevel),
			ReplaceAttr: nil,
		}),
	}
}

var _ Handler = (*JsonHandler)(nil)

func (me *JsonHandler) Handle(r Record) {
	SlogHandlerAsHandler{me.SlogHandler}.Handle(r)
}
