package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestUpdateOrderStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Status User', 'status@example.com', 'hashed')
		RETURNING id
	`).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	var orderID int

	err = db.QueryRow(`
		INSERT INTO orders (user_id, total_kobo, status)
		VALUES ($1, 500000, 'paid')
		RETURNING id
	`, userID).Scan(&orderID)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/orders/status",
		strings.NewReader(
			"order_id="+strconv.Itoa(orderID)+"&status=preparing",
		),
	)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	var status string

	err = db.QueryRow(`
		SELECT status FROM orders WHERE id = $1
	`, orderID).Scan(&status)

	if err != nil {
		t.Fatal(err)
	}

	if status != "preparing" {
		t.Fatalf("expected preparing, got %s", status)
	}
}

func TestUpdateOrderStatusInvalidOrderID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/orders/status",
		strings.NewReader("order_id=abc&status=ready"),
	)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOrderStatusInvalidStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/orders/status",
		strings.NewReader("order_id=1&status=invalid"),
	)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOrderStatusMethodNotAllowed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/orders/status",
		nil,
	)

	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
