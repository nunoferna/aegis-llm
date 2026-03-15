package providers

import (
	"encoding/json"
	"net/http"
	"strings"
)

type GeminiAdapter struct{}

type geminiPayload struct {
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

func (a *GeminiAdapter) ParseRequest(r *http.Request, body []byte) (string, string, bool, error) {
	var payload geminiPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false, err
	}

	// Extract model from URL path (e.g., /v1beta/models/gemini-1.5-pro:generateContent)
	model := "unknown-gemini-model"
	pathSegments := strings.Split(r.URL.Path, "/")
	for _, segment := range pathSegments {
		if strings.HasPrefix(segment, "models/") || strings.Contains(segment, ":") {
			model = strings.Split(strings.TrimPrefix(segment, "models/"), ":")[0]
			break
		}
	}

	isStream := strings.Contains(r.URL.Path, "streamGenerateContent")

	if len(payload.Contents) == 0 {
		return "", model, isStream, ErrUnsupportedProvider
	}

	lastContent := payload.Contents[len(payload.Contents)-1]
	if len(lastContent.Parts) == 0 {
		return "", model, isStream, ErrUnsupportedProvider
	}

	// Safely aggregate all text parts in the final message
	prompt := ""
	for _, part := range lastContent.Parts {
		prompt += part.Text
	}

	if prompt == "" {
		return "", model, isStream, ErrUnsupportedProvider
	}

	return prompt, model, isStream, nil
}
