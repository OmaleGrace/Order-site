package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type VendorProfilePage struct {
	Name             string
	Description      string
	Area             string
	Address          string
	Phone            string
	LogoURL          string
	OffersPickup     bool
	OffersDelivery   bool
	DeliveryFeeNaira string
	Notice           string
}

func (h *Handlers) VendorProfile(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	var p VendorProfilePage
	var feeKobo int
	err := h.DB.QueryRow(`
		SELECT name, description, area, address, phone, logo_url,
		       offers_pickup, offers_delivery, delivery_fee_kobo
		FROM vendors WHERE id = $1
	`, vendorID).Scan(&p.Name, &p.Description, &p.Area, &p.Address, &p.Phone,
		&p.LogoURL, &p.OffersPickup, &p.OffersDelivery, &feeKobo)
	if err != nil {
		log.Printf("vendor profile: query failed: %v", err)
		errors.Render(w, "Could not load your shop details", http.StatusInternalServerError)
		return
	}

	p.DeliveryFeeNaira = fmt.Sprintf("%.2f", float64(feeKobo)/100)
	p.Notice = r.URL.Query().Get("msg")

	tmpl, err := brand.ParseE("templates/vendor-profile.html")
	if err != nil {
		log.Printf("vendor profile: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, p); err != nil {
		log.Printf("vendor profile: execute failed: %v", err)
	}
}

func validPhone(s string) bool {
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == ' ' || r == '-':
		default:
			return false
		}
	}
	return digits >= 7 && digits <= 15
}

func (h *Handlers) VendorProfileSave(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	back := func(msg string) {
		http.Redirect(w, r, "/vendor/profile?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	description := strings.TrimSpace(r.FormValue("description"))
	area := strings.TrimSpace(r.FormValue("area"))
	address := strings.TrimSpace(r.FormValue("address"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	logo := strings.TrimSpace(r.FormValue("logo_url"))
	pickup := r.FormValue("offers_pickup") == "on"
	delivery := r.FormValue("offers_delivery") == "on"

	if len([]rune(description)) > 300 {
		back("Description must be 300 characters or fewer")
		return
	}
	if len([]rune(area)) > 60 {
		back("Area must be 60 characters or fewer")
		return
	}
	if len([]rune(address)) > 150 {
		back("Address must be 150 characters or fewer")
		return
	}
	if phone != "" {
		normalized := normalizeNigerianPhone(phone)
		if normalized == "" {
			back("Enter a valid Nigerian phone number: 11 digits starting with 0 (for example 08012345678), or starting with +234")
			return
		}
		phone = normalized
	}
	if msg := validImageURL(logo); msg != "" {
		back(msg)
		return
	}
	if !pickup && !delivery {
		back("Choose at least one way customers can get their food: pickup or delivery")
		return
	}

	feeKobo := 0
	if delivery {
		raw := strings.TrimSpace(r.FormValue("delivery_fee"))
		if raw == "" {
			raw = "0"
		}
		naira, err := strconv.ParseFloat(raw, 64)
		if err != nil || naira < 0 || naira > 100000 {
			back("Enter a valid delivery fee in naira (0 for free delivery)")
			return
		}
		feeKobo = int(math.Round(naira * 100))
	}

	_, err := h.DB.Exec(`
		UPDATE vendors
		SET description = $1, area = $2, address = $3, phone = $4, logo_url = $5,
		    offers_pickup = $6, offers_delivery = $7, delivery_fee_kobo = $8
		WHERE id = $9
	`, description, area, address, phone, logo, pickup, delivery, feeKobo, vendorID)
	if err != nil {
		log.Printf("vendor profile: save failed: %v", err)
		errors.Render(w, "Could not save your shop details", http.StatusInternalServerError)
		return
	}

	back("Shop details saved")
}
