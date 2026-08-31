package middleware

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"
)

func Logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next(w, r)

		duration := time.Since(start)
		log.Printf("%s %s (%v)", r.Method, r.URL.Path, duration)
	}
}

func RequireLogin(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("user_id")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, err := strconv.Atoi(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var exists bool

		err = db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)",
			userID,
		).Scan(&exists)

		if err != nil || !exists {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}
