package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startServer runs serve on a random local port and returns its base URL.
// The server is shut down, and serve's error checked, when the test ends.
func startServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve returned %v after shutdown", err)
			}
		case <-time.After(shutdownTimeout):
			t.Error("serve did not return after shutdown")
		}
	})
	return "http://" + ln.Addr().String()
}

// Integration test: the real calculator behind the real router, over HTTP.
func TestCalculateEndToEnd(t *testing.T) {
	baseURL := startServer(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantResult string
		wantExact  bool
		wantCode   string
	}{
		{"exact decimals", `{"operation":"add","operands":["0.1","0.2"]}`, 200, "0.3", true, ""},
		{"json numbers", `{"operation":"add","operands":[0.1,0.2]}`, 200, "0.3", true, ""},
		{"rounded division", `{"operation":"divide","operands":["1","3"]}`, 200, "0.3333333333333333", false, ""},
		{"power", `{"operation":"power","operands":["2","53"]}`, 200, "9007199254740992", true, ""},
		{"tiny result rounds to zero", `{"operation":"multiply","operands":["0.0000000001","0.0000000001"]}`, 200, "0", false, ""},
		{"sqrt", `{"operation":"sqrt","operands":["16"]}`, 200, "4", true, ""},
		{"percent", `{"operation":"percent","operands":["50"]}`, 200, "0.5", true, ""},
		{"percent of", `{"operation":"percent_of","operands":["200","10"]}`, 200, "20", true, ""},
		{"negative zero", `{"operation":"multiply","operands":["-0","5"]}`, 200, "0", true, ""},
		{"division by zero", `{"operation":"divide","operands":["1","0"]}`, 422, "", false, "DIVISION_BY_ZERO"},
		{"zero to negative power", `{"operation":"power","operands":["0","-1"]}`, 422, "", false, "DIVISION_BY_ZERO"},
		{"negative sqrt", `{"operation":"sqrt","operands":["-4"]}`, 422, "", false, "UNDEFINED_RESULT"},
		{"overflow", `{"operation":"power","operands":["2","64"]}`, 422, "", false, "RESULT_OUT_OF_RANGE"},
		{"exponent notation", `{"operation":"add","operands":["1E+5","1"]}`, 400, "", false, "INVALID_NUMBER"},
		{"invalid number", `{"operation":"add","operands":["1","abc"]}`, 400, "", false, "INVALID_NUMBER"},
		{"unknown operation", `{"operation":"modulo","operands":["1","2"]}`, 400, "", false, "UNKNOWN_OPERATION"},
		{"wrong arity", `{"operation":"sqrt","operands":["1","2"]}`, 400, "", false, "INVALID_OPERAND_COUNT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(baseURL+"/api/v1/calculate", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var body struct {
				Result string `json:"result"`
				Exact  bool   `json:"exact"`
				Error  struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Result != tt.wantResult || body.Exact != tt.wantExact || body.Error.Code != tt.wantCode {
				t.Errorf("got {result: %q, exact: %v, code: %q}, want {%q, %v, %q}",
					body.Result, body.Exact, body.Error.Code, tt.wantResult, tt.wantExact, tt.wantCode)
			}
		})
	}
}

func TestCheckHealth(t *testing.T) {
	t.Run("running server", func(t *testing.T) {
		if err := checkHealth(startServer(t) + "/healthz"); err != nil {
			t.Errorf("checkHealth() = %v, want nil", err)
		}
	})

	t.Run("unhealthy status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if err := checkHealth(srv.URL); err == nil || !strings.Contains(err.Error(), "503") {
			t.Errorf("checkHealth() = %v, want unexpected status 503", err)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		if err := checkHealth(url); err == nil {
			t.Error("checkHealth() = nil, want connection error")
		}
	})
}

func TestServeReturnsListenerErrors(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // Serve fails immediately on a closed listener.
	if err := serve(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), ln); err == nil {
		t.Error("serve() = nil, want error from closed listener")
	}
}

func TestRun(t *testing.T) {
	env := func(port string) func(string) string {
		return func(key string) string {
			if key == "PORT" {
				return port
			}
			return ""
		}
	}
	portOf := func(baseURL string) string { return baseURL[strings.LastIndex(baseURL, ":")+1:] }

	t.Run("healthcheck healthy", func(t *testing.T) {
		port := portOf(startServer(t))
		if code := run(context.Background(), []string{"-healthcheck"}, env(port), io.Discard, io.Discard); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	t.Run("healthcheck unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		port := portOf(srv.URL)
		srv.Close()
		var stderr strings.Builder
		if code := run(context.Background(), []string{"-healthcheck"}, env(port), io.Discard, &stderr); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "healthcheck") {
			t.Errorf("stderr = %q, want healthcheck error", stderr.String())
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		if code := run(context.Background(), []string{"-nope"}, env(""), io.Discard, io.Discard); code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		if code := run(context.Background(), nil, env("not-a-port"), io.Discard, io.Discard); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	t.Run("serves until cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // shut down as soon as the server is up
		var stdout strings.Builder
		if code := run(ctx, nil, env("0"), &stdout, io.Discard); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		for _, msg := range []string{"server listening", "server stopped"} {
			if !strings.Contains(stdout.String(), msg) {
				t.Errorf("logs %q missing %q", stdout.String(), msg)
			}
		}
	})
}
