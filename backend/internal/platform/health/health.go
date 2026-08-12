// Package health serves the liveness and readiness endpoints for the non-HTTP
// processes. The api process exposes equivalent paths through internal/httpapi;
// the worker and scheduler need the same contract without the CORS, envelope
// and routing concerns of the public API.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// ReadyFunc reports why a process is not ready, or nil when it is.
type ReadyFunc func(context.Context) error

type Options struct {
	Addr        string
	ServiceName string
	Version     string
	Commit      string
	Environment string
	// Ready is probed on every readiness request. A nil Ready means the process
	// is ready as soon as it is live.
	Ready        ReadyFunc
	ProbeTimeout time.Duration
	Logger       *slog.Logger
}

const defaultProbeTimeout = 2 * time.Second

func Handler(options Options) http.Handler {
	if options.ProbeTimeout <= 0 {
		options.ProbeTimeout = defaultProbeTimeout
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{"status": "method_not_allowed"})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"status":  "live",
			"service": options.ServiceName,
			"version": options.Version,
			"commit":  options.Commit,
		})
	})
	mux.HandleFunc("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{"status": "method_not_allowed"})
			return
		}
		if options.Ready == nil {
			writeJSON(writer, http.StatusOK, map[string]any{"status": "ready"})
			return
		}

		ctx, cancel := context.WithTimeout(request.Context(), options.ProbeTimeout)
		defer cancel()
		if err := options.Ready(ctx); err != nil {
			// The reason is returned so an operator reading a failing probe knows
			// which dependency is down without correlating logs by timestamp.
			writeJSON(writer, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"reason": err.Error(),
			})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"status": "ready"})
	})
	mux.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusNotFound, map[string]any{"status": "not_found"})
	})

	return mux
}

// NewServer wires the handler with the same conservative timeouts the api
// process uses.
func NewServer(options Options) *http.Server {
	return &http.Server{
		Addr:              options.Addr,
		Handler:           Handler(options),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func writeJSON(writer http.ResponseWriter, status int, body map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
