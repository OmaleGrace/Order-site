package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type ShopCard struct {
	Name            string
	Slug            string
	Description     string
	Area            string
	LogoURL         string
	Verified        bool
	OffersPickup    bool
	OffersDelivery  bool
	DeliveryFeeKobo int
	Dishes          int
	Completed       int
}

type VendorsPage struct {
	Shops    []ShopCard
	Query    string
	Service  string
	Sort     string
	Verified bool
}

func shorten(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

func (h *Handlers) VendorsList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	service := r.URL.Query().Get("service")
	if service != "pickup" && service != "delivery" {
		service = ""
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy != "name" && sortBy != "dishes" && sortBy != "completed" {
		sortBy = "recommended"
	}
	verifiedOnly := r.URL.Query().Get("verified") == "1"
	if len([]rune(q)) > 60 {
		q = string([]rune(q)[:60])
	}

	query := `
		SELECT v.name, v.slug, v.description, v.area, v.logo_url, v.verified,
		       v.offers_pickup, v.offers_delivery, v.delivery_fee_kobo,
		       (SELECT count(*) FROM menu_items m WHERE m.vendor_id = v.id) AS dishes,
		       (SELECT count(*) FROM orders o WHERE o.vendor_id = v.id AND o.status = 'completed') AS completed
		FROM vendors v
		WHERE v.status = 'active' AND v.paid_until > NOW()
		  AND EXISTS (SELECT 1 FROM menu_items m WHERE m.vendor_id = v.id)
	`
	var args []interface{}
	if q != "" {
		query += " AND (v.name ILIKE $1 OR v.area ILIKE $1 OR v.description ILIKE $1)"
		args = append(args, "%"+q+"%")
	}
	if service == "pickup" {
		query += " AND v.offers_pickup = TRUE"
	}
	if service == "delivery" {
		query += " AND v.offers_delivery = TRUE"
	}
	if verifiedOnly {
		query += " AND v.verified = TRUE"
	}
	switch sortBy {
	case "name":
		query += " ORDER BY v.name ASC"
	case "dishes":
		query += " ORDER BY dishes DESC, v.name ASC"
	case "completed":
		query += " ORDER BY completed DESC, v.name ASC"
	default:
		query += " ORDER BY v.verified DESC, completed DESC, v.name"
	}

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		log.Printf("vendors list: query failed: %v", err)
		errors.Render(w, "Could not load the shops", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := VendorsPage{Query: q, Service: service, Sort: sortBy, Verified: verifiedOnly}
	for rows.Next() {
		var s ShopCard
		if err := rows.Scan(&s.Name, &s.Slug, &s.Description, &s.Area, &s.LogoURL, &s.Verified,
			&s.OffersPickup, &s.OffersDelivery, &s.DeliveryFeeKobo, &s.Dishes, &s.Completed); err != nil {
			log.Printf("vendors list: scan failed: %v", err)
			errors.Render(w, "Could not read the shops", http.StatusInternalServerError)
			return
		}
		s.Description = shorten(s.Description, 110)
		page.Shops = append(page.Shops, s)
	}
	if err := rows.Err(); err != nil {
		log.Printf("vendors list: rows failed: %v", err)
		errors.Render(w, "Could not read the shops", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("vendors.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/vendors.html")
	if err != nil {
		log.Printf("vendors list: template failed: %v", err)
		errors.Render(w, "Could not display the shops", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("vendors list: execute failed: %v", err)
	}
}
