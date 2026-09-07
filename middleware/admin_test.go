package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAdminTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open(
		"postgres",
		"postgresql://testuser:testpass@localhost:5433/food_ordering_test?sslmode=disable",
	)
	if err != nil {
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

		DELETE FROM sessions;
		DELETE FROM users;
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	return db
}

func TestAdminAllowsAdminUser(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ('Admin', 'admin@example.com', 'password', TRUE)
	`)
	if err != nil {
		t.Fatal(err)
	}

	var userID int
	err = db.QueryRow(
		"SELECT id FROM users WHERE email = $1",
		"admin@example.com",
	).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}

	token, err := CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	called := false

	handler := Admin(db, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Fatal("expected admin handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestAdminBlocksNormalUser(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO users (name, email, password, is_admin)
		VALUES ('Normal User', 'user@example.com', 'password', FALSE)
	`)
	if err != nil {
		t.Fatal(err)
	}

	var userID int
	err = db.QueryRow(
		"SELECT id FROM users WHERE email = $1",
		"user@example.com",
	).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}

	token, err := CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	handler := Admin(db, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("normal user should not reach admin handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
}

func TestAdminRequiresSession(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	handler := Admin(db, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("user without session should not reach admin handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)

	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", w.Code)
	}

	if location := w.Header().Get("Location"); location != "/login" {
		t.Fatalf("expected redirect to /login, got %q", location)
	}
}
