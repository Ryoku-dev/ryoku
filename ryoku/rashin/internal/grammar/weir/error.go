package weir

import "fmt"

type ErrorKind uint8

const (
	UnsupportedToken ErrorKind = iota
	EndOfInput
	UnmatchedBrace
	ExpectedComma
	UnexpectedToken
	ExpectedVariableUndefined
	InvalidLintKind
	InvalidReplacementStrategy
	InvalidScope
	ExpectedDifferentVariableType
	UnableToResolveExpr
)

type Error struct {
	Kind  ErrorKind
	Value string
}

func (e Error) Error() string {
	switch e.Kind {
	case UnsupportedToken:
		return "encountered a token that is unsupported by the parser"
	case EndOfInput:
		return "reached the end of the input token stream prematurely"
	case UnmatchedBrace:
		return "unmatched brace"
	case ExpectedComma:
		return "expected a comma here"
	case UnexpectedToken:
		return fmt.Sprintf("expected a valid keyword, got: %s", e.Value)
	case ExpectedVariableUndefined:
		return "expected a value to be defined"
	case InvalidLintKind:
		return "invalid lint kind"
	case InvalidReplacementStrategy:
		return "invalid replacement strategy"
	case InvalidScope:
		return "invalid scope"
	case ExpectedDifferentVariableType:
		return "expected a variable type other than the one provided"
	case UnableToResolveExpr:
		return "unable to resolve expression reference " + e.Value
	}
	return "invalid Weir error"
}

var (
	ErrEndOfInput     = Error{Kind: EndOfInput}
	ErrUnmatchedBrace = Error{Kind: UnmatchedBrace}
	ErrExpectedComma  = Error{Kind: ExpectedComma}
)
