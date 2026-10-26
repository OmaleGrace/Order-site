package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

const mainAdminEmail = "omalegrace2009@gmail.com"

// MainAdminEmail returns the designated platform owner's email. It can be
// overridden in deployment without changing source code.
func MainAdminEmail() string {
	if email := strings.ToLower(strings.TrimSpace(os.Getenv("MAIN_ADMIN_EMAIL"))); email != "" {
		return email
	}
	return mainAdminEmail
}

type AdminUser struct {
	ID        int
	Name      string
	Email     string
	CreatedAt string
	IsAdmin   bool
	Vendors   int
	Orders    int
}

type AdminUsersPage struct {
	Users      []AdminUser
	Notice     string
	CurrentID  int
	OwnerEmail string
	TotalUsers int
	AdminCount int
}

type AdminDashboardPage struct {
	Users                int
	Vendors              int
	Orders               int
	MenuItems            int
	Applications         int
	SupportOpen          int
	ActiveRiders         int
	UnassignedDeliveries int
	TodaySalesKobo       int
}

func (h *Handlers) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	page := AdminDashboardPage{}
	queries := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM users", &page.Users},
		{"SELECT COUNT(*) FROM vendors", &page.Vendors},
		{"SELECT COUNT(*) FROM orders", &page.Orders},
		{"SELECT COUNT(*) FROM menu_items", &page.MenuItems},
		{"SELECT COUNT(*) FROM vendor_applications WHERE status = 'pending'", &page.Applications},
		{"SELECT COUNT(*) FROM support_tickets WHERE status IN ('open','pending')", &page.SupportOpen},
		{"SELECT COUNT(*) FROM riders WHERE active = TRUE", &page.ActiveRiders},
		{"SELECT COUNT(*) FROM orders WHERE delivery_method='delivery' AND status IN ('paid','preparing','ready') AND rider_id IS NULL", &page.UnassignedDeliveries},
		{"SELECT COALESCE(SUM(total_kobo),0) FROM orders WHERE created_at >= CURRENT_DATE AND status NOT IN ('pending','cancelled')", &page.TodaySalesKobo},
	}
	for _, q := range queries {
		if err := h.DB.QueryRow(q.query).Scan(q.dest); err != nil {
			// Keep the dashboard useful even if an optional table is not present yet.
			if strings.Contains(q.query, "vendor_applications") {
				page.Applications = 0
				continue
			}
			log.Printf("admin dashboard: stats query failed: %v", err)
			errors.Render(w, "Could not load dashboard statistics", http.StatusInternalServerError)
			return
		}
	}
	tmpl, err := brand.ParseE("templates/admin-dashboard.html")
	if err != nil {
		log.Printf("admin dashboard: template failed: %v", err)
		errors.Render(w, "Could not display dashboard", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin dashboard: execute failed: %v", err)
	}
}

func (h *Handlers) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	currentID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	page := AdminUsersPage{Notice: r.URL.Query().Get("msg"), CurrentID: currentID, OwnerEmail: MainAdminEmail()}
	rows, err := h.DB.Query(`
		SELECT u.id, u.name, u.email, TO_CHAR(u.created_at, 'DD Mon YYYY'), u.is_admin,
		       (SELECT COUNT(*) FROM vendors v WHERE v.owner_user_id = u.id),
		       (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id)
		FROM users u ORDER BY u.created_at DESC, u.id DESC
	`)
	if err != nil {
		log.Printf("admin users: query failed: %v", err)
		errors.Render(w, "Could not load users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var user AdminUser
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.IsAdmin, &user.Vendors, &user.Orders); err != nil {
			log.Printf("admin users: scan failed: %v", err)
			errors.Render(w, "Could not read users", http.StatusInternalServerError)
			return
		}
		page.Users = append(page.Users, user)
		page.TotalUsers++
		if user.IsAdmin {
			page.AdminCount++
		}
	}
	if err := rows.Err(); err != nil {
		errors.Render(w, "Could not read users", http.StatusInternalServerError)
		return
	}
	tmpl, err := brand.ParseE("templates/admin-users.html")
	if err != nil {
		log.Printf("admin users: template failed: %v", err)
		errors.Render(w, "Could not display users", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin users: execute failed: %v", err)
	}
}

func (h *Handlers) AdminUserDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Render(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid request", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(r.FormValue("user_id"))
	if err != nil || id < 1 {
		adminUsersRedirect(w, r, "Select a valid user")
		return
	}
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	currentID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if id == currentID {
		adminUsersRedirect(w, r, "You cannot delete the account you are currently using")
		return
	}
	var targetEmail string
	if err := h.DB.QueryRow(`SELECT LOWER(email) FROM users WHERE id = $1`, id).Scan(&targetEmail); err == sql.ErrNoRows {
		adminUsersRedirect(w, r, "That user no longer exists")
		return
	} else if err != nil {
		log.Printf("admin users: target lookup failed: %v", err)
		errors.Render(w, "Could not look up user", http.StatusInternalServerError)
		return
	}
	if targetEmail == MainAdminEmail() {
		adminUsersRedirect(w, r, "The main administrator account cannot be deleted here")
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		errors.Render(w, "Could not start user deletion", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	// Keep a vendor's public records but suspend the shop and detach its deleted owner.
	if _, err = tx.Exec(`UPDATE vendors SET owner_user_id = NULL, status = 'suspended' WHERE owner_user_id = $1`, id); err == nil {
		_, err = tx.Exec(`DELETE FROM users WHERE id = $1`, id)
	}
	if err != nil {
		log.Printf("admin users: delete failed: %v", err)
		adminUsersRedirect(w, r, "Could not delete this user because related platform records still depend on the account")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("admin users: commit failed: %v", err)
		errors.Render(w, "Could not finish user deletion", http.StatusInternalServerError)
		return
	}
	adminUsersRedirect(w, r, "User account deleted. Any shop owned by that account was suspended.")
}

func adminUsersRedirect(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/admin/users?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}
