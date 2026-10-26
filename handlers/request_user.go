package handlers

import (
	"database/sql"
	"net/http"

	"Order-site/middleware"
)

// requestUserID returns the authenticated user ID without another database query
// when RequireLogin has already verified the session for this request.
func requestUserID(h *Handlers, r *http.Request) (int, error) {
	userID, err := middleware.UserIDFromRequest(r)
	if err == nil && userID > 0 {
		return userID, nil
	}

	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return 0, sql.ErrNoRows
	}

	return middleware.GetUserID(h.DB, cookie.Value)
}
