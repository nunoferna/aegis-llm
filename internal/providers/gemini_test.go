package providers

import (
	"net/http"
	"testing"
)

func TestGeminiAdapter_ParseRequest(t *testing.T) {
	tests := []struct {
		name           string
		urlPath        string
		body           []byte
		expectedPrompt string
		expectedModel  string
		expectedStream bool
		expectErr      bool
	}{
		{
			name:           "Standard generateContent request",
			urlPath:        "/v1beta/models/gemini-1.5-pro:generateContent",
			body:           []byte(`{"contents":[{"role":"user","parts":[{"text":"hello gemini"}]}]}`),
			expectedPrompt: "hello gemini",
			expectedModel:  "gemini-1.5-pro",
			expectedStream: false,
			expectErr:      false,
		},
		{
			name:           "Streaming request",
			urlPath:        "/v1beta/models/gemini-1.5-flash:streamGenerateContent",
			body:           []byte(`{"contents":[{"role":"user","parts":[{"text":"hello stream"}]}]}`),
			expectedPrompt: "hello stream",
			expectedModel:  "gemini-1.5-flash",
			expectedStream: true,
			expectErr:      false,
		},
		{
			name:      "Empty contents",
			urlPath:   "/v1beta/models/gemini-1.5-pro:generateContent",
			body:      []byte(`{"contents":[]}`),
			expectErr: true,
		},
	}

	adapter := &GeminiAdapter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, tt.urlPath, nil)
			prompt, model, stream, err := adapter.ParseRequest(req, tt.body)
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if err == nil {
				if prompt != tt.expectedPrompt {
					t.Errorf("expected prompt %q, got %q", tt.expectedPrompt, prompt)
				}
				if model != tt.expectedModel {
					t.Errorf("expected model %q, got %q", tt.expectedModel, model)
				}
				if stream != tt.expectedStream {
					t.Errorf("expected stream %v, got %v", tt.expectedStream, stream)
				}
			}
		})
	}
}
