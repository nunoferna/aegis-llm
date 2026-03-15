package providers

import (
	"encoding/json"
	"net/http"
)

type OpenAIAdapter struct{}

// openAIPayload represents the standard OpenAI chat completion request.
type openAIPayload struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"` // Can be string or array for multimodal
	} `json:"messages"`
}

func (a *OpenAIAdapter) ParseRequest(r *http.Request, body []byte) (string, string, bool, error) {
	var payload openAIPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false, err
	}

	if len(payload.Messages) == 0 {
		return "", payload.Model, payload.Stream, ErrUnsupportedProvider
	}

	lastMsg := payload.Messages[len(payload.Messages)-1]

	// Handle multimodal arrays (OpenAI Vision) gracefully by extracting just the text
	prompt := ""
	switch v := lastMsg.Content.(type) {
	case string:
		prompt = v
	case []interface{}:
		for _, part := range v {
			if pMap, ok := part.(map[string]interface{}); ok {
				if pMap["type"] == "text" {
					if text, ok := pMap["text"].(string); ok {
						prompt += text
					}
				}
			}
		}
	}

	if prompt == "" {
		return "", payload.Model, payload.Stream, ErrUnsupportedProvider
	}

	return prompt, payload.Model, payload.Stream, nil
}
