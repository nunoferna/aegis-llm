package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func benchmarkLimiterWithEvaluate(allowed, remaining, resetMs int64) *Limiter {
	return &Limiter{
		maxRequests: 5,
		window:      time.Minute,
		now: func() time.Time {
			return time.Unix(1700000000, 0).UTC()
		},
		evaluate: func(context.Context, string, int64, time.Duration, int64) (int64, int64, int64, error) {
			return allowed, remaining, resetMs, nil
		},
	}
}

func BenchmarkRateLimitMissingAuthorization(b *testing.B) {
	// 1 = allowed, 4 = remaining, resetMs = arbitrary future timestamp
	rl := benchmarkLimiterWithEvaluate(1, 4, 1700000060000)
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func BenchmarkRateLimitInvalidAuthorizationPrefix(b *testing.B) {
	rl := benchmarkLimiterWithEvaluate(1, 4, 1700000060000)
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Token abc")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}
