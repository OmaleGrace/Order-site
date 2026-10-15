package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"
)

// VendorBillingStatus tells the vendor pages whether to show a payment banner.
func (h *Handlers) VendorBillingStatus(w http.ResponseWriter, r *http.Request) {
	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	var status string
	var setupPaid bool
	var paidUntil sql.NullTime
	err := h.DB.QueryRow(`
		SELECT status, setup_paid_at IS NOT NULL, paid_until
		FROM vendors WHERE id = $1
	`, vendorID).Scan(&status, &setupPaid, &paidUntil)
	if err != nil {
		log.Printf("vendor billing status: query failed: %v", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	state := "ok"
	days := 0
	switch {
	case status == "suspended":
		state = "suspended"
	case !setupPaid:
		state = "setup"
	case !paidUntil.Valid || !paidUntil.Time.After(time.Now()):
		state = "expired"
	default:
		days = int(math.Ceil(time.Until(paidUntil.Time).Hours() / 24))
		if days <= 7 {
			state = "soon"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{"state": state, "days_left": days})
}
