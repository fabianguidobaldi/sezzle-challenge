package calculator

import "github.com/cockroachdb/apd/v3"

// apd computes x^y with a fractional y through exp and ln, which always
// flags the result as inexact, even for 4^0.5 = 2. exactPower recovers the
// cases that are in fact exact by checking the rounded result with exact
// integer arithmetic.

// Limits that keep the check to a few thousand digits of integer arithmetic.
// Beyond them, results simply keep apd's conservative inexact flag.
const (
	maxExactDenominator = 100  // y = num/den with den ≤ 100: covers .5, .25, .2, .125, ...
	maxExactDigits      = 2000 // digits of x^|num|
)

// exactCtx multiplies without rounding for any value within the limits above:
// r^den has at most Precision × maxExactDenominator digits, x^|num| at most
// maxExactDigits, and their product at most the sum.
var exactCtx = apd.Context{
	Precision:   Precision*maxExactDenominator + maxExactDigits,
	MaxExponent: apd.MaxExponent,
	MinExponent: apd.MinExponent,
}

// exactPower reports whether x^y, which apd computed as d and flagged
// inexact, is exactly representable in the number format. If so, it returns
// that value. With y = num/den in lowest terms and r = d rounded to the
// format, x^y = r exactly if and only if r^den = x^num, which only needs
// integer powers of finite decimals and can be checked without rounding.
func exactPower(ctx *apd.Context, x, y, d *apd.Decimal) (*apd.Decimal, bool) {
	num, den, ok := smallFraction(y)
	if !ok || x.NumDigits()*abs(num) > maxExactDigits {
		return nil, false
	}

	r := new(apd.Decimal).Set(d)
	if _, err := roundResult(ctx, r); err != nil || r.IsZero() {
		return nil, false
	}

	var lhs, rhs apd.Decimal
	if !exactly(exactCtx.Pow(&lhs, r, apd.New(den, 0))) || !exactly(exactCtx.Pow(&rhs, x, apd.New(abs(num), 0))) {
		return nil, false
	}
	if num < 0 {
		// r^den = 1/x^|num|  ⇔  r^den × x^|num| = 1
		if !exactly(exactCtx.Mul(&lhs, &lhs, &rhs)) {
			return nil, false
		}
		return r, lhs.Cmp(one) == 0
	}
	return r, lhs.Cmp(&rhs) == 0
}

// smallFraction writes a non-integer y as num/den in lowest terms. It fails
// for integers (apd already reports their powers' exactness correctly) and
// for denominators above maxExactDenominator.
func smallFraction(y *apd.Decimal) (num, den int64, ok bool) {
	var r apd.Decimal
	r.Reduce(y)
	if r.Exponent >= 0 {
		return 0, 0, false
	}
	// Operands have at most 16 significant digits and 16 decimal places, so
	// both the coefficient and 10^places fit in an int64.
	coeff := r
	coeff.Exponent = 0
	num, err := coeff.Int64()
	if err != nil {
		return 0, 0, false
	}
	den = 1
	for range -r.Exponent {
		den *= 10
	}
	g := gcd(abs(num), den)
	num, den = num/g, den/g
	return num, den, den <= maxExactDenominator
}

// exactly reports whether an exactCtx operation succeeded without rounding.
func exactly(cond apd.Condition, err error) bool {
	return err == nil && !cond.Inexact()
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
