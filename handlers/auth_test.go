package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open(
		"postgres",
		"postgresql://testuser:testpass@localhost:5433/food_ordering_test?sslmode=disable",
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE
		);

		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS menu_items (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price_kobo INTEGER NOT NULL,
			image_url TEXT
		);

		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			total_kobo INTEGER NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			payment_reference VARCHAR(100),
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			quantity INTEGER NOT NULL,
			price_kobo INTEGER NOT NULL
		);

		DELETE FROM order_items;
		DELETE FROM orders;
		DELETE FROM sessions;
		DELETE FROM menu_items;
		DELETE FROM users;
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
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

	if location := w.Header().Get("Location"); location != "/admin/orders" {
		t.Fatalf(
			"expected redirect to /admin/orders, got %s",
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
