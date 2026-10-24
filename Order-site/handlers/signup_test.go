package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSignupSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := New(db)

	form := "name=Test+User&email=test%40example.com&password=secret123"

	req := httptest.NewRequest(
		http.MethodPost,
		"/signup",
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Account created successfully!") {
		t.Fatal("expected success message")
	}

	if !strings.Contains(body, `window.location.href = "/login"`) {
		t.Fatal("expected redirect script to /login")
	}

	var storedPassword string

	err := db.QueryRow(
		"SELECT password FROM users WHERE email = $1",
		"test@example.com",
	).Scan(&storedPassword)

	if err != nil {
		t.Fatal(err)
	}

	if storedPassword == "secret123" {
		t.Fatal("password was stored as plain text")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(storedPassword),
		[]byte("secret123"),
	); err != nil {
		t.Fatal("stored password is not a valid bcrypt hash")
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
	`, "Existing User", "duplicate@example.com", "hashed-password")

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	form := "name=Another+User&email=duplicate%40example.com&password=secret123"

	req := httptest.NewRequest(
		http.MethodPost,
		"/signup",
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Could not create account") {
		t.Fatal("expected account creation error message")
	}

	var count int

	err = db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE email = $1",
		"duplicate@example.com",
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}
}
