package middleware

import (
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var publisherPattern = regexp.MustCompile(`^pub-[0-9]{10,20}$`)

// adsenseEnabled is true once a valid ADSENSE_PUBLISHER_ID is configured.
func adsenseEnabled() bool {
	return publisherPattern.MatchString(strings.TrimSpace(os.Getenv("ADSENSE_PUBLISHER_ID")))
}

// pages that may show ads. Everything else (login, checkout, admin, vendor
// tools, account pages) keeps the strict policy and never loads ad code.
func isAdPage(path string) bool {
	switch {
	case path == "/", path == "/menu", path == "/vendors", path == "/about",
		path == "/contact", path == "/privacy", path == "/terms", path == "/guides":
		return true
	case strings.HasPrefix(path, "/guides/"), strings.HasPrefix(path, "/v/"):
		return true
	}
	return false
}

// pages that show personal information and must never be kept in a cache
func isPrivatePage(path string) bool {
	for _, p := range []string{
		"/account", "/orders", "/cart", "/checkout", "/admin", "/vendor",
		"/favorites", "/become-a-vendor", "/login", "/signup",
		"/forgot-password", "/reset-password", "/order-success", "/payment",
	} {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

const (
	strictCSP = "default-src 'self'; " +
		"img-src 'self' https: data:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"connect-src 'self'; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"form-action 'self' https://checkout.paystack.com https://*.paystack.com"

	// Google AdSense needs to load its script, show ad frames and report back.
	adsenseCSP = "default-src 'self'; " +
		"img-src 'self' https: data:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"script-src 'self' 'unsafe-inline' https://pagead2.googlesyndication.com https://*.googlesyndication.com https://*.google.com https://*.gstatic.com https://*.doubleclick.net https://partner.googleadservices.com https://www.googletagservices.com https://adservice.google.com; " +
		"frame-src https://*.googlesyndication.com https://*.doubleclick.net https://*.google.com; " +
		"connect-src 'self' https://*.googlesyndication.com https://*.google.com https://*.doubleclick.net https://*.gstatic.com; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"form-action 'self' https://checkout.paystack.com https://*.paystack.com"
)

// SecurityHeaders adds the browser protections to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")

		csp := strictCSP
		if adsenseEnabled() && isAdPage(r.URL.Path) {
			csp = adsenseCSP
		}
		h.Set("Content-Security-Policy", csp)

		// Tell browsers to use HTTPS only. Sent only when the visit really is HTTPS,
		// so local testing over plain http is not affected.
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		if isPrivatePage(r.URL.Path) {
			h.Set("Cache-Control", "no-store")
		}

		next.ServeHTTP(w, r)
	})
}

// LimitBody stops anyone from sending a huge request body.
// The profile-picture upload is allowed a little more.
func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(1 << 20) // 1 MB
		if r.URL.Path == "/account/edit" {
			limit = 6 << 20
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// SameOrigin blocks forged cross-site form posts (CSRF). Browsers say where a
// request came from; any request that changes data must come from this site.
// Paystack's server-to-server webhook has no browser headers and is verified
// by its own signature instead, so it is let through here.
func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/payment/webhook" {
			next.ServeHTTP(w, r)
			return
		}

		// Modern browsers always send this header.
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
			if site == "same-origin" || site == "none" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Cross-site request blocked", http.StatusForbidden)
			return
		}

		// Older browsers: compare the Origin (or Referer) with this site's host.
		src := r.Header.Get("Origin")
		if src == "" || src == "null" {
			src = r.Header.Get("Referer")
		}
		if src == "" {
			http.Error(w, "Cross-site request blocked", http.StatusForbidden)
			return
		}
		u, err := url.Parse(src)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			http.Error(w, "Cross-site request blocked", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
