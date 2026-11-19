package utp

import "log/slog"

const (
	logCallbacks = false
	utpLogging   = false
)

// Logger is the logger a Socket is given when [NewSocket] isn't passed [WithLogger]. When it's
// nil, which is the default, a Socket uses slog.Default() as it stands when the Socket is created.
var Logger *slog.Logger

// The logger for a Socket created without one of its own.
func defaultLogger() *slog.Logger {
	if Logger != nil {
		return Logger
	}
	return slog.Default()
}
