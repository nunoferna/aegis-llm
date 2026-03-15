package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

const (
	DefaultUpstreamBaseURL      = "http://localhost:11434"
	DefaultEmbeddingURL         = DefaultUpstreamBaseURL + "/api/embeddings"
	DefaultEmbeddingModel       = "all-minilm"
	DefaultEmbeddingTimeout     = 5 * time.Second
	DefaultRateLimitMaxRequests = int64(5)
	DefaultRateLimitWindow      = time.Minute
	DefaultCacheSaveQueueSize   = 1024
	DefaultCacheSaveWorkers     = 4
	DefaultCacheSaveTimeout     = 2 * time.Second
	DefaultMaxCacheableBody     = int64(1 << 20)
	DefaultMaxCacheableResponse = int64(1 << 20)
	DefaultCacheEntryTTL        = 24 * time.Hour
	DefaultCacheSearchLimit     = 5
	DefaultCacheCleanupInterval = 15 * time.Minute
	DefaultCacheCleanupTimeout  = 5 * time.Second
	DefaultCacheCleanupEnabled  = true
	DefaultCacheIndexPayload    = true
	DefaultTelemetryExporter    = "stdout"
	DefaultOTLPEndpoint         = "localhost:4317"
	DefaultOTLPInsecure         = true
	DefaultMetricInterval       = 10 * time.Second
	DefaultTraceSampleRatio     = 1.0
	DefaultServiceName          = "aegis-llm-gateway"
	DefaultServiceVersion       = "1.0.0"
)

// Config holds all the environment-level configuration for our Gateway.
type Config struct {
	Port                 string
	OpenAIAPIKey         string
	AnthropicAPIKey      string
	GeminiAPIKey         string
	UpstreamBaseURL      string
	QdrantHost           string
	QdrantPort           int
	RedisHost            string
	RedisPort            string
	RateLimitMaxRequests int64
	RateLimitWindow      time.Duration

	EmbeddingURL     string
	EmbeddingModel   string
	EmbeddingTimeout time.Duration

	CacheSaveQueueSize        int
	CacheSaveWorkers          int
	CacheSaveTimeout          time.Duration
	CacheVectorSize           int
	MaxCacheableBodyBytes     int64
	MaxCacheableResponseBytes int64
	CacheEntryTTL             time.Duration
	CacheSearchLimit          int
	CacheCleanupInterval      time.Duration
	CacheCleanupTimeout       time.Duration
	CacheCleanupEnabled       bool
	CacheIndexPayloadFields   bool

	TelemetryExporter         string
	TelemetryOTLPEndpoint     string
	TelemetryOTLPInsecure     bool
	TelemetryMetricInterval   time.Duration
	TelemetryTraceSampleRatio float64
	ServiceName               string
	ServiceVersion            string
}

