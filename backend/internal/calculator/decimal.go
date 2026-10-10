package calculator

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// maxOperandLength bounds the raw input size before any parsing work is done.
const maxOperandLength = 64

// Plain notation is used for adjusted exponents in [plainMinExponent, Precision);
// anything outside uses scientific notation.
const plainMinExponent = -7

// parseOperand converts the operand at the given index into a decimal.
// Operands are rejected, never rounded: anything that doesn't fit the
// calculator's precision or exponent range is an *OperandError.
func parseOperand(index int, s string) (*apd.Decimal, error) {
	reject := func(reason string) error {
		return &OperandError{Index: index, Value: s, Reason: reason}
	}

	switch {
	case s == "":
		return nil, reject("must not be empty")
	case len(s) > maxOperandLength:
		return nil, reject(fmt.Sprintf("must be at most %d characters", maxOperandLength))
	}

	d, _, err := apd.NewFromString(s)
	if err != nil {
		return nil, reject("is not a valid decimal number")
	}
	if d.Form != apd.Finite {
		return nil, reject("must be a finite number")
	}

	// Trailing zeros are not significant: "1.000" and "1E+20" have one digit.
	d.Reduce(d)
	if d.NumDigits() > Precision {
		return nil, reject(fmt.Sprintf("must have at most %d significant digits", Precision))
	}
	if adj := adjustedExponent(d); adj < minExponent || adj > maxExponent {
		return nil, reject(fmt.Sprintf("exponent must be between %d and %d", minExponent, maxExponent))
	}
	return d, nil
}

// formatResult renders d as a canonical decimal string: trailing zeros
// removed, no negative zero, plain notation for everyday magnitudes and
// scientific notation otherwise. Integers beyond Precision digits are never
// written out in full, since the padding zeros would imply false precision.
func formatResult(d *apd.Decimal) string {
	if d.IsZero() {
		return "0"
	}
	var r apd.Decimal
	r.Reduce(d)
	if adj := adjustedExponent(&r); adj >= plainMinExponent && adj < Precision {
		return r.Text('f')
	}
	return r.String()
}

// adjustedExponent is the exponent of d's most significant digit
// (e.g. 3 for 1234, -2 for 0.0123).
func adjustedExponent(d *apd.Decimal) int64 {
	return int64(d.Exponent) + d.NumDigits() - 1
}
