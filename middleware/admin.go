package middleware

import (
	"Order-site/errors"
	"database/sql"
	"net/http"
)

func Admin(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, err := GetUserID(db, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var isAdmin bool

		err = db.QueryRow(
			"SELECT is_admin FROM users WHERE id = $1",
			userID,
		).Scan(&isAdmin)

		if err != nil {
			errors.Render(w, "User not found", http.StatusUnauthorized)
			return
		}

		if !isAdmin {
			errors.Render(w, "Access denied", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
