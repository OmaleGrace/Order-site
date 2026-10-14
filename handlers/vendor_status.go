package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"Order-site/email"
	"Order-site/errors"
)

func (h *Handlers) VendorUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vendorID, _, ok := h.ownedVendor(w, r)
	if !ok {
		return
	}

	orderID, err := strconv.Atoi(r.FormValue("order_id"))
	if err != nil {
		errors.Render(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	allowed := map[string]bool{"preparing": true, "ready": true, "completed": true}
	if !allowed[status] {
		errors.Render(w, "Invalid order status", http.StatusBadRequest)
		return
	}

	res, err := h.DB.Exec(`
		UPDATE orders
		SET status = $1
		WHERE id = $2 AND vendor_id = $3 AND status NOT IN ('pending', 'cancelled')
	`, status, orderID, vendorID)
	if err != nil {
		log.Printf("vendor status: update failed: %v", err)
		errors.Render(w, "Could not update order", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		errors.Render(w, "Order not found", http.StatusNotFound)
		return
	}

	var userEmail string
	h.DB.QueryRow(`
		SELECT u.email FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $1
	`, orderID).Scan(&userEmail)

	if userEmail != "" {
		go func() {
			if err := email.Send(userEmail, "Order Update - Order #"+strconv.Itoa(orderID), fmt.Sprintf("<h1>Order status updated</h1><p>Your order #%d is now: <strong>%s</strong>.</p>", orderID, status)); err != nil {
				fmt.Println("Order status email error:", err)
			}
		}()
	}

	http.Redirect(w, r, "/vendor/orders", http.StatusSeeOther)
}