package cache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// OllamaEmbeddingReq maps to Ollama's /api/embeddings endpoint payload
type OllamaEmbeddingReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResp holds the vector array returned by the model
type OllamaEmbeddingResp struct {
	Embedding []float32 `json:"embedding"`
}

// GetEmbedding sends text to Ollama and returns the 384-dimensional vector.
// Note: In a production app, the URL "http://localhost:11434" would come from your config!
func GetEmbedding(prompt string) ([]float32, error) {
	reqBody := OllamaEmbeddingReq{
		Model:  "all-minilm", // The fast embedding model we downloaded
		Prompt: prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	// Call the local Ollama embedding API
	resp, err := http.Post("http://localhost:11434/api/embeddings", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned status: %d", resp.StatusCode)
	}

	var embedResp OllamaEmbeddingResp
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	return embedResp.Embedding, nil
}
