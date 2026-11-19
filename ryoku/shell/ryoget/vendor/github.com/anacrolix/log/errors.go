package log

import (
	"errors"
	"log/slog"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/generics/option"
)

type errorWithLevel struct {
	Level Level
	error
}

func (me errorWithLevel) Unwrap() error {
	return me.error
}

// Extracts the most recent error level added to err with [WithLevel], or NotSet.
func ErrorLevel(err error) Level {
	var withLevel errorWithLevel
	if errors.As(err, &withLevel) {
		return withLevel.Level
	}
	return option.Map(fromSlogLevel, justSlogErrorLevel(err)).UnwrapOr(NotSet)
}

// Extracts the most recent error level added to err with [WithLevel], or NotSet.
func justAnalogErrorLevel(err error) Level {
	var withLevel errorWithLevel
	if !errors.As(err, &withLevel) {
		return NotSet
	}
	return withLevel.Level
}

// Adds the error level to err, it can be extracted with [ErrorLevel].
func WithLevel(level Level, err error) error {
	return errorWithLevel{level, err}
}

type errorWithSlogLevel struct {
	Level slog.Level
	error
}

func WithSlogLevel(level slog.Level, err error) error {
	return errorWithSlogLevel{level, err}
}

// Extracts the most recent error level added to err with [WithLevel], or NotSet.
func SlogErrorLevel(err error) (ret g.Option[slog.Level]) {
	return justSlogErrorLevel(err).Or(g.OptionFromTuple(toSlogLevel(justAnalogErrorLevel(err))))
}

// Extracts the most recent error level added to err with [WithLevel], or NotSet.
func justSlogErrorLevel(err error) (ret g.Option[slog.Level]) {
	var withLevel errorWithSlogLevel
	if errors.As(err, &withLevel) {
		ret.Set(withLevel.Level)
	}
	return
}
