package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Order-site/middleware"
)

func TestMyOrders(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create user
	var userID int
	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Order User', 'order@example.com', 'hashed-password')
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

	// Create order
	var orderID int
	err = db.QueryRow(`
		INSERT INTO orders
			(user_id, total_kobo, status, payment_reference)
		VALUES
			($1, $2, $3, $4)
		RETURNING id
	`, userID, 350000, "paid", "test-reference-123").Scan(&orderID)

	if err != nil {
		t.Fatal(err)
	}

	// Create order item
	_, err = db.Exec(`
		INSERT INTO order_items
			(order_id, menu_item_id, quantity, price_kobo)
		VALUES
			($1, $2, $3, $4)
	`, orderID, menuItemID, 1, 350000)

	if err != nil {
		t.Fatal(err)
	}

	// Create session
	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/orders",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	h.MyOrders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	checks := []string{
		"Jollof Rice",
		"paid",
		"3500",
		"ENIE-",
		"order-item-placeholder",
	}

	for _, expected := range checks {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response to contain %q", expected)
		}
	}
}

func TestMyOrdersRequiresSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/orders",
		nil,
	)

	w := httptest.NewRecorder()

	h.MyOrders(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status 401, got %d",
			w.Code,
		)
	}
}

func TestPublicOrderReferenceIsStableAndNotSequentialID(t *testing.T) {
	createdAt := time.Date(2026, time.October, 23, 10, 30, 0, 0, time.UTC)
	first := publicOrderReference(12345, createdAt)
	second := publicOrderReference(12345, createdAt)
	if first != second {
		t.Fatalf("reference should be stable: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "ENIE-") {
		t.Fatalf("expected ENIE reference, got %q", first)
	}
	if first == "12345" || strings.Contains(first, "12345") {
		t.Fatalf("reference should not expose sequential database ID: %q", first)
	}
	if first == publicOrderReference(12346, createdAt) {
		t.Fatal("different orders should have different references")
	}
}
