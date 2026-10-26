package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type ShopDish struct {
	ID          int
	Name        string
	Description string
	PriceKobo   int
	ImageURL    string
	Category    string
}

type ShopPage struct {
	VendorID        int
	Name            string
	Description     string
	Area            string
	Address         string
	Phone           string
	PhoneLink       template.URL
	LogoURL         string
	Verified        bool
	OffersPickup    bool
	OffersDelivery  bool
	DeliveryFeeKobo int
	Completed       int
	Dishes          []ShopDish
	Categories      []string
}

func (h *Handlers) VendorShop(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var vendorID int
	var p ShopPage
	err := h.DB.QueryRow(`
		SELECT id, name, description, area, address, phone, logo_url,
		       verified, offers_pickup, offers_delivery, delivery_fee_kobo
		FROM vendors
		WHERE slug = $1 AND status = 'active' AND paid_until > NOW()
	`, slug).Scan(&vendorID, &p.Name, &p.Description, &p.Area, &p.Address, &p.Phone,
		&p.LogoURL, &p.Verified, &p.OffersPickup, &p.OffersDelivery, &p.DeliveryFeeKobo)
	p.VendorID = vendorID
	if err == sql.ErrNoRows {
		errors.Render(w, "We couldn't find that shop.", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("shop: vendor query failed: %v", err)
		errors.Render(w, "Could not load this shop", http.StatusInternalServerError)
		return
	}

	if p.Phone != "" && validPhone(p.Phone) {
		p.PhoneLink = template.URL("tel:" + strings.NewReplacer(" ", "", "-", "").Replace(p.Phone))
	}

	h.DB.QueryRow(
		"SELECT count(*) FROM orders WHERE vendor_id = $1 AND status = 'completed'",
		vendorID,
	).Scan(&p.Completed)

	rows, err := h.DB.Query(`
		SELECT id, name, COALESCE(description, ''), price_kobo,
		       COALESCE(image_url, ''), COALESCE(category, '')
		FROM menu_items
		WHERE vendor_id = $1
		ORDER BY category, name
	`, vendorID)
	if err != nil {
		log.Printf("shop: dishes query failed: %v", err)
		errors.Render(w, "Could not load this shop", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	seenCategories := make(map[string]bool)
	for rows.Next() {
		var d ShopDish
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.PriceKobo, &d.ImageURL, &d.Category); err != nil {
			log.Printf("shop: dishes scan failed: %v", err)
			errors.Render(w, "Could not read this shop", http.StatusInternalServerError)
			return
		}
		p.Dishes = append(p.Dishes, d)
		category := strings.TrimSpace(d.Category)
		if category != "" && !seenCategories[category] {
			seenCategories[category] = true
			p.Categories = append(p.Categories, category)
		}
	}

	tmpl, err := template.New("shop.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/shop.html")
	if err != nil {
		log.Printf("shop: template failed: %v", err)
		errors.Render(w, "Could not display this shop", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, p); err != nil {
		log.Printf("shop: execute failed: %v", err)
	}
}
