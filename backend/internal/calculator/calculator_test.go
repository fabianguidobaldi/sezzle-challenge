package calculator

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		op       Operation
		operands []string
		want     string
		exact    bool
	}{
		// add
		{Add, []string{"2", "3"}, "5", true},
		{Add, []string{"0.1", "0.2"}, "0.3", true},
		{Add, []string{"-5", "2.5"}, "-2.5", true},
		{Add, []string{"0.0000000000000001", "0.0000000000000001"}, "0.0000000000000002", true},
		{Add, []string{"9999999999999998", "1"}, "9999999999999999", true},
		{Add, []string{"1234567890.123456", "0.0000000000000001"}, "1234567890.123456", false},
		// subtract
		{Subtract, []string{"10", "4"}, "6", true},
		{Subtract, []string{"2.50", "2.50"}, "0", true},
		{Subtract, []string{"-1", "-1.25"}, "0.25", true},
		{Subtract, []string{"999999999999999", "0.1"}, "999999999999998.9", true},
		{Subtract, []string{"9999999999999999", "0.5"}, "9999999999999998", false}, // ties to even
		// multiply
		{Multiply, []string{"6", "7"}, "42", true},
		{Multiply, []string{"-0", "5"}, "0", true},
		{Multiply, []string{"-1.5", "4"}, "-6", true},
		{Multiply, []string{"0.1", "0.1"}, "0.01", true},
		{Multiply, []string{"1234567890123456", "3"}, "3703703670370368", true},
		{Multiply, []string{"99999999", "99999999"}, "9999999800000001", true},
		{Multiply, []string{"1.000000000000001", "1.000000000000001"}, "1.000000000000002", false},
		{Multiply, []string{"0.0000000001", "0.0000000001"}, "0", false}, // too small: rounds to 0
		{Multiply, []string{"1.5", "0.0000000000000001"}, "0.0000000000000002", false},
		{Multiply, []string{"0.5", "0.0000000000000001"}, "0", false}, // tie at the last place: to even
		// divide
		{Divide, []string{"1", "4"}, "0.25", true},
		{Divide, []string{"1", "3"}, "0.3333333333333333", false},
		{Divide, []string{"-2", "3"}, "-0.6666666666666667", false},
		{Divide, []string{"-10", "-4"}, "2.5", true},
		{Divide, []string{"0", "5"}, "0", true},
		{Divide, []string{"1", "3000"}, "0.0003333333333333", false}, // limited by decimal places
		// power
		{Power, []string{"2", "10"}, "1024", true},
		{Power, []string{"2", "53"}, "9007199254740992", true},
		{Power, []string{"2", "-3"}, "0.125", true},
		{Power, []string{"-2", "3"}, "-8", true},
		{Power, []string{"4", "0.5"}, "2", false}, // exp/ln path: conservative
		{Power, []string{"2", "0.5"}, "1.414213562373095", false},
		{Power, []string{"5", "0"}, "1", true},
		{Power, []string{"0", "5"}, "0", true},
		{Power, []string{"10", "15"}, "1000000000000000", true},
		{Power, []string{"0.1", "16"}, "0.0000000000000001", true},
		{Power, []string{"0.1", "17"}, "0", false},
		{Power, []string{"1", "9999999999999999"}, "1", true},
		{Power, []string{"0.5", "9999999999999999"}, "0", false},  // beyond the working range, tends to 0
		{Power, []string{"-0.5", "9999999999999999"}, "0", false}, // likewise with a negative base
		{Power, []string{"2", "-9999999999999999"}, "0", false},   // |x| > 1, y < 0
		// sqrt
		{Sqrt, []string{"16"}, "4", true},
		{Sqrt, []string{"0.25"}, "0.5", true},
		{Sqrt, []string{"2"}, "1.414213562373095", false},
		{Sqrt, []string{"0"}, "0", true},
		{Sqrt, []string{"-0"}, "0", true},
		{Sqrt, []string{"0.0000000000000001"}, "0.00000001", true},
		// percent
		{Percent, []string{"50"}, "0.5", true},
		{Percent, []string{"-12.5"}, "-0.125", true},
		{Percent, []string{"0"}, "0", true},
		{Percent, []string{"9999999999999999"}, "99999999999999.99", true},
		{Percent, []string{"0.0000000000000001"}, "0", false},
		// percent_of
		{PercentOf, []string{"200", "10"}, "20", true},
		{PercentOf, []string{"-80", "12.5"}, "-10", true},
		{PercentOf, []string{"33.33", "33.33"}, "11.108889", true},
		{PercentOf, []string{"9999999999999999", "100"}, "9999999999999999", true},
	}
	c := New()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s%v", tt.op, tt.operands), func(t *testing.T) {
			got, err := c.Calculate(tt.op, tt.operands)
			if err != nil {
				t.Fatalf("Calculate error: %v", err)
			}
			if got.Value != tt.want || got.Exact != tt.exact {
				t.Errorf("Calculate = {%s, exact=%t}, want {%s, exact=%t}", got.Value, got.Exact, tt.want, tt.exact)
			}
		})
	}
}

