package api

import (
	"log/slog"
	"net/http"
)

// NewRouter returns the HTTP handler for the whole API. Every response it
// writes, including 404 and 405, is JSON.
func NewRouter(calc Calculator, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	// Routes are registered without a method so a wrong method gets a JSON
	// 405; the standard mux would reply in plain text.
	mux.Handle("/api/v1/calculate", allowMethod(http.MethodPost, handleCalculate(calc, logger)))
	mux.Handle("/healthz", allowMethod(http.MethodGet, handleHealth))
	mux.HandleFunc("/", handleNotFound)

	return recoverPanics(logger, logRequests(logger, mux))
}

// allowMethod rejects requests whose method isn't method with a JSON 405.
func allowMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed,
				"method "+r.Method+" is not allowed; use "+method)
			return
		}
		next(w, r)
	}
}

// handleHealth serves GET /healthz.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleNotFound serves every path without a route.
func handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, codeNotFound, "no route for "+r.URL.Path)
}
