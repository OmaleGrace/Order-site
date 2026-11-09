package handlers

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"Order-site/brand"
	"Order-site/email"
	"Order-site/middleware"
	"Order-site/render"
	"Order-site/safe"
)

// deleteBlocker explains why an account cannot be closed right now, or returns "" when it can.
// The codes are turned into sentences by profileMessage.
func deleteBlocker(db *sql.DB, userID int) (string, error) {
	var isAdmin bool
	var addr string
	if err := db.QueryRow(`SELECT is_admin, LOWER(email) FROM users WHERE id = $1`, userID).Scan(&isAdmin, &addr); err != nil {
		return "", err
	}
	if isAdmin || addr == MainAdminEmail() {
		return "del-admin", nil
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vendors WHERE owner_user_id = $1`, userID).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "del-vendor", nil
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id = $1 AND status IN ('paid', 'preparing', 'ready')`, userID).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "del-active", nil
	}
	var balance int
	if err := db.QueryRow(`SELECT COALESCE(SUM(amount_kobo), 0) FROM wallet_transactions WHERE user_id = $1`, userID).Scan(&balance); err != nil {
		return "", err
	}
	if balance > 0 {
		return "del-wallet", nil
	}
	return "", nil
}

// DeleteAccount closes the signed-in person's own account.
//
// The person's details are erased, but the account row is kept as an anonymous "Deleted user"
// instead of being removed: orders and chats point at it, and shops need those records for their
// sales, refunds and payouts (a hard delete would cascade and erase them).
func (h *Handlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := requestUserID(h, r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	back := func(code string) {
		http.Redirect(w, r, "/account/edit?err="+code+"#danger", http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		back("del-error")
		return
	}
	if strings.TrimSpace(r.FormValue("confirm")) != "DELETE" {
		back("del-confirm")
		return
	}

	var stored sql.NullString
	var addr, name string
	if err := h.DB.QueryRow(`SELECT password, email, name FROM users WHERE id = $1`, userID).Scan(&stored, &addr, &name); err != nil {
		back("del-error")
		return
	}
	if stored.Valid && stored.String != "" {
		key := "delete:" + strings.ToLower(addr)
		if middleware.LoginFailsByEmail.Blocked(key) {
			back("del-slow")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(stored.String), []byte(r.FormValue("password"))) != nil {
			middleware.LoginFailsByEmail.Hit(key)
			back("del-wrong")
			return
		}
	}

	blocker, err := deleteBlocker(h.DB, userID)
	if err != nil {
		log.Printf("account delete: blocker check failed for user %d: %v", userID, err)
		back("del-error")
		return
	}
	if blocker != "" {
		back(blocker)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		back("del-error")
		return
	}
	defer tx.Rollback()
	for _, table := range []string{"sessions", "notifications", "cart_items", "favorites", "password_reset_tokens", "phone_verifications", "push_subscriptions", "vendor_applications", "group_requests", "support_tickets"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE user_id = $1`, userID); err != nil {
			log.Printf("account delete: clearing %s for user %d failed: %v", table, userID, err)
			back("del-error")
			return
		}
	}
	placeholder := fmt.Sprintf("deleted-%d@deleted.invalid", userID)
	if _, err := tx.Exec(`
		UPDATE users SET
			name = 'Deleted user', first_name = '', middle_name = '', last_name = '',
			email = $2, password = NULL, auth_provider = 'deleted',
			profile_picture = NULL, phone = NULL, delivery_area = NULL, state = '',
			address_line = '', delivery_note = '', referral_code = NULL,
			marketing_opt_in = FALSE, active_status_enabled = FALSE, last_seen_at = NULL,
			theme = 'system', deleted_at = NOW()
		WHERE id = $1`, userID, placeholder); err != nil {
		log.Printf("account delete: anonymising user %d failed: %v", userID, err)
		back("del-error")
		return
	}
	if err := tx.Commit(); err != nil {
		back("del-error")
		return
	}

	// Tell the old address, so a hijacked-account deletion does not go unnoticed.
	go func() {
		defer safe.Recover("account deleted email")
		body := fmt.Sprintf(`<p>Hi %s,</p><p>Your %s account was deleted at your request. Your profile, saved addresses and sign-in details have been erased.</p><p>Order records stay with the shops you ordered from, as required for their accounts, but are no longer linked to your name.</p><p>If this was not you, reply to this email right away.</p>`,
			html.EscapeString(name), html.EscapeString(brand.Name()))
		if err := email.Send(addr, "Your "+brand.Name()+" account was deleted", body); err != nil {
			log.Printf("account delete: confirmation email failed: %v", err)
		}
	}()

	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: middleware.SecureCookie(r), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/account/deleted", http.StatusSeeOther)
}

// AccountDeleted is the goodbye page shown after closing an account.
func (h *Handlers) AccountDeleted(w http.ResponseWriter, r *http.Request) {
	tmpl, err := render.Page("account-deleted.html")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = tmpl.Execute(w, nil)
}
