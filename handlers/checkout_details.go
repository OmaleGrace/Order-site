package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"Order-site/cart"
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

type CheckoutLine struct {
	Name     string
	Quantity int
	LineKobo int
	Options  []string
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
	CouponError     string
	States          []string
	Rates           []CheckoutRate // per-area delivery pricing, used for the fee preview
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
		var options []string
		for _, option := range it.Options {
			if option.PriceKobo > 0 {
				options = append(options, option.Name+" (+₦"+fmt.Sprintf("%d", option.PriceKobo/100)+")")
			} else {
				options = append(options, option.Name)
			}
		}
		page.Lines = append(page.Lines, CheckoutLine{Name: it.Name, Quantity: it.Quantity, LineKobo: line, Options: options})
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
	if rates, err := loadCheckoutRates(h.DB, userID); err != nil {
		// The page still works: it falls back to the shop's default fee, and the
		// server applies the real per-area price when the order is placed.
		log.Printf("checkout details: delivery rates failed: %v", err)
	} else {
		page.Rates = rates
	}
	page.DefaultMethod = "pickup"
	if !page.OffersPickup {
		page.DefaultMethod = "delivery"
	}

	tmpl, err := render.Page("checkout-details.html", template.FuncMap{"naira": naira})
	if err != nil {
		log.Printf("checkout details: template failed: %v", err)
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("checkout details: execute failed: %v", err)
	}
}
