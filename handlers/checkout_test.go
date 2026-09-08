package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"Order-site/middleware"
)

func TestCheckoutCreatesOrder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("PAYSTACK_SECRET_KEY", "test-secret")
	os.Setenv("PAYSTACK_CALLBACK_URL", "http://localhost:8080/payment/callback")
	defer os.Unsetenv("PAYSTACK_SECRET_KEY")
	defer os.Unsetenv("PAYSTACK_CALLBACK_URL")

	// Create user
	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Checkout User', 'checkout@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	// Create menu item
	var menuItemID int

	err = db.QueryRow(`
		INSERT INTO menu_items
			(name, description, price_kobo)
		VALUES
			('Jollof Rice', 'Delicious jollof rice', 350000)
		RETURNING id
	`).Scan(&menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	// Add item to cart
	_, err = db.Exec(`
		INSERT INTO cart_items
			(user_id, menu_item_id, quantity, price_kobo_at_addition)
		VALUES
			($1, $2, 2, 350000)
	`, userID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	// Create session
	token, err := middleware.CreateSession(db, userID)

	if err != nil {
		t.Fatal(err)
	}

	// Mock Paystack
	oldClient := httpClient

	httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.paystack.co/transaction/initialize" {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}, nil
		}),
	}

	defer func() {
		httpClient = oldClient
	}()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/checkout",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	h.Checkout(w, req)

	// The mocked response has no JSON data, so the handler
	// should fail while decoding the Paystack response.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	// Verify order was created before payment initialization.
	var total int

	err = db.QueryRow(`
		SELECT total_kobo
		FROM orders
		WHERE user_id = $1
	`, userID).Scan(&total)

	if err != nil {
		t.Fatal(err)
	}

	if total != 700000 {
		t.Fatalf("expected total 700000, got %d", total)
	}

	// Verify order item was created.
	var quantity int

	err = db.QueryRow(`
		SELECT quantity
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.user_id = $1
	`, userID).Scan(&quantity)

	if err != nil {
		t.Fatal(err)
	}

	if quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", quantity)
	}
}

func TestCheckoutEmptyCart(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Empty Cart User', 'empty@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	token, err := middleware.CreateSession(db, userID)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/checkout",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	h.Checkout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Your cart is empty") {
		t.Fatal("expected empty cart message")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestOrderSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/order-success",
		nil,
	)

	w := httptest.NewRecorder()

	h.OrderSuccess(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if w.Body.Len() == 0 {
		t.Fatal("expected order success page content")
	}
}
