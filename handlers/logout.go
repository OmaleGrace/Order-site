package handlers

import (
	"net/http"
)

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")

	if err == nil && cookie.Value != "" {
		_, err = h.DB.Exec(
			"DELETE FROM sessions WHERE token = $1",
			cookie.Value,
		)

		if err != nil {
			http.Error(
				w,
				"Could not log out",
				http.StatusInternalServerError,
			)
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}