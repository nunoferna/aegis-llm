package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type EmbedderConfig struct {
	URL     string
	Model   string
	Timeout time.Duration
}

var embeddingConfig EmbedderConfig

var embeddingHTTPClient *http.Client

func ConfigureEmbedder(cfg EmbedderConfig) {
	embeddingConfig = cfg

	embeddingHTTPClient = &http.Client{Timeout: embeddingConfig.Timeout}
}

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
func GetEmbedding(ctx context.Context, prompt string) ([]float32, error) {
	if embeddingHTTPClient == nil || embeddingConfig.URL == "" || embeddingConfig.Model == "" || embeddingConfig.Timeout <= 0 {
		return nil, fmt.Errorf("embedder is not configured")
	}

	reqBody := OllamaEmbeddingReq{
		Model:  embeddingConfig.Model,
		Prompt: prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, embeddingConfig.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := embeddingHTTPClient.Do(req)
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

// GetEmbeddingVectorSize probes the configured embedding model and returns its vector dimension.
func GetEmbeddingVectorSize(ctx context.Context) (int, error) {
	embedding, err := GetEmbedding(ctx, "vector-size-probe")
	if err != nil {
		return 0, err
	}
	if len(embedding) == 0 {
		return 0, fmt.Errorf("embedding API returned an empty vector")
	}
	return len(embedding), nil
}
