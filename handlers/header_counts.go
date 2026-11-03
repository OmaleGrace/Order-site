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
		h.writeGuestHeaderCounts(w, r)
		return
	}

	token := middleware.HashToken(cookie.Value)
	var userID, cartCount, notificationCount, messageCount int
	err = h.DB.QueryRowContext(r.Context(), `
        SELECT s.user_id,
               (SELECT COALESCE(SUM(quantity), 0) FROM cart_items WHERE user_id = s.user_id),
               (SELECT COUNT(*) FROM notifications WHERE user_id = s.user_id AND read_at IS NULL),
               (SELECT COUNT(*) FROM messages m JOIN message_threads t ON t.id = m.thread_id JOIN vendors v ON v.id = t.vendor_id WHERE m.sender_user_id <> s.user_id AND m.read_at IS NULL AND (t.customer_id = s.user_id OR v.owner_user_id = s.user_id))
        FROM sessions s
        WHERE s.token = $1 AND s.expires_at > NOW()
    `, token).Scan(&userID, &cartCount, &notificationCount, &messageCount)
	if err != nil {
		if err == sql.ErrNoRows {
			h.writeGuestHeaderCounts(w, r)
			return
		}
		http.Error(w, `{"error":"Could not load header counts"}`, http.StatusInternalServerError)
		return
	}

	_ = userID // kept in the query result to make the session-to-count relationship explicit.
	_ = json.NewEncoder(w).Encode(map[string]int{
		"cart_count":         cartCount,
		"notification_count": notificationCount,
		"message_count":      messageCount,
	})
}

// writeGuestHeaderCounts answers for a visitor who is not logged in: the only counter
// that can be non-zero is the cart, for a guest who has already added something.
func (h *Handlers) writeGuestHeaderCounts(w http.ResponseWriter, r *http.Request) {
	cartCount := 0
	if c, err := r.Cookie(middleware.GuestCookieName); err == nil && c.Value != "" {
		if id, err := middleware.GetGuestUserID(h.DB, c.Value); err == nil {
			_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(quantity), 0) FROM cart_items WHERE user_id = $1`, id).Scan(&cartCount)
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]int{"cart_count": cartCount, "notification_count": 0, "message_count": 0})
}
