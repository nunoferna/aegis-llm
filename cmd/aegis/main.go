package main

import (
	"log"
	"net/http"

	"github.com/nunoferna/aegis-llm/internal/config"
	"github.com/nunoferna/aegis-llm/internal/proxy"
)

func main() {
	// 1. Load Configuration (Fails fast if missing API key)
	cfg := config.Load()

	// 2. Setup the HTTP Multiplexer
	mux := http.NewServeMux()

	// 3. Initialize our Reverse Proxy Handler
	llmProxy := proxy.NewHandler(cfg)

	// 4. Route all traffic hitting /v1/ to our proxy.
	// This matches the standard OpenAI SDK paths.
	mux.Handle("/v1/", llmProxy)

	// 5. Start the server
	log.Printf("🛡️ Aegis-LLM Gateway starting on port %s...", cfg.Port)
	log.Printf("Routing traffic to: %s", cfg.OpenAIBaseURL)

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
