package middleware

import (
	"database/sql"
	"net/http"
	"strconv"
)

func Admin(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("user_id")
		if err != nil {
			http.Error(w, "You must be logged in", http.StatusUnauthorized)
			return
		}

		userID, err := strconv.Atoi(cookie.Value)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var isAdmin bool

		err = db.QueryRow(
			"SELECT is_admin FROM users WHERE id = $1",
			userID,
		).Scan(&isAdmin)

		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if !isAdmin {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
