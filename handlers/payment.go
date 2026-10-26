package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"Order-site/cart"
	"Order-site/errors"
)

var paymentHTTPClient = &http.Client{Timeout: 20 * time.Second}

func (h *Handlers) PaymentCallback(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		errors.Render(w, "Payment service is not configured", http.StatusServiceUnavailable)
		return
	}

	reference := strings.TrimSpace(r.URL.Query().Get("reference"))

	if reference == "" {
		errors.Render(w, "Payment reference missing", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(
		http.MethodGet,
		"https://api.paystack.co/transaction/verify/"+url.PathEscape(reference),
		nil,
	)
	if err != nil {
		errors.Render(w, "Could not create verification request", http.StatusInternalServerError)
		return
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+secret,
	)

	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		fmt.Println("Paystack verification error:", err)
		errors.Render(w, "Could not verify payment", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var paystackResponse struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status    string `json:"status"`
			Reference string `json:"reference"`
			Amount    int    `json:"amount"`
		} `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&paystackResponse)
	if err != nil {
		errors.Render(w, "Could not read payment response", http.StatusInternalServerError)
		return
	}

	// Make sure Paystack says the payment was successful
	if !paystackResponse.Status ||
		strings.ToLower(paystackResponse.Data.Status) != "success" {
		fmt.Println("Paystack payment failed:", paystackResponse.Message)
		errors.Render(w, "Payment was not successful", http.StatusBadRequest)
		return
	}

	var orderTotal int

	err = h.DB.QueryRow(`
    SELECT total_kobo
    FROM orders
    WHERE payment_reference = $1
`, paystackResponse.Data.Reference).Scan(&orderTotal)

	if err != nil {
		fmt.Println("Get order total error:", err)
		errors.Render(w, "Could not find order", http.StatusInternalServerError)
		return
	}

	if paystackResponse.Data.Amount != orderTotal {
		errors.Render(w, "Payment amount does not match order", http.StatusBadRequest)
		return
	}

	// Get the order from our database
	var orderID int
	var userID int
	var orderStatus string

	err = h.DB.QueryRow(`
		SELECT id, user_id, total_kobo, status
		FROM orders
		WHERE payment_reference = $1
	`, paystackResponse.Data.Reference).Scan(
		&orderID,
		&userID,
		&orderTotal,
		&orderStatus,
	)

	if err != nil {
		fmt.Println("Get order error:", err)
		errors.Render(w, "Could not find order", http.StatusInternalServerError)
		return
	}

	// Don't process an already completed order again
	if orderStatus == "paid" {
		http.Redirect(w, r, "/order-success", http.StatusSeeOther)
		return
	}

	// Verify that the amount paid matches the order amount
	if paystackResponse.Data.Amount != orderTotal {
		fmt.Printf(
			"Payment amount mismatch for order %d: expected %d, received %d\n",
			orderID,
			orderTotal,
			paystackResponse.Data.Amount,
		)

		errors.Render(w, "Payment amount does not match order", http.StatusBadRequest)
		return
	}

	// Mark the order as paid and redeem loyalty points atomically.
	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var loyaltyPoints int
	if err := tx.QueryRow(`SELECT loyalty_points_redeemed FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&loyaltyPoints); err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}

	result, err := tx.Exec(`
	UPDATE orders
	SET status = CASE WHEN scheduled_for IS NOT NULL AND scheduled_for > NOW() THEN 'scheduled' ELSE 'paid' END
	WHERE id = $1
	AND status = 'pending'
`, orderID)
	if err != nil {
		fmt.Println("Update order error:", err)
		errors.Render(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Redirect(w, r, "/order-success", http.StatusSeeOther)
		return
	}

	if loyaltyPoints > 0 {
		if err := redeemLoyaltyForOrder(tx, userID, orderID, loyaltyPoints); err != nil {
			fmt.Println("Loyalty redemption error:", err)
			errors.Render(w, "Could not apply loyalty points", http.StatusBadRequest)
			return
		}
	}

	if err := settleCoupon(tx, orderID); err != nil {
		fmt.Println("Coupon settlement error:", err)
		errors.Render(w, "Could not apply coupon", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}

	// Clear the user's cart
	err = cart.ClearCart(h.DB, userID)
	if err != nil {
		fmt.Println("Clear cart error:", err)
		errors.Render(w, "Could not clear cart", http.StatusInternalServerError)
		return
	}

	go func(id int) {
		if err := sendOrderPlacedEmail(h.DB, id); err != nil {
			fmt.Println("Order confirmation email error:", err)
		}
	}(orderID)
	var currentStatus string
	_ = h.DB.QueryRow("SELECT status FROM orders WHERE id = $1", orderID).Scan(&currentStatus)
	notificationTitle := "Order confirmed"
	notificationMessage := "Your payment was confirmed and your order is now being processed."
	if currentStatus == "scheduled" {
		notificationTitle = "Scheduled order confirmed"
		notificationMessage = "Your payment was confirmed. Your order will become active at the scheduled time."
		if err := notifyVendorScheduledOrder(h.DB, orderID); err != nil {
			fmt.Println("Vendor scheduled-order notification error:", err)
		}
	}
	if err := createNotification(h.DB, userID, "order", notificationTitle, notificationMessage, "/orders"); err != nil {
		fmt.Println("Order notification error:", err)
	}

	http.Redirect(w, r, "/order-success", http.StatusSeeOther)
}

func (h *Handlers) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secret := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	if secret == "" {
		errors.Render(w, "Payment service is not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		errors.Render(w, "Could not read webhook", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("x-paystack-signature")
	if signature == "" {
		errors.Render(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write(body)

	expectedSignature := hex.EncodeToString(hash.Sum(nil))

	if !hmac.Equal(
		[]byte(signature),
		[]byte(expectedSignature),
	) {
		errors.Render(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			Status    string `json:"status"`
			Reference string `json:"reference"`
			Amount    int    `json:"amount"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		errors.Render(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// We only process successful transactions.
	if event.Event != "charge.success" ||
		strings.ToLower(event.Data.Status) != "success" {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Vendor setup and renewal payments are handled separately from customer orders
	if strings.HasPrefix(event.Data.Reference, "VENDOR-") {
		h.vendorWebhook(w, event.Data.Reference, event.Data.Amount)
		return
	}

	var orderID int
	var userID int
	var orderTotal int
	var orderStatus string

	err = h.DB.QueryRow(`
		SELECT id, user_id, total_kobo, status
		FROM orders
		WHERE payment_reference = $1
	`, event.Data.Reference).Scan(
		&orderID,
		&userID,
		&orderTotal,
		&orderStatus,
	)

	if err != nil {
		fmt.Println("Webhook order lookup error:", err)
		errors.Render(w, "Order not found", http.StatusNotFound)
		return
	}

	if event.Data.Amount != orderTotal {
		fmt.Printf(
			"Webhook amount mismatch for order %d: expected %d, received %d\n",
			orderID,
			orderTotal,
			event.Data.Amount,
		)

		errors.Render(w, "Payment amount mismatch", http.StatusBadRequest)
		return
	}

	// Prevent duplicate fulfillment.
	if orderStatus == "paid" {
		w.WriteHeader(http.StatusOK)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var loyaltyPoints int
	if err := tx.QueryRow(`SELECT loyalty_points_redeemed FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&loyaltyPoints); err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}

	result, err := tx.Exec(`
	UPDATE orders
	SET status = CASE WHEN scheduled_for IS NOT NULL AND scheduled_for > NOW() THEN 'scheduled' ELSE 'paid' END
	WHERE id = $1
	AND status = 'pending'
`, orderID)
	if err != nil {
		fmt.Println("Webhook update order error:", err)
		errors.Render(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if loyaltyPoints > 0 {
		if err := redeemLoyaltyForOrder(tx, userID, orderID, loyaltyPoints); err != nil {
			fmt.Println("Webhook loyalty redemption error:", err)
			errors.Render(w, "Could not apply loyalty points", http.StatusBadRequest)
			return
		}
	}

	if err := settleCoupon(tx, orderID); err != nil {
		fmt.Println("Webhook coupon settlement error:", err)
		errors.Render(w, "Could not apply coupon", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		errors.Render(w, "Could not confirm payment", http.StatusInternalServerError)
		return
	}

	if err := cart.ClearCart(h.DB, userID); err != nil {
		fmt.Println("Webhook clear cart error:", err)
		errors.Render(w, "Could not clear cart", http.StatusInternalServerError)
		return
	}

	go func(id int) {
		if err := sendOrderPlacedEmail(h.DB, id); err != nil {
			fmt.Println("Order confirmation email error:", err)
		}
	}(orderID)
	var currentStatus string
	_ = h.DB.QueryRow("SELECT status FROM orders WHERE id = $1", orderID).Scan(&currentStatus)
	notificationTitle := "Order confirmed"
	notificationMessage := "Your payment was confirmed and your order is now being processed."
	if currentStatus == "scheduled" {
		notificationTitle = "Scheduled order confirmed"
		notificationMessage = "Your payment was confirmed. Your order will become active at the scheduled time."
		if err := notifyVendorScheduledOrder(h.DB, orderID); err != nil {
			fmt.Println("Vendor scheduled-order notification error:", err)
		}
	}
	if err := createNotification(h.DB, userID, "order", notificationTitle, notificationMessage, "/orders"); err != nil {
		fmt.Println("Order notification error:", err)
	}

	w.WriteHeader(http.StatusOK)
}
