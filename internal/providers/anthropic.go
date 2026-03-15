package providers

import (
	"encoding/json"
	"net/http"
)

type AnthropicAdapter struct{}

type anthropicPayload struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"` // Claude accepts string or array of blocks
	} `json:"messages"`
}

func (a *AnthropicAdapter) ParseRequest(r *http.Request, body []byte) (string, string, bool, error) {
	var payload anthropicPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false, err
	}

	if len(payload.Messages) == 0 {
		return "", payload.Model, payload.Stream, ErrUnsupportedProvider
	}

	lastMsg := payload.Messages[len(payload.Messages)-1]
	prompt := ""

	switch v := lastMsg.Content.(type) {
	case string:
		prompt = v
	case []interface{}:
		for _, block := range v {
			if bMap, ok := block.(map[string]interface{}); ok {
				// Anthropic uses {"type": "text", "text": "..."}
				if bMap["type"] == "text" {
					if text, ok := bMap["text"].(string); ok {
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
