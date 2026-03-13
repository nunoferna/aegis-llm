package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// OpenAIRequest models the standard incoming JSON payload
type OpenAIRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// responseCapturer wraps the standard http.ResponseWriter so we can copy the bytes
type responseCapturer struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// Write intercepts the bytes being sent to the client and saves a copy to our buffer
func (rc *responseCapturer) Write(b []byte) (int, error) {
	rc.body.Write(b)
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
		// 1. THE TRAP: Read the body bytes fully into memory
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		// 2. RESTORE the body stream so the Reverse Proxy can still use it!
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// 3. Parse the JSON to get the user's actual question
		var payload OpenAIRequest
		if err := json.Unmarshal(bodyBytes, &payload); err != nil || len(payload.Messages) == 0 {
			// If it's not a standard LLM request, just pass it through untouched
			next.ServeHTTP(w, r)
			return
		}

		// Extract the last message (the prompt the user just sent)
		prompt := payload.Messages[len(payload.Messages)-1].Content
		log.Printf("🧠 Intercepted Prompt: %q", prompt)

		// ---------------------------------------------------------
		// PHASE 2: THE SEMANTIC CACHE LOGIC
		// ---------------------------------------------------------

		// 4. Convert 'prompt' to a Vector using local Ollama (all-minilm)
		vector, err := GetEmbedding(prompt)
		if err != nil {
			log.Printf("⚠️ Failed to generate embedding: %v", err)
			// If embedding fails, we don't break the app. We just fall back to the real LLM.
			next.ServeHTTP(w, r)
			return
		}

		// 5. Query Qdrant
		isHit, cachedResponse := qc.Search(r.Context(), vector)
		if isHit {
			log.Println("⚡ CACHE HIT! Bypassing LLM and serving from memory.")
			w.Header().Set("Content-Type", "application/json")
			w.Write(cachedResponse)
			return // EXIT EARLY! We saved the company money!
		}

		// 6. CACHE MISS: Forward to the real LLM proxy
		log.Println("🐢 CACHE MISS. Forwarding to LLM provider...")

		// Create our interceptor
		capturer := &responseCapturer{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		// Pass the request to the proxy using our interceptor instead of the normal writer
		next.ServeHTTP(capturer, r)

		// 7. Save the captured response to Qdrant asynchronously
		// (so we don't block the user from getting their response)
		if capturer.statusCode == http.StatusOK {
			go func(v []float32, resp []byte) {
				if err := qc.Save(context.Background(), v, resp); err != nil {
					log.Printf("⚠️ Failed to save to Qdrant: %v", err)
				} else {
					log.Println("💾 Saved new response to Qdrant cache!")
				}
			}(vector, capturer.body.Bytes())
		}
	})
}
