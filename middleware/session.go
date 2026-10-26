package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// SecureCookie reports whether a cookie should be marked Secure for this request.
// Local development commonly runs over plain HTTP, while production is normally
// terminated at an HTTPS reverse proxy.
func SecureCookie(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// HashToken is what gets stored in the database. The browser keeps the real
// token; the database only keeps its fingerprint, so a leaked copy of the
// database cannot be used to take over anyone's login.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type userIDContextKey struct{}

// WithUserID stores the authenticated user ID in the request context.
func WithUserID(r *http.Request, userID int) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userIDContextKey{}, userID))
}

// UserIDFromRequest returns the authenticated user ID attached by RequireLogin.
func UserIDFromRequest(r *http.Request) (int, error) {
	if r == nil {
		return 0, sql.ErrNoRows
	}
	userID, ok := r.Context().Value(userIDContextKey{}).(int)
	if !ok || userID <= 0 {
		return 0, sql.ErrNoRows
	}
	return userID, nil
}

func CreateSession(db *sql.DB, userID int) (string, error) {
	return CreateSessionWithDuration(db, userID, 24*time.Hour)
}

// CreateSessionWithDuration creates a secure server-side session for the requested duration.
func CreateSessionWithDuration(db *sql.DB, userID int, duration time.Duration) (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(duration)

	_, err := db.Exec(`
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, HashToken(token), userID, expiresAt)

	if err != nil {
		return "", err
	}

	return token, nil
}

func GetUserID(db *sql.DB, token string) (int, error) {
	var userID int

	err := db.QueryRow(`
		SELECT user_id
		FROM sessions
		WHERE token = $1
		AND expires_at > NOW()
	`, HashToken(token)).Scan(&userID)

	return userID, err
}

// DeleteSession ends one login.
func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = $1", HashToken(token))
	return err
}

// DeleteUserSessions ends every login a person has, for example after a password reset.
func DeleteUserSessions(db *sql.DB, userID int) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}

func CleanupExpiredSessions(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM sessions
		WHERE expires_at <= NOW()
	`)

	return err
}
