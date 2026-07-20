package quanta

import (
	"errors"
	"fmt"
)

// Sentinel errors returned (usually wrapped) by this package. Match them with
// [errors.Is] rather than by comparing error strings.
var (
	// ErrDimensionMismatch is returned when an operation requires two
	// quantities to share a dimension and they do not — for example adding a
	// length to a time, or converting metres to seconds.
	ErrDimensionMismatch = errors.New("quanta: dimension mismatch")

	// ErrAffineUnit is returned when an affine unit (one with a non-zero zero
	// point, such as °C or °F) is used where only a linear unit is meaningful:
	// multiplication, division, exponentiation, or as part of a compound unit
	// expression.
	ErrAffineUnit = errors.New("quanta: affine unit not permitted here")

	// ErrUnknownUnit is returned when a symbol cannot be resolved by a
	// registry, either directly or as a prefixed form of a known unit.
	ErrUnknownUnit = errors.New("quanta: unknown unit")

	// ErrDuplicateUnit is returned when a definition or alias would overwrite
	// a symbol that is already registered. Registration never silently
	// replaces an existing unit.
	ErrDuplicateUnit = errors.New("quanta: duplicate unit symbol")

	// ErrSyntax is returned for malformed unit or quantity expressions. Such
	// errors are always also a [*ParseError], which carries the offending
	// input and the byte offset at which the problem was detected.
	ErrSyntax = errors.New("quanta: syntax error")

	// ErrExponentRange is returned when an operation would drive a dimension
	// exponent outside the representable int8 range [-128, 127].
	ErrExponentRange = errors.New("quanta: dimension exponent out of range")

	// ErrInvalidUnit is returned when a unit definition is malformed: a nil or
	// zero conversion factor, or an affine unit used as the base of another
	// definition.
	ErrInvalidUnit = errors.New("quanta: invalid unit definition")
)

// ParseError describes a failure to parse a unit or quantity expression. It
// records the full input and the byte offset at which the parser gave up, and
// wraps a sentinel error ([ErrSyntax], [ErrUnknownUnit], [ErrAffineUnit] or
// [ErrExponentRange]) so that [errors.Is] works as expected.
type ParseError struct {
	// Input is the complete string that was being parsed.
	Input string
	// Offset is the byte offset within Input at which the error was detected.
	// It is len(Input) when the input ended unexpectedly.
	Offset int
	// Msg is a short human-readable description of the problem.
	Msg string
	// Err is the wrapped sentinel error.
	Err error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("quanta: parse %q at offset %d: %s", e.Input, e.Offset, e.Msg)
}

// Unwrap returns the wrapped sentinel error so that [errors.Is] can match it.
func (e *ParseError) Unwrap() error { return e.Err }

func parseErr(input string, offset int, sentinel error, format string, args ...any) *ParseError {
	return &ParseError{
		Input:  input,
		Offset: offset,
		Msg:    fmt.Sprintf(format, args...),
		Err:    sentinel,
	}
}
