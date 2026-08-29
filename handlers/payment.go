package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

	if !paystackResponse.Status ||
		strings.ToLower(paystackResponse.Data.Status) != "success" {
		http.Error(w, "Payment was not successful", http.StatusBadRequest)
		return
	}

	// Mark order as paid
	_, err = h.DB.Exec(`
		UPDATE orders
		SET status = 'paid'
		WHERE payment_reference = $1
		AND status = 'pending'
	`, paystackResponse.Data.Reference)

	if err != nil {
		fmt.Println("Update order error:", err)
		http.Error(w, "Could not update order", http.StatusInternalServerError)
		return
	}

	// Get the order's user
	var userID int

	err = h.DB.QueryRow(`
		SELECT user_id
		FROM orders
		WHERE payment_reference = $1
	`, paystackResponse.Data.Reference).Scan(&userID)

	if err != nil {
		fmt.Println("Get order user error:", err)
		http.Error(w, "Could not find order", http.StatusInternalServerError)
		return
	}

	// Clear the user's cart
	_, err = h.DB.Exec(`
		DELETE FROM cart_items
		WHERE user_id = $1
	`, userID)

	if err != nil {
		fmt.Println("Clear cart error:", err)
		http.Error(w, "Could not clear cart", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/order-success", http.StatusSeeOther)
}