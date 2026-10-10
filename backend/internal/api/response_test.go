package api

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

func TestDomainErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"unknown operation", fmt.Errorf("%w: %q", calculator.ErrUnknownOperation, "modulo"), http.StatusBadRequest, codeUnknownOperation},
		{"operand count", fmt.Errorf("%w: add takes 2", calculator.ErrOperandCount), http.StatusBadRequest, codeInvalidOperandCount},
		{"invalid number", calculator.ErrInvalidNumber, http.StatusBadRequest, codeInvalidNumber},
		{"operand error", &calculator.OperandError{Index: 1, Value: "abc", Reason: "is not a valid decimal number"}, http.StatusBadRequest, codeInvalidNumber},
		{"wrapped operand error", fmt.Errorf("parsing: %w", &calculator.OperandError{Index: 0, Value: "x", Reason: "bad"}), http.StatusBadRequest, codeInvalidNumber},
		{"division by zero", calculator.ErrDivisionByZero, http.StatusUnprocessableEntity, codeDivisionByZero},
		{"undefined", fmt.Errorf("%w: square root of a negative number", calculator.ErrUndefined), http.StatusUnprocessableEntity, codeUndefinedResult},
		{"out of range", fmt.Errorf("%w: overflow", calculator.ErrOutOfRange), http.StatusUnprocessableEntity, codeResultOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := calculate(t, &stubCalculator{err: tt.err}, `{"operation":"add","operands":["1","2"]}`)
			if msg := checkError(t, rec, tt.status, tt.code); msg != tt.err.Error() {
				t.Errorf("message = %q, want %q", msg, tt.err.Error())
			}
		})
	}
}

func TestDomainErrorTableCoversEverySentinel(t *testing.T) {
	sentinels := []error{
		calculator.ErrUnknownOperation,
		calculator.ErrOperandCount,
		calculator.ErrInvalidNumber,
		calculator.ErrDivisionByZero,
		calculator.ErrUndefined,
		calculator.ErrOutOfRange,
	}
	if len(domainErrors) != len(sentinels) {
		t.Fatalf("domainErrors has %d entries, want %d", len(domainErrors), len(sentinels))
	}
	for i, err := range sentinels {
		if domainErrors[i].err != err {
			t.Errorf("domainErrors[%d] = %v, want %v", i, domainErrors[i].err, err)
		}
	}
}

func TestUnexpectedErrorIsGeneric(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	secret := errors.New("database password is hunter2")

	rec := httptest.NewRecorder()
	writeFailure(rec, logger, secret)

	if msg := checkError(t, rec, http.StatusInternalServerError, codeInternalError); msg != internalErrorMessage {
		t.Errorf("message = %q, want %q", msg, internalErrorMessage)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Error("response leaks the internal error")
	}
	if !strings.Contains(logs.String(), "hunter2") {
		t.Errorf("internal error was not logged; logs: %s", logs.String())
	}
}

func TestUnexpectedErrorThroughRouter(t *testing.T) {
	rec := calculate(t, &stubCalculator{err: errors.New("boom")}, `{"operation":"add","operands":["1","2"]}`)
	if msg := checkError(t, rec, http.StatusInternalServerError, codeInternalError); msg != internalErrorMessage {
		t.Errorf("message = %q, want %q", msg, internalErrorMessage)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusTeapot, "TEAPOT", "short and stout")
	want := `{"error":{"code":"TEAPOT","message":"short and stout"}}` + "\n"
	if rec.Code != http.StatusTeapot || rec.Body.String() != want {
		t.Errorf("got %d %s, want 418 %s", rec.Code, rec.Body, want)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}
