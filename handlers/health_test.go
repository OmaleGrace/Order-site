package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"Order-site/middleware"
)

func TestHealthReportsOkWhenTheDatabaseAnswers(t *testing.T) {
	if err := os.Chdir(".."); err == nil {
		defer os.Chdir("handlers")
	}
	db := setupTestDB(t)
	h := New(db)
	w := httptest.NewRecorder()
	h.Health(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	db.Close()
	w = httptest.NewRecorder()
	h.Health(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"status":"degraded"`) {
		t.Fatalf("closed database: got %d %s", w.Code, w.Body.String())
	}
}

func TestRemoveProfilePhotoClearsOnlyMyPicture(t *testing.T) {
	if err := os.Chdir(".."); err == nil {
		defer os.Chdir("handlers")
	}
	db := setupTestDB(t)
	defer db.Close()
	h := New(db)
	var me, other int
	db.QueryRow(`INSERT INTO users (name, email, password, profile_picture) VALUES ('Me', 'me@example.com', 'x', '/media/1') RETURNING id`).Scan(&me)
	db.QueryRow(`INSERT INTO users (name, email, password, profile_picture) VALUES ('Other', 'o@example.com', 'x', '/media/2') RETURNING id`).Scan(&other)
	session, _ := middleware.CreateSession(db, me)
	req := httptest.NewRequest(http.MethodPost, "/account/photo/remove", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session})
	w := httptest.NewRecorder()
	h.RemoveProfilePhoto(w, req)
	if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Location"), "photo-removed") {
		t.Fatalf("got %d %s", w.Code, w.Header().Get("Location"))
	}
	var mine, theirs string
	db.QueryRow(`SELECT COALESCE(profile_picture,'') FROM users WHERE id=$1`, me).Scan(&mine)
	db.QueryRow(`SELECT COALESCE(profile_picture,'') FROM users WHERE id=$1`, other).Scan(&theirs)
	if mine != "" || theirs != "/media/2" {
		t.Fatalf("mine=%q theirs=%q", mine, theirs)
	}
}
