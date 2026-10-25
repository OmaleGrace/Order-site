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

	form := "first_name=Test&middle_name=&last_name=User&email=test%40example.com&phone=08012345678&country=Nigeria&state=Lagos&password=Secret123%21&confirm_password=Secret123%21&terms_accepted=on"

	req := httptest.NewRequest(
		http.MethodPost,
		"/signup",
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) || !strings.Contains(body, `"redirect":"/menu"`) {
		t.Fatalf("expected successful JSON response, got %s", body)
	}

	var firstName, middleName, lastName, country, state string
	var termsAccepted bool
	err := db.QueryRow(`SELECT first_name, middle_name, last_name, country, state, terms_accepted_at IS NOT NULL FROM users WHERE email = $1`, "test@example.com").Scan(&firstName, &middleName, &lastName, &country, &state, &termsAccepted)
	if err != nil {
		t.Fatal(err)
	}
	if firstName != "Test" || middleName != "" || lastName != "User" || country != "Nigeria" || state != "Lagos" || !termsAccepted {
		t.Fatalf("unexpected signup profile: %q %q %q %q %q terms=%v", firstName, middleName, lastName, country, state, termsAccepted)
	}

	var storedPassword string

	err = db.QueryRow(
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
		[]byte("Secret123!"),
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

	form := "first_name=Another&middle_name=&last_name=User&email=duplicate%40example.com&phone=08012345679&country=Nigeria&state=Lagos&password=Secret123%21&confirm_password=Secret123%21&terms_accepted=on"

	req := httptest.NewRequest(
		http.MethodPost,
		"/signup",
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "already exists") {
		t.Fatal("expected duplicate account error message")
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
