package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

func TestCalculateSuccess(t *testing.T) {
	tests := []struct {
		name string
		body string
		res  calculator.Result
		want string
	}{
		{
			name: "string operands",
			body: `{"operation":"divide","operands":["1","3"]}`,
			res:  calculator.Result{Value: "0.3333333333333333", Exact: false},
			want: `{"operation":"divide","operands":["1","3"],"result":"0.3333333333333333","exact":false}`,
		},
		{
			name: "number operands keep their text",
			body: `{"operation":"add","operands":[0.1, 0.2]}`,
			res:  calculator.Result{Value: "0.3", Exact: true},
			want: `{"operation":"add","operands":["0.1","0.2"],"result":"0.3","exact":true}`,
		},
		{
			name: "number beyond float64 precision",
			body: `{"operation":"sqrt","operands":[1234567890.123456]}`,
			res:  calculator.Result{Value: "35136.41828644957", Exact: false},
			want: `{"operation":"sqrt","operands":["1234567890.123456"],"result":"35136.41828644957","exact":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := calculate(t, &stubCalculator{result: tt.res}, tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCalculateMethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			calc := &stubCalculator{}
			rec := serve(t, calc, method, "/api/v1/calculate", "application/json", `{"operation":"add","operands":["1","2"]}`)
			checkError(t, rec, http.StatusMethodNotAllowed, codeMethodNotAllowed)
			if got := rec.Header().Get("Allow"); got != http.MethodPost {
				t.Errorf("Allow = %q, want POST", got)
			}
			if calc.called {
				t.Error("calculator was called")
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	rec := serve(t, &stubCalculator{}, http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body) != 1 || body["status"] != "ok" {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", rec.Body)
	}
}

func TestHealthzMethodNotAllowed(t *testing.T) {
	rec := serve(t, &stubCalculator{}, http.MethodPost, "/healthz", "", "")
	checkError(t, rec, http.StatusMethodNotAllowed, codeMethodNotAllowed)
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow = %q, want GET", got)
	}
}

func TestNotFound(t *testing.T) {
	paths := []string{"/", "/index.html", "/api", "/api/v1", "/api/v1/calculate/", "/api/v2/calculate", "/healthz/x"}
	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+" "+path, func(t *testing.T) {
				rec := serve(t, &stubCalculator{}, method, path, "application/json", "")
				checkError(t, rec, http.StatusNotFound, codeNotFound)
			})
		}
	}
}

func TestPanicRecovered(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(`{"operation":"add","operands":["1","2"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouter(&stubCalculator{panicWith: "secret panic detail"}, logger).ServeHTTP(rec, req)

	if msg := checkError(t, rec, http.StatusInternalServerError, codeInternalError); msg != internalErrorMessage {
		t.Errorf("message = %q, want %q", msg, internalErrorMessage)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("response leaks the panic value")
	}
	if !strings.Contains(logs.String(), "secret panic detail") {
		t.Errorf("panic was not logged; logs: %s", logs.String())
	}
}

func TestAbortHandlerPanicPropagates(t *testing.T) {
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler", v)
		}
	}()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(`{"operation":"add","operands":["1","2"]}`))
	req.Header.Set("Content-Type", "application/json")
	NewRouter(&stubCalculator{panicWith: http.ErrAbortHandler}, discardLogger()).ServeHTTP(httptest.NewRecorder(), req)
	t.Error("ServeHTTP returned normally")
}

func TestRequestLogging(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	req := httptest.NewRequest(http.MethodGet, "/nowhere", nil)
	NewRouter(&stubCalculator{}, logger).ServeHTTP(httptest.NewRecorder(), req)

	var entry struct {
		Msg      string `json:"msg"`
		Method   string `json:"method"`
		Path     string `json:"path"`
		Status   int    `json:"status"`
		Duration *int64 `json:"duration"`
	}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decoding log entry %q: %v", logs.String(), err)
	}
	if entry.Msg != "request" || entry.Method != http.MethodGet || entry.Path != "/nowhere" ||
		entry.Status != http.StatusNotFound || entry.Duration == nil {
		t.Errorf("log entry = %s", logs.String())
	}
}

func TestStatusRecorder(t *testing.T) {
	tests := []struct {
		name  string
		write func(w http.ResponseWriter)
		want  int
	}{
		{"nothing written", func(http.ResponseWriter) {}, http.StatusOK},
		{"implicit header", func(w http.ResponseWriter) { _, _ = w.Write([]byte("x")) }, http.StatusOK},
		{"explicit header", func(w http.ResponseWriter) { w.WriteHeader(http.StatusCreated) }, http.StatusCreated},
		{"first header wins", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusAccepted)
			w.WriteHeader(http.StatusTeapot)
		}, http.StatusAccepted},
		{"header after body", func(w http.ResponseWriter) {
			_, _ = w.Write([]byte("x"))
			w.WriteHeader(http.StatusTeapot)
		}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &statusRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
			tt.write(rec)
			if rec.status != tt.want {
				t.Errorf("status = %d, want %d", rec.status, tt.want)
			}
		})
	}
}
