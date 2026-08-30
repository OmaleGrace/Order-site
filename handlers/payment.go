package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"Order-site/cart"
)

func (h *Handlers) PaymentCallback(w http.ResponseWriter, r *http.Request) {
	reference := r.URL.Query().Get("reference")

	if reference == "" {
		http.Error(w, "Payment reference missing", http.StatusBadRequest)
		return
	}

	// Verify transaction with Paystack
	req, err := http.NewRequest(
		http.MethodGet,
		"https://api.paystack.co/transaction/verify/"+reference,
		nil,
	)
	if err != nil {
		http.Error(w, "Could not create verification request", http.StatusInternalServerError)
		return
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+os.Getenv("PAYSTACK_SECRET_KEY"),
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Paystack verification error:", err)
		http.Error(w, "Could not verify payment", http.StatusInternalServerError)
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
		http.Error(w, "Could not read payment response", http.StatusInternalServerError)
		return
	}

	// Make sure Paystack says the payment was successful
	if !paystackResponse.Status ||
		strings.ToLower(paystackResponse.Data.Status) != "success" {
		fmt.Println("Paystack payment failed:", paystackResponse.Message)
		http.Error(w, "Payment was not successful", http.StatusBadRequest)
		return
	}

	// Get the order from our database
	var orderID int
	var userID int
	var orderTotal int
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
		http.Error(w, "Could not find order", http.StatusInternalServerError)
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

		http.Error(w, "Payment amount does not match order", http.StatusBadRequest)
		return
	}

	// Mark the order as paid
	_, err = h.DB.Exec(`
		UPDATE orders
		SET status = 'paid'
		WHERE id = $1
		AND status = 'pending'
	`, orderID)

	if err != nil {
		fmt.Println("Update order error:", err)
		http.Error(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	// Clear the user's cart
	err = cart.ClearCart(h.DB, userID)
	if err != nil {
		fmt.Println("Clear cart error:", err)
		http.Error(w, "Could not clear cart", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/order-success", http.StatusSeeOther)
}