func TestCalculateErrors(t *testing.T) {
	tests := []struct {
		op       Operation
		operands []string
		want     error
		msg      string // substring expected in the error
	}{
		{"modulo", []string{"1", "2"}, ErrUnknownOperation, `"modulo"`},
		{"", nil, ErrUnknownOperation, `""`},
		{Divide, []string{"1"}, ErrOperandCount, "divide expects 2 operands, got 1"},
		{Add, nil, ErrOperandCount, "add expects 2 operands, got 0"},
		{Sqrt, []string{"1", "2"}, ErrOperandCount, "sqrt expects 1 operand, got 2"},
		{Percent, []string{}, ErrOperandCount, "percent expects 1 operand, got 0"},
		{Divide, []string{"5", "0"}, ErrDivisionByZero, "division by zero"},
		{Divide, []string{"-5", "0.0"}, ErrDivisionByZero, "division by zero"},
		{Divide, []string{"0", "0"}, ErrDivisionByZero, "division by zero"},
		{Power, []string{"0", "-1"}, ErrDivisionByZero, "negative power"},
		{Power, []string{"0", "-0.5"}, ErrDivisionByZero, "negative power"},
		{Power, []string{"0", "0"}, ErrUndefined, "result is undefined"},
		{Power, []string{"-8", "0.5"}, ErrUndefined, "result is undefined"},
		{Sqrt, []string{"-4"}, ErrUndefined, "square root of a negative number"},
		{Sqrt, []string{"-0.0000000000000001"}, ErrUndefined, "square root of a negative number"},
		// Results of 10^16 or more don't fit in 16 integer digits.
		{Add, []string{"9999999999999999", "1"}, ErrOutOfRange, "less than 10^16"},
		{Add, []string{"9999999999999999", "0.5"}, ErrOutOfRange, "less than 10^16"}, // rounding carries to 10^16
		{Subtract, []string{"-9999999999999999", "1"}, ErrOutOfRange, "less than 10^16"},
		{Multiply, []string{"100000000", "100000000"}, ErrOutOfRange, "less than 10^16"},
		{Divide, []string{"1", "0.0000000000000001"}, ErrOutOfRange, "less than 10^16"},
		{Power, []string{"2", "64"}, ErrOutOfRange, "less than 10^16"},
		{Power, []string{"10", "16"}, ErrOutOfRange, "less than 10^16"},
		{Power, []string{"2", "9999999999999999"}, ErrOutOfRange, "less than 10^16"},    // beyond the working range
		{Power, []string{"0.5", "-9999999999999999"}, ErrOutOfRange, "less than 10^16"}, // apd calls this underflow
		{PercentOf, []string{"1234567890123456", "1234567890123456"}, ErrOutOfRange, "less than 10^16"},
	}
	c := New()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s%v", tt.op, tt.operands), func(t *testing.T) {
			got, err := c.Calculate(tt.op, tt.operands)
			if err == nil {
				t.Fatalf("Calculate = %+v, want error %v", got, tt.want)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("error %v does not wrap %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("error %q does not mention %q", err, tt.msg)
			}
			if got != (Result{}) {
				t.Errorf("Calculate returned %+v alongside an error", got)
			}
		})
	}
}

func TestCalculateInvalidOperand(t *testing.T) {
	_, err := New().Calculate(Add, []string{"1", "abc"})
	if !errors.Is(err, ErrInvalidNumber) {
		t.Fatalf("error %v does not wrap ErrInvalidNumber", err)
	}
	var opErr *OperandError
	if !errors.As(err, &opErr) {
		t.Fatalf("error %v is not an *OperandError", err)
	}
	if opErr.Index != 1 || opErr.Value != "abc" {
		t.Errorf("OperandError = {Index: %d, Value: %q}, want {1, \"abc\"}", opErr.Index, opErr.Value)
	}
}

// Every registered operation must be reachable and every documented
// operation registered.
func TestRegistryCoversOperations(t *testing.T) {
	ops := []Operation{Add, Subtract, Multiply, Divide, Power, Sqrt, Percent, PercentOf}
	if len(registry) != len(ops) {
		t.Errorf("registry has %d operations, want %d", len(registry), len(ops))
	}
	for _, op := range ops {
		if _, ok := registry[op]; !ok {
			t.Errorf("operation %q is not registered", op)
		}
	}
}

func TestDomainErrorFallbacks(t *testing.T) {
	t.Run("unexpected apd error", func(t *testing.T) {
		cause := errors.New("boom")
		err := domainError(Add, nil, 0, cause)
		if !errors.Is(err, cause) {
			t.Errorf("error %v does not wrap the cause", err)
		}
		if isDomainError(err) {
			t.Errorf("error %v should not match a domain sentinel", err)
		}
	})
	t.Run("non-finite result", func(t *testing.T) {
		d, err := parseOperand(0, "1")
		if err != nil {
			t.Fatal(err)
		}
		d.Form = apd.Infinite
		if err := domainError(Power, d, 0, nil); !errors.Is(err, ErrUndefined) {
			t.Errorf("error %v does not wrap ErrUndefined", err)
		}
	})
}

// Run with -race: one Calculator shared by many goroutines.
func TestCalculateConcurrent(t *testing.T) {
	c := New()
	cases := []struct {
		op       Operation
		operands []string
		want     string
	}{
		{Add, []string{"0.1", "0.2"}, "0.3"},
		{Divide, []string{"1", "3"}, "0.3333333333333333"},
		{Power, []string{"2", "53"}, "9007199254740992"},
		{Sqrt, []string{"2"}, "1.414213562373095"},
		{PercentOf, []string{"200", "10"}, "20"},
	}
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			for j := range 50 {
				tc := cases[(i+j)%len(cases)]
				got, err := c.Calculate(tc.op, tc.operands)
				if err != nil || got.Value != tc.want {
					t.Errorf("%s%v = %q, %v; want %q", tc.op, tc.operands, got.Value, err, tc.want)
				}
			}
		})
	}
	wg.Wait()
}
