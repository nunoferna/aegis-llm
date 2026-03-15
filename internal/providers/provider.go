package providers

import (
	"errors"
	"net/http"
	"strings"
)

var ErrUnsupportedProvider = errors.New("unsupported provider format")

// LLMProvider defines how to extract caching metadata from different AI APIs.
type LLMProvider interface {
	ParseRequest(r *http.Request, body []byte) (prompt string, model string, isStream bool, err error)
}

// DetectProvider inspects the URL path to route the request to the correct parsing adapter.
func DetectProvider(r *http.Request) LLMProvider {
	if strings.Contains(r.URL.Path, "/v1/messages") {
		return &AnthropicAdapter{}
	}
	if strings.Contains(r.URL.Path, ":generateContent") || strings.Contains(r.URL.Path, ":streamGenerateContent") {
		return &GeminiAdapter{}
	}
	// Default to the OpenAI standard (which Ollama also uses)
	return &OpenAIAdapter{}
}
