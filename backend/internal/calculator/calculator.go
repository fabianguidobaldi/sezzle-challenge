// Package calculator implements exact decimal arithmetic for the calculator API.
//
// The package is pure domain logic: it has no knowledge of HTTP or JSON. Numbers
// cross its boundary as decimal strings, so callers never depend on the
// underlying decimal library.
package calculator

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Operation identifies a calculator operation.
type Operation string

// Supported operations.
const (
	Add       Operation = "add"
	Subtract  Operation = "subtract"
	Multiply  Operation = "multiply"
	Divide    Operation = "divide"
	Power     Operation = "power"
	Sqrt      Operation = "sqrt"
	Percent   Operation = "percent"    // x / 100
	PercentOf Operation = "percent_of" // base × rate / 100
)

// Result is the outcome of a successful calculation.
type Result struct {
	Value string // canonical decimal string; see formatResult
	Exact bool   // false if the value may have been rounded
}

// Precision is the number of significant digits for operands and results.
// Results that fit are exact; anything longer is rounded half-even.
const Precision = 16

// Exponent range for operands and results; values outside it are out of range.
const (
	minExponent = -9999
	maxExponent = 9999
)

// newContext returns the arithmetic context shared by every operation.
func newContext() apd.Context {
	return apd.Context{
		Precision:   Precision,
		Rounding:    apd.RoundHalfEven,
		MinExponent: minExponent,
		MaxExponent: maxExponent,
		Traps:       apd.Overflow | apd.Underflow | apd.DivisionByZero | apd.DivisionUndefined | apd.InvalidOperation,
	}
}

// Calculator evaluates operations on decimal strings. It holds no mutable
// state after construction, so a single Calculator is safe for concurrent
// use: every call allocates its own decimals and only reads the context.
type Calculator struct {
	ctx apd.Context
}

// New returns a Calculator using the shared arithmetic context.
func New() *Calculator {
	return &Calculator{ctx: newContext()}
}

// operation describes how many operands an operation takes and how to
// compute it. apply returns the apd condition so Calculate can derive both
// the domain error and the Exact flag.
type operation struct {
	arity int
	apply func(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error)
}

// registry maps every supported operation to its implementation.
// Adding an operation means adding one function and one entry here.
var registry = map[Operation]operation{
	Add:       {2, add},
	Subtract:  {2, subtract},
	Multiply:  {2, multiply},
	Divide:    {2, divide},
	Power:     {2, power},
	Sqrt:      {1, sqrt},
	Percent:   {1, percent},
	PercentOf: {2, percentOf},
}

// Calculate validates the operation and operands, computes the result and
// formats it. Errors wrap one of the sentinels in errors.go, or are an
// *OperandError for a rejected operand.
func (c *Calculator) Calculate(op Operation, operands []string) (Result, error) {
	o, ok := registry[op]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownOperation, op)
	}
	if len(operands) != o.arity {
		return Result{}, fmt.Errorf("%w: %s expects %d %s, got %d",
			ErrOperandCount, op, o.arity, plural(o.arity, "operand"), len(operands))
	}

	args := make([]*apd.Decimal, len(operands))
	for i, s := range operands {
		d, err := parseOperand(i, s)
		if err != nil {
			return Result{}, err
		}
		args[i] = d
	}

	d, cond, err := o.apply(c, args)
	if err := domainError(op, d, cond, err); err != nil {
		return Result{}, err
	}
	return Result{Value: formatResult(d), Exact: !cond.Inexact()}, nil
}

// domainError maps the outcome of an apd computation to a domain error, or
// nil if d is a usable result. Errors already in domain terms (from checks
// done before calling apd) pass through unchanged.
func domainError(op Operation, d *apd.Decimal, cond apd.Condition, err error) error {
	switch {
	case cond.DivisionByZero() || cond.DivisionUndefined():
		return fmt.Errorf("%w: %s", ErrDivisionByZero, op)
	case cond.InvalidOperation():
		return fmt.Errorf("%w: %s", ErrUndefined, op)
	case cond.Overflow() || cond.Underflow() || cond.Subnormal():
		// Subnormal results are below the exponent range even when exact
		// (apd only raises Underflow if they were also rounded); they would
		// not be accepted back as operands, so they are out of range too.
		return fmt.Errorf("%w: %s", ErrOutOfRange, op)
	case err != nil:
		if isDomainError(err) {
			return err
		}
		return fmt.Errorf("calculator: %s: %w", op, err)
	case d.Form != apd.Finite:
		// Safety net: apd can return Infinity or NaN without raising a condition.
		return fmt.Errorf("%w: %s", ErrUndefined, op)
	}
	return nil
}

// isDomainError reports whether err already wraps one of the package's sentinels.
func isDomainError(err error) bool {
	for _, target := range []error{ErrDivisionByZero, ErrUndefined, ErrOutOfRange} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// plural returns word with an "s" appended unless n is 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
