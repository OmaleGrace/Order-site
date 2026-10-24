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
		strings.NewReader("delivery_method=pickup"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

func TestCheckoutRejectsDeliveryOutsideVendorArea(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID, menuItemID int
	if err := db.QueryRow(`INSERT INTO users (name, email, password) VALUES ('Delivery User', 'delivery-area@example.com', 'hashed') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO menu_items (name, description, price_kobo, vendor_id) VALUES ('Jollof Rice', 'Rice', 350000, 1) RETURNING id`).Scan(&menuItemID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition) VALUES ($1, $2, 1, 350000)`, userID, menuItemID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE vendors SET offers_delivery = TRUE WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vendor_delivery_areas (vendor_id, state, lga) VALUES (1, 'Lagos', 'Ikeja')`); err != nil {
		t.Fatal(err)
	}
	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	form := "delivery_method=delivery&delivery_state=Lagos&delivery_lga=Surulere&delivery_address=12+Example+Street&delivery_phone=08012345678"
	req := httptest.NewRequest(http.MethodPost, "/checkout", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	New(db).Checkout(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected outside-area delivery to be rejected with 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "does not deliver to the selected state and LGA") {
		t.Fatalf("expected clear delivery-area error, got %q", w.Body.String())
	}
	var orders int
	if err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if orders != 0 {
		t.Fatalf("expected no order to be created, found %d", orders)
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
