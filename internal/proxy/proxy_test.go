package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nunoferna/aegis-llm/internal/config"
)

func TestNewHandlerRejectsInvalidURL(t *testing.T) {
	cfg := &config.Config{
		UpstreamBaseURL: "://bad-url",
		UpstreamAPIKey:  "test-key",
	}

	h, err := NewHandler(cfg)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
	if h != nil {
		t.Fatal("expected nil handler for invalid base URL")
	}
}

func TestNewHandlerInjectsAuthorizationHeader(t *testing.T) {
	seenAuth := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		UpstreamBaseURL: upstream.URL,
		UpstreamAPIKey:  "secret-key",
	}

	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if seenAuth != "Bearer secret-key" {
		t.Fatalf("unexpected Authorization header: %q", seenAuth)
	}
}

func TestNewHandlerSkipsAuthorizationWhenNoUpstreamKey(t *testing.T) {
	seenAuth := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		UpstreamBaseURL: upstream.URL,
	}

	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if seenAuth != "" {
		t.Fatalf("expected no Authorization header, got %q", seenAuth)
	}
}
