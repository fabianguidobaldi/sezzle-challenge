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
		{"0.00000000000000000000", "0"},
		{"000000000000000000000000001", "1"},
		{"9999999999999999", "9999999999999999"}, // 16 integer digits
		{"0.1234567890123456", "0.1234567890123456"},
		{"123456789.1234567", "123456789.1234567"},
		{"0.0000000000000001", "0.0000000000000001"}, // 16 decimal places
		{"0.00000000000000010000", "0.0000000000000001"},
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
		{"exponent", "1E+5", "without an exponent"},
		{"lowercase exponent", "1e-3", "without an exponent"},
		{"letters", "abc", "not a valid decimal"},
		{"leading space", " 1", "not a valid decimal"},
		{"trailing space", "1 ", "not a valid decimal"},
		{"underscore", "1_000", "not a valid decimal"},
		{"comma", "1,5", "not a valid decimal"},
		{"two points", "1.2.3", "not a valid decimal"},
		{"hex", "0x10", "not a valid decimal"},
		{"NaN", "NaN", "finite"},
		{"sNaN", "sNaN", "finite"},
		{"Infinity", "Infinity", "finite"},
		{"negative infinity", "-Inf", "finite"},
		{"too many digits", "12345678901234567", "16 significant digits"},
		{"too many digits in decimals", "0.12345678901234567", "16 significant digits"},
		{"too many decimal places", "0.00000000000000001", "16 decimal places"},
		{"too many integer digits", "10000000000000000", "16 integer digits"},
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

func TestRoundResult(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		inexact bool
		err     error
	}{
		{"0", "0", false, nil},
		{"1.5", "1.5", false, nil},
		{"1E+15", "1000000000000000", false, nil},
		{"0.33333333333333333333", "0.3333333333333333", true, nil},
		{"0.00033333333333333333", "0.0003333333333333", true, nil}, // decimal places bind first
		{"1E-17", "0", true, nil},
		{"5E-17", "0", true, nil}, // tie: to even
		{"1.5E-16", "0.0000000000000002", true, nil},
		{"0.99999999999999999", "1", true, nil},
		{"9999999999999998.5", "9999999999999998", true, nil},
		{"12345678901234567", "", true, ErrOutOfRange},
		{"9999999999999999.5", "", true, ErrOutOfRange}, // rounding carries into a 17th digit
		{"1E+16", "", false, ErrOutOfRange},
	}
	ctx := newContext()
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			d, _, err := apd.NewFromString(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			cond, err := roundResult(&ctx, d)
			if !errors.Is(err, tt.err) {
				t.Fatalf("roundResult(%s) error = %v, want %v", tt.in, err, tt.err)
			}
			if cond.Inexact() != tt.inexact {
				t.Errorf("roundResult(%s) inexact = %t, want %t", tt.in, cond.Inexact(), tt.inexact)
			}
			if err == nil && formatResult(d) != tt.want {
				t.Errorf("roundResult(%s) = %s, want %s", tt.in, formatResult(d), tt.want)
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
		{"1E-16", "0.0000000000000001"},
		{"9999999999999999", "9999999999999999"},
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

// Any result must be accepted back as an operand with the same value, so the
// frontend can always chain calculations.
func TestResultsRoundTrip(t *testing.T) {
	cases := []struct {
		op       Operation
		operands []string
	}{
		{Divide, []string{"1", "3"}},
		{Divide, []string{"-2", "3"}},
		{Divide, []string{"1", "3000"}},
		{Divide, []string{"9999999999999999", "7"}},
		{Divide, []string{"0.0000000000000001", "3"}},
		{Sqrt, []string{"2"}},
		{Power, []string{"2", "53"}},
		{Power, []string{"1.1", "0.5"}},
		{Multiply, []string{"1.000000000000001", "1.000000000000001"}},
		{PercentOf, []string{"12345678.12345678", "33.3"}},
	}
	c := New()
	for _, tc := range cases {
		res, err := c.Calculate(tc.op, tc.operands)
		if err != nil {
			t.Fatalf("%s%v: %v", tc.op, tc.operands, err)
		}
		back, err := parseOperand(0, res.Value)
		if err != nil {
			t.Errorf("%s%v = %q, rejected as an operand: %v", tc.op, tc.operands, res.Value, err)
			continue
		}
		if formatResult(back) != res.Value {
			t.Errorf("%s%v: round trip changed %q to %q", tc.op, tc.operands, res.Value, formatResult(back))
		}
	}
}

func TestOperandErrorMessage(t *testing.T) {
	err := &OperandError{Index: 2, Value: "x", Reason: "is not a valid decimal number"}
	if got, want := err.Error(), "operands[2]: is not a valid decimal number"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
