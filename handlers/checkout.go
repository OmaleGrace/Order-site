package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
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

	// Get the user's email
	var email string

	err = h.DB.QueryRow(
		"SELECT email FROM users WHERE id = $1",
		userID,
	).Scan(&email)

	if err != nil {
		fmt.Println("Get user email error:", err)
		http.Error(w, "Could not get user information", http.StatusInternalServerError)
		return
	}

	// Get cart items
	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		http.Error(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		http.Error(w, "Your cart is empty", http.StatusBadRequest)
		return
	}

	// Calculate total
	var totalKobo int

	for _, item := range items {
		totalKobo += item.PriceKobo * item.Quantity
	}

	// Create pending order
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

	// Save order items
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

	// Paystack request
	paystackData := map[string]interface{}{
		"email":        email,
		"amount":       totalKobo,
		"reference":    fmt.Sprintf("ORDER-%d", orderID),
		"callback_url": "https://grace-ordering-site.onrender.com/payment/callback",
	}

	requestBody, err := json.Marshal(paystackData)
	if err != nil {
		http.Error(w, "Could not prepare payment", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.paystack.co/transaction/initialize",
		bytes.NewBuffer(requestBody),
	)

	if err != nil {
		http.Error(w, "Could not create payment request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Paystack request error:", err)
		http.Error(w, "Could not connect to payment service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var paystackResponse struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AuthorizationURL string `json:"authorization_url"`
			AccessCode       string `json:"access_code"`
			Reference        string `json:"reference"`
		} `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&paystackResponse)
	if err != nil {
		http.Error(w, "Could not read payment response", http.StatusInternalServerError)
		return
	}

	if !paystackResponse.Status {
		fmt.Println("Paystack error:", paystackResponse.Message)
		http.Error(w, "Could not initialize payment", http.StatusInternalServerError)
		return
	}

	// Save Paystack reference
	_, err = h.DB.Exec(`
		UPDATE orders
		SET payment_reference = $1
		WHERE id = $2
	`,
		paystackResponse.Data.Reference,
		orderID,
	)

	if err != nil {
		fmt.Println("Save payment reference error:", err)
		http.Error(w, "Could not save payment information", http.StatusInternalServerError)
		return
	}

	// Send customer to Paystack
	http.Redirect(
		w,
		r,
		paystackResponse.Data.AuthorizationURL,
		http.StatusSeeOther,
	)
}

func (h *Handlers) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(
		template.ParseFiles("templates/order-success.html"),
	)

	err := tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
