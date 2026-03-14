package proxy

import (
	"fmt"
	"log"
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

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL routes the request to our target while preserving the client's requested path
			pr.SetURL(target)

			// Overwrite the Host header. Many firewalls (like Cloudflare) will block
			// requests if the Host header doesn't match the destination IP.
			pr.Out.Host = target.Host

			// Inject upstream API key securely. This cannot be stripped by a malicious client.
			if cfg.UpstreamAPIKey != "" {
				pr.Out.Header.Set("Authorization", "Bearer "+cfg.UpstreamAPIKey)
			}

			// Safely handle X-Forwarded-For, Host, and Proto headers to prevent IP spoofing
			pr.SetXForwarded()
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("⚠️ Upstream proxy error: %v", err)
			http.Error(w, "Upstream LLM provider unavailable", http.StatusBadGateway)
		},
	}

	return proxy, nil
}
