package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type ReviewItem struct {
	MenuItemID int
	Name       string
	Rating     int
}

type ReviewPage struct {
	OrderID    int
	Reference  string
	VendorName string
	Rating     int
	Comment    string
	Items      []ReviewItem
	Error      string
}

// ReviewOrder lets the customer review a completed order once. The server checks
// ownership and completion so reviews are always tied to a real purchase.
func (h *Handlers) ReviewOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.reviewForm(w, r)
		return
	}
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.reviewUser(w, r)
	if !ok {
		return
	}
	orderID, err := strconv.Atoi(r.PathValue("orderID"))
	if err != nil || orderID < 1 {
		errors.Render(w, "Invalid order", http.StatusBadRequest)
		return
	}
	rating, err := strconv.Atoi(r.FormValue("rating"))
	if err != nil || rating < 1 || rating > 5 {
		h.reviewError(w, r, orderID, "Choose a rating from 1 to 5 stars.")
		return
	}
	comment := strings.TrimSpace(strings.ReplaceAll(r.FormValue("comment"), "\x00", ""))
	if len([]rune(comment)) > 1000 {
		h.reviewError(w, r, orderID, "Your review must be 1,000 characters or fewer.")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not save your review", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var vendorID int
	var vendorName string
	var createdAt time.Time
	if err := tx.QueryRow(`
        SELECT o.vendor_id, v.name, o.created_at
        FROM orders o JOIN vendors v ON v.id = o.vendor_id
        WHERE o.id = $1 AND o.user_id = $2 AND o.status = 'completed'
        FOR SHARE`, orderID, userID).Scan(&vendorID, &vendorName, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			errors.Render(w, "Only completed orders can be reviewed.", http.StatusForbidden)
		} else {
			errors.Render(w, "Could not verify this order", http.StatusInternalServerError)
		}
		return
	}

	var reviewID int64
	err = tx.QueryRow(`
        INSERT INTO order_reviews (order_id, user_id, vendor_id, rating, comment)
        VALUES ($1,$2,$3,$4,$5)
        ON CONFLICT (order_id) DO NOTHING
        RETURNING id`, orderID, userID, vendorID, rating, comment).Scan(&reviewID)
	if err == sql.ErrNoRows {
		h.reviewError(w, r, orderID, "You have already reviewed this order.")
		return
	}
	if err != nil {
		log.Printf("review insert failed: %v", err)
		errors.Render(w, "Could not save your review", http.StatusInternalServerError)
		return
	}

	itemRows, err := tx.Query(`SELECT oi.menu_item_id FROM order_items oi WHERE oi.order_id = $1`, orderID)
	if err != nil {
		errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
		return
	}

	// Read all rows before executing another statement on the same transaction.
	// With lib/pq, leaving the result set open while issuing DELETE/INSERT can
	// desynchronize the PostgreSQL protocol and produce "unexpected Parse
	// response (C) CommandComplete" errors.
	var itemIDs []int
	for itemRows.Next() {
		var itemID int
		if err := itemRows.Scan(&itemID); err != nil {
			itemRows.Close()
			errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
			return
		}
		itemIDs = append(itemIDs, itemID)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
		return
	}
	if err := itemRows.Close(); err != nil {
		errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
		return
	}

	for _, itemID := range itemIDs {
		value := rating
		if raw := r.FormValue("item_rating_" + strconv.Itoa(itemID)); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 1 && parsed <= 5 {
				value = parsed
			}
		}
		// Replace an existing item rating explicitly instead of relying on an
		// ON CONFLICT target. This also works with databases created before the
		// Stage 14 unique constraint was added.
		if _, err := tx.Exec(`DELETE FROM order_item_reviews WHERE order_review_id = $1 AND menu_item_id = $2`, reviewID, itemID); err != nil {
			log.Printf("delete item rating failed: order=%d review=%d item=%d: %v", orderID, reviewID, itemID, err)
			errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`INSERT INTO order_item_reviews (order_review_id, order_id, menu_item_id, rating) VALUES ($1,$2,$3,$4)`, reviewID, orderID, itemID, value); err != nil {
			log.Printf("insert item rating failed: order=%d review=%d item=%d rating=%d: %v", orderID, reviewID, itemID, value, err)
			errors.Render(w, "Could not save item ratings", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		errors.Render(w, "Could not save your review", http.StatusInternalServerError)
		return
	}
	_ = vendorName
	_ = createdAt
	http.Redirect(w, r, "/orders?message=Thanks%20for%20your%20review!", http.StatusSeeOther)
}

