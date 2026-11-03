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
	var err error
	hash := HashToken(token)

	// A hosted database can drop an idle connection or pause for a moment. That is not the same
	// as "this login is invalid", so retry a failed lookup once before giving up. Only a clean
	// "no such session" answer (sql.ErrNoRows) means the person really is logged out.
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err = db.QueryRowContext(ctx, `
			SELECT user_id
			FROM sessions
			WHERE token = $1
			AND expires_at > NOW()
		`, hash).Scan(&userID)
		cancel()
		if err == nil || err == sql.ErrNoRows {
			return userID, err
		}
		time.Sleep(250 * time.Millisecond)
	}
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

// GuestCookieName is the cookie that carries a guest shopper's cart session. It is
// deliberately separate from session_token, so a guest can never reach account pages.
const GuestCookieName = "guest_token"

// GuestSessionDuration is how long a guest's cart is remembered.
const GuestSessionDuration = 30 * 24 * time.Hour

// CreateGuestUser makes the placeholder row a guest's cart and order hang off. The
// email ends in .invalid, which can never be a real address, so it cannot be used to
// log in, receive a password reset, or clash with a real customer.
func CreateGuestUser(db *sql.DB) (int, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	placeholder := "guest-" + hex.EncodeToString(b) + "@guest.invalid"
	var id int
	err := db.QueryRow(`
		INSERT INTO users (name, email, auth_provider, is_guest)
		VALUES ('Guest', $1, 'guest', TRUE)
		RETURNING id
	`, placeholder).Scan(&id)
	return id, err
}

// GetGuestUserID returns the guest behind a guest_token cookie value. A normal
// customer's session token is rejected here because the user must be a guest.
func GetGuestUserID(db *sql.DB, token string) (int, error) {
	var userID int
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := db.QueryRowContext(ctx, `
		SELECT s.user_id
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1
		  AND s.expires_at > NOW()
		  AND u.is_guest
	`, HashToken(token)).Scan(&userID)
	return userID, err
}

// SetGuestCookie stores the guest cart token in the browser.
func SetGuestCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     GuestCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   SecureCookie(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(GuestSessionDuration / time.Second),
	})
}
