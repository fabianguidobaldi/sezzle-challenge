package calculator

import (
	"errors"
	"fmt"
)

// Errors returned by Calculate. Callers match them with errors.Is; the
// returned errors wrap these with a more specific message.
var (
	ErrUnknownOperation = errors.New("unknown operation")
	ErrOperandCount     = errors.New("wrong number of operands")
	ErrInvalidNumber    = errors.New("invalid number")
	ErrDivisionByZero   = errors.New("division by zero")
	ErrUndefined        = errors.New("result is undefined")
	ErrOutOfRange       = errors.New("result out of range")
)

// errTooLarge is ErrOutOfRange for results that don't fit in Precision
// integer digits.
var errTooLarge = fmt.Errorf("%w: results must be less than 10^%d in magnitude", ErrOutOfRange, Precision)

// OperandError reports an operand that is not an acceptable decimal number.
// It wraps ErrInvalidNumber.
type OperandError struct {
	Index  int    // zero-based position in the operands list
	Value  string // the rejected input
	Reason string
}

func (e *OperandError) Error() string {
	return fmt.Sprintf("operands[%d]: %s", e.Index, e.Reason)
}

func (e *OperandError) Unwrap() error { return ErrInvalidNumber }
