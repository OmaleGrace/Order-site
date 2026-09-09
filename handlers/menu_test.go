package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMenuShowsItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO menu_items (name, description, price_kobo)
		VALUES ('Jollof Rice', 'Delicious jollof rice', 350000)
	`)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/menu", nil)
	w := httptest.NewRecorder()

	h.Menu(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Jollof Rice") {
		t.Fatal("expected menu page to contain Jollof Rice")
	}
}

func TestMenuEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/menu", nil)
	w := httptest.NewRecorder()

	h.Menu(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestMenuShowsMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(http.MethodGet, "/menu?message=Successfully+added+to+cart", nil)
	w := httptest.NewRecorder()

	h.Menu(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Successfully added to cart") {
		t.Fatal("expected menu page to display the message")
	}
}

func TestNairaConversion(t *testing.T) {
	tests := []struct {
		kobo     int
		expected int
	}{
		{350000, 3500},
		{0, 0},
		{100, 1},
		{99, 0},
	}

	for _, tt := range tests {
		result := naira(tt.kobo)
		if result != tt.expected {
			t.Fatalf("naira(%d) = %d, expected %d", tt.kobo, result, tt.expected)
		}
	}
}