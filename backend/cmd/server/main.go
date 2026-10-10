// Command server runs the calculator REST API.
//
// Configuration comes from the environment: PORT (default 8080). With the
// -healthcheck flag, it instead probes /healthz on the local server and exits
// 0 or 1, which lets the distroless image run a Docker healthcheck without a
// shell or curl.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/api"
	"github.com/fabianguidobaldi/sezzle-challenge/backend/internal/calculator"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run is main without the process globals, so it can be tested. It returns
// the process exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	healthcheck := flags.Bool("healthcheck", false, "probe /healthz on the local server and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if *healthcheck {
		if err := checkHealth("http://localhost:" + port + "/healthz"); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	logger := slog.New(slog.NewJSONHandler(stdout, nil))
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Error("listen failed", "err", err)
		return 1
	}
	logger.Info("server listening", "addr", ln.Addr().String())
	if err := serve(ctx, logger, ln); err != nil {
		logger.Error("server failed", "err", err)
		return 1
	}
	logger.Info("server stopped")
	return 0
}

// serve runs the API on ln until ctx is cancelled, then shuts down gracefully,
// giving in-flight requests up to shutdownTimeout to finish.
func serve(ctx context.Context, logger *slog.Logger, ln net.Listener) error {
	srv := &http.Server{
		Handler:           api.NewRouter(calculator.New(), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// checkHealth returns an error unless url answers 200 OK.
func checkHealth(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: unexpected status %d", resp.StatusCode)
	}
	return nil
}
