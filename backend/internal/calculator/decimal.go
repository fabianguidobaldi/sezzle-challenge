package calculator

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/apd/v3"
)

// maxOperandLength bounds the raw input size before any parsing work is done.
const maxOperandLength = 64

// parseOperand converts the operand at the given index into a decimal.
// Operands must already be in the number format (see Precision); they are
// rejected, never rounded. Every error is an *OperandError.
func parseOperand(index int, s string) (*apd.Decimal, error) {
	reject := func(reason string) error {
		return &OperandError{Index: index, Value: s, Reason: reason}
	}

	switch {
	case s == "":
		return nil, reject("must not be empty")
	case len(s) > maxOperandLength:
		return nil, reject(fmt.Sprintf("must be at most %d characters", maxOperandLength))
	case strings.ContainsAny(s, "eE"):
		return nil, reject("must be a plain decimal number, without an exponent")
	}

	d, _, err := apd.NewFromString(s)
	if err != nil {
		return nil, reject("is not a valid decimal number")
	}
	if d.Form != apd.Finite {
		return nil, reject("must be a finite number")
	}

	// Trailing zeros are not significant: "1.000" has one digit.
	d.Reduce(d)
	switch {
	case d.NumDigits() > Precision:
		return nil, reject(fmt.Sprintf("must have at most %d significant digits", Precision))
	case d.Exponent < -MaxDecimalPlaces:
		return nil, reject(fmt.Sprintf("must have at most %d decimal places", MaxDecimalPlaces))
	case adjustedExponent(d) >= Precision:
		return nil, reject(fmt.Sprintf("must have at most %d integer digits", Precision))
	}
	return d, nil
}

// roundResult rounds d in place to the number format: at most Precision
// significant digits and MaxDecimalPlaces decimal places, so values too small
// for the format become 0. It returns an ErrOutOfRange error if the rounded
// value has more than Precision integer digits.
func roundResult(ctx *apd.Context, d *apd.Decimal) (apd.Condition, error) {
	if d.IsZero() {
		return 0, nil
	}
	var cond apd.Condition
	exp := max(adjustedExponent(d)-(Precision-1), -MaxDecimalPlaces)
	if int64(d.Exponent) < exp {
		var err error
		if cond, err = ctx.Quantize(d, d, int32(exp)); err != nil {
			return cond, err
		}
	}
	// Rounding can carry into a new digit (9999999999999999.5 → 1E+16).
	if !d.IsZero() && adjustedExponent(d) >= Precision {
		return cond, errTooLarge
	}
	return cond, nil
}

// formatResult renders a value in the number format as a canonical plain
// decimal string: trailing zeros removed and no negative zero.
func formatResult(d *apd.Decimal) string {
	if d.IsZero() {
		return "0"
	}
	var r apd.Decimal
	r.Reduce(d)
	return r.Text('f')
}

// adjustedExponent is the exponent of d's most significant digit
// (e.g. 3 for 1234, -2 for 0.0123).
func adjustedExponent(d *apd.Decimal) int64 {
	return int64(d.Exponent) + d.NumDigits() - 1
}
