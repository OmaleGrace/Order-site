package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"Order-site/middleware"
)

func TestAddToCartSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Cart User', 'cartuser@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}

	var menuItemID int
	err = db.QueryRow(`
		INSERT INTO menu_items (name, description, price_kobo)
		VALUES ('Jollof Rice', 'Delicious jollof rice', 350000)
		RETURNING id
	`).Scan(&menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
	req.Form = map[string][]string{
		"id": {strconv.Itoa(menuItemID)},
	}
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})

	w := httptest.NewRecorder()

	h.AddToCart(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/menu?message=Successfully%20added%20to%20cart" {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	var quantity int
	err = db.QueryRow(`
		SELECT quantity FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, menuItemID).Scan(&quantity)
	if err != nil {
		t.Fatal(err)
	}

	if quantity != 1 {
		t.Fatalf("expected quantity 1, got %d", quantity)
	}
}

func TestAddToCartNoSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodPost, "/cart/add", nil)
	req.Form = map[string][]string{"id": {"1"}}

	w := httptest.NewRecorder()

	h.AddToCart(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAddToCartWrongMethod(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/cart/add", nil)
	w := httptest.NewRecorder()

	h.AddToCart(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestCartShowsItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Cart Viewer', 'cartviewer@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}

	var menuItemID int
	err = db.QueryRow(`
		INSERT INTO menu_items (name, description, price_kobo)
		VALUES ('Fried Rice', 'Delicious fried rice', 400000)
		RETURNING id
	`).Scan(&menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition)
		VALUES ($1, $2, 2, 400000)
	`, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})

	w := httptest.NewRecorder()

	h.Cart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.Len() == 0 {
		t.Fatal("expected cart page content")
	}
}

func TestCartNoSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	w := httptest.NewRecorder()

	h.Cart(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRemoveFromCartSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ('Remove User', 'removeuser@example.com', 'hashed-password')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}

	var menuItemID int
	err = db.QueryRow(`
		INSERT INTO menu_items (name, description, price_kobo)
		VALUES ('Spaghetti', 'Delicious spaghetti', 300000)
		RETURNING id
	`).Scan(&menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO cart_items (user_id, menu_item_id, quantity, price_kobo_at_addition)
		VALUES ($1, $2, 1, 300000)
	`, userID, menuItemID)
	if err != nil {
		t.Fatal(err)
	}

	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(http.MethodPost, "/cart/remove", nil)
	req.Form = map[string][]string{
		"id": {strconv.Itoa(menuItemID)},
	}
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})

	w := httptest.NewRecorder()

	h.RemoveFromCart(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, w.Code)
	}

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cart_items
		WHERE user_id = $1 AND menu_item_id = $2
	`, userID, menuItemID).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected item to be removed, found %d rows", count)
	}
}

func TestRemoveFromCartNoSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodPost, "/cart/remove", nil)
	w := httptest.NewRecorder()

	h.RemoveFromCart(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}