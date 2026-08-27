package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"Order-site/cart"
)

func (h *Handlers) Checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("user_id")
	if err != nil {
		http.Error(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		http.Error(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		http.Error(w, "Your cart is empty", http.StatusBadRequest)
		return
	}

	var totalKobo int

	for _, item := range items {
		totalKobo += item.PriceKobo * item.Quantity
	}

	var orderID int

	err = h.DB.QueryRow(`
		INSERT INTO orders (user_id, total_kobo, status)
		VALUES ($1, $2, 'pending')
		RETURNING id
	`, userID, totalKobo).Scan(&orderID)

	if err != nil {
		fmt.Println("Create order error:", err)
		http.Error(w, "Could not create order", http.StatusInternalServerError)
		return
	}

	for _, item := range items {
		_, err = h.DB.Exec(`
			INSERT INTO order_items
			(order_id, menu_item_id, quantity, price_kobo)
			VALUES ($1, $2, $3, $4)
		`,
			orderID,
			item.MenuItemID,
			item.Quantity,
			item.PriceKobo,
		)

		if err != nil {
			fmt.Println("Create order item error:", err)
			http.Error(w, "Could not create order items", http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprintf(w, "Order #%d created successfully. Payment integration coming next.", orderID)
}