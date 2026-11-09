package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okSite stands in for the real site: it answers "site", so a test can tell a request got through.
var okSite = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("site"))
})

func legacyRequest(method, host, path, accept string) *http.Request {
	r := httptest.NewRequest(method, "http://"+host+path, nil)
	r.Host = host
	if accept != "" {
		r.Header.Set("Accept", accept)
	}
	return r
}

func TestLegacyHostShowsNoticeOnOldAddress(t *testing.T) {
	t.Setenv("LEGACY_HOSTS", "")
	t.Setenv("CANONICAL_BASE_URL", "")
	h := LegacyHost(okSite)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "grace-ordering-site.onrender.com", "/menu?x=1", "text/html"))

	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "rebranded") {
		t.Fatalf("expected the notice page, got %d %q", w.Code, body)
	}
	if !strings.Contains(body, `href="https://laikihub.onrender.com/menu?x=1"`) {
		t.Fatalf("notice does not link to the same page on the new address: %s", body)
	}
}

func TestLegacyHostLeavesOtherAddressesAlone(t *testing.T) {
	t.Setenv("LEGACY_HOSTS", "")
	t.Setenv("CANONICAL_BASE_URL", "")
	h := LegacyHost(okSite)

	for _, host := range []string{"laikihub.onrender.com", "localhost:8080", "example.com"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, legacyRequest("GET", host, "/menu", "text/html"))
		if w.Body.String() != "site" {
			t.Fatalf("%s should reach the site, got %q", host, w.Body.String())
		}
	}
}

func TestLegacyHostKeepsPaymentAddressesWorking(t *testing.T) {
	t.Setenv("LEGACY_HOSTS", "")
	t.Setenv("CANONICAL_BASE_URL", "")
	h := LegacyHost(okSite)

	for _, path := range []string{"/payment/callback", "/vendor/payment/callback", "/wallet/callback", "/static/css/ui.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, legacyRequest("GET", "grace-ordering-site.onrender.com", path, "text/html"))
		if w.Body.String() != "site" {
			t.Fatalf("%s must keep working on the old address, got %q", path, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("POST", "grace-ordering-site.onrender.com", "/payment/webhook", ""))
	if w.Body.String() != "site" {
		t.Fatalf("the payment webhook must keep working on the old address, got %d %q", w.Code, w.Body.String())
	}
}

func TestLegacyHostRedirectsOtherRequests(t *testing.T) {
	t.Setenv("LEGACY_HOSTS", "")
	t.Setenv("CANONICAL_BASE_URL", "")
	h := LegacyHost(okSite)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("POST", "grace-ordering-site.onrender.com", "/login", ""))
	if w.Code != http.StatusPermanentRedirect || w.Header().Get("Location") != "https://laikihub.onrender.com/login" {
		t.Fatalf("POST: got %d %q", w.Code, w.Header().Get("Location"))
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "grace-ordering-site.onrender.com", "/api/data", "application/json"))
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "https://laikihub.onrender.com/api/data" {
		t.Fatalf("non-page GET: got %d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestLegacyHostSettingsAndSafety(t *testing.T) {
	// Custom addresses
	t.Setenv("LEGACY_HOSTS", "Old.Example.com, other.example.com")
	t.Setenv("CANONICAL_BASE_URL", "https://new.example.com/")
	h := LegacyHost(okSite)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "old.example.com:443", "/a", "text/html"))
	if !strings.Contains(w.Body.String(), `href="https://new.example.com/a"`) {
		t.Fatalf("custom settings not used: %s", w.Body.String())
	}

	// The new address listed by mistake as an old one must not loop
	t.Setenv("LEGACY_HOSTS", "new.example.com")
	h = LegacyHost(okSite)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "new.example.com", "/a", "text/html"))
	if w.Body.String() != "site" {
		t.Fatalf("the new address must never redirect to itself, got %q", w.Body.String())
	}

	// "off" switches it off
	t.Setenv("LEGACY_HOSTS", "off")
	h = LegacyHost(okSite)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "grace-ordering-site.onrender.com", "/", "text/html"))
	if w.Body.String() != "site" {
		t.Fatalf("LEGACY_HOSTS=off should switch the redirect off, got %q", w.Body.String())
	}
}

func TestLegacyHostCannotBeUsedToRedirectElsewhere(t *testing.T) {
	t.Setenv("LEGACY_HOSTS", "")
	t.Setenv("CANONICAL_BASE_URL", "")
	h := LegacyHost(okSite)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, legacyRequest("GET", "grace-ordering-site.onrender.com", "//evil.example.com/x", "application/json"))
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://laikihub.onrender.com/") {
		t.Fatalf("redirect must always stay on the new address, got %q", loc)
	}
}
