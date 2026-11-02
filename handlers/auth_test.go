package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"Order-site/testutil"
	"golang.org/x/crypto/bcrypt"
)

// setupTestDB gives each test a clean database built exactly like production (schema + migrations).
func setupTestDB(t *testing.T) *sql.DB {
	return testutil.NewDB(t)
}

func TestLoginSuccess(t *testing.T) {
	err := os.Chdir("..")
	if err != nil {
		t.Fatal(err)
	}

	db := setupTestDB(t)
	defer db.Close()

	password, err := bcrypt.GenerateFromPassword(
		[]byte("password123"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ($1, $2, $3, $4)
	`, "Test User", "test@example.com", string(password), false)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		nil,
	)

	req.Form = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"password123"},
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// In production the site is served over HTTPS (usually behind a proxy that sets this header),
	// and only then is the session cookie marked Secure.
	req.Header.Set("X-Forwarded-Proto", "https")

	w := httptest.NewRecorder()

	h.Login(w, req)

	t.Logf("response body: %s", w.Body.String())
	t.Logf("headers: %v", w.Header())

	if w.Code != http.StatusSeeOther {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusSeeOther,
			w.Code,
		)
	}

	location := w.Header().Get("Location")

	if location != "/menu" {
		t.Fatalf(
			"expected redirect to /menu, got %s",
			location,
		)
	}

	cookies := w.Result().Cookies()

	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	if cookies[0].Name != "session_token" {
		t.Fatalf(
			"expected session_token cookie, got %s",
			cookies[0].Name,
		)
	}

	if !cookies[0].HttpOnly {
		t.Fatal("expected session cookie to be HttpOnly")
	}

	if !cookies[0].Secure {
		t.Fatal("expected session cookie to be Secure")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	password, err := bcrypt.GenerateFromPassword(
		[]byte("correctpassword"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ($1, $2, $3, $4)
	`, "Test User", "test@example.com", string(password), false)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		nil,
	)

	req.Form = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"wrongpassword"},
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			w.Code,
		)
	}
}

func TestAdminLogin(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	password, err := bcrypt.GenerateFromPassword(
		[]byte("adminpassword"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ($1, $2, $3, $4)
	`, "Admin User", "admin@example.com", string(password), true)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		nil,
	)

	req.Form = map[string][]string{
		"email":    {"admin@example.com"},
		"password": {"adminpassword"},
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusSeeOther,
			w.Code,
		)
	}

	if location := w.Header().Get("Location"); location != "/admin" {
		t.Fatalf(
			"expected redirect to /admin, got %s",
			location,
		)
	}
}

func TestLoginWrongEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	password, err := bcrypt.GenerateFromPassword(
		[]byte("password123"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ($1, $2, $3, $4)
	`, "Test User", "test@example.com", string(password), false)

	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		nil,
	)

	req.Form = map[string][]string{
		"email":    {"wrong@example.com"},
		"password": {"password123"},
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			w.Code,
		)
	}
}
