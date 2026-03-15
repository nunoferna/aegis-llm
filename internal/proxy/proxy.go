package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/nunoferna/aegis-llm/internal/config"
)

// NewHandler creates a reverse proxy that forwards traffic to the detected LLM provider.
func NewHandler(cfg *config.Config) (http.Handler, error) {
	// Parse the default target URL (OpenAI / Ollama)
	defaultTarget, err := url.Parse(cfg.UpstreamBaseURL)
	if err != nil || defaultTarget.Scheme == "" || defaultTarget.Host == "" {
		return nil, fmt.Errorf("invalid upstream base URL: %q", cfg.UpstreamBaseURL)
	}

	// Hardcode the official endpoints for Anthropic and Gemini
	anthropicURL, _ := url.Parse("https://api.anthropic.com")
	geminiURL, _ := url.Parse("https://generativelanguage.googleapis.com")

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			path := pr.In.URL.Path

			if strings.Contains(path, "/v1/messages") {
				// Route to Anthropic
				pr.SetURL(anthropicURL)
				pr.Out.Host = anthropicURL.Host
				if cfg.AnthropicAPIKey != "" {
					pr.Out.Header.Set("x-api-key", cfg.AnthropicAPIKey)
					pr.Out.Header.Set("anthropic-version", "2023-06-01")
				}
			} else if strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent") {
				// Route to Google Gemini
				pr.SetURL(geminiURL)
				pr.Out.Host = geminiURL.Host
				if cfg.GeminiAPIKey != "" {
					pr.Out.Header.Set("x-goog-api-key", cfg.GeminiAPIKey)
				}
			} else {
				// Default to OpenAI / Ollama
				pr.SetURL(defaultTarget)
				pr.Out.Host = defaultTarget.Host
				if cfg.OpenAIAPIKey != "" {
					pr.Out.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
				}
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
