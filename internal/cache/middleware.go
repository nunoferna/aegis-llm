package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

var getEmbedding = GetEmbedding

// UpstreamRequest models the standard incoming LLM JSON payload.
type UpstreamRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream,omitempty"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type responseCapturer struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	maxBytes   int64
	overLimit  bool
}

// Write intercepts the bytes being sent to the client and saves a copy to our buffer
func (rc *responseCapturer) Write(b []byte) (int, error) {
	if !rc.overLimit {
		if rc.maxBytes <= 0 || int64(rc.body.Len()+len(b)) <= rc.maxBytes {
			rc.body.Write(b)
		} else {
			rc.overLimit = true
		}
	}
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapturer) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}

// Middleware intercepts the request to check the vector database first.
// We attach this as a method to our QdrantClient so it has access to the DB.
func (qc *QdrantClient) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > qc.maxCacheableBodyBytes && qc.maxCacheableBodyBytes > 0 {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		limitedBody := io.Reader(r.Body)
		if qc.maxCacheableBodyBytes > 0 {
			limitedBody = io.LimitReader(r.Body, qc.maxCacheableBodyBytes+1)
		}

		bodyBytes, err := io.ReadAll(limitedBody)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		if qc.maxCacheableBodyBytes > 0 && int64(len(bodyBytes)) > qc.maxCacheableBodyBytes {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var payload UpstreamRequest
		if err := json.Unmarshal(bodyBytes, &payload); err != nil || len(payload.Messages) == 0 {

			next.ServeHTTP(w, r)
			return
		}
		if payload.Stream {
			next.ServeHTTP(w, r)
			return
		}

		prompt := payload.Messages[len(payload.Messages)-1].Content
		promptHash := hashPrompt(prompt)
		log.Printf("🧠 Intercepted Prompt: %q", prompt)

		vector, err := getEmbedding(r.Context(), prompt)
		if err != nil {
			log.Printf("⚠️ Failed to generate embedding: %v", err)

			next.ServeHTTP(w, r)
			return
		}

		isHit, cachedResponse := qc.search(r.Context(), vector, payload.Model, promptHash)
		if isHit {
			log.Println("⚡ CACHE HIT! Bypassing LLM and serving from memory.")
			w.Header().Set("Content-Type", "application/json")
			w.Write(cachedResponse)
			return
		}

		log.Println("🐢 CACHE MISS. Forwarding to LLM provider...")

		capturer := &responseCapturer{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
			maxBytes:       qc.maxCacheableResponseBytes,
		}

		next.ServeHTTP(capturer, r)

		if r.Context().Err() != nil {
			log.Println("⚠️ Client disconnected early, dropping cache write to prevent corruption")
			return
		}

		if capturer.statusCode == http.StatusOK && !capturer.overLimit {
			if !qc.enqueue(vector, capturer.body.Bytes(), payload.Model, promptHash) {
				log.Println("⚠️ Cache save queue is full, dropping async cache write")
			}
		}
		if capturer.overLimit {
			log.Println("⚠️ Response exceeded cacheable size limit, skipping cache write")
		}
	})
}

func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:16])
}
