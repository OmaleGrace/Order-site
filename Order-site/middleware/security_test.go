package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

func do(h http.Handler, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("a=b"))
	req.Host = "example.com"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestSameOrigin(t *testing.T) {
	h := SameOrigin(ok)
	cases := []struct {
		name   string
		method string
		path   string
		hdr    map[string]string
		want   int
	}{
		{"get is always fine", "GET", "/menu", nil, 200},
		{"post same-origin", "POST", "/cart/add", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"post cross-site", "POST", "/cart/add", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"post same-site other subdomain", "POST", "/cart/add", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"old browser matching origin", "POST", "/cart/add", map[string]string{"Origin": "https://example.com"}, 200},
		{"old browser other origin", "POST", "/cart/add", map[string]string{"Origin": "https://evil.com"}, 403},
		{"old browser origin null", "POST", "/cart/add", map[string]string{"Origin": "null"}, 403},
		{"old browser referer match", "POST", "/cart/add", map[string]string{"Referer": "https://example.com/cart"}, 200},
		{"no headers at all", "POST", "/cart/add", nil, 403},
		{"paystack webhook has no browser headers", "POST", "/payment/webhook", nil, 200},
	}
	for _, c := range cases {
		if got := do(h, c.method, c.path, c.hdr).Code; got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Setenv("ADSENSE_PUBLISHER_ID", "")
	h := SecurityHeaders(ok)
	w := do(h, "GET", "/login", map[string]string{"X-Forwarded-Proto": "https"})
	for _, k := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy", "Strict-Transport-Security"} {
		if w.Header().Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Error("CSP lacks frame-ancestors")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("login page should not be cached")
	}
	if do(h, "GET", "/login", nil).Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be sent over plain http")
	}
	if strings.Contains(w.Header().Get("Content-Security-Policy"), "googlesyndication") {
		t.Error("ads must not be allowed when AdSense is not configured")
	}
}

func TestAdSenseCSPOnlyOnContentPages(t *testing.T) {
	t.Setenv("ADSENSE_PUBLISHER_ID", "pub-1234567890123456")
	h := SecurityHeaders(ok)
	for _, p := range []string{"/", "/menu", "/about", "/guides/ordering-food-safely", "/v/mama-put"} {
		if !strings.Contains(do(h, "GET", p, nil).Header().Get("Content-Security-Policy"), "googlesyndication") {
			t.Errorf("%s should allow ads", p)
		}
	}
	for _, p := range []string{"/login", "/signup", "/cart", "/checkout/details", "/admin/orders", "/vendor/billing", "/account", "/order-success", "/forgot-password"} {
		if strings.Contains(do(h, "GET", p, nil).Header().Get("Content-Security-Policy"), "googlesyndication") {
			t.Errorf("%s must NOT allow ad code", p)
		}
	}
	t.Setenv("ADSENSE_PUBLISHER_ID", "not-an-id")
	if strings.Contains(do(h, "GET", "/", nil).Header().Get("Content-Security-Policy"), "googlesyndication") {
		t.Error("invalid publisher id must not enable ads")
	}
}

func TestLimitBody(t *testing.T) {
	h := LimitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 2<<20)
		n, err := r.Body.Read(buf)
		_ = n
		for err == nil {
			_, err = r.Body.Read(buf)
		}
		if err != nil && err.Error() != "EOF" {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/login", strings.NewReader(strings.Repeat("x", 2<<20)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("2MB body to /login should be refused, got %d", w.Code)
	}
	req = httptest.NewRequest("POST", "/login", strings.NewReader("a=b"))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("small body should pass, got %d", w.Code)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if !l.Hit("a") {
			t.Fatalf("hit %d should be allowed", i)
		}
	}
	if l.Hit("a") {
		t.Error("4th hit should be refused")
	}
	if !l.Blocked("a") {
		t.Error("a should be blocked")
	}
	if l.Blocked("b") {
		t.Error("b is a different key")
	}
	time.Sleep(60 * time.Millisecond)
	if l.Blocked("a") {
		t.Error("window should have expired")
	}
	l.Hit("c")
	l.Hit("c")
	l.Hit("c")
	l.Reset("c")
	if l.Blocked("c") {
		t.Error("reset should clear the key")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9")
	t.Setenv("TRUSTED_PROXY_COUNT", "1")
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Errorf("got %s; the visitor must not be able to choose their own address", got)
	}
	t.Setenv("TRUSTED_PROXY_COUNT", "0")
	if got := ClientIP(r); got != "10.0.0.5" {
		t.Errorf("with no proxy the socket address is used, got %s", got)
	}
	t.Setenv("TRUSTED_PROXY_COUNT", "1")
	r.Header.Set("X-Forwarded-For", "garbage")
	if got := ClientIP(r); got != "10.0.0.5" {
		t.Errorf("bad header falls back to socket, got %s", got)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	h := RateLimit(l, func(w http.ResponseWriter, r *http.Request) {})
	codes := []int{}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = "1.2.3.4:5"
		w := httptest.NewRecorder()
		h(w, req)
		codes = append(codes, w.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Errorf("got %v", codes)
	}
}

func TestHashTokenStable(t *testing.T) {
	if HashToken("abc") != HashToken("abc") || HashToken("abc") == HashToken("abd") || HashToken("abc") == "abc" {
		t.Error("HashToken misbehaves")
	}
}
