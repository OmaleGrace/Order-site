package middleware

import (
	"Order-site/errors"
	"database/sql"
	"net/http"
	"strings"
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

// SuperAdmin limits destructive account-management actions to the designated platform owner.
// The account must also already have is_admin=true in the database.
func SuperAdmin(db *sql.DB, ownerEmail string, next http.HandlerFunc) http.HandlerFunc {
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
		var email string
		var isAdmin bool
		err = db.QueryRow(`SELECT LOWER(email), is_admin FROM users WHERE id = $1`, userID).Scan(&email, &isAdmin)
		if err != nil || !isAdmin || email != strings.ToLower(strings.TrimSpace(ownerEmail)) {
			errors.Render(w, "Only the main platform administrator can manage user accounts", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
