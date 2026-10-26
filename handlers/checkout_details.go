package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
	"Order-site/cart"
	"Order-site/errors"
	"Order-site/middleware"
)

type CheckoutLine struct {
	Name     string
	Quantity int
	LineKobo int
}

type CheckoutPage struct {
	VendorName      string
	Lines           []CheckoutLine
	SubtotalKobo    int
	OffersPickup    bool
	OffersDelivery  bool
	DeliveryFeeKobo int
	PickupAddress   string
	Phone           string
	DefaultMethod   string
	LoyaltyPoints   int
	States          []string
}

func (h *Handlers) CheckoutDetails(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		log.Printf("checkout details: cart failed: %v", err)
		errors.Render(w, "Could not load your cart", http.StatusInternalServerError)
		return
	}
	if len(items) == 0 {
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}

	rows, err := h.DB.Query(`
		SELECT DISTINCT v.name, v.offers_pickup, v.offers_delivery, v.delivery_fee_kobo, v.address,
		       COALESCE(v.status = 'active' AND v.paid_until > NOW(), false)
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ci.user_id = $1
	`, userID)
	if err != nil {
		log.Printf("checkout details: vendor query failed: %v", err)
		errors.Render(w, "Could not check your cart", http.StatusInternalServerError)
		return
	}

	var page CheckoutPage
	vendors := 0
	live := true
	for rows.Next() {
		var l bool
		if err := rows.Scan(&page.VendorName, &page.OffersPickup, &page.OffersDelivery,
			&page.DeliveryFeeKobo, &page.PickupAddress, &l); err != nil {
			rows.Close()
			log.Printf("checkout details: vendor scan failed: %v", err)
			errors.Render(w, "Could not check your cart", http.StatusInternalServerError)
			return
		}
		vendors++
		live = live && l
	}
	rows.Close()

	if vendors != 1 {
		errors.Render(w, "Your cart has dishes from more than one vendor. Please order from one vendor at a time.", http.StatusBadRequest)
		return
	}
	if !live {
		errors.Render(w, "This vendor is not taking orders right now.", http.StatusBadRequest)
		return
	}

	for _, it := range items {
		line := it.PriceKobo * it.Quantity
		page.Lines = append(page.Lines, CheckoutLine{Name: it.Name, Quantity: it.Quantity, LineKobo: line})
		page.SubtotalKobo += line
	}

	h.DB.QueryRow("SELECT COALESCE(phone, '') FROM users WHERE id = $1", userID).Scan(&page.Phone)
	if err := h.DB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN type = 'earned' THEN points ELSE -points END), 0)
		FROM loyalty_transactions
		WHERE user_id = $1
	`, userID).Scan(&page.LoyaltyPoints); err != nil {
		// Existing databases may not have the loyalty table yet; checkout should
		// still work without rewards in that case.
		page.LoyaltyPoints = 0
	}

	page.States = nigerianStates
	page.DefaultMethod = "pickup"
	if !page.OffersPickup {
		page.DefaultMethod = "delivery"
	}

	tmpl, err := template.New("checkout-details.html").
		Funcs(template.FuncMap{"naira": naira}).
		Funcs(brand.Funcs()).
		ParseFiles("templates/checkout-details.html")
	if err != nil {
		log.Printf("checkout details: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("checkout details: execute failed: %v", err)
	}
}
