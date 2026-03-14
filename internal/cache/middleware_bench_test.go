package cache

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const benchmarkVectorSize = 384

func silenceBenchmarkLogs(b *testing.B) {
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() {
		log.SetOutput(oldWriter)
	})
}

func BenchmarkMiddlewareCacheHit(b *testing.B) {
	silenceBenchmarkLogs(b)

	originalGetEmbedding := getEmbedding
	getEmbedding = func(context.Context, string) ([]float32, error) {
		return make([]float32, benchmarkVectorSize), nil
	}
	b.Cleanup(func() { getEmbedding = originalGetEmbedding })

	qc := &QdrantClient{
		maxCacheableBodyBytes: 1 << 20,
		searchOverride: func(context.Context, []float32, string, string) (bool, []byte) {
			return true, []byte(`{"id":"cached","choices":[]}`)
		},
	}

	handler := qc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"live","choices":[]}`))
	}))

	body := `{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func BenchmarkMiddlewareCacheMiss(b *testing.B) {
	silenceBenchmarkLogs(b)

	originalGetEmbedding := getEmbedding
	getEmbedding = func(context.Context, string) ([]float32, error) {
		return make([]float32, benchmarkVectorSize), nil
	}
	b.Cleanup(func() { getEmbedding = originalGetEmbedding })

	qc := &QdrantClient{
		maxCacheableBodyBytes: 1 << 20,
		searchOverride: func(context.Context, []float32, string, string) (bool, []byte) {
			return false, nil
		},
		enqueueOverride: func([]float32, []byte, string, string) bool {
			return true
		},
	}

	handler := qc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"live","choices":[]}`))
	}))

	body := `{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func BenchmarkMiddlewareStreamBypass(b *testing.B) {
	silenceBenchmarkLogs(b)

	qc := &QdrantClient{maxCacheableBodyBytes: 1 << 20}
	handler := qc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"live-stream","choices":[]}`))
	}))

	body := `{"model":"gpt-4.1","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func BenchmarkMiddlewareOversizedPayloadRejected(b *testing.B) {
	silenceBenchmarkLogs(b)

	qc := &QdrantClient{maxCacheableBodyBytes: 256}
	handler := qc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	largeContent := strings.Repeat("x", 1024)
	body := `{"model":"gpt-4.1","messages":[{"role":"user","content":"` + largeContent + `"}]}`
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}
