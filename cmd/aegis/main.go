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

	rateLimiter, err := ratelimit.NewLimiterWithOptions(cfg.RedisHost, cfg.RedisPort, ratelimit.Options{
		MaxRequests: cfg.RateLimitMaxRequests,
		Window:      cfg.RateLimitWindow,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	llmProxy, err := proxy.NewHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	cachedProxy := qClient.Middleware(llmProxy)

	protectedProxy := rateLimiter.Middleware(cachedProxy)

	instrumentedProxy := otelhttp.NewHandler(protectedProxy, "llm_gateway_request")

	mux := http.NewServeMux()
	mux.Handle("/v1/", instrumentedProxy)

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down gracefully, flushing telemetry data...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
