package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"Order-site/errors"
	"Order-site/middleware"
	"Order-site/render"
)

// feedbackLimiter keeps one address from flooding the inbox.
var feedbackLimiter = middleware.NewLimiter(8, time.Hour)

var feedbackCategories = []string{"Idea or suggestion", "Something isn't working", "Compliment", "Complaint", "About a shop or vendor", "Delivery or riders", "Other"}

type FeedbackPage struct {
	Name       string
	Email      string
	Categories []string
	Page       string
	Sent       bool
	Error      string
}

// Feedback shows the site-wide feedback form. Anyone can use it, logged in or not.
func (h *Handlers) Feedback(w http.ResponseWriter, r *http.Request) {
	page := FeedbackPage{Categories: feedbackCategories, Sent: r.URL.Query().Get("sent") == "1", Error: r.URL.Query().Get("err")}
	// Remember where the visitor came from so the team knows which page the comment is about.
	if ref := r.URL.Query().Get("from"); ref != "" && strings.HasPrefix(ref, "/") && !strings.HasPrefix(ref, "//") && len(ref) <= 200 {
		page.Page = ref
	}
	if id, err := currentUserID(h, r); err == nil {
		_ = h.DB.QueryRow(`SELECT name, email FROM users WHERE id = $1 AND NOT is_guest`, id).Scan(&page.Name, &page.Email)
	}
	tmpl, err := render.Page("feedback.html")
	if err != nil {
		errors.Render(w, "Could not display the page", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// SubmitFeedback saves one piece of feedback for the admins to read.
func (h *Handlers) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	back := func(msg string) {
		http.Redirect(w, r, "/feedback?err="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("Something went wrong. Please try again.")
		return
	}
	// A hidden field real people never fill in. Bots usually do.
	if strings.TrimSpace(r.FormValue("website")) != "" {
		http.Redirect(w, r, "/feedback?sent=1", http.StatusSeeOther)
		return
	}
	if !feedbackLimiter.Hit(middleware.ClientIP(r)) {
		back("You've sent a lot of feedback recently. Please try again later.")
		return
	}

	category := strings.TrimSpace(r.FormValue("category"))
	valid := false
	for _, c := range feedbackCategories {
		if c == category {
			valid = true
		}
	}
	if !valid {
		category = "Other"
	}
	message := strings.TrimSpace(r.FormValue("message"))
	if n := len([]rune(message)); n < 5 || n > 2000 {
		back("Please write your feedback (5 to 2,000 characters).")
		return
	}
	name := strings.Join(strings.Fields(r.FormValue("name")), " ")
	if len([]rune(name)) > 80 {
		back("Your name is too long.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if email != "" && (len(email) > 254 || validateEmail(email) != nil) {
		back("That email address doesn't look right. Leave it empty if you don't want a reply.")
		return
	}
	rating := 0
	if v, err := strconv.Atoi(r.FormValue("rating")); err == nil && v >= 1 && v <= 5 {
		rating = v
	}
	page := strings.TrimSpace(r.FormValue("page"))
	if !strings.HasPrefix(page, "/") || strings.HasPrefix(page, "//") || len(page) > 200 {
		page = ""
	}

	var userID interface{}
	if id, err := currentUserID(h, r); err == nil {
		userID = id
	}
	var ratingValue interface{}
	if rating > 0 {
		ratingValue = rating
	}
	if _, err := h.DB.Exec(`
		INSERT INTO site_feedback (user_id, name, email, category, rating, message, page)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, name, email, category, ratingValue, message, page); err != nil {
		log.Printf("feedback: save failed: %v", err)
		back("We couldn't save your feedback. Please try again.")
		return
	}
	http.Redirect(w, r, "/feedback?sent=1", http.StatusSeeOther)
}

type FeedbackItem struct {
	ID        int64
	Name      string
	Email     string
	Account   bool
	Category  string
	Rating    sql.NullInt64
	Message   string
	Page      string
	Status    string
	AdminNote string
	CreatedAt string
}

type AdminFeedbackPage struct {
	Items     []FeedbackItem
	Filter    string
	NewCount  int
	Total     int
	AvgRating string
}

// AdminFeedback lists everything people have sent, newest first.
func (h *Handlers) AdminFeedback(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("status")
	if filter != "new" && filter != "read" && filter != "resolved" {
		filter = ""
	}
	page := AdminFeedbackPage{Filter: filter}

	var avg sql.NullFloat64
	_ = h.DB.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'new'), AVG(rating) FROM site_feedback`).Scan(&page.Total, &page.NewCount, &avg)
	if avg.Valid {
		page.AvgRating = strconv.FormatFloat(avg.Float64, 'f', 1, 64)
	}

	rows, err := h.DB.Query(`
		SELECT id, name, email, user_id IS NOT NULL, category, rating, message, page, status, admin_note,
		       TO_CHAR(created_at, 'DD Mon YYYY HH24:MI')
		FROM site_feedback
		WHERE ($1 = '' OR status = $1)
		ORDER BY CASE status WHEN 'new' THEN 0 WHEN 'read' THEN 1 ELSE 2 END, created_at DESC
		LIMIT 200`, filter)
	if err != nil {
		log.Printf("admin feedback: list failed: %v", err)
		errors.Render(w, "Could not load feedback", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var it FeedbackItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Email, &it.Account, &it.Category, &it.Rating, &it.Message, &it.Page, &it.Status, &it.AdminNote, &it.CreatedAt); err != nil {
			errors.Render(w, "Could not read feedback", http.StatusInternalServerError)
			return
		}
		page.Items = append(page.Items, it)
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read feedback", http.StatusInternalServerError)
		return
	}
	tmpl, err := render.Page("admin-feedback.html")
	if err != nil {
		errors.Render(w, "Could not display feedback", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, page)
}

// AdminFeedbackUpdate sets the status and a private note on one item.
func (h *Handlers) AdminFeedbackUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || id <= 0 {
		errors.Render(w, "Invalid feedback item", http.StatusBadRequest)
		return
	}
	status := r.FormValue("status")
	if status != "new" && status != "read" && status != "resolved" {
		status = "read"
	}
	note := strings.TrimSpace(r.FormValue("admin_note"))
	if len([]rune(note)) > 1000 {
		errors.Render(w, "The note is too long", http.StatusBadRequest)
		return
	}
	if _, err := h.DB.Exec(`UPDATE site_feedback SET status = $1, admin_note = $2 WHERE id = $3`, status, note, id); err != nil {
		log.Printf("admin feedback: update failed: %v", err)
		errors.Render(w, "Could not update feedback", http.StatusInternalServerError)
		return
	}
	back := "/admin/feedback"
	if f := r.FormValue("filter"); f == "new" || f == "read" || f == "resolved" {
		back += "?status=" + f
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
