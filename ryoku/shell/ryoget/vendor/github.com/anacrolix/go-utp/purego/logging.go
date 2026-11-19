package purego

import "log/slog"

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

// The name a category goes out under in a log record.
func (me logLevel) String() string {
	switch me {
	case logNormal:
		return "normal"
	case logMTU:
		return "mtu"
	case logDebug:
		return "debug"
	}
	return "unknown"
}
