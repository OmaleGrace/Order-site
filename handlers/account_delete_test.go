package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"Order-site/middleware"
)

func delSetup(t *testing.T, provider string) (*Handlers, int, *http.Cookie) {
	t.Helper()
	if err := os.Chdir(".."); err == nil {
		t.Cleanup(func() { os.Chdir("handlers") })
	}
	db := setupTestDB(t)
	t.Cleanup(func() { db.Close() })
	var uid int
	if err := db.QueryRow(`INSERT INTO users (name, email, auth_provider, phone) VALUES ('Ada Obi', 'ada@example.com', $1, '08012345678') RETURNING id`, provider).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	tok, _ := middleware.CreateSession(db, uid)
	return New(db), uid, &http.Cookie{Name: "session_token", Value: tok}
}

func postForm(h http.HandlerFunc, path string, ck *http.Cookie, v url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(ck)
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestDeleteAccountNeedsTypedConfirmation(t *testing.T) {
	h, uid, ck := delSetup(t, "google")
	w := postForm(h.DeleteAccount, "/account/delete", ck, url.Values{"confirm": {"delete me"}})
	if !strings.Contains(w.Header().Get("Location"), "del-confirm") {
		t.Fatalf("expected del-confirm, got %q", w.Header().Get("Location"))
	}
	var name string
	h.DB.QueryRow(`SELECT name FROM users WHERE id=$1`, uid).Scan(&name)
	if name != "Ada Obi" {
		t.Fatal("account changed without confirmation")
	}
}

func TestDeleteAccountAnonymisesButKeepsOrders(t *testing.T) {
	h, uid, ck := delSetup(t, "google")
	h.DB.Exec(`INSERT INTO orders (user_id, total_kobo, status) VALUES ($1, 500000, 'delivered')`, uid)
	h.DB.Exec(`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES ($1, 'https://fcm.googleapis.com/x', 'a', 'b')`, uid)
	w := postForm(h.DeleteAccount, "/account/delete", ck, url.Values{"confirm": {"DELETE"}})
	if loc := w.Header().Get("Location"); loc != "/account/deleted" {
		t.Fatalf("redirect %q", loc)
	}
	var name, email, provider string
	var deleted bool
	h.DB.QueryRow(`SELECT name, email, auth_provider, deleted_at IS NOT NULL FROM users WHERE id=$1`, uid).Scan(&name, &email, &provider, &deleted)
	if name != "Deleted user" || !strings.HasSuffix(email, "@deleted.invalid") || provider != "deleted" || !deleted {
		t.Fatalf("not anonymised: %q %q %q %v", name, email, provider, deleted)
	}
	var orders, sessions, subs int
	h.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id=$1`, uid).Scan(&orders)
	h.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=$1`, uid).Scan(&sessions)
	h.DB.QueryRow(`SELECT COUNT(*) FROM push_subscriptions WHERE user_id=$1`, uid).Scan(&subs)
	if orders != 1 || sessions != 0 || subs != 0 {
		t.Fatalf("orders=%d sessions=%d subs=%d", orders, sessions, subs)
	}
	// The email address is free again for a new sign-up.
	if _, err := h.DB.Exec(`INSERT INTO users (name, email) VALUES ('New Ada', 'ada@example.com')`); err != nil {
		t.Fatalf("email not freed: %v", err)
	}
}

func TestDeleteAccountBlockedWhileOrderActive(t *testing.T) {
	h, uid, ck := delSetup(t, "google")
	h.DB.Exec(`INSERT INTO orders (user_id, total_kobo, status) VALUES ($1, 500000, 'preparing')`, uid)
	w := postForm(h.DeleteAccount, "/account/delete", ck, url.Values{"confirm": {"DELETE"}})
	if !strings.Contains(w.Header().Get("Location"), "del-active") {
		t.Fatalf("got %q", w.Header().Get("Location"))
	}
	var name string
	h.DB.QueryRow(`SELECT name FROM users WHERE id=$1`, uid).Scan(&name)
	if name != "Ada Obi" {
		t.Fatal("account was changed")
	}
}

func TestSaveThemeValidatesAndSetsCookie(t *testing.T) {
	h, uid, ck := delSetup(t, "google")
	w := postForm(h.SaveTheme, "/account/theme", ck, url.Values{"theme": {"dark"}})
	var got string
	h.DB.QueryRow(`SELECT theme FROM users WHERE id=$1`, uid).Scan(&got)
	if got != "dark" {
		t.Fatalf("theme = %q", got)
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "theme" && c.Value == "dark" {
			found = true
		}
	}
	if !found {
		t.Fatal("theme cookie not set")
	}
	postForm(h.SaveTheme, "/account/theme", ck, url.Values{"theme": {"neon"}})
	h.DB.QueryRow(`SELECT theme FROM users WHERE id=$1`, uid).Scan(&got)
	if got != "dark" {
		t.Fatalf("invalid theme was stored: %q", got)
	}
}

func TestAccountPagesRenderWithNewSections(t *testing.T) {
	h, _, ck := delSetup(t, "google")
	for path, want := range map[string]struct {
		fn   http.HandlerFunc
		text string
	}{
		"/account":         {h.Account, "data-theme-panel"},
		"/account/edit":    {h.EditProfile, "Delete account"},
		"/account/deleted": {h.AccountDeleted, ""},
	} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(ck)
		w := httptest.NewRecorder()
		want.fn(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), want.text) {
			t.Errorf("%s: code %d, missing %q", path, w.Code, want.text)
		}
	}
}
