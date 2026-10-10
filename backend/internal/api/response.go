package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

// Error codes returned in the "code" field of error responses.
const (
	codeInvalidRequest       = "INVALID_REQUEST"
	codeUnknownOperation     = "UNKNOWN_OPERATION"
	codeInvalidOperandCount  = "INVALID_OPERAND_COUNT"
	codeInvalidNumber        = "INVALID_NUMBER"
	codeNotFound             = "NOT_FOUND"
	codeMethodNotAllowed     = "METHOD_NOT_ALLOWED"
	codeRequestTooLarge      = "REQUEST_TOO_LARGE"
	codeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"
	codeDivisionByZero       = "DIVISION_BY_ZERO"
	codeUndefinedResult      = "UNDEFINED_RESULT"
	codeResultOutOfRange     = "RESULT_OUT_OF_RANGE"
	codeInternalError        = "INTERNAL_ERROR"
)

// internalErrorMessage is the only message clients see for unexpected errors;
// the details are logged instead.
const internalErrorMessage = "internal server error"

// domainErrors maps the calculator's errors to HTTP responses. The 400s mean
// the request is malformed; the 422s mean it is well formed but the math has
// no answer.
var domainErrors = []struct {
	err    error
	status int
	code   string
}{
	{calculator.ErrUnknownOperation, http.StatusBadRequest, codeUnknownOperation},
	{calculator.ErrOperandCount, http.StatusBadRequest, codeInvalidOperandCount},
	{calculator.ErrInvalidNumber, http.StatusBadRequest, codeInvalidNumber},
	{calculator.ErrDivisionByZero, http.StatusUnprocessableEntity, codeDivisionByZero},
	{calculator.ErrUndefined, http.StatusUnprocessableEntity, codeUndefinedResult},
	{calculator.ErrOutOfRange, http.StatusUnprocessableEntity, codeResultOutOfRange},
}

// errorResponse is the body of every error response.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Response types always encode; an error here means the client went away.
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error response with the given status, code and message.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}

// writeFailure writes the response for an error from request decoding or from
// the calculator. Errors it doesn't recognize are logged and reported as a
// generic 500, so internal details never reach the client.
func writeFailure(w http.ResponseWriter, logger *slog.Logger, err error) {
	var reqErr *requestError
	if errors.As(err, &reqErr) {
		writeError(w, reqErr.status, reqErr.code, reqErr.message)
		return
	}
	for _, m := range domainErrors {
		if errors.Is(err, m.err) {
			writeError(w, m.status, m.code, err.Error())
			return
		}
	}
	logger.Error("unexpected error", slog.Any("error", err))
	writeError(w, http.StatusInternalServerError, codeInternalError, internalErrorMessage)
}
