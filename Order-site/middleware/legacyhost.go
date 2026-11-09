package middleware

import (
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// The site changed its name and its address. Anyone who opens the old address is shown a short
// "we have moved" page that sends them to the same page on the new address.
//
// Settings (both optional):
//
//	LEGACY_HOSTS        old addresses, comma separated. Default grace-ordering-site.onrender.com.
//	                    Set it to "off" to switch the redirect off.
//	CANONICAL_BASE_URL  the new address. Default https://laikihub.onrender.com
const (
	defaultLegacyHost   = "grace-ordering-site.onrender.com"
	defaultCanonicalURL = "https://laikihub.onrender.com"
	legacyNewName       = "LaikiHub"
	legacyWaitSeconds   = "10"
)

// legacyPassThrough are addresses that other services call (Paystack sends customers back to the
// callbacks and posts to the webhook). They may still be set to the old address, so they must keep
// working there instead of being turned into a notice.
var legacyPassThrough = map[string]bool{
	"/payment/callback":        true,
	"/vendor/payment/callback": true,
	"/wallet/callback":         true,
	"/payment/webhook":         true,
	"/sw.js":                   true,
	"/healthz":                 true,
	"/manifest.webmanifest":    true,
}

// LegacyHost wraps the whole site. Requests for any other address go straight through.
func LegacyHost(next http.Handler) http.Handler {
	canonical := legacyCanonicalBase()
	legacy := legacyHostSet(canonical)
	if len(legacy) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !legacy[legacyRequestHost(r)] || legacyPassesThrough(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		target := canonical + r.URL.RequestURI()

		// Forms and other changes keep their method; the browser repeats them on the new address.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}

		// Pictures, scripts and data requests just move on. Only real page visits get the notice.
		wantsPage := strings.Contains(r.Header.Get("Accept"), "text/html") || r.Header.Get("Sec-Fetch-Dest") == "document"
		if !wantsPage {
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		legacyWriteNotice(w, canonical, target)
	})
}

func legacyPassesThrough(path string) bool {
	return legacyPassThrough[path] || strings.HasPrefix(path, "/static/")
}

func legacyRequestHost(r *http.Request) string {
	host := strings.ToLower(strings.TrimSpace(r.Host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

func legacyCanonicalBase() string {
	v := strings.TrimRight(strings.TrimSpace(os.Getenv("CANONICAL_BASE_URL")), "/")
	if strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://") {
		return v
	}
	return defaultCanonicalURL
}

func legacyHostSet(canonical string) map[string]bool {
	raw := strings.TrimSpace(os.Getenv("LEGACY_HOSTS"))
	if strings.EqualFold(raw, "off") {
		return map[string]bool{}
	}
	if raw == "" {
		raw = defaultLegacyHost
	}
	set := map[string]bool{}
	for _, h := range strings.Split(raw, ",") {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			set[h] = true
		}
	}
	// The new address must never be sent to itself.
	if u, err := url.Parse(canonical); err == nil {
		delete(set, strings.ToLower(u.Hostname()))
	}
	return set
}

func legacyWriteNotice(w http.ResponseWriter, canonical, target string) {
	shown := canonical
	if u, err := url.Parse(canonical); err == nil && u.Host != "" {
		shown = u.Host
	}
	page := strings.NewReplacer(
		"{{TARGET}}", html.EscapeString(target),
		"{{SHOWN}}", html.EscapeString(shown),
		"{{NAME}}", legacyNewName,
		"{{SECONDS}}", legacyWaitSeconds,
	).Replace(legacyNoticePage)

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}

const legacyNoticePage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<meta name="theme-color" content="#174a32">
<title>We’ve moved | {{NAME}}</title>
<link rel="icon" href="/static/img/favicon.svg" type="image/svg+xml">
<link rel="canonical" href="{{TARGET}}">
<meta http-equiv="refresh" content="{{SECONDS}};url={{TARGET}}">
<style>
*{box-sizing:border-box}
html,body{margin:0;min-height:100%}
body{display:grid;place-items:center;padding:24px;background:#fbfaf6;color:#1c2d24;font-family:Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;line-height:1.5}
.card{width:100%;max-width:460px;padding:34px 26px;border-radius:26px;background:#fff;text-align:center;box-shadow:0 18px 50px rgba(23,74,50,.14)}
.mark{width:72px;height:72px;border-radius:18px}
.tag{margin:18px 0 6px;color:#5b6a61;font-size:.78rem;font-weight:800;letter-spacing:.06em;text-transform:uppercase}
h1{margin:0 0 12px;color:#174a32;font-size:1.6rem;line-height:1.2;text-wrap:balance}
p{margin:0 0 14px;color:#3d4f44}
.url{display:inline-block;max-width:100%;overflow-wrap:anywhere;padding:10px 16px;border-radius:12px;background:#e7f1ea;color:#174a32;font-weight:800}
.btn{display:flex;align-items:center;justify-content:center;min-height:54px;margin:8px 0 14px;border-radius:14px;background:#174a32;color:#fff;font-weight:800;font-size:1.05rem;text-decoration:none;box-shadow:0 8px 20px rgba(23,74,50,.25)}
.btn:focus-visible{outline:3px solid #c2500e;outline-offset:2px}
.wait{font-size:.95rem}
.tip{margin:18px 0 0;padding:12px 14px;border-radius:14px;background:#f6f8f5;color:#3d4f44;font-size:.92rem;text-align:left}
</style>
</head>
<body>
<main class="card">
<img class="mark" src="/static/img/laikihub-mark.svg" width="72" height="72" alt="">
<p class="tag">We’ve rebranded</p>
<h1>We have a new name and a new link</h1>
<p>This site is now <strong>{{NAME}}</strong>. Please use the new link from now on:</p>
<p><span class="url">{{SHOWN}}</span></p>
<a id="go" class="btn" href="{{TARGET}}">Continue to {{NAME}}</a>
<p class="wait">Taking you there in <span id="n">{{SECONDS}}</span> seconds…</p>
<p class="tip">Added the app to your home screen? Remove the old icon, then open the new link and add the app again.</p>
</main>
<script>
(function () {
  var n = parseInt(document.getElementById('n').textContent, 10) || 10;
  var out = document.getElementById('n'), go = document.getElementById('go');
  var t = setInterval(function () {
    n -= 1;
    if (n <= 0) { clearInterval(t); location.href = go.href; return; }
    out.textContent = n;
  }, 1000);
}());
</script>
</body>
</html>
`
