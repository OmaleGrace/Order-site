package middleware

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limiter counts events per key (an IP address, an email...) inside a time window.
// It lives in memory, which is right for a single small server: it needs no
// database and its counters reset when the server restarts.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
	last   time.Time
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

// prune drops old hits for one key and, now and then, every idle key.
func (l *Limiter) prune(key string, now time.Time) {
	cut := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
	} else {
		l.hits[key] = kept
	}

	if now.Sub(l.last) > l.window {
		l.last = now
		for k, ts := range l.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(cut) {
				delete(l.hits, k)
			}
		}
	}
}

// Blocked says whether the key has already used up its allowance. It records nothing.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key, time.Now())
	return len(l.hits[key]) >= l.limit
}

// Hit records one event. It returns false when the key is over its allowance.
func (l *Limiter) Hit(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.prune(key, now)
	if len(l.hits[key]) >= l.limit {
		return false
	}
	l.hits[key] = append(l.hits[key], now)
	return true
}

// Reset forgets a key, for example after a correct login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// ClientIP works out the visitor's address. On Render the server sits behind
// a proxy that appends the real address to X-Forwarded-For, so the address is
// read from the right-hand end (a visitor can fake the left-hand entries but
// not the one the proxy adds). TRUSTED_PROXY_COUNT says how many proxies sit
// in front of the app (default 1; use 0 when running with no proxy).
func ClientIP(r *http.Request) string {
	hops := 1
	if v := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_COUNT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 5 {
			hops = n
		}
	}

	if hops > 0 {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			idx := len(parts) - hops
			if idx >= 0 {
				if ip := net.ParseIP(strings.TrimSpace(parts[idx])); ip != nil {
					return ip.String()
				}
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimit wraps a handler so one visitor (by IP) can only call it so often.
func RateLimit(l *Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.Hit(ClientIP(r)) {
			w.Header().Set("Retry-After", "900")
			http.Error(w, "Too many requests. Please wait a few minutes and try again.", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// Shared limiters. Each is a ceiling per visitor address; the login and
// reset code add stricter per-account limits on top.
var (
	LoginIPLimiter    = NewLimiter(40, 15*time.Minute)
	SignupIPLimiter   = NewLimiter(30, time.Hour)
	ForgotIPLimiter   = NewLimiter(20, time.Hour)
	ResetIPLimiter    = NewLimiter(30, time.Hour)
	OTPSendUser       = NewLimiter(3, time.Hour)
	OTPSendIP         = NewLimiter(10, time.Hour)
	OTPVerifyUser     = NewLimiter(10, time.Hour)
	ApplyLimiter      = NewLimiter(10, time.Hour)
	VendorAILimiter   = NewLimiter(10, time.Hour)
	PayLimiter        = NewLimiter(20, time.Hour)
	LoginFailsByEmail = NewLimiter(5, 15*time.Minute)
	ForgotByEmail     = NewLimiter(3, time.Hour)
)
