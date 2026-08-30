package handlers

import (
	"net/http"
	"strconv"
)

func (h *Handlers) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID, err := strconv.Atoi(r.FormValue("order_id"))
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")

	validStatuses := map[string]bool{
		"pending":   true,
		"paid":      true,
		"preparing": true,
		"ready":     true,
		"completed": true,
		"cancelled": true,
	}

	if !validStatuses[status] {
		http.Error(w, "Invalid order status", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec(`
		UPDATE orders
		SET status = $1
		WHERE id = $2
	`, status, orderID)

	if err != nil {
		http.Error(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
}