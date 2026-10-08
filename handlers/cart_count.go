package handlers

import (
	"encoding/json"
	"net/http"

	"Order-site/cart"
	"Order-site/middleware"
)

func (h *Handlers) CartCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	count := 0

	if cookie, err := r.Cookie("session_token"); err == nil {
		if userID, err := middleware.GetUserID(h.DB, cookie.Value); err == nil {
			if items, err := cart.GetItems(h.DB, userID); err == nil {
				for _, it := range items {
					count += it.Quantity
				}
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]int{"count": count})
}