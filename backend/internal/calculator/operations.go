package calculator

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// hundred is the divisor for percentages. It is never modified.
var hundred = apd.New(100, 0)

// maxPowerExponent bounds |y| in x^y. Larger exponents would over- or
// underflow for any base other than ±1 or 0, and cost CPU to find out.
var maxPowerExponent = apd.New(maxExponent, 0)

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

// power computes x^y. Two cases are rejected before calling apd: a zero base
// with a negative exponent, which apd answers with Infinity and no condition,
// and exponents beyond maxPowerExponent, which would only waste CPU.
func power(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	x, y := args[0], args[1]
	if x.IsZero() && y.Negative && !y.IsZero() {
		return nil, 0, fmt.Errorf("%w: zero raised to a negative power", ErrDivisionByZero)
	}
	var absY apd.Decimal
	absY.Abs(y)
	if absY.Cmp(maxPowerExponent) > 0 {
		return nil, 0, fmt.Errorf("%w: exponent magnitude must be at most %d", ErrOutOfRange, maxExponent)
	}
	d := new(apd.Decimal)
	cond, err := c.ctx.Pow(d, x, y)
	return d, cond, err
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

// percentOf computes base × rate / 100 ("rate% of base"). The conditions of
// both steps are combined, so the result is inexact if either step rounded.
//
// The product is computed with two extra digits of exponent headroom: dividing
// by 100 lowers the exponent by 2, so a product just above the range may still
// give an in-range result. The headroom lives in a local copy of the context.
func percentOf(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error) {
	wide := c.ctx
	wide.MaxExponent += 2
	d := new(apd.Decimal)
	cond, err := wide.Mul(d, args[0], args[1])
	if err != nil {
		return d, cond, err
	}
	cond2, err := c.ctx.Quo(d, d, hundred)
	return d, cond | cond2, err
}
