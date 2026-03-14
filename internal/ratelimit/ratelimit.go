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

// tokenBucketLua implements an atomic O(1) Token Bucket rate limiter.
// Returns an array: {allowed (1 or 0), remaining_tokens, reset_timestamp_ms}
var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])

local fill_rate = capacity / window_ms

local bucket = redis.call('HMGET', key, 'tokens', 'last_update')
local tokens = tonumber(bucket[1])
local last_update = tonumber(bucket[2])

if not tokens then
	tokens = capacity
	last_update = now_ms
else
	local elapsed_ms = math.max(0, now_ms - last_update)
	local generated = elapsed_ms * fill_rate
	tokens = math.min(capacity, tokens + generated)
end

local allowed = 0
if tokens >= 1 then
	tokens = tokens - 1
	allowed = 1
	redis.call('HMSET', key, 'tokens', tokens, 'last_update', now_ms)
	redis.call('PEXPIRE', key, window_ms)
end

local remaining = math.floor(tokens)
local reset_ms = now_ms
if remaining < capacity then
	local missing = capacity - remaining
	reset_ms = now_ms + math.ceil(missing / fill_rate)
end

return {allowed, remaining, reset_ms}
`)

// Limiter holds our Redis connection
type Limiter struct {
	client      *redis.Client
	maxRequests int64
	window      time.Duration
	now         func() time.Time
	evaluate    func(context.Context, string, int64, time.Duration, int64) (int64, int64, int64, error)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("🚦 Connected to Redis Rate Limiter (Token Bucket)")

	rl := &Limiter{
		client:      client,
		maxRequests: opts.MaxRequests,
		window:      opts.Window,
		now:         time.Now,
	}

	rl.evaluate = func(ctx context.Context, key string, capacity int64, window time.Duration, nowMs int64) (int64, int64, int64, error) {
		res, err := tokenBucketLua.Run(ctx, rl.client, []string{key}, capacity, int64(window/time.Millisecond), nowMs).Result()
		if err != nil {
			return 0, 0, 0, err
		}

		vals := res.([]interface{})
		allowed := vals[0].(int64)
		remaining := vals[1].(int64)
		resetMs := vals[2].(int64)

		return allowed, remaining, resetMs, nil
	}

	return rl, nil
}

// Middleware intercepts the request and checks the quota before passing it on
func (rl *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		tokenHash := hashToken(token)

		now := rl.now().UTC()
		redisKey := fmt.Sprintf("ratelimit:tb:%s", tokenHash)

		ctx := r.Context()
		allowed, remaining, resetMs, err := rl.evaluate(ctx, redisKey, rl.maxRequests, rl.window, now.UnixMilli())
		if err != nil {
			log.Printf("⚠️ Redis error, failing open: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		resetTime := time.UnixMilli(resetMs)
		retryAfter := int(resetTime.Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}

		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(rl.maxRequests, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if allowed == 0 {
			log.Printf("🚫 Rate Limit Exceeded for token hash: %s", tokenHash)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "Rate limit exceeded. Try again shortly."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:16])
}
