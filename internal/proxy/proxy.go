package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/nunoferna/aegis-llm/internal/config" // Change this to your actual module name
)

// NewHandler creates a reverse proxy that forwards traffic to the LLM provider.
func NewHandler(cfg *config.Config) (http.Handler, error) {
	// Parse the target URL (e.g., http://localhost:11434)
	target, err := url.Parse(cfg.UpstreamBaseURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid upstream base URL: %q", cfg.UpstreamBaseURL)
	}

	// Go's standard library provides a robust Reverse Proxy out of the box!
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Upstream LLM provider unavailable", http.StatusBadGateway)
	}

	// The Director is a function that modifies the request *before* it is sent out.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Run the default director to set up the basic URL routing
		originalDirector(req)

		// Overwrite the Host header. Many firewalls (like Cloudflare) will block
		// requests if the Host header doesn't match the destination IP.
		req.Host = target.Host

		// Inject upstream API key only when configured.
		if cfg.UpstreamAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.UpstreamAPIKey)
		}
	}

	return proxy, nil
}
