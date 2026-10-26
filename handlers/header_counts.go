package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"Order-site/middleware"
)

// HeaderCounts returns the small set of customer counters used by the shared
// header/bottom navigation in one database round trip. This avoids doing a
// session lookup plus separate cart and notification queries on every page.
func (h *Handlers) HeaderCounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, `{"count":0}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")

	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		_ = json.NewEncoder(w).Encode(map[string]int{"cart_count": 0, "notification_count": 0})
		return
	}

	token := middleware.HashToken(cookie.Value)
	var userID, cartCount, notificationCount int
	err = h.DB.QueryRowContext(r.Context(), `
        SELECT s.user_id,
               (SELECT COALESCE(SUM(quantity), 0) FROM cart_items WHERE user_id = s.user_id),
               (SELECT COUNT(*) FROM notifications WHERE user_id = s.user_id AND read_at IS NULL)
        FROM sessions s
        WHERE s.token = $1 AND s.expires_at > NOW()
    `, token).Scan(&userID, &cartCount, &notificationCount)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = json.NewEncoder(w).Encode(map[string]int{"cart_count": 0, "notification_count": 0})
			return
		}
		http.Error(w, `{"error":"Could not load header counts"}`, http.StatusInternalServerError)
		return
	}

	_ = userID // kept in the query result to make the session-to-count relationship explicit.
	_ = json.NewEncoder(w).Encode(map[string]int{
		"cart_count":         cartCount,
		"notification_count": notificationCount,
	})
}
