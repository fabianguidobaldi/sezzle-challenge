package calculator

import (
	"errors"
	"fmt"
	"testing"
)

// Fractional powers go through apd's exp/ln, which flags every result
// inexact; exactPower must report the exact ones as exact, and only those.
func TestFractionalPowerExactness(t *testing.T) {
	tests := []struct {
		x, y  string
		want  string
		exact bool
	}{
		// Exact: r^den = x^num holds.
		{"4", "0.5", "2", true},
		{"9", "1.5", "27", true},
		{"32", "0.2", "2", true},
		{"16", "0.25", "2", true},
		{"1024", "0.1", "2", true},
		{"0.25", "0.5", "0.5", true},
		{"4", "-0.5", "0.5", true},
		{"16", "-1.25", "0.03125", true},
		{"1000000", "0.5", "1000", true},
		{"0.0001", "0.75", "0.001", true},
		{"0", "0.5", "0", true},
		// Inexact: irrational results stay flagged.
		{"2", "0.5", "1.414213562373095", false},
		{"8", "0.75", "4.756828460010884", false},
		{"10", "-0.5", "0.3162277660168379", false},
		{"2", "1.5", "2.82842712474619", false},
		{"0.0000000000000001", "1.5", "0", false}, // rounds to 0: nothing to check
		// Outside the check's limits: conservative, even when exact.
		{"2", "0.001", "1.000693387462581", false},                 // den = 1000
		{"1.000000000000001", "200.5", "1.000000000000201", false}, // x^|num| too large to check
	}
	c := New()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s^%s", tt.x, tt.y), func(t *testing.T) {
			got, err := c.Calculate(Power, []string{tt.x, tt.y})
			if err != nil {
				t.Fatalf("Calculate error: %v", err)
			}
			if got.Value != tt.want || got.Exact != tt.exact {
				t.Errorf("Calculate = {%s, exact=%t}, want {%s, exact=%t}", got.Value, got.Exact, tt.want, tt.exact)
			}
		})
	}
}

func TestSmallFraction(t *testing.T) {
	tests := []struct {
		y        string
		num, den int64
		ok       bool
	}{
		{"0.5", 1, 2, true},
		{"-0.5", -1, 2, true},
		{"1.25", 5, 4, true},
		{"0.2", 1, 5, true},
		{"0.75", 3, 4, true},
		{"0.010", 1, 100, true},
		{"0.001", 0, 0, false}, // den 1000
		{"3", 0, 0, false},     // integer
		{"3.000", 0, 0, false}, // integer after reduction
		{"0.1234567890123456", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.y, func(t *testing.T) {
			y, err := parseOperand(0, tt.y)
			if err != nil {
				t.Fatal(err)
			}
			num, den, ok := smallFraction(y)
			if ok != tt.ok || (ok && (num != tt.num || den != tt.den)) {
				t.Errorf("smallFraction(%s) = %d/%d, %t; want %d/%d, %t", tt.y, num, den, ok, tt.num, tt.den, tt.ok)
			}
		})
	}
}

func TestFractionalPowerTooLarge(t *testing.T) {
	// apd's result is inexact and too large for the format: the exactness
	// check bails out and Calculate reports the range error.
	_, err := New().Calculate(Power, []string{"9999999999999999", "1.5"})
	if !errors.Is(err, ErrOutOfRange) {
		t.Errorf("error = %v, want ErrOutOfRange", err)
	}
}
