package calculator

import (
	"errors"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func TestParseOperandAccepts(t *testing.T) {
	tests := []struct {
		in   string
		want string // canonical form of the parsed value
	}{
		{"42", "42"},
		{"-0.5", "-0.5"},
		{"+5", "5"},
		{".5", "0.5"},
		{"5.", "5"},
		{"0", "0"},
		{"-0", "0"},
		{"1.000", "1"},
		{"1E+20", "1E+20"},
		{"1.5E+40", "1.5E+40"},
		{"1e-3", "0.001"},
		{"100000000000000000000", "1E+20"},       // 21 chars but only 1 significant digit
		{"1234567890123456", "1234567890123456"}, // exactly Precision digits
		{"0.000000000000000000001234567890123456", "1.234567890123456E-21"},
		{"1E+9999", "1E+9999"},
		{"1E-9999", "1E-9999"},
		{"9.999999999999999E+9999", "9.999999999999999E+9999"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			d, err := parseOperand(0, tt.in)
			if err != nil {
				t.Fatalf("parseOperand(%q) error: %v", tt.in, err)
			}
			if got := formatResult(d); got != tt.want {
				t.Errorf("parseOperand(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseOperandRejects(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		reason string // substring expected in the error
	}{
		{"empty", "", "empty"},
		{"too long", strings.Repeat("0", maxOperandLength+1), "at most 64 characters"},
		{"letters", "abc", "not a valid decimal"},
		{"dangling exponent", "1e", "not a valid decimal"},
		{"leading space", " 1", "not a valid decimal"},
		{"trailing space", "1 ", "not a valid decimal"},
		{"underscore", "1_000", "not a valid decimal"},
		{"comma", "1,5", "not a valid decimal"},
		{"two points", "1.2.3", "not a valid decimal"},
		{"hex", "0x10", "not a valid decimal"},
		{"NaN", "NaN", "finite"},
		{"sNaN", "sNaN", "finite"},
		{"Infinity", "Infinity", "finite"},
		{"negative Infinity", "-Infinity", "finite"},
		{"too many digits", "12345678901234567", "16 significant digits"},
		{"too many decimals", "0.12345678901234567", "16 significant digits"},
		{"exponent too large", "1E+10000", "exponent"},
		{"exponent too small", "1E-10000", "exponent"},
		{"adjusted exponent too large", "12E+9999", "exponent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOperand(1, tt.in)
			if err == nil {
				t.Fatalf("parseOperand(%q) succeeded, want error", tt.in)
			}
			if !errors.Is(err, ErrInvalidNumber) {
				t.Errorf("error %v does not wrap ErrInvalidNumber", err)
			}
			var opErr *OperandError
			if !errors.As(err, &opErr) {
				t.Fatalf("error %v is not an *OperandError", err)
			}
			if opErr.Index != 1 || opErr.Value != tt.in {
				t.Errorf("OperandError = {Index: %d, Value: %q}, want {1, %q}", opErr.Index, opErr.Value, tt.in)
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("error %q does not mention %q", err, tt.reason)
			}
		})
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"3.00", "3"},
		{"0.2500", "0.25"},
		{"1000.000000", "1000"},
		{"1E+3", "1000"},
		{"-0", "0"},
		{"0E-10", "0"},
		{"-0.00", "0"},
		{"-12.50", "-12.5"},
		{"0.0000001", "0.0000001"}, // adjusted exponent -7: still plain
		{"0.00000001", "1E-8"},     // -8: scientific
		{"0.000000015", "1.5E-8"},
		{"1234567890123456", "1234567890123456"},       // 16 integer digits: plain
		{"12345678901234560", "1.234567890123456E+16"}, // 17: scientific
		{"1844674407370955E+4", "1.844674407370955E+19"},
		{"-1.5E+45", "-1.5E+45"},
		{"0.3333333333333333", "0.3333333333333333"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			d, _, err := apd.NewFromString(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got := formatResult(d); got != tt.want {
				t.Errorf("formatResult(%s) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// Any formatted result must be accepted back as an operand with the same
// value, so the frontend can always chain calculations.
func TestFormattedResultsRoundTrip(t *testing.T) {
	ctx := newContext()
	n := func(s string) *apd.Decimal {
		d, _, err := apd.NewFromString(s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	type op func(d *apd.Decimal) (apd.Condition, error)
	cases := map[string]op{
		"1/3":       func(d *apd.Decimal) (apd.Condition, error) { return ctx.Quo(d, n("1"), n("3")) },
		"-2/3":      func(d *apd.Decimal) (apd.Condition, error) { return ctx.Quo(d, n("-2"), n("3")) },
		"sqrt 2":    func(d *apd.Decimal) (apd.Condition, error) { return ctx.Sqrt(d, n("2")) },
		"2^64":      func(d *apd.Decimal) (apd.Condition, error) { return ctx.Pow(d, n("2"), n("64")) },
		"tiny/7":    func(d *apd.Decimal) (apd.Condition, error) { return ctx.Quo(d, n("1E-9990"), n("7")) },
		"huge*9":    func(d *apd.Decimal) (apd.Condition, error) { return ctx.Mul(d, n("9.999999999999999E+9998"), n("9")) },
		"small/3":   func(d *apd.Decimal) (apd.Condition, error) { return ctx.Quo(d, n("-0.0000001"), n("3")) },
		"0.1+0.2":   func(d *apd.Decimal) (apd.Condition, error) { return ctx.Add(d, n("0.1"), n("0.2")) },
		"big-small": func(d *apd.Decimal) (apd.Condition, error) { return ctx.Sub(d, n("1E+15"), n("0.1")) },
	}
	for name, compute := range cases {
		t.Run(name, func(t *testing.T) {
			var d apd.Decimal
			if _, err := compute(&d); err != nil {
				t.Fatal(err)
			}
			s := formatResult(&d)
			back, err := parseOperand(0, s)
			if err != nil {
				t.Fatalf("formatted result %q rejected as operand: %v", s, err)
			}
			if back.Cmp(&d) != 0 {
				t.Errorf("round trip changed value: %s -> %q -> %s", d.String(), s, back.String())
			}
		})
	}
}

func TestOperandErrorMessage(t *testing.T) {
	err := &OperandError{Index: 2, Value: "x", Reason: "is not a valid decimal number"}
	if got, want := err.Error(), "operands[2]: is not a valid decimal number"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
