package middleware

import (
	"math"
	"net"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a simple per-IP token-bucket rate limiter for login endpoints.
type RateLimiter struct {
	mu           sync.Mutex
	attempts     map[string]*bucket
	limit        int           // max attempts per window
	window       time.Duration // sliding window
	trustProxy   bool          // only trust X-Forwarded-For / X-Real-IP when behind a trusted reverse proxy
	trustedCIDRs []*net.IPNet  // allowed proxy CIDRs when trustProxy is true
	stopCh       chan struct{} // closed by Shutdown to stop the cleanup goroutine
}

type bucket struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a rate limiter. limit is the max number of attempts
// within the window duration. trustedProxyCIDRs, if non-empty, are the set of
// reverse-proxy CIDRs whose X-Forwarded-For / X-Real-IP headers will be honored.
func NewRateLimiter(limit int, window time.Duration, trustedProxyCIDRs ...string) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*bucket),
		limit:    limit,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	if len(trustedProxyCIDRs) > 0 {
		rl.trustProxy = true
		for _, cidr := range trustedProxyCIDRs {
			_, n, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			rl.trustedCIDRs = append(rl.trustedCIDRs, n)
		}
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

// Shutdown stops the cleanup goroutine. Call once when the rate limiter is no
// longer needed. Safe to call multiple times (idempotent).
func (rl *RateLimiter) Shutdown() {
	select {
	case <-rl.stopCh:
		// already closed
	default:
		close(rl.stopCh)
	}
}

// LimitLogin returns a middleware that rate-limits login attempts per remote IP.
// Responds with 429 Too Many Requests when the limit is exceeded.
func (rl *RateLimiter) LimitLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := rl.clientIP(r)
		rl.mu.Lock()
		b, ok := rl.attempts[ip]
		if !ok || time.Now().After(b.resetAt) {
			b = &bucket{resetAt: time.Now().Add(rl.window)}
			rl.attempts[ip] = b
		}
		b.count++
		remaining := rl.limit - b.count
		if remaining < 0 {
			retryAfter := strconv.Itoa(int(math.Ceil(time.Until(b.resetAt).Seconds())))
			rl.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", retryAfter)
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte("{\"error\":\"请求过于频繁，请稍后再试\"}")); err != nil {
				slog.Warn("ratelimit write failed", "err", err)
			}
			return
		}
		rl.mu.Unlock()
		next(w, r)
	}
}

// isTrustedProxy checks whether remoteAddr matches a trusted proxy CIDR.
func (rl *RateLimiter) isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range rl.trustedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP extracts the real client IP. When trustProxy is enabled, X-Forwarded-For
// and X-Real-IP headers are only honored if the immediate remote peer is a trusted
// proxy 鈥?otherwise RemoteAddr is used directly, preventing IP spoofing.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustProxy && rl.isTrustedProxy(r.RemoteAddr) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
				return ip
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
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
	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
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
}
