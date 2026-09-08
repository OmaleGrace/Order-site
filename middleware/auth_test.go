package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func setupMiddlewareTestDB(t *testing.T) *sql.DB {
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
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`DELETE FROM sessions`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`DELETE FROM users`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestRequireLoginWithoutSession(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	defer db.Close()

	nextCalled := false

	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}

	handler := RequireLogin(db, next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/cart",
		nil,
	)

	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusSeeOther,
			w.Code,
		)
	}

	if location := w.Header().Get("Location"); location != "/login" {
		t.Fatalf(
			"expected redirect to /login, got %s",
			location,
		)
	}

	if nextCalled {
		t.Fatal("expected protected handler not to be called")
	}
}

func TestRequireLoginWithValidSession(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
	`, "Test User", "test@example.com", "hashed-password")

	if err != nil {
		t.Fatal(err)
	}

	var userID int

	err = db.QueryRow(`
		SELECT id FROM users WHERE email = $1
	`, "test@example.com").Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	token, err := CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	nextCalled := false

	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}

	handler := RequireLogin(db, next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/cart",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			w.Code,
		)
	}

	if !nextCalled {
		t.Fatal("expected protected handler to be called")
	}
}

func TestRequireLoginWithExpiredSession(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
	`, "Test User", "test@example.com", "hashed-password")

	if err != nil {
		t.Fatal(err)
	}

	var userID int

	err = db.QueryRow(`
		SELECT id FROM users WHERE email = $1
	`, "test@example.com").Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	token := "expired-test-token"

	_, err = db.Exec(`
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, time.Now().Add(-time.Hour))

	if err != nil {
		t.Fatal(err)
	}

	nextCalled := false

	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}

	handler := RequireLogin(db, next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/cart",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusSeeOther,
			w.Code,
		)
	}

	if location := w.Header().Get("Location"); location != "/login" {
		t.Fatalf(
			"expected redirect to /login, got %s",
			location,
		)
	}

	if nextCalled {
		t.Fatal("expected protected handler not to be called")
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	defer db.Close()

	var userID int

	err := db.QueryRow(`
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
		RETURNING id
	`, "Test User", "cleanup@example.com", "hashed-password").Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, "expired-token", userID, time.Now().Add(-time.Hour))

	if err != nil {
		t.Fatal(err)
	}

	err = CleanupExpiredSessions(db)
	if err != nil {
		t.Fatal(err)
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE token = $1
	`, "expired-token").Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("expected expired session to be deleted")
	}
}