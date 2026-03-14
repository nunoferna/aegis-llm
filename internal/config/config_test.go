package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("UPSTREAM_API_KEY", "")
	t.Setenv("PORT", "")
	t.Setenv("UPSTREAM_BASE_URL", "")
	t.Setenv("QDRANT_HOST", "")
	t.Setenv("QDRANT_PORT", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("RATE_LIMIT_MAX_REQUESTS", "")
	t.Setenv("RATE_LIMIT_WINDOW", "")
	t.Setenv("OLLAMA_EMBEDDING_URL", "")
	t.Setenv("OLLAMA_EMBEDDING_MODEL", "")
	t.Setenv("OLLAMA_EMBEDDING_TIMEOUT", "")
	t.Setenv("CACHE_SAVE_QUEUE_SIZE", "")
	t.Setenv("CACHE_SAVE_WORKERS", "")
	t.Setenv("CACHE_SAVE_TIMEOUT", "")
	t.Setenv("CACHE_VECTOR_SIZE", "")
	t.Setenv("MAX_CACHEABLE_BODY_BYTES", "")
	t.Setenv("MAX_CACHEABLE_RESPONSE_BYTES", "")
	t.Setenv("CACHE_ENTRY_TTL", "")
	t.Setenv("CACHE_SEARCH_LIMIT", "")
	t.Setenv("CACHE_CLEANUP_INTERVAL", "")
	t.Setenv("CACHE_CLEANUP_TIMEOUT", "")
	t.Setenv("CACHE_CLEANUP_ENABLED", "")
	t.Setenv("CACHE_INDEX_PAYLOAD_FIELDS", "")
	t.Setenv("TELEMETRY_EXPORTER", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_SERVICE_VERSION", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.UpstreamAPIKey != "" {
		t.Fatalf("expected empty upstream key by default, got %q", cfg.UpstreamAPIKey)
	}
	if cfg.UpstreamBaseURL != "http://localhost:11434" {
		t.Fatalf("unexpected default upstream base URL: %s", cfg.UpstreamBaseURL)
	}
	if cfg.QdrantHost != "localhost" || cfg.QdrantPort != 6334 {
		t.Fatalf("unexpected qdrant defaults: %s:%d", cfg.QdrantHost, cfg.QdrantPort)
	}
	if cfg.RedisHost != "localhost" || cfg.RedisPort != "6379" {
		t.Fatalf("unexpected redis defaults: %s:%s", cfg.RedisHost, cfg.RedisPort)
	}
	if cfg.RateLimitMaxRequests != 5 {
		t.Fatalf("unexpected rate-limit max default: %d", cfg.RateLimitMaxRequests)
	}
	if cfg.RateLimitWindow != time.Minute {
		t.Fatalf("unexpected rate-limit window default: %s", cfg.RateLimitWindow)
	}
	if cfg.EmbeddingURL != "http://localhost:11434/api/embeddings" {
		t.Fatalf("unexpected embedding URL default: %s", cfg.EmbeddingURL)
	}
	if cfg.EmbeddingModel != "all-minilm" {
		t.Fatalf("unexpected embedding model default: %s", cfg.EmbeddingModel)
	}
	if cfg.EmbeddingTimeout != 5*time.Second {
		t.Fatalf("unexpected embedding timeout default: %s", cfg.EmbeddingTimeout)
	}
	if cfg.CacheSaveQueueSize != DefaultCacheSaveQueueSize || cfg.CacheSaveWorkers != DefaultCacheSaveWorkers {
		t.Fatalf("unexpected cache worker defaults: queue=%d workers=%d", cfg.CacheSaveQueueSize, cfg.CacheSaveWorkers)
	}
	if cfg.CacheSaveTimeout != DefaultCacheSaveTimeout {
		t.Fatalf("unexpected cache save timeout default: %s", cfg.CacheSaveTimeout)
	}
	if cfg.CacheVectorSize != 0 {
		t.Fatalf("unexpected cache vector size default: %d", cfg.CacheVectorSize)
	}
	if cfg.MaxCacheableBodyBytes != DefaultMaxCacheableBody {
		t.Fatalf("unexpected body bytes default: %d", cfg.MaxCacheableBodyBytes)
	}
	if cfg.MaxCacheableResponseBytes != DefaultMaxCacheableResponse {
		t.Fatalf("unexpected response bytes default: %d", cfg.MaxCacheableResponseBytes)
	}
	if cfg.CacheEntryTTL != DefaultCacheEntryTTL {
		t.Fatalf("unexpected cache entry TTL default: %s", cfg.CacheEntryTTL)
	}
	if cfg.CacheSearchLimit != DefaultCacheSearchLimit {
		t.Fatalf("unexpected cache search limit default: %d", cfg.CacheSearchLimit)
	}
	if cfg.CacheCleanupInterval != DefaultCacheCleanupInterval {
		t.Fatalf("unexpected cache cleanup interval default: %s", cfg.CacheCleanupInterval)
	}
	if cfg.CacheCleanupTimeout != DefaultCacheCleanupTimeout {
		t.Fatalf("unexpected cache cleanup timeout default: %s", cfg.CacheCleanupTimeout)
	}
	if cfg.CacheCleanupEnabled != DefaultCacheCleanupEnabled {
		t.Fatalf("unexpected cache cleanup enabled default: %t", cfg.CacheCleanupEnabled)
	}
	if cfg.CacheIndexPayloadFields != DefaultCacheIndexPayload {
		t.Fatalf("unexpected payload index default: %t", cfg.CacheIndexPayloadFields)
	}
	if cfg.TelemetryExporter != "stdout" {
		t.Fatalf("unexpected telemetry exporter default: %s", cfg.TelemetryExporter)
	}
	if cfg.TelemetryOTLPEndpoint != "localhost:4317" {
		t.Fatalf("unexpected OTLP endpoint default: %s", cfg.TelemetryOTLPEndpoint)
	}
	if cfg.TelemetryOTLPInsecure != true {
		t.Fatalf("unexpected OTLP insecure default: %t", cfg.TelemetryOTLPInsecure)
	}
	if cfg.TelemetryMetricInterval != 10*time.Second {
		t.Fatalf("unexpected metric interval default: %s", cfg.TelemetryMetricInterval)
	}
	if cfg.TelemetryTraceSampleRatio != 1.0 {
		t.Fatalf("unexpected trace sample ratio default: %f", cfg.TelemetryTraceSampleRatio)
	}
	if cfg.ServiceName != "aegis-llm-gateway" {
		t.Fatalf("unexpected service name default: %s", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "1.0.0" {
		t.Fatalf("unexpected service version default: %s", cfg.ServiceVersion)
	}
}

func TestLoadParsesOverridesAndFallbacks(t *testing.T) {
	t.Setenv("UPSTREAM_API_KEY", "upstream-key")
	t.Setenv("PORT", "9000")
	t.Setenv("UPSTREAM_BASE_URL", "https://example.org")
	t.Setenv("QDRANT_HOST", "qdrant")
	t.Setenv("QDRANT_PORT", "not-a-number")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("RATE_LIMIT_MAX_REQUESTS", "not-a-number")
	t.Setenv("RATE_LIMIT_WINDOW", "30s")
	t.Setenv("OLLAMA_EMBEDDING_URL", "http://ollama:11434/api/embeddings")
	t.Setenv("OLLAMA_EMBEDDING_MODEL", "mxbai-embed-large")
	t.Setenv("OLLAMA_EMBEDDING_TIMEOUT", "7s")
	t.Setenv("CACHE_SAVE_QUEUE_SIZE", "2048")
	t.Setenv("CACHE_SAVE_WORKERS", "8")
	t.Setenv("CACHE_SAVE_TIMEOUT", "1500ms")
	t.Setenv("CACHE_VECTOR_SIZE", "1024")
	t.Setenv("MAX_CACHEABLE_BODY_BYTES", "2048")
	t.Setenv("MAX_CACHEABLE_RESPONSE_BYTES", "4096")
	t.Setenv("CACHE_ENTRY_TTL", "6h")
	t.Setenv("CACHE_SEARCH_LIMIT", "8")
	t.Setenv("CACHE_CLEANUP_INTERVAL", "2m")
	t.Setenv("CACHE_CLEANUP_TIMEOUT", "7s")
	t.Setenv("CACHE_CLEANUP_ENABLED", "false")
	t.Setenv("CACHE_INDEX_PAYLOAD_FIELDS", "false")
	t.Setenv("TELEMETRY_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "not-bool")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "5s")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.2")
	t.Setenv("OTEL_SERVICE_NAME", "aegis-prod")
	t.Setenv("OTEL_SERVICE_VERSION", "2.1.0")

	cfg := Load()

	if cfg.UpstreamAPIKey != "upstream-key" {
		t.Fatalf("expected UPSTREAM_API_KEY to be used, got %q", cfg.UpstreamAPIKey)
	}

	if cfg.Port != "9000" {
		t.Fatalf("expected overridden port 9000, got %s", cfg.Port)
	}
	if cfg.UpstreamBaseURL != "https://example.org" {
		t.Fatalf("unexpected upstream base URL: %s", cfg.UpstreamBaseURL)
	}
	if cfg.QdrantHost != "qdrant" {
		t.Fatalf("unexpected qdrant host: %s", cfg.QdrantHost)
	}
	if cfg.QdrantPort != 6334 {
		t.Fatalf("invalid qdrant port should fall back to 6334, got %d", cfg.QdrantPort)
	}
	if cfg.RedisHost != "redis" || cfg.RedisPort != "6380" {
		t.Fatalf("unexpected redis values: %s:%s", cfg.RedisHost, cfg.RedisPort)
	}
	if cfg.RateLimitMaxRequests != 5 {
		t.Fatalf("invalid rate-limit max should fall back to 5, got %d", cfg.RateLimitMaxRequests)
	}
	if cfg.RateLimitWindow != 30*time.Second {
		t.Fatalf("unexpected rate-limit window: %s", cfg.RateLimitWindow)
	}
	if cfg.EmbeddingURL != "http://ollama:11434/api/embeddings" {
		t.Fatalf("unexpected embedding URL: %s", cfg.EmbeddingURL)
	}
	if cfg.EmbeddingModel != "mxbai-embed-large" {
		t.Fatalf("unexpected embedding model: %s", cfg.EmbeddingModel)
	}
	if cfg.EmbeddingTimeout != 7*time.Second {
		t.Fatalf("unexpected embedding timeout: %s", cfg.EmbeddingTimeout)
	}
	if cfg.CacheSaveQueueSize != 2048 || cfg.CacheSaveWorkers != 8 {
		t.Fatalf("unexpected cache queue/workers: queue=%d workers=%d", cfg.CacheSaveQueueSize, cfg.CacheSaveWorkers)
	}
	if cfg.CacheSaveTimeout != 1500*time.Millisecond {
		t.Fatalf("unexpected cache save timeout: %s", cfg.CacheSaveTimeout)
	}
	if cfg.CacheVectorSize != 1024 {
		t.Fatalf("unexpected cache vector size: %d", cfg.CacheVectorSize)
	}
	if cfg.MaxCacheableBodyBytes != 2048 {
		t.Fatalf("unexpected max body bytes: %d", cfg.MaxCacheableBodyBytes)
	}
	if cfg.MaxCacheableResponseBytes != 4096 {
		t.Fatalf("unexpected max response bytes: %d", cfg.MaxCacheableResponseBytes)
	}
	if cfg.CacheEntryTTL != 6*time.Hour {
		t.Fatalf("unexpected cache entry TTL: %s", cfg.CacheEntryTTL)
	}
	if cfg.CacheSearchLimit != 8 {
		t.Fatalf("unexpected cache search limit: %d", cfg.CacheSearchLimit)
	}
	if cfg.CacheCleanupInterval != 2*time.Minute {
		t.Fatalf("unexpected cache cleanup interval: %s", cfg.CacheCleanupInterval)
	}
	if cfg.CacheCleanupTimeout != 7*time.Second {
		t.Fatalf("unexpected cache cleanup timeout: %s", cfg.CacheCleanupTimeout)
	}
	if cfg.CacheCleanupEnabled != false {
		t.Fatalf("unexpected cache cleanup enabled: %t", cfg.CacheCleanupEnabled)
	}
	if cfg.CacheIndexPayloadFields != false {
		t.Fatalf("unexpected payload index enabled: %t", cfg.CacheIndexPayloadFields)
	}
	if cfg.TelemetryExporter != "otlp" {
		t.Fatalf("unexpected telemetry exporter: %s", cfg.TelemetryExporter)
	}
	if cfg.TelemetryOTLPEndpoint != "otel-collector:4317" {
		t.Fatalf("unexpected OTLP endpoint: %s", cfg.TelemetryOTLPEndpoint)
	}
	if cfg.TelemetryOTLPInsecure != true {
		t.Fatalf("invalid OTLP insecure should fall back to true, got %t", cfg.TelemetryOTLPInsecure)
	}
	if cfg.TelemetryMetricInterval != 5*time.Second {
		t.Fatalf("unexpected metric interval: %s", cfg.TelemetryMetricInterval)
	}
	if cfg.TelemetryTraceSampleRatio != 0.2 {
		t.Fatalf("unexpected trace sample ratio: %f", cfg.TelemetryTraceSampleRatio)
	}
	if cfg.ServiceName != "aegis-prod" {
		t.Fatalf("unexpected service name: %s", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "2.1.0" {
		t.Fatalf("unexpected service version: %s", cfg.ServiceVersion)
	}
}
