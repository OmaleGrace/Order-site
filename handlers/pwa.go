package handlers

import (
	"fmt"
	"html"
	"net/http"

	"Order-site/brand"
)

// ServiceWorker is served from the site root so it can control every page.
func (h *Handlers) ServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	http.ServeFile(w, r, "static/sw.js")
}

// WebManifest lets phones offer "Add to home screen".
func (h *Handlers) WebManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	http.ServeFile(w, r, "static/manifest.webmanifest")
}

// OfflinePage is what the service worker shows when the phone has no connection.
func (h *Handlers) OfflinePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>You're offline</title>
<style>body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;background:#f6f8f5;color:#202820;text-align:center;padding:24px}
h1{color:#174a32;margin:0 0 8px}button{margin-top:16px;background:#174a32;color:#fff;border:0;border-radius:10px;padding:12px 20px;font-size:1rem}</style></head>
<body><main><h1>You're offline</h1><p>%s needs an internet connection to show shops and take orders. Check your network and try again.</p><button onclick="location.reload()">Try again</button></main></body></html>`, html.EscapeString(brand.Name()))
}
