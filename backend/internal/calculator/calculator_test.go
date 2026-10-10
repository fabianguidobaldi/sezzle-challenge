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
		{Add, []string{"1E+20", "1"}, "1E+20", false},
		{Add, []string{"9999999999999999", "1"}, "1E+16", true},
		// subtract
		{Subtract, []string{"10", "4"}, "6", true},
		{Subtract, []string{"2.50", "2.50"}, "0", true},
		{Subtract, []string{"-1", "-1.25"}, "0.25", true},
		{Subtract, []string{"1E+15", "0.1"}, "999999999999999.9", true},
		// multiply
		{Multiply, []string{"6", "7"}, "42", true},
		{Multiply, []string{"-0", "5"}, "0", true},
		{Multiply, []string{"-1.5", "4"}, "-6", true},
		{Multiply, []string{"0.1", "0.1"}, "0.01", true},
		{Multiply, []string{"1234567890123456", "3"}, "3703703670370368", true},
		{Multiply, []string{"1234567890123456", "1234567890123456"}, "1.524157875323882E+30", false},
		// divide
		{Divide, []string{"1", "4"}, "0.25", true},
		{Divide, []string{"1", "3"}, "0.3333333333333333", false},
		{Divide, []string{"-2", "3"}, "-0.6666666666666667", false},
		{Divide, []string{"-10", "-4"}, "2.5", true},
		{Divide, []string{"0", "5"}, "0", true},
		// power
		{Power, []string{"2", "10"}, "1024", true},
		{Power, []string{"2", "64"}, "1.844674407370955E+19", false},
		{Power, []string{"2", "-3"}, "0.125", true},
		{Power, []string{"-2", "3"}, "-8", true},
		{Power, []string{"4", "0.5"}, "2", false}, // exp/ln path: conservative
		{Power, []string{"5", "0"}, "1", true},
		{Power, []string{"0", "5"}, "0", true},
		{Power, []string{"1", "9999"}, "1", true},
		{Power, []string{"1", "-9999"}, "1", true},
		{Power, []string{"10", "9999"}, "1E+9999", true},
		{Power, []string{"10", "-9999"}, "1E-9999", true},
		// sqrt
		{Sqrt, []string{"16"}, "4", true},
		{Sqrt, []string{"0.25"}, "0.5", true},
		{Sqrt, []string{"2"}, "1.414213562373095", false},
		{Sqrt, []string{"0"}, "0", true},
		{Sqrt, []string{"-0"}, "0", true},
		// percent
		{Percent, []string{"50"}, "0.5", true},
		{Percent, []string{"-12.5"}, "-0.125", true},
		{Percent, []string{"0"}, "0", true},
		{Percent, []string{"1E+9999"}, "1E+9997", true},
		// percent_of
		{PercentOf, []string{"200", "10"}, "20", true},
		{PercentOf, []string{"-80", "12.5"}, "-10", true},
		{PercentOf, []string{"1234567890123456", "1234567890123456"}, "1.524157875323882E+28", false},
		{PercentOf, []string{"1E+5000", "1E+5001"}, "1E+9999", true}, // product briefly above the range
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
		{Divide, []string{"5", "0"}, ErrDivisionByZero, "divide"},
		{Divide, []string{"-5", "0.0"}, ErrDivisionByZero, "divide"},
		{Divide, []string{"0", "0"}, ErrDivisionByZero, "divide"},
		{Power, []string{"0", "-1"}, ErrDivisionByZero, "negative power"},
		{Power, []string{"0", "-0.5"}, ErrDivisionByZero, "negative power"},
		{Power, []string{"0", "0"}, ErrUndefined, "power"},
		{Power, []string{"-8", "0.5"}, ErrUndefined, "power"},
		{Power, []string{"2", "10000"}, ErrOutOfRange, "at most 9999"},
		{Power, []string{"1", "-10000"}, ErrOutOfRange, "at most 9999"},
		{Power, []string{"11", "9999"}, ErrOutOfRange, "power"},
		{Power, []string{"11", "-9999"}, ErrOutOfRange, "power"},
		{Sqrt, []string{"-4"}, ErrUndefined, "square root of a negative number"},
		{Sqrt, []string{"-1E-9999"}, ErrUndefined, "square root of a negative number"},
		{Multiply, []string{"1E+9000", "1E+9000"}, ErrOutOfRange, "multiply"},
		{Multiply, []string{"1E-9000", "1E-9000"}, ErrOutOfRange, "multiply"},
		{Multiply, []string{"1E-9999", "0.1"}, ErrOutOfRange, "multiply"}, // exact but subnormal
		{Add, []string{"9.999999999999999E+9999", "9.999999999999999E+9999"}, ErrOutOfRange, "add"},
		{Divide, []string{"1E-9999", "3"}, ErrOutOfRange, "divide"},
		{Percent, []string{"1E-9999"}, ErrOutOfRange, "percent"},
		{PercentOf, []string{"1E+9999", "1E+3"}, ErrOutOfRange, "percent_of"},
		{PercentOf, []string{"1E-9999", "1"}, ErrOutOfRange, "percent_of"},
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
		{Power, []string{"2", "64"}, "1.844674407370955E+19"},
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
