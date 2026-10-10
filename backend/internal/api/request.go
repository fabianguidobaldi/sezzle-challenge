package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

// maxBodyBytes bounds the size of a calculate request body.
const maxBodyBytes = 4 << 10

// errInvalidOperand is returned by operand.UnmarshalJSON for any JSON value
// that is neither a string nor a number.
var errInvalidOperand = errors.New("each operand must be a decimal string or a number")

// operand is a calculator operand as sent by the client. It accepts a JSON
// string or a JSON number; numbers keep their original text, so they are never
// converted to a float and lose no precision.
type operand string

// UnmarshalJSON implements json.Unmarshaler. It rejects null, booleans,
// objects and arrays.
func (o *operand) UnmarshalJSON(data []byte) error {
	// The decoder only passes complete, valid JSON values, so the first byte
	// identifies the value's type.
	switch c := data[0]; {
	case c == '"':
		return json.Unmarshal(data, (*string)(o))
	case c == '-' || (c >= '0' && c <= '9'):
		*o = operand(data)
	default:
		return errInvalidOperand
	}
	return nil
}

// calculateRequest is a decoded and validated calculate request.
type calculateRequest struct {
	Operation calculator.Operation
	Operands  []string
}

// requestError is a request that was rejected before reaching the calculator.
// It carries everything needed to write the error response.
type requestError struct {
	status  int
	code    string
	message string
}

func (e *requestError) Error() string { return e.message }

// invalidRequest returns a 400 INVALID_REQUEST error with the given message.
func invalidRequest(format string, args ...any) *requestError {
	return &requestError{status: http.StatusBadRequest, code: codeInvalidRequest, message: fmt.Sprintf(format, args...)}
}

// decodeCalculateRequest reads and validates the body of a calculate request.
// Every error it returns is a *requestError.
func decodeCalculateRequest(w http.ResponseWriter, r *http.Request) (calculateRequest, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return calculateRequest{}, &requestError{
			status:  http.StatusUnsupportedMediaType,
			code:    codeUnsupportedMediaType,
			message: "Content-Type must be application/json",
		}
	}

	// Pointers tell a missing or null field apart from a zero value.
	var body struct {
		Operation *string   `json:"operation"`
		Operands  []operand `json:"operands"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return calculateRequest{}, decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return calculateRequest{}, decodeError(err)
		}
		return calculateRequest{}, invalidRequest("request body must contain a single JSON object")
	}

	switch {
	case body.Operation == nil:
		return calculateRequest{}, invalidRequest("operation is required")
	case *body.Operation == "":
		return calculateRequest{}, invalidRequest("operation must not be empty")
	case body.Operands == nil: // missing or null; [] is left to the calculator
		return calculateRequest{}, invalidRequest("operands is required and must be an array")
	}

	operands := make([]string, len(body.Operands))
	for i, o := range body.Operands {
		operands[i] = string(o)
	}
	return calculateRequest{Operation: calculator.Operation(*body.Operation), Operands: operands}, nil
}

// decodeError translates a json.Decoder error into a client-facing error.
func decodeError(err error) *requestError {
	var (
		maxBytesErr *http.MaxBytesError
		syntaxErr   *json.SyntaxError
		typeErr     *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &maxBytesErr):
		return &requestError{
			status:  http.StatusRequestEntityTooLarge,
			code:    codeRequestTooLarge,
			message: fmt.Sprintf("request body must be at most %d bytes", maxBodyBytes),
		}
	case errors.Is(err, io.EOF):
		return invalidRequest("request body must not be empty")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return invalidRequest("request body is not valid JSON")
	case errors.As(err, &typeErr):
		if typeErr.Field == "" {
			return invalidRequest("request body must be a JSON object")
		}
		return invalidRequest("%s has the wrong type: got a JSON %s", typeErr.Field, typeErr.Value)
	case errors.Is(err, errInvalidOperand):
		return invalidRequest("%s", errInvalidOperand)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json has no typed error for unknown fields.
		return invalidRequest("%s", strings.TrimPrefix(err.Error(), "json: "))
	default:
		return invalidRequest("invalid request body")
	}
}
