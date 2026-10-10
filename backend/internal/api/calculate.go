// Package api exposes the calculator over HTTP as a JSON API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

// Calculator performs a single calculation. It is defined here, on the
// consumer side, so handlers can be tested with a stub.
type Calculator interface {
	Calculate(op calculator.Operation, operands []string) (calculator.Result, error)
}

// calculateResponse is the body of a successful calculate response.
type calculateResponse struct {
	Operation calculator.Operation `json:"operation"`
	Operands  []string             `json:"operands"` // echoed as received
	Result    string               `json:"result"`
	Exact     bool                 `json:"exact"`
}

// handleCalculate serves POST /api/v1/calculate.
func handleCalculate(calc Calculator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decodeCalculateRequest(w, r)
		if err != nil {
			writeFailure(w, logger, err)
			return
		}
		res, err := calc.Calculate(req.Operation, req.Operands)
		if err != nil {
			writeFailure(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, calculateResponse{
			Operation: req.Operation,
			Operands:  req.Operands,
			Result:    res.Value,
			Exact:     res.Exact,
		})
	}
}
