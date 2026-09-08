package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHome(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	w := httptest.NewRecorder()

	h.Home(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Grace") {
		t.Fatal("expected home page content")
	}
}
