package providers

import (
	"net/http"
	"testing"
)

func TestOpenAIAdapter_ParseRequest(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		expectedPrompt string
		expectedModel  string
		expectedStream bool
		expectErr      bool
	}{
		{
			name:           "Standard text prompt",
			body:           []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}]}`),
			expectedPrompt: "hello",
			expectedModel:  "gpt-4o",
			expectedStream: true,
			expectErr:      false,
		},
		{
			name:           "Multimodal array content",
			body:           []byte(`{"model":"gpt-4-vision","messages":[{"role":"user","content":[{"type":"text","text":"describe this"}]}]}`),
			expectedPrompt: "describe this",
			expectedModel:  "gpt-4-vision",
			expectedStream: false,
			expectErr:      false,
		},
		{
			name:      "Empty messages",
			body:      []byte(`{"model":"gpt-4o","messages":[]}`),
			expectErr: true,
		},
		{
			name:      "Malformed JSON",
			body:      []byte(`{bad json`),
			expectErr: true,
		},
	}

	adapter := &OpenAIAdapter{}
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
