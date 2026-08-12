package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestLiveIsIndependentOfReadiness(t *testing.T) {
	handler := Handler(Options{
		ServiceName: "ranking-worker",
		Version:     "test",
		Ready:       func(context.Context) error { return errors.New("redis down") },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("live status = %d, want 200", recorder.Code)
	}
	if body := decodeBody(t, recorder); body["service"] != "ranking-worker" {
		t.Fatalf("live body = %#v", body)
	}
}

func TestReadyReportsFailingDependency(t *testing.T) {
	handler := Handler(Options{
		Ready: func(context.Context) error { return errors.New("redis unreachable") },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["status"] != "not_ready" || body["reason"] != "redis unreachable" {
		t.Fatalf("ready body = %#v", body)
	}
}

func TestReadySucceedsWithoutProbe(t *testing.T) {
	handler := Handler(Options{})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", recorder.Code)
	}
}

// A dependency that hangs must surface as not-ready rather than holding the
// probe open until the load balancer's own timeout fires.
func TestReadyProbeIsBounded(t *testing.T) {
	handler := Handler(Options{
		ProbeTimeout: 10 * time.Millisecond,
		Ready: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", recorder.Code)
	}
}

func TestHealthResponsesAreNotCached(t *testing.T) {
	handler := Handler(Options{})

	for _, path := range []string{"/health/live", "/health/ready"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s Cache-Control = %q, want no-store", path, got)
		}
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	handler := Handler(Options{})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestNonGetIsRejected(t *testing.T) {
	handler := Handler(Options{})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/health/ready", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
