package log

import (
	"fmt"
	"log/slog"
	"slices"
)

// Returns a new Logger with the names given, and Default's handlers. I'm not sure copying those
// handlers is the right choice yet, but it's better than having your messages vanish if you forget
// to configure them.
func NewLogger(names ...string) Logger {
	// Should this include a default log level? (Not the same as filter level.)
	return loggerCore{
		nonZero:  true,
		Handlers: slices.Clone(Default.Handlers),
	}.WithNames(names...).asLogger()
}

// Logger handles logging in a specific context. It includes a bunch of helpers and compatibility
// over the loggerCore.
type Logger struct {
	loggerCore
}

func (l Logger) WithText(f func(Msg) string) Logger {
	l.msgMaps = append(l.msgMaps, func(msg Msg) Msg {
		return msg.WithText(f)
	})
	return l
}

// Helper for compatibility with "log".Logger.
func (l Logger) Printf(format string, a ...interface{}) {
	l.LazyLog(l.defaultLevel, func() Msg {
		return Fmsg(format, a...).Skip(1)
	})
}

func (l Logger) Log(m Msg) {
	l.LogLevel(l.defaultLevel, m.Skip(1))
}

func (l Logger) LogLevel(level Level, m Msg) {
	l.LazyLog(level, func() Msg {
		return m.Skip(1)
	})
}

// Helper for compatibility with "log".Logger.
func (l Logger) Print(v ...interface{}) {
	l.LazyLog(l.defaultLevel, func() Msg {
		return Str(fmt.Sprint(v...)).Skip(1)
	})
}

func (l Logger) WithContextValue(v interface{}) Logger {
	return l.WithText(func(m Msg) string {
		return fmt.Sprintf("%v: %v", v, m)
	})
}

func (l Logger) WithContextText(s string) Logger {
	return l.WithText(func(m Msg) string {
		return s + ": " + m.Text()
	})
}

func (l Logger) SkipCallers(skip int) Logger {
	return l.WithMap(func(m Msg) Msg {
		return m.Skip(skip)
	})
}

func (l Logger) Levelf(level Level, format string, a ...interface{}) {
	l.LazyLog(level, func() Msg {
		return Fmsg(format, a...).Skip(1)
	})
}

// Efficiently print arguments at the given level.
func (l Logger) LevelPrint(level Level, a ...interface{}) {
	l.LazyLog(level, func() Msg {
		return Str(fmt.Sprint(a...)).Skip(1)
	})
}

func (l Logger) Println(a ...interface{}) {
	l.LazyLogDefaultLevel(func() Msg {
		return Msgln(a...).Skip(1)
	})
}

// Well this allocates every time. And slog.Logger is just a dumb wrapper for slog.Handler. So you
// probably want to use SlogHandler and set something up yourself or this function should store the
// slog.Logger it makes for you.
func (l Logger) Slogger() *slog.Logger {
	return slog.New(slogHandler{l: l})
}

// Logger and slog.Handler are very similar. Logger has attrs much like slog.Handler.
func (l Logger) SlogHandler() slog.Handler {
	return slogHandler{l: l}
}
