package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/nunoferna/aegis-llm/internal/config" // Change this to your actual module name
)

// NewHandler creates a reverse proxy that forwards traffic to the LLM provider.
func NewHandler(cfg *config.Config) http.Handler {
	// Parse the target URL (e.g., https://api.openai.com)
	target, err := url.Parse(cfg.OpenAIBaseURL)
	if err != nil {
		panic("Invalid OpenAI Base URL in config")
	}

	// Go's standard library provides a robust Reverse Proxy out of the box!
	proxy := httputil.NewSingleHostReverseProxy(target)

	// The Director is a function that modifies the request *before* it is sent out.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Run the default director to set up the basic URL routing
		originalDirector(req)

		// Overwrite the Host header. Many firewalls (like Cloudflare) will block 
		// requests if the Host header doesn't match the destination IP.
		req.Host = target.Host

		// Inject the Enterprise API Key. 
		// Now internal developers don't need real OpenAI keys; they just use the Gateway.
		req.Header.Set("Authorization", "Bearer "+cfg.OpenAIKey)
	}

	return proxy
}