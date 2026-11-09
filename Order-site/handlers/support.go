package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

type SupportTicket struct {
	ID         int64
	Category   string
	Subject    string
	Message    string
	AdminReply string
	Status     string
	OrderID    sql.NullInt64
	CreatedAt  string
	UpdatedAt  string
	UserName   string
	UserEmail  string
}

type SupportPage struct {
	Tickets        []SupportTicket
	Notice         string
	PrefillOrderID string
}

func currentUserID(h *Handlers, r *http.Request) (int, error) {
	if id, err := middleware.UserIDFromRequest(r); err == nil && id > 0 {
		return id, nil
	}
	c, err := r.Cookie("session_token")
	if err != nil || c.Value == "" {
		return 0, sql.ErrNoRows
	}
	return middleware.GetUserID(h.DB, c.Value)
}

func (h *Handlers) Help(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := currentUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/help", http.StatusSeeOther)
		return
	}
	rows, err := h.DB.Query(`SELECT id, category, subject, message, admin_reply, status, order_id, TO_CHAR(created_at,'DD Mon YYYY HH24:MI'), TO_CHAR(updated_at,'DD Mon YYYY HH24:MI') FROM support_tickets WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 20`, userID)
	if err != nil {
		log.Printf("support: list failed: %v", err)
		errors.Render(w, "Could not load support requests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	page := SupportPage{Notice: r.URL.Query().Get("msg"), PrefillOrderID: r.URL.Query().Get("order_id")}
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.Category, &t.Subject, &t.Message, &t.AdminReply, &t.Status, &t.OrderID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			errors.Render(w, "Could not read support requests", 500)
			return
		}
		page.Tickets = append(page.Tickets, t)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read support requests", 500)
		return
	}
	tmpl, err := render.Page("help.html")
	if err != nil {
		errors.Render(w, "Could not display support", 500)
		return
	}
	_ = tmpl.Execute(w, page)
}

func (h *Handlers) CreateSupportTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	userID, err := currentUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login?next=/help", 303)
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form", 400)
		return
	}
	category := strings.TrimSpace(r.FormValue("category"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	message := strings.TrimSpace(r.FormValue("message"))
	orderStr := strings.TrimSpace(r.FormValue("order_id"))
	if category == "" {
		category = "General"
	}
	if len([]rune(subject)) < 3 || len([]rune(subject)) > 160 || len([]rune(message)) < 5 || len([]rune(message)) > 4000 {
		errors.Render(w, "Please provide a subject and a message (message max 4,000 characters).", 400)
		return
	}
	var orderID interface{}
	if orderStr != "" {
		id, e := strconv.Atoi(orderStr)
		if e != nil || id <= 0 {
			errors.Render(w, "Invalid order number", 400)
			return
		}
		var owns bool
		if e = h.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1 AND user_id=$2)", id, userID).Scan(&owns); e != nil || !owns {
			errors.Render(w, "That order could not be found", 400)
			return
		}
		orderID = id
	}
	if _, err = h.DB.Exec(`INSERT INTO support_tickets(user_id,order_id,category,subject,message) VALUES($1,$2,$3,$4,$5)`, userID, orderID, category, subject, message); err != nil {
		log.Printf("support: create failed: %v", err)
		errors.Render(w, "Could not send support request", 500)
		return
	}
	http.Redirect(w, r, "/help?msg=Support+request+sent", 303)
}

func (h *Handlers) AdminSupport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	rows, err := h.DB.Query(`SELECT t.id,t.category,t.subject,t.message,t.admin_reply,t.status,t.order_id,TO_CHAR(t.created_at,'DD Mon YYYY HH24:MI'),TO_CHAR(t.updated_at,'DD Mon YYYY HH24:MI'),u.name,u.email FROM support_tickets t JOIN users u ON u.id=t.user_id ORDER BY CASE t.status WHEN 'open' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,t.updated_at DESC LIMIT 100`)
	if err != nil {
		log.Printf("admin support: list failed: %v", err)
		errors.Render(w, "Could not load support requests", 500)
		return
	}
	defer rows.Close()
	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.Category, &t.Subject, &t.Message, &t.AdminReply, &t.Status, &t.OrderID, &t.CreatedAt, &t.UpdatedAt, &t.UserName, &t.UserEmail); err != nil {
			errors.Render(w, "Could not read support requests", 500)
			return
		}
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read support requests", 500)
		return
	}
	tmpl, err := render.Page("admin-support.html")
	if err != nil {
		errors.Render(w, "Could not display support", 500)
		return
	}
	_ = tmpl.Execute(w, tickets)
}

func (h *Handlers) AdminSupportUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form", 400)
		return
	}
	id, e := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if e != nil || id <= 0 {
		errors.Render(w, "Invalid ticket", 400)
		return
	}
	status := r.FormValue("status")
	if status != "open" && status != "pending" && status != "resolved" {
		status = "pending"
	}
	reply := strings.TrimSpace(r.FormValue("admin_reply"))
	if len([]rune(reply)) > 4000 {
		errors.Render(w, "Reply is too long", 400)
		return
	}
	if _, e = h.DB.Exec(`UPDATE support_tickets SET status=$1,admin_reply=$2,updated_at=NOW() WHERE id=$3`, status, reply, id); e != nil {
		log.Printf("admin support: update failed: %v", e)
		errors.Render(w, "Could not update support request", 500)
		return
	}
	http.Redirect(w, r, "/admin/support", 303)
}
