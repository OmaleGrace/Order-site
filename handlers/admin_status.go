package handlers

import (
	"Order-site/errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handlers) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID, err := strconv.Atoi(r.FormValue("order_id"))
	if err != nil {
		errors.Render(w, "Invalid order ID", http.StatusBadRequest)
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
		errors.Render(w, "Invalid order status", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec(`
	UPDATE orders
	SET status = $1
	WHERE id = $2
`, status, orderID)

	if err != nil {
		errors.Render(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	if status == "completed" {
		if err := h.awardLoyaltyForOrder(orderID); err != nil {
			fmt.Printf("Loyalty award error: %v\n", err)
		}
	}

	var customerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM orders WHERE id=$1`, orderID).Scan(&customerID); err == nil {
		statusLabel := strings.Title(strings.ReplaceAll(status, "_", " "))
		_ = createNotification(h.DB, customerID, "order", "Order status updated", "Your order is now "+statusLabel+".", "/orders")
	}

	go func(id int, newStatus string) {
		if err := sendOrderStatusEmail(h.DB, id, newStatus); err != nil {
			fmt.Println("Order status email error:", err)
		}
	}(orderID, status)

	http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
}
