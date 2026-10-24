package handlers

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var publisherIDPattern = regexp.MustCompile(`^pub-[0-9]{10,20}$`)

// siteBase returns the public address of the site, without a trailing slash.
func siteBase(r *http.Request) string {
	if b := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/"); b != "" {
		return b
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func (h *Handlers) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, `User-agent: Mediapartners-Google
Allow: /

User-agent: *
Disallow: /admin/
Disallow: /vendor/
Disallow: /account
Disallow: /cart
Disallow: /checkout
Disallow: /orders
Disallow: /order-success
Disallow: /favorites
Disallow: /payment/
Disallow: /auth/
Disallow: /login
Disallow: /signup
Disallow: /forgot-password
Disallow: /reset-password
Disallow: /become-a-vendor

Sitemap: %s/sitemap.xml
`, siteBase(r))
}

func (h *Handlers) SitemapXML(w http.ResponseWriter, r *http.Request) {
	base := siteBase(r)
	paths := []string{
		"/", "/menu", "/vendors", "/about", "/guides",
		"/guides/nigerian-rice-dishes",
		"/guides/ordering-food-safely",
		"/guides/pickup-or-delivery",
		"/contact", "/privacy", "/terms",
	}

	rows, err := h.DB.Query(`
		SELECT v.slug
		FROM vendors v
		WHERE v.status = 'active' AND v.paid_until > NOW()
		  AND EXISTS (SELECT 1 FROM menu_items m WHERE m.vendor_id = v.id)
		ORDER BY v.slug
	`)
	if err != nil {
		log.Printf("sitemap: vendors query failed: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var slug string
			if rows.Scan(&slug) == nil {
				paths = append(paths, "/v/"+slug)
			}
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range paths {
		b.WriteString("  <url><loc>" + html.EscapeString(base+p) + "</loc></url>\n")
	}
	b.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(b.String()))
}

// AdsTxt serves the ads.txt file once ADSENSE_PUBLISHER_ID is set (for example pub-1234567890123456).
func (h *Handlers) AdsTxt(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(os.Getenv("ADSENSE_PUBLISHER_ID"))
	if !publisherIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "google.com, %s, DIRECT, f08c47fec0942fa0\n", id)
}
