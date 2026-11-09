package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAdminOrders(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Make sure templates are found.
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chdir("..")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Chdir(oldDir)

	// Create customer.
	var userID int

	err = db.QueryRow(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ('Customer One', 'customer@example.com', 'hashed', false)
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	// Create menu item.
	var menuItemID int

	err = db.QueryRow(`
		INSERT INTO menu_items
			(name, description, price_kobo)
		VALUES
			('Jollof Rice', 'Delicious rice', 350000)
		RETURNING id
	`).Scan(&menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	// Create order.
	var orderID int

	err = db.QueryRow(`
		INSERT INTO orders
			(user_id, total_kobo, status)
		VALUES
			($1, 700000, 'paid')
		RETURNING id
	`, userID).Scan(&orderID)

	if err != nil {
		t.Fatal(err)
	}

	// Create order item.
	_, err = db.Exec(`
		INSERT INTO order_items
			(order_id, menu_item_id, quantity, price_kobo)
		VALUES
			($1, $2, 2, 350000)
	`, orderID, menuItemID)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/orders",
		nil,
	)

	w := httptest.NewRecorder()

	h.AdminOrders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Customer One") {
		t.Fatal("expected customer name in response")
	}

	if !strings.Contains(body, "customer@example.com") {
		t.Fatal("expected customer email in response")
	}

	if !strings.Contains(body, "Jollof Rice") {
		t.Fatal("expected order item in response")
	}

	if !strings.Contains(body, "paid") {
		t.Fatal("expected order status in response")
	}

	if !strings.Contains(body, "700000") && !strings.Contains(body, "7000") {
		t.Fatal("expected order total in response")
	}
}

func TestAdminOrdersMethodNotAllowed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/orders",
		nil,
	)

	w := httptest.NewRecorder()

	h.AdminOrders(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status 405, got %d",
			w.Code,
		)
	}
}
