package ratelimit

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// MaxRequestsPerMinute is our quota. Set it to 5 for easy testing!
	MaxRequestsPerMinute = 5 
)

// Limiter holds our Redis connection
type Limiter struct {
	client *redis.Client
}

// NewLimiter connects to Redis
func NewLimiter(host string, port string) (*Limiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port),
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("🚦 Connected to Redis Rate Limiter")
	return &Limiter{client: client}, nil
}

// Middleware intercepts the request and checks the quota before passing it on
func (rl *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract the Token from the headers
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Define our Fixed Time Window (Current Minute)
		currentMinute := time.Now().Unix() / 60
		redisKey := fmt.Sprintf("ratelimit:%s:%d", token, currentMinute)

		// 3. Increment the counter in Redis
		ctx := r.Context()
		count, err := rl.client.Incr(ctx, redisKey).Result()
		if err != nil {
			log.Printf("⚠️ Redis error, failing open: %v", err)
			next.ServeHTTP(w, r) // If Redis crashes, don't bring down the whole API
			return
		}

		// 4. Set an expiration so our Redis memory doesn't fill up with old keys
		if count == 1 {
			rl.client.Expire(ctx, redisKey, time.Minute)
		}

		// 5. The Block: Check if they exceeded the quota
		if count > MaxRequestsPerMinute {
			log.Printf("🚫 Rate Limit Exceeded for token: %s", token)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "Rate limit exceeded. Try again in a minute."}`))
			return // DROP THE REQUEST
		}

		// 6. Quota OK: Pass to the next middleware (which is our Qdrant Cache!)
		next.ServeHTTP(w, r)
	})
}