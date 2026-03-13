package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	shutdown, err := telemetry.InitProvider()
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	// Ensure we flush all telemetry data before the app closes
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	qClient, err := cache.NewQdrantClient("localhost", 6334)
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qClient.Close()

	// Initialize Redis Rate Limiter
	rateLimiter, err := ratelimit.NewLimiter("localhost", "6379")
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// 2. Initialize our Reverse Proxy Handler
	llmProxy := proxy.NewHandler(cfg)

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
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("🛡️ Aegis-LLM Gateway starting on port %s...", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal (Ctrl+C)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down gracefully, flushing telemetry data...")
	_ = srv.Shutdown(context.Background())
}
