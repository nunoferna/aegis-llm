package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHashTokenDeterministicAndCompact(t *testing.T) {
	input := "BearerTokenValue"
	first := hashToken(input)
	second := hashToken(input)

	if first != second {
		t.Fatal("expected hashToken to be deterministic")
	}
	if len(first) != 32 {
		t.Fatalf("expected compact hex hash length 32, got %d", len(first))
	}
}

func TestHashTokenDifferentInputs(t *testing.T) {
	a := hashToken("token-a")
	b := hashToken("token-b")

	if a == b {
		t.Fatal("expected different tokens to produce different hashes")
	}
}

func TestMiddlewareAllowsWithinConfiguredLimit(t *testing.T) {
	rl := &Limiter{
		maxRequests: 3,
		window:      30 * time.Second,
		now: func() time.Time {
			return time.Unix(1700000000, 0).UTC()
		},
		increment: func(context.Context, string, time.Duration) (int64, error) {
			return 2, nil
		},
	}

	nextCalled := false
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("unexpected X-RateLimit-Limit: %q", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Fatalf("unexpected X-RateLimit-Remaining: %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestMiddlewareBlocksWhenOverConfiguredLimit(t *testing.T) {
	rl := &Limiter{
		maxRequests: 3,
		window:      30 * time.Second,
		now: func() time.Time {
			return time.Unix(1700000000, 0).UTC()
		},
		increment: func(context.Context, string, time.Duration) (int64, error) {
			return 4, nil
		},
	}

	nextCalled := false
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
	if rec.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("unexpected X-RateLimit-Limit: %q", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("unexpected X-RateLimit-Remaining: %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}
