package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

// stubCalculator returns a fixed result or error, or panics, and records the
// arguments it was called with.
type stubCalculator struct {
	result    calculator.Result
	err       error
	panicWith any

	called      bool
	gotOp       calculator.Operation
	gotOperands []string
}

func (s *stubCalculator) Calculate(op calculator.Operation, operands []string) (calculator.Result, error) {
	s.called, s.gotOp, s.gotOperands = true, op, operands
	if s.panicWith != nil {
		panic(s.panicWith)
	}
	return s.result, s.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// serve sends a request through NewRouter and checks that the response is
// JSON, as every response must be.
func serve(t *testing.T, calc Calculator, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	NewRouter(calc, discardLogger()).ServeHTTP(rec, req)
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	return rec
}

// calculate posts body as JSON to the calculate endpoint.
func calculate(t *testing.T, calc Calculator, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, calc, http.MethodPost, "/api/v1/calculate", "application/json", body)
}

// checkError checks that rec is an error response with the given status and
// code, and returns its message.
func checkError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) string {
	t.Helper()
	if rec.Code != status {
		t.Errorf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	var body errorResponse
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("decoding error body: %v", err)
	}
	if body.Error.Code != code {
		t.Errorf("code = %q, want %q", body.Error.Code, code)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
	return body.Error.Message
}
