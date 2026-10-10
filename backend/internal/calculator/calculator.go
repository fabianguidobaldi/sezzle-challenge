// Package calculator implements exact decimal arithmetic for the calculator API.
//
// The package is pure domain logic: it has no knowledge of HTTP or JSON. Numbers
// cross its boundary as decimal strings, so callers never depend on the
// underlying decimal library.
package calculator

import "github.com/cockroachdb/apd/v3"

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