func (h *Handlers) reviewUser(w http.ResponseWriter, r *http.Request) (int, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, false
	}
	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, false
	}
	return userID, true
}

func (h *Handlers) reviewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.reviewUser(w, r)
	if !ok {
		return
	}
	orderID, err := strconv.Atoi(r.PathValue("orderID"))
	if err != nil || orderID < 1 {
		errors.Render(w, "Invalid order", http.StatusBadRequest)
		return
	}
	var p ReviewPage
	var createdAt time.Time
	if err := h.DB.QueryRow(`SELECT o.id, v.name, o.created_at FROM orders o JOIN vendors v ON v.id=o.vendor_id WHERE o.id=$1 AND o.user_id=$2 AND o.status='completed'`, orderID, userID).Scan(&p.OrderID, &p.VendorName, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			errors.Render(w, "Only completed orders can be reviewed.", http.StatusForbidden)
		} else {
			errors.Render(w, "Could not load this order", http.StatusInternalServerError)
		}
		return
	}
	p.Reference = publicOrderReference(orderID, createdAt)
	var exists bool
	if err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM order_reviews WHERE order_id=$1)`, orderID).Scan(&exists); err != nil {
		errors.Render(w, "Could not load review status", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Redirect(w, r, "/orders?message=This%20order%20has%20already%20been%20reviewed.", http.StatusSeeOther)
		return
	}
	rows, err := h.DB.Query(`SELECT DISTINCT oi.menu_item_id, m.name FROM order_items oi JOIN menu_items m ON m.id=oi.menu_item_id WHERE oi.order_id=$1 ORDER BY m.name`, orderID)
	if err != nil {
		errors.Render(w, "Could not load order items", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item ReviewItem
		if err := rows.Scan(&item.MenuItemID, &item.Name); err != nil {
			errors.Render(w, "Could not read order items", http.StatusInternalServerError)
			return
		}
		p.Items = append(p.Items, item)
	}
	tmpl, err := template.New("review-order.html").Funcs(brand.Funcs()).Funcs(template.FuncMap{"slice": func(values ...int) []int { return values }}).ParseFiles("templates/review-order.html")
	if err != nil {
		errors.Render(w, "Could not display review form", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, p); err != nil {
		log.Printf("review form execute: %v", err)
	}
}

func (h *Handlers) reviewError(w http.ResponseWriter, r *http.Request, orderID int, message string) {
	userID, ok := h.reviewUser(w, r)
	if !ok {
		return
	}
	var p ReviewPage
	p.OrderID, p.Error = orderID, message
	var createdAt time.Time
	if err := h.DB.QueryRow(`SELECT v.name, o.created_at FROM orders o JOIN vendors v ON v.id=o.vendor_id WHERE o.id=$1 AND o.user_id=$2 AND o.status='completed'`, orderID, userID).Scan(&p.VendorName, &createdAt); err != nil {
		errors.Render(w, message, http.StatusBadRequest)
		return
	}
	p.Reference = publicOrderReference(orderID, createdAt)
	rows, err := h.DB.Query(`SELECT DISTINCT oi.menu_item_id, m.name FROM order_items oi JOIN menu_items m ON m.id=oi.menu_item_id WHERE oi.order_id=$1 ORDER BY m.name`, orderID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item ReviewItem
			if rows.Scan(&item.MenuItemID, &item.Name) == nil {
				p.Items = append(p.Items, item)
			}
		}
	}
	tmpl, err := template.New("review-order.html").Funcs(brand.Funcs()).Funcs(template.FuncMap{"slice": func(values ...int) []int { return values }}).ParseFiles("templates/review-order.html")
	if err != nil {
		errors.Render(w, message, http.StatusBadRequest)
		return
	}
	_ = tmpl.Execute(w, p)
}

// ReviewSummary is embedded in public shop pages and vendor dashboards.
type ReviewSummary struct {
	Average float64
	Count   int
}

func (h *Handlers) vendorReviewSummary(vendorID int) ReviewSummary {
	var s ReviewSummary
	_ = h.DB.QueryRow(`SELECT COALESCE(AVG(rating),0), COUNT(*) FROM order_reviews WHERE vendor_id=$1`, vendorID).Scan(&s.Average, &s.Count)
	return s
}
