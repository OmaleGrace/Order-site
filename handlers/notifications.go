package handlers

import (
	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
	"database/sql"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

type Notification struct {
	ID        int
	Type      string
	Title     string
	Message   string
	Link      string
	Read      bool
	CreatedAt time.Time
}

func createNotification(db *sql.DB, userID int, kind, title, message, link string) error {
	_, err := db.Exec(`INSERT INTO notifications (user_id, type, title, message, link) VALUES ($1,$2,$3,$4,$5)`, userID, kind, title, message, link)
	return err
}

func (h *Handlers) Notifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", http.StatusUnauthorized)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	rows, err := h.DB.Query(`SELECT id, type, title, message, COALESCE(link,''), read_at IS NOT NULL, created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 60`, userID)
	if err != nil {
		errors.Render(w, "Could not load notifications", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Link, &n.Read, &n.CreatedAt); err != nil {
			errors.Render(w, "Could not read notifications", 500)
			return
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read notifications", 500)
		return
	}

	tmpl := render.MustPage("notifications.html", template.FuncMap{"date": func(t time.Time) string { return t.Format("02 Jan 2006, 3:04 PM") }})
	if err := tmpl.Execute(w, notifications); err != nil {
		errors.Render(w, "Could not display notifications", 500)
	}
}

func (h *Handlers) NotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"count":0}`))
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, `{"count":0}`, http.StatusUnauthorized)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Error(w, `{"count":0}`, http.StatusUnauthorized)
		return
	}
	var count int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&count); err != nil {
		http.Error(w, `{"count":0}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"count":` + strconv.Itoa(count) + `}`))
}

func (h *Handlers) NotificationRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", 401)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", 401)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid notification", 400)
		return
	}
	if _, err := h.DB.Exec(`UPDATE notifications SET read_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID); err != nil {
		errors.Render(w, "Could not update notification", 500)
		return
	}
	if link := r.FormValue("link"); link != "" {
		http.Redirect(w, r, link, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

func (h *Handlers) NotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		errors.Render(w, "You must be logged in", 401)
		return
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		errors.Render(w, "Invalid session", 401)
		return
	}
	if _, err := h.DB.Exec(`UPDATE notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, userID); err != nil {
		errors.Render(w, "Could not update notifications", 500)
		return
	}
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

func notificationForOrder(db *sql.DB, userID, orderID int, title, message, status string) error {
	return createNotification(db, userID, "order", title, message, "/orders")
}
