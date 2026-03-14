package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/nunoferna/aegis-llm/internal/config"
)

// NewHandler creates a reverse proxy that forwards traffic to the LLM provider.
func NewHandler(cfg *config.Config) (http.Handler, error) {

	target, err := url.Parse(cfg.UpstreamBaseURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid upstream base URL: %q", cfg.UpstreamBaseURL)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {

			pr.SetURL(target)

			pr.Out.Host = target.Host

			if cfg.UpstreamAPIKey != "" {
				pr.Out.Header.Set("Authorization", "Bearer "+cfg.UpstreamAPIKey)
			}

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
