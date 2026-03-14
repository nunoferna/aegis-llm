package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nunoferna/aegis-llm/internal/cache"
	"github.com/nunoferna/aegis-llm/internal/config"
	"github.com/nunoferna/aegis-llm/internal/proxy"
	"github.com/nunoferna/aegis-llm/internal/ratelimit"
	"github.com/nunoferna/aegis-llm/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := config.Load()

	// 1. Initialize OpenTelemetry
	shutdown, err := telemetry.InitProvider(telemetry.Options{
		Exporter:         cfg.TelemetryExporter,
		OTLPEndpoint:     cfg.TelemetryOTLPEndpoint,
		OTLPInsecure:     cfg.TelemetryOTLPInsecure,
		MetricInterval:   cfg.TelemetryMetricInterval,
		TraceSampleRatio: cfg.TelemetryTraceSampleRatio,
		ServiceName:      cfg.ServiceName,
		ServiceVersion:   cfg.ServiceVersion,
	})
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	// Ensure we flush all telemetry data before the app closes
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	cache.ConfigureEmbedder(cache.EmbedderConfig{
		URL:     cfg.EmbeddingURL,
		Model:   cfg.EmbeddingModel,
		Timeout: cfg.EmbeddingTimeout,
	})

	qClient, err := cache.NewQdrantClient(cfg.QdrantHost, cfg.QdrantPort, cache.ClientOptions{
		SaveQueueSize:             cfg.CacheSaveQueueSize,
		SaveWorkers:               cfg.CacheSaveWorkers,
		SaveTimeout:               cfg.CacheSaveTimeout,
		VectorSize:                cfg.CacheVectorSize,
		MaxCacheableBodyBytes:     cfg.MaxCacheableBodyBytes,
		MaxCacheableResponseBytes: cfg.MaxCacheableResponseBytes,
		CacheEntryTTL:             cfg.CacheEntryTTL,
		SearchLimit:               cfg.CacheSearchLimit,
		CleanupInterval:           cfg.CacheCleanupInterval,
		CleanupTimeout:            cfg.CacheCleanupTimeout,
		CleanupEnabled:            cfg.CacheCleanupEnabled,
		IndexPayloadFields:        cfg.CacheIndexPayloadFields,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qClient.Close()

	// Initialize Redis Rate Limiter
	rateLimiter, err := ratelimit.NewLimiterWithOptions(cfg.RedisHost, cfg.RedisPort, ratelimit.Options{
		MaxRequests: cfg.RateLimitMaxRequests,
		Window:      cfg.RateLimitWindow,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// 2. Initialize our Reverse Proxy Handler
	llmProxy, err := proxy.NewHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	cachedProxy := qClient.Middleware(llmProxy)

	// 3. Wrap the Cache with the Redis Rate Limiter (Tollbooth)
	protectedProxy := rateLimiter.Middleware(cachedProxy)

	// 4. Wrap the Proxy with OpenTelemetry Middleware
	// This automatically creates traces and metrics for every incoming request!
	instrumentedProxy := otelhttp.NewHandler(protectedProxy, "llm_gateway_request")

	// 5. Setup Router
	mux := http.NewServeMux()
	mux.Handle("/v1/", instrumentedProxy)

	// 6. Start the Server with Graceful Shutdown (Senior Engineer best practice)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("🛡️ Aegis-LLM Gateway starting on port %s...", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal (Ctrl+C)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down gracefully, flushing telemetry data...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
