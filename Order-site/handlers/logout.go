package handlers

import (
	"Order-site/errors"
	"Order-site/middleware"
	"net/http"
)

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	// A link on another website must not be able to log someone out
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		errors.Render(w, "That request was blocked", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("session_token")

	if err == nil && cookie.Value != "" {
		err = middleware.DeleteSession(h.DB, cookie.Value)

		if err != nil {
			errors.Render(
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
		Secure:   middleware.SecureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
