package middleware

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func Logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next(w, r)

		duration := time.Since(start)
		log.Printf("%s %s ip=%s (%v)", r.Method, r.URL.Path, ClientIP(r), duration)
	}
}

func RequireLogin(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			if strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Your session has expired. Please log in again."})
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, err := GetUserID(db, cookie.Value)
		if err != nil {
			if err == sql.ErrNoRows {
				http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", HttpOnly: true, Secure: SecureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
				if strings.Contains(r.Header.Get("Accept"), "application/json") {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Your session has expired. Please log in again."})
					return
				}
				http.Redirect(w, r, "/login?next="+r.URL.EscapedPath(), http.StatusSeeOther)
				return
			}
			log.Printf("require login: session lookup failed for %s: %v", r.URL.Path, err)
			http.Error(w, "Could not verify your session. Please refresh and try again.", http.StatusInternalServerError)
			return
		}

		r = WithUserID(r, userID)
		next(w, r)
	}
}
