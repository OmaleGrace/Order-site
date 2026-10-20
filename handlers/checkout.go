package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/cart"
	"Order-site/errors"
	"Order-site/middleware"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

func (h *Handlers) Checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
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
		errors.Render(w, "Could not get user information", http.StatusInternalServerError)
		return
	}

	// Get cart items
	items, err := cart.GetItems(h.DB, userID)
	if err != nil {
		errors.Render(w, "Could not load cart", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		errors.Render(w, "Your cart is empty", http.StatusBadRequest)
		return
	}

	// Calculate total
	var totalKobo int

	for _, item := range items {
		totalKobo += item.PriceKobo * item.Quantity
	}

	// One vendor per order, and that vendor must be live
	vendorRows, err := h.DB.Query(`
		SELECT DISTINCT m.vendor_id,
		       COALESCE(v.status = 'active' AND v.paid_until > NOW(), false)
		FROM cart_items ci
		JOIN menu_items m ON m.id = ci.menu_item_id
		JOIN vendors v ON v.id = m.vendor_id
		WHERE ci.user_id = $1
	`, userID)
	if err != nil {
		fmt.Println("Vendor check error:", err)
		errors.Render(w, "Could not check your cart", http.StatusInternalServerError)
		return
	}

	var vendorIDs []int
	allLive := true
	for vendorRows.Next() {
		var vid int
		var live bool
		if err := vendorRows.Scan(&vid, &live); err != nil {
			vendorRows.Close()
			fmt.Println("Vendor check scan error:", err)
			errors.Render(w, "Could not check your cart", http.StatusInternalServerError)
			return
		}
		vendorIDs = append(vendorIDs, vid)
		if !live {
			allLive = false
		}
	}
	vendorRows.Close()

	if len(vendorIDs) != 1 {
		errors.Render(w, "Your cart has dishes from more than one vendor. Please order from one vendor at a time.", http.StatusBadRequest)
		return
	}
	if !allLive {
		errors.Render(w, "This vendor is not taking orders right now.", http.StatusBadRequest)
		return
	}
	vendorID := vendorIDs[0]

	// Delivery choice, checked against what this vendor really offers
	var offersPickup, offersDelivery bool
	var deliveryFeeKobo int
	err = h.DB.QueryRow(`
		SELECT offers_pickup, offers_delivery, delivery_fee_kobo
		FROM vendors WHERE id = $1
	`, vendorID).Scan(&offersPickup, &offersDelivery, &deliveryFeeKobo)
	if err != nil {
		fmt.Println("Vendor delivery options error:", err)
		errors.Render(w, "Could not check delivery options", http.StatusInternalServerError)
		return
	}

	method := r.FormValue("delivery_method")
	state := strings.TrimSpace(r.FormValue("delivery_state"))
	lga := strings.TrimSpace(r.FormValue("delivery_lga"))
	address := strings.TrimSpace(r.FormValue("delivery_address"))
	deliveryPhone := strings.TrimSpace(r.FormValue("delivery_phone"))

	switch method {
	case "pickup":
		if !offersPickup {
			errors.Render(w, "This shop does not offer pickup.", http.StatusBadRequest)
			return
		}
		state, lga, address = "", "", ""
		deliveryFeeKobo = 0
	case "delivery":
		if !offersDelivery {
			errors.Render(w, "This shop does not offer delivery.", http.StatusBadRequest)
			return
		}
		if !validState(state) {
			errors.Render(w, "Choose your state.", http.StatusBadRequest)
			return
		}
		if n := len([]rune(lga)); n < 3 || n > 60 {
			errors.Render(w, "Enter your local government area.", http.StatusBadRequest)
			return
		}
		if n := len([]rune(address)); n < 8 || n > 250 {
			errors.Render(w, "Enter your house address (8 to 250 characters).", http.StatusBadRequest)
			return
		}
		if deliveryPhone == "" {
			errors.Render(w, "Enter a phone number the rider can reach you on.", http.StatusBadRequest)
			return
		}
	default:
		errors.Render(w, "Choose pickup or delivery.", http.StatusBadRequest)
		return
	}

	if deliveryPhone != "" {
		normalized := normalizeNigerianPhone(deliveryPhone)
		if normalized == "" {
			errors.Render(w, "Enter a valid Nigerian phone number: 11 digits starting with 0 (for example 08012345678), or starting with +234.", http.StatusBadRequest)
			return
		}
		deliveryPhone = normalized
	}

	// The platform's cut is taken from the food only, not the delivery fee.
	// It is fixed now, so later rate changes never alter an existing order.
	commission := commissionKobo(totalKobo)

	// The delivery fee is part of what the customer pays
	totalKobo += deliveryFeeKobo
	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not start order", http.StatusInternalServerError)
		return
	}

	defer tx.Rollback()

	var orderID int

	err = tx.QueryRow(`
	INSERT INTO orders (user_id, total_kobo, status, vendor_id,
	                    delivery_method, delivery_state, delivery_lga, delivery_address,
	                    delivery_phone, delivery_fee_kobo, commission_kobo)
	VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id
`, userID, totalKobo, vendorID, method, state, lga, address, deliveryPhone, deliveryFeeKobo, commission).Scan(&orderID)

	if err != nil {
		fmt.Println("Create order error:", err)
		errors.Render(w, "Could not create order", http.StatusInternalServerError)
		return
	}

	for _, item := range items {
		_, err = tx.Exec(`
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
			errors.Render(w, "Could not create order items", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		fmt.Println("Commit order error:", err)
		errors.Render(w, "Could not save order", http.StatusInternalServerError)
		return
	}

	// Paystack request
	paystackData := map[string]interface{}{
		"email":        email,
		"amount":       totalKobo,
		"reference":    fmt.Sprintf("ORDER-%d-%d", orderID, time.Now().UnixNano()),
		"callback_url": os.Getenv("PAYSTACK_CALLBACK_URL"),
	}

	requestBody, err := json.Marshal(paystackData)
	if err != nil {
		errors.Render(w, "Could not prepare payment", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.paystack.co/transaction/initialize",
		bytes.NewBuffer(requestBody),
	)

	if err != nil {
		errors.Render(w, "Could not create payment request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("Paystack request error:", err)
		errors.Render(w, "Could not connect to payment service", http.StatusInternalServerError)
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
		errors.Render(w, "Could not read payment response", http.StatusInternalServerError)
		return
	}

	if !paystackResponse.Status {
		fmt.Println("Paystack error:", paystackResponse.Message)
		errors.Render(w, "Could not initialize payment", http.StatusInternalServerError)
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
		errors.Render(w, "Could not save payment information", http.StatusInternalServerError)
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
		brand.ParseE("templates/order-success.html"),
	)

	err := tmpl.Execute(w, nil)
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
