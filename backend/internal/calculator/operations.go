package calculator

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Constants used by the operations. They are never modified.
var (
	one     = apd.New(1, 0)
	hundred = apd.New(100, 0)
)

// binary adapts a two-operand apd.Context method to an operation.
func binary(f func(ctx *apd.Context, d, x, y *apd.Decimal) (apd.Condition, error)) func(*Calculator, []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	return func(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
		d := new(apd.Decimal)
		cond, err := f(&c.ctx, d, args[0], args[1])
		return d, cond, err
	}
}

var (
	add      = binary((*apd.Context).Add)
	subtract = binary((*apd.Context).Sub)
	multiply = binary((*apd.Context).Mul)
	divide   = binary((*apd.Context).Quo)
)

// power computes x^y. A zero base with a negative exponent is rejected before
// calling apd, which answers it with Infinity and no condition. Large
// exponents need no limit: apd's cost grows with log(y).
//
// Results beyond the working exponent range come back as overflow or
// underflow, but apd doesn't always pick the right one (0.5^-1E+15 is
// reported as underflow), so the math decides: if the true result tends to 0
// it rounds to 0 like any other tiny result; otherwise it is out of range.
func power(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	x, y := args[0], args[1]
	if x.IsZero() && y.Negative && !y.IsZero() {
		return nil, 0, fmt.Errorf("%w: zero raised to a negative power", ErrDivisionByZero)
	}
	d := new(apd.Decimal)
	cond, err := c.ctx.Pow(d, x, y)
	if cond.Overflow() || cond.Underflow() {
		if tendsToZero(x, y) {
			return new(apd.Decimal), apd.Inexact | apd.Rounded, nil
		}
		return nil, 0, fmt.Errorf("%w: power", ErrOutOfRange)
	}
	return d, cond, err
}

// tendsToZero reports whether |x^y| < 1, i.e. |x| < 1 with y > 0 or |x| > 1
// with y < 0.
func tendsToZero(x, y *apd.Decimal) bool {
	var absX apd.Decimal
	absX.Abs(x)
	cmp := absX.Cmp(one)
	return (cmp < 0 && y.Sign() > 0) || (cmp > 0 && y.Sign() < 0)
}

// sqrt computes √x. Negative inputs are rejected up front with a clearer
// message than apd's invalid operation.
func sqrt(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	x := args[0]
	if x.Negative && !x.IsZero() {
		return nil, 0, fmt.Errorf("%w: square root of a negative number", ErrUndefined)
	}
	d := new(apd.Decimal)
	cond, err := c.ctx.Sqrt(d, x)
	return d, cond, err
}

// percent computes x / 100.
func percent(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	d := new(apd.Decimal)
	cond, err := c.ctx.Quo(d, args[0], hundred)
	return d, cond, err
}

// percentOf computes base × rate / 100 ("rate% of base"). Dividing the rate
// first is exact (it only shifts the decimal point), so the product is the
// only step that can round. The conditions of both steps are combined.
func percentOf(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	rate, cond, err := percent(c, args[1:])
	if err != nil {
		return rate, cond, err
	}
	d := new(apd.Decimal)
	mulCond, err := c.ctx.Mul(d, args[0], rate)
	return d, cond | mulCond, err
}
