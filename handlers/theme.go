package handlers

import (
	"net/http"
	"strings"

	"Order-site/middleware"
)

func validTheme(t string) bool { return t == "light" || t == "dark" || t == "system" }

// themeFromDB reads the saved appearance setting, defaulting to "system".
func (h *Handlers) themeFromDB(userID int) string {
	var t string
	if err := h.DB.QueryRow(`SELECT COALESCE(theme, 'system') FROM users WHERE id = $1`, userID).Scan(&t); err != nil || !validTheme(t) {
		return "system"
	}
	return t
}

// SaveTheme stores the appearance choice (light, dark or follow the device). The choice is also
// kept in a cookie so every page can apply it before it is drawn, with no flash of the wrong theme.
func (h *Handlers) SaveTheme(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	theme := strings.TrimSpace(r.FormValue("theme"))
	if !validTheme(theme) {
		http.Error(w, "Choose light, dark or system", http.StatusBadRequest)
		return
	}
	if _, err := h.DB.Exec(`UPDATE users SET theme = $1 WHERE id = $2`, theme, userID); err != nil {
		http.Error(w, "Could not save your choice", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "theme", Value: theme, Path: "/", MaxAge: 365 * 24 * 3600,
		Secure: middleware.SecureCookie(r), SameSite: http.SameSiteLaxMode, // readable by the page script on purpose
	})
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		return
	}
	http.Redirect(w, r, "/account?theme=saved#appearance", http.StatusSeeOther)
}
