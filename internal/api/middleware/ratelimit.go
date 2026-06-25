package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a simple per-IP token-bucket rate limiter for login endpoints.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*bucket
	limit    int           // max attempts per window
	window   time.Duration // sliding window
}

type bucket struct {
	count    int
	resetAt  time.Time
}

// NewRateLimiter creates a rate limiter. limit is the max number of attempts
// within the window duration.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*bucket),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

// LimitLogin returns a middleware that rate-limits login attempts per remote IP.
// Responds with 429 Too Many Requests when the limit is exceeded.
func (rl *RateLimiter) LimitLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		rl.mu.Lock()
		b, ok := rl.attempts[ip]
		if !ok || time.Now().After(b.resetAt) {
			b = &bucket{resetAt: time.Now().Add(rl.window)}
			rl.attempts[ip] = b
		}
		b.count++
		remaining := rl.limit - b.count
		rl.mu.Unlock()
		if remaining < 0 {
			http.Error(w, `{"error":"请求过于频繁，请稍后再试"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// clientIP extracts the real client IP, preferring X-Forwarded-For / X-Real-IP
// over RemoteAddr when behind a reverse proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.attempts {
			if now.After(b.resetAt) {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}