// Load reads environment variables and returns a populated Config struct.
func Load() *Config {
	cfg := &Config{
		Port:                 getEnvOrDefault("PORT", "8080"),
		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey:      os.Getenv("ANTHROPIC_API_KEY"),
		GeminiAPIKey:         os.Getenv("GEMINI_API_KEY"),
		UpstreamBaseURL:      getEnvOrDefault("UPSTREAM_BASE_URL", DefaultUpstreamBaseURL),
		QdrantHost:           getEnvOrDefault("QDRANT_HOST", "localhost"),
		QdrantPort:           getEnvIntOrDefault("QDRANT_PORT", 6334),
		RedisHost:            getEnvOrDefault("REDIS_HOST", "localhost"),
		RedisPort:            getEnvOrDefault("REDIS_PORT", "6379"),
		RateLimitMaxRequests: getEnvInt64OrDefault("RATE_LIMIT_MAX_REQUESTS", DefaultRateLimitMaxRequests),
		RateLimitWindow:      getEnvDurationOrDefault("RATE_LIMIT_WINDOW", DefaultRateLimitWindow),

		EmbeddingURL:     getEnvOrDefault("EMBEDDING_URL", DefaultEmbeddingURL),
		EmbeddingModel:   getEnvOrDefault("EMBEDDING_MODEL", DefaultEmbeddingModel),
		EmbeddingTimeout: getEnvDurationOrDefault("EMBEDDING_TIMEOUT", DefaultEmbeddingTimeout),

		CacheSaveQueueSize:        getEnvIntOrDefault("CACHE_SAVE_QUEUE_SIZE", DefaultCacheSaveQueueSize),
		CacheSaveWorkers:          getEnvIntOrDefault("CACHE_SAVE_WORKERS", DefaultCacheSaveWorkers),
		CacheSaveTimeout:          getEnvDurationOrDefault("CACHE_SAVE_TIMEOUT", DefaultCacheSaveTimeout),
		CacheVectorSize:           getEnvIntOrDefault("CACHE_VECTOR_SIZE", 0),
		MaxCacheableBodyBytes:     getEnvInt64OrDefault("MAX_CACHEABLE_BODY_BYTES", DefaultMaxCacheableBody),
		MaxCacheableResponseBytes: getEnvInt64OrDefault("MAX_CACHEABLE_RESPONSE_BYTES", DefaultMaxCacheableResponse),
		CacheEntryTTL:             getEnvDurationOrDefault("CACHE_ENTRY_TTL", DefaultCacheEntryTTL),
		CacheSearchLimit:          getEnvIntOrDefault("CACHE_SEARCH_LIMIT", DefaultCacheSearchLimit),
		CacheCleanupInterval:      getEnvDurationOrDefault("CACHE_CLEANUP_INTERVAL", DefaultCacheCleanupInterval),
		CacheCleanupTimeout:       getEnvDurationOrDefault("CACHE_CLEANUP_TIMEOUT", DefaultCacheCleanupTimeout),
		CacheCleanupEnabled:       getEnvBoolOrDefault("CACHE_CLEANUP_ENABLED", DefaultCacheCleanupEnabled),
		CacheIndexPayloadFields:   getEnvBoolOrDefault("CACHE_INDEX_PAYLOAD_FIELDS", DefaultCacheIndexPayload),

		TelemetryExporter:         getEnvOrDefault("TELEMETRY_EXPORTER", DefaultTelemetryExporter),
		TelemetryOTLPEndpoint:     getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", DefaultOTLPEndpoint),
		TelemetryOTLPInsecure:     getEnvBoolOrDefault("OTEL_EXPORTER_OTLP_INSECURE", DefaultOTLPInsecure),
		TelemetryMetricInterval:   getEnvDurationOrDefault("OTEL_METRIC_EXPORT_INTERVAL", DefaultMetricInterval),
		TelemetryTraceSampleRatio: getEnvFloat64OrDefault("OTEL_TRACE_SAMPLE_RATIO", DefaultTraceSampleRatio),
		ServiceName:               getEnvOrDefault("OTEL_SERVICE_NAME", DefaultServiceName),
		ServiceVersion:            getEnvOrDefault("OTEL_SERVICE_VERSION", DefaultServiceVersion),
	}

	if cfg.OpenAIAPIKey == "" {
		log.Printf("No OpenAI API key configured (OPENAI_API_KEY). Continuing without Authorization injection")
	}

	return cfg
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvIntOrDefault(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Invalid integer for %s=%q, using default=%d", key, val, fallback)
		return fallback
	}

	return parsed
}

func getEnvInt64OrDefault(key string, fallback int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		log.Printf("Invalid int64 for %s=%q, using default=%d", key, val, fallback)
		return fallback
	}

	return parsed
}

func getEnvDurationOrDefault(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("Invalid duration for %s=%q, using default=%s", key, val, fallback)
		return fallback
	}

	return parsed
}

func getEnvBoolOrDefault(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		log.Printf("Invalid bool for %s=%q, using default=%t", key, val, fallback)
		return fallback
	}

	return parsed
}

func getEnvFloat64OrDefault(key string, fallback float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		log.Printf("Invalid float for %s=%q, using default=%f", key, val, fallback)
		return fallback
	}

	if parsed < 0 || parsed > 1 {
		log.Printf("Out-of-range float for %s=%q, using default=%f", key, val, fallback)
		return fallback
	}

	return parsed
}
