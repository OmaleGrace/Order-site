package handlers

import (
	"Order-site/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogout(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user.
	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
	`, "Logout User", "logout@example.com", "hashed-password")

	if err != nil {
		t.Fatal(err)
	}

	var userID int

	err = db.QueryRow(`
		SELECT id FROM users WHERE email = $1
	`, "logout@example.com").Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	// Create a real server-side session.
	token, err := middleware.CreateSession(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	h := New(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/logout",
		nil,
	)

	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()

	h.Logout(w, req)

	// Check redirect.
	if w.Code != http.StatusSeeOther {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusSeeOther,
			w.Code,
		)
	}

	if location := w.Header().Get("Location"); location != "/login" {
		t.Fatalf(
			"expected redirect to /, got %s",
			location,
		)
	}

	// Confirm the session was deleted.
	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE token = $1
	`, token).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("expected session to be deleted after logout")
	}

	// Confirm the browser cookie is cleared.
	cookies := w.Result().Cookies()

	if len(cookies) == 0 {
		t.Fatal("expected logout cookie")
	}

	if cookies[0].Name != "session_token" {
		t.Fatalf(
			"expected session_token cookie, got %s",
			cookies[0].Name,
		)
	}

	if cookies[0].MaxAge != -1 {
		t.Fatalf(
			"expected cookie MaxAge -1, got %d",
			cookies[0].MaxAge,
		)
	}
}
