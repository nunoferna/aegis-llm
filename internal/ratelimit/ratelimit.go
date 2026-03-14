package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultMaxRequestsPerWindow int64 = 5
	DefaultWindow                     = time.Minute
)

var rateLimitLua = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

// Limiter holds our Redis connection
type Limiter struct {
	client      *redis.Client
	maxRequests int64
	window      time.Duration
	now         func() time.Time
	increment   func(context.Context, string, time.Duration) (int64, error)
}

type Options struct {
	MaxRequests int64
	Window      time.Duration
}

// NewLimiter connects to Redis
func NewLimiter(host string, port string) (*Limiter, error) {
	return NewLimiterWithOptions(host, port, Options{})
}

// NewLimiterWithOptions connects to Redis and allows tuning enforcement policy.
func NewLimiterWithOptions(host string, port string, opts Options) (*Limiter, error) {
	if opts.MaxRequests <= 0 {
		opts.MaxRequests = DefaultMaxRequestsPerWindow
	}
	if opts.Window <= 0 {
		opts.Window = DefaultWindow
	}

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

	rl := &Limiter{
		client:      client,
		maxRequests: opts.MaxRequests,
		window:      opts.Window,
		now:         time.Now,
	}
	rl.increment = func(ctx context.Context, key string, window time.Duration) (int64, error) {
		return rateLimitLua.Run(ctx, rl.client, []string{key}, int64(window/time.Millisecond)).Int64()
	}

	return rl, nil
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
		tokenHash := hashToken(token)

		// 2. Define the active time window.
		now := rl.now().UTC()
		windowStart := now.Truncate(rl.window)
		redisKey := fmt.Sprintf("ratelimit:%s:%d", tokenHash, windowStart.Unix())
		nextWindow := windowStart.Add(rl.window)
		retryAfter := int(nextWindow.Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}

		// 3. Increment the counter in Redis and apply TTL atomically.
		ctx := r.Context()
		count, err := rl.increment(ctx, redisKey, rl.window)
		if err != nil {
			log.Printf("⚠️ Redis error, failing open: %v", err)
			next.ServeHTTP(w, r) // If Redis crashes, don't bring down the whole API
			return
		}

		remaining := rl.maxRequests - count
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.maxRequests, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(nextWindow.Unix(), 10))

		// 5. The Block: Check if they exceeded the quota
		if count > rl.maxRequests {
			log.Printf("🚫 Rate Limit Exceeded for token hash: %s", tokenHash)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "Rate limit exceeded. Try again in a minute."}`))
			return // DROP THE REQUEST
		}

		// 6. Quota OK: Pass to the next middleware (which is our Qdrant Cache!)
		next.ServeHTTP(w, r)
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	// 16 bytes is enough entropy for keying while keeping Redis keys compact.
	return hex.EncodeToString(sum[:16])
}
