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
	mockNow := time.Unix(1700000000, 0).UTC()
	rl := &Limiter{
		maxRequests: 3,
		window:      30 * time.Second,
		now: func() time.Time {
			return mockNow
		},
		evaluate: func(ctx context.Context, key string, capacity int64, window time.Duration, nowMs int64) (int64, int64, int64, error) {
			// Simulate an ALLOWED request. 
			// allowed=1, remaining=2, reset in 30 seconds
			resetMs := mockNow.Add(30 * time.Second).UnixMilli()
			return 1, 2, resetMs, nil
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
	if rec.Header().Get("X-RateLimit-Remaining") != "2" {
		t.Fatalf("unexpected X-RateLimit-Remaining: %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestMiddlewareBlocksWhenOverConfiguredLimit(t *testing.T) {
	mockNow := time.Unix(1700000000, 0).UTC()
	rl := &Limiter{
		maxRequests: 3,
		window:      30 * time.Second,
		now: func() time.Time {
			return mockNow
		},
		evaluate: func(ctx context.Context, key string, capacity int64, window time.Duration, nowMs int64) (int64, int64, int64, error) {
			// Simulate a BLOCKED request.
			// allowed=0, remaining=0, reset in 30 seconds
			resetMs := mockNow.Add(30 * time.Second).UnixMilli()
			return 0, 0, resetMs, nil
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