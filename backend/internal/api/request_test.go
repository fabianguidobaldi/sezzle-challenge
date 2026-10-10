package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestOperandUnmarshalJSON(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`"1.5"`, "1.5"},
		{`"abc"`, "abc"}, // validating numbers is the calculator's job
		{`""`, ""},
		{`"1"`, "1"},
		{`0.1`, "0.1"},
		{`-7`, "-7"},
		{`1e400`, "1e400"}, // beyond float64: kept as text; the calculator rejects it
		{`12345678901234567890`, "12345678901234567890"},
		{`2.50`, "2.50"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var o operand
			if err := json.Unmarshal([]byte(tt.in), &o); err != nil {
				t.Fatalf("Unmarshal(%s) error: %v", tt.in, err)
			}
			if string(o) != tt.want {
				t.Errorf("Unmarshal(%s) = %q, want %q", tt.in, o, tt.want)
			}
		})
	}
}

func TestOperandUnmarshalJSONRejects(t *testing.T) {
	for _, in := range []string{`null`, `true`, `false`, `{}`, `[]`, `["1"]`} {
		t.Run(in, func(t *testing.T) {
			var o operand
			if err := json.Unmarshal([]byte(in), &o); !errors.Is(err, errInvalidOperand) {
				t.Errorf("Unmarshal(%s) error = %v, want errInvalidOperand", in, err)
			}
		})
	}
}

func TestDecodeAcceptsMixedOperands(t *testing.T) {
	calc := &stubCalculator{}
	rec := calculate(t, calc, ` {"operands": ["1.5", 0.1, 1e400, -2], "operation": "add"} `+"\n")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if calc.gotOp != "add" {
		t.Errorf("operation = %q, want add", calc.gotOp)
	}
	if want := []string{"1.5", "0.1", "1e400", "-2"}; !slices.Equal(calc.gotOperands, want) {
		t.Errorf("operands = %q, want %q", calc.gotOperands, want)
	}
}

func TestDecodeEmptyOperandsReachCalculator(t *testing.T) {
	// Arity is the calculator's concern, so an empty array is passed through.
	calc := &stubCalculator{}
	calculate(t, calc, `{"operation":"add","operands":[]}`)
	if !calc.called || calc.gotOperands == nil || len(calc.gotOperands) != 0 {
		t.Errorf("calculator got operands %#v, want an empty slice", calc.gotOperands)
	}
}

func TestDecodeInvalidRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string // substring expected in the error message
	}{
		{"empty body", ``, "must not be empty"},
		{"malformed", `{"operation":`, "not valid JSON"},
		{"syntax error", `{"operation" "add"}`, "not valid JSON"},
		{"not an object", `["add"]`, "must be a JSON object"},
		{"string body", `"add"`, "must be a JSON object"},
		{"unknown field", `{"operation":"add","operands":["1","2"],"extra":1}`, `unknown field "extra"`},
		{"trailing object", `{"operation":"add","operands":["1","2"]}{}`, "single JSON object"},
		{"trailing garbage", `{"operation":"add","operands":["1","2"]} x`, "single JSON object"},
		{"null body", `null`, "operation is required"},
		{"missing operation", `{"operands":["1","2"]}`, "operation is required"},
		{"null operation", `{"operation":null,"operands":["1","2"]}`, "operation is required"},
		{"empty operation", `{"operation":"","operands":["1","2"]}`, "operation must not be empty"},
		{"numeric operation", `{"operation":1,"operands":["1","2"]}`, "operation has the wrong type"},
		{"missing operands", `{"operation":"add"}`, "operands is required"},
		{"null operands", `{"operation":"add","operands":null}`, "operands is required"},
		{"string operands", `{"operation":"add","operands":"1"}`, "operands has the wrong type"},
		{"object operands", `{"operation":"add","operands":{"a":"1"}}`, "operands has the wrong type"},
		{"boolean operand", `{"operation":"add","operands":[true,"2"]}`, "decimal string or a number"},
		{"null operand", `{"operation":"add","operands":["1",null]}`, "decimal string or a number"},
		{"nested array operand", `{"operation":"add","operands":[["1"],"2"]}`, "decimal string or a number"},
		{"object operand", `{"operation":"add","operands":[{},"2"]}`, "decimal string or a number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := &stubCalculator{}
			msg := checkError(t, calculate(t, calc, tt.body), http.StatusBadRequest, codeInvalidRequest)
			if !strings.Contains(msg, tt.message) {
				t.Errorf("message %q does not mention %q", msg, tt.message)
			}
			if calc.called {
				t.Error("calculator was called for an invalid request")
			}
		})
	}
}

func TestDecodeContentType(t *testing.T) {
	body := `{"operation":"add","operands":["1","2"]}`
	tests := []struct {
		contentType string
		status      int
	}{
		{"application/json", http.StatusOK},
		{"application/json; charset=utf-8", http.StatusOK},
		{"Application/JSON", http.StatusOK},
		{"", http.StatusUnsupportedMediaType},
		{"text/plain", http.StatusUnsupportedMediaType},
		{"application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"application/json-patch+json", http.StatusUnsupportedMediaType},
		{"application/json; charset", http.StatusUnsupportedMediaType}, // malformed parameter
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			rec := serve(t, &stubCalculator{}, http.MethodPost, "/api/v1/calculate", tt.contentType, body)
			if tt.status == http.StatusOK {
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
				}
				return
			}
			checkError(t, rec, tt.status, codeUnsupportedMediaType)
		})
	}
}

func TestDecodeBodyTooLarge(t *testing.T) {
	padding := strings.Repeat(" ", maxBodyBytes)
	tests := map[string]string{
		"inside the object": `{"operation":"add","operands":["` + strings.Repeat("1", maxBodyBytes) + `"]}`,
		"after the object":  `{"operation":"add","operands":["1","2"]}` + padding,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			calc := &stubCalculator{}
			checkError(t, calculate(t, calc, body), http.StatusRequestEntityTooLarge, codeRequestTooLarge)
			if calc.called {
				t.Error("calculator was called for an oversized request")
			}
		})
	}
}

func TestDecodeBodyAtLimit(t *testing.T) {
	body := `{"operation":"add","operands":["1","2"]}`
	body += strings.Repeat(" ", maxBodyBytes-len(body))
	if rec := calculate(t, &stubCalculator{}, body); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
}

// failingReader fails every read, like a client connection that drops.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestDecodeReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", failingReader{})
	req.Header.Set("Content-Type", "application/json")
	_, err := decodeCalculateRequest(httptest.NewRecorder(), req)
	var reqErr *requestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("error %v is not a *requestError", err)
	}
	if reqErr.status != http.StatusBadRequest || reqErr.code != codeInvalidRequest || err.Error() != reqErr.message {
		t.Errorf("error = %+v, want 400 %s", reqErr, codeInvalidRequest)
	}
}
