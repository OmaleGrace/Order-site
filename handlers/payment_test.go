package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPaymentCallbackMissingReference(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/payment/callback",
		nil,
	)

	w := httptest.NewRecorder()

	h.PaymentCallback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentCallbackSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("PAYSTACK_SECRET_KEY", "test-secret")
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Payment User', 'payment@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	var orderID int

	err = db.QueryRow(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, 700000, 'pending', 'TEST-REF-123')
		RETURNING id
	`, userID).Scan(&orderID)

	if err != nil {
		t.Fatal(err)
	}

	// Put an item in the cart so we can verify it gets cleared.
	var menuItemID int

	err = db.QueryRow(`
		INSERT INTO menu_items
			(name, description, price_kobo)
		VALUES
			('Jollof Rice', 'Rice', 350000)
		RETURNING id
	`).Scan(&menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO cart_items
			(user_id, menu_item_id, quantity, price_kobo_at_addition)
		VALUES ($1, $2, 2, 350000)
	`, userID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	oldClient := paymentHTTPClient

	// Replace the transport with a response body that contains JSON.
	paymentHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": true,
				"message": "Verification successful",
				"data": {
					"status": "success",
					"reference": "TEST-REF-123",
					"amount": 700000
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       ioNopCloser{strings.NewReader(body)},
			}, nil
		}),
	}

	defer func() {
		paymentHTTPClient = oldClient
	}()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/payment/callback?reference=TEST-REF-123",
		nil,
	)

	w := httptest.NewRecorder()

	h.PaymentCallback(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	var status string

	err = db.QueryRow(`
		SELECT status
		FROM orders
		WHERE id = $1
	`, orderID).Scan(&status)

	if err != nil {
		t.Fatal(err)
	}

	if status != "paid" {
		t.Fatalf("expected order status paid, got %s", status)
	}

	var cartCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM cart_items
		WHERE user_id = $1
	`, userID).Scan(&cartCount)

	if err != nil {
		t.Fatal(err)
	}

	if cartCount != 0 {
		t.Fatalf("expected cart to be cleared, got %d items", cartCount)
	}
}

func TestPaymentCallbackAmountMismatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("PAYSTACK_SECRET_KEY", "test-secret")
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Mismatch User', 'mismatch@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, 700000, 'pending', 'MISMATCH-REF')
	`, userID)

	if err != nil {
		t.Fatal(err)
	}

	oldClient := paymentHTTPClient

	paymentHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": true,
				"message": "Verification successful",
				"data": {
					"status": "success",
					"reference": "MISMATCH-REF",
					"amount": 500000
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       ioNopCloser{strings.NewReader(body)},
			}, nil
		}),
	}

	defer func() {
		paymentHTTPClient = oldClient
	}()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/payment/callback?reference=MISMATCH-REF",
		nil,
	)

	w := httptest.NewRecorder()

	h.PaymentCallback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentWebhookMissingSignature(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(`{"event":"charge.success"}`),
	)

	w := httptest.NewRecorder()

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPaymentWebhookInvalidSignature(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("PAYSTACK_SECRET_KEY", "test-secret")
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(`{"event":"charge.success"}`),
	)

	req.Header.Set("x-paystack-signature", "invalid-signature")

	w := httptest.NewRecorder()

	h := New(db)

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPaymentWebhookSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"
	os.Setenv("PAYSTACK_SECRET_KEY", secret)
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Webhook User', 'webhook@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	var orderID int

	err = db.QueryRow(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, 700000, 'pending', 'WEBHOOK-REF')
		RETURNING id
	`, userID).Scan(&orderID)

	if err != nil {
		t.Fatal(err)
	}

	body := `{
		"event": "charge.success",
		"data": {
			"status": "success",
			"reference": "WEBHOOK-REF",
			"amount": 700000
		}
	}`

	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write([]byte(body))

	signature := hex.EncodeToString(hash.Sum(nil))

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(body),
	)

	req.Header.Set("x-paystack-signature", signature)

	w := httptest.NewRecorder()

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status string

	err = db.QueryRow(`
		SELECT status
		FROM orders
		WHERE id = $1
	`, orderID).Scan(&status)

	if err != nil {
		t.Fatal(err)
	}

	if status != "paid" {
		t.Fatalf("expected paid, got %s", status)
	}
}

func TestPaymentWebhookAmountMismatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"
	os.Setenv("PAYSTACK_SECRET_KEY", secret)
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Webhook Mismatch', 'webhook-mismatch@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, 700000, 'pending', 'WEBHOOK-MISMATCH')
	`, userID)

	if err != nil {
		t.Fatal(err)
	}

	body := `{
		"event": "charge.success",
		"data": {
			"status": "success",
			"reference": "WEBHOOK-MISMATCH",
			"amount": 500000
		}
	}`

	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write([]byte(body))

	signature := hex.EncodeToString(hash.Sum(nil))

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(body),
	)

	req.Header.Set("x-paystack-signature", signature)

	w := httptest.NewRecorder()

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentWebhookIgnoresFailedPayment(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"
	os.Setenv("PAYSTACK_SECRET_KEY", secret)
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	body := `{
		"event": "charge.failed",
		"data": {
			"status": "failed",
			"reference": "FAILED-REF",
			"amount": 700000
		}
	}`

	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write([]byte(body))

	signature := hex.EncodeToString(hash.Sum(nil))

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(body),
	)

	req.Header.Set("x-paystack-signature", signature)

	w := httptest.NewRecorder()

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPaymentWebhookAlreadyPaid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"
	os.Setenv("PAYSTACK_SECRET_KEY", secret)
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Already Paid', 'already@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, 700000, 'paid', 'ALREADY-PAID')
	`, userID)

	if err != nil {
		t.Fatal(err)
	}

	body := `{
		"event": "charge.success",
		"data": {
			"status": "success",
			"reference": "ALREADY-PAID",
			"amount": 700000
		}
	}`

	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write([]byte(body))

	signature := hex.EncodeToString(hash.Sum(nil))

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/payment/webhook",
		strings.NewReader(body),
	)

	req.Header.Set("x-paystack-signature", signature)

	w := httptest.NewRecorder()

	h.PaymentWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

type ioNopCloser struct {
	*strings.Reader
}

func (ioNopCloser) Close() error {
	return nil
}
