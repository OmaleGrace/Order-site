package middleware

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

func CreateSession(db *sql.DB, userID int) (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err := db.Exec(`
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)

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
	`, token).Scan(&userID)

	return userID, err
}

func CleanupExpiredSessions(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM sessions
		WHERE expires_at <= NOW()
	`)

	return err
}
