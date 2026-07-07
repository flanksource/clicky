package rpchttp

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

var serverTimingRe = regexp.MustCompile(`^total;dur=[0-9]+\.[0-9], find;dur=[0-9]+\.[0-9]$`)

func TestMiddlewareStampsServerTiming(t *testing.T) {
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AddTiming(r.Context(), "find", 2*time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	got := rec.Header().Get("Server-Timing")
	if !serverTimingRe.MatchString(got) {
		t.Fatalf("Server-Timing = %q, want match %s", got, serverTimingRe)
	}
}

func TestMiddlewareTotalOnlyWhenNoPhases(t *testing.T) {
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Server-Timing"); !regexp.MustCompile(`^total;dur=[0-9]+\.[0-9]$`).MatchString(got) {
		t.Fatalf("Server-Timing = %q", got)
	}
}

func TestMiddlewareStampsWhenHandlerDoesNotWrite(t *testing.T) {
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AddTiming(r.Context(), "find", 2*time.Millisecond)
		// Return without calling WriteHeader/Write/Flush.
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	got := rec.Header().Get("Server-Timing")
	if !serverTimingRe.MatchString(got) {
		t.Fatalf("Server-Timing = %q, want match %s", got, serverTimingRe)
	}
}

func TestMiddlewarePreservesFlush(t *testing.T) {
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("recorder does not implement http.Flusher")
			return
		}
		w.WriteHeader(http.StatusOK)
		f.Flush()
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Fatal("expected inner writer to be flushed")
	}
}
