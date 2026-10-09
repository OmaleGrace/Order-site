package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type AdminVendor struct {
	ID         int
	Name       string
	Slug       string
	Status     string
	OwnerEmail string
	Items      int
	PaidUntil  string
	SetupPaid  bool
}

type AdminVendorsPage struct {
	Vendors []AdminVendor
	Notice  string
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func vendorsRedirect(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/admin/vendors?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (h *Handlers) AdminVendors(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT v.id, v.name, v.slug, v.status,
		       COALESCE(u.email, ''),
		       (SELECT count(*) FROM menu_items m WHERE m.vendor_id = v.id),
		       CASE
		           WHEN v.paid_until IS NULL THEN ''
		           WHEN v.paid_until > NOW() + INTERVAL '50 years' THEN 'No expiry'
		           ELSE TO_CHAR(v.paid_until, 'DD Mon YYYY')
		       END,
		       v.setup_paid_at IS NOT NULL
		FROM vendors v
		LEFT JOIN users u ON u.id = v.owner_user_id
		ORDER BY v.id
	`)
	if err != nil {
		log.Printf("admin vendors: query failed: %v", err)
		errors.Render(w, "Could not load vendors", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminVendorsPage{Notice: r.URL.Query().Get("msg")}

	for rows.Next() {
		var v AdminVendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Slug, &v.Status, &v.OwnerEmail, &v.Items, &v.PaidUntil, &v.SetupPaid); err != nil {
			log.Printf("admin vendors: scan failed: %v", err)
			errors.Render(w, "Could not read vendors", http.StatusInternalServerError)
			return
		}
		page.Vendors = append(page.Vendors, v)
	}
	if err := rows.Err(); err != nil {
		log.Printf("admin vendors: rows failed: %v", err)
		errors.Render(w, "Could not read vendors", http.StatusInternalServerError)
		return
	}

	tmpl, err := brand.ParseE("templates/admin-vendors.html")
	if err != nil {
		log.Printf("admin vendors: template failed: %v", err)
		errors.Render(w, "Could not display vendors", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin vendors: execute failed: %v", err)
	}
}

func (h *Handlers) AdminVendorSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	ownerEmail := strings.ToLower(strings.TrimSpace(r.FormValue("owner_email")))

	if len([]rune(name)) < 3 || len([]rune(name)) > 80 {
		vendorsRedirect(w, r, "Vendor name must be between 3 and 80 characters")
		return
	}

	slug := slugify(name)
	if slug == "" {
		vendorsRedirect(w, r, "Vendor name needs some letters or numbers")
		return
	}

	var ownerID interface{}
	if ownerEmail != "" {
		var id int
		err := h.DB.QueryRow("SELECT id FROM users WHERE LOWER(email) = $1", ownerEmail).Scan(&id)
		if err == sql.ErrNoRows {
			vendorsRedirect(w, r, "No account with that email. The vendor must sign up as a customer first.")
			return
		}
		if err != nil {
			log.Printf("admin vendors: owner lookup failed: %v", err)
			errors.Render(w, "Could not look up the owner", http.StatusInternalServerError)
			return
		}
		ownerID = id
	}

	res, err := h.DB.Exec(`
		INSERT INTO vendors (name, slug, owner_user_id, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (slug) DO NOTHING
	`, name, slug, ownerID)
	if err != nil {
		log.Printf("admin vendors: insert failed: %v", err)
		errors.Render(w, "Could not save the vendor", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		vendorsRedirect(w, r, "A vendor with that name already exists")
		return
	}

	vendorsRedirect(w, r, "Vendor added as pending. Activate it when they have paid.")
}

func (h *Handlers) AdminVendorStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		errors.Render(w, "Invalid vendor", http.StatusBadRequest)
		return
	}

	var msg string
	switch r.FormValue("action") {
	case "activate":
		_, err = h.DB.Exec("UPDATE vendors SET status = 'active' WHERE id = $1", id)
		msg = "Vendor activated"
	case "suspend":
		_, err = h.DB.Exec("UPDATE vendors SET status = 'suspended' WHERE id = $1", id)
		msg = "Vendor suspended"
	case "extend":
		_, err = h.DB.Exec(`
			UPDATE vendors
			SET setup_paid_at = COALESCE(setup_paid_at, NOW()),
			    paid_until = GREATEST(COALESCE(paid_until, NOW()), NOW()) + INTERVAL '1 month'
			WHERE id = $1
		`, id)
		msg = "Paid period extended by one month"
	default:
		errors.Render(w, "Unknown action", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("admin vendors: status update failed: %v", err)
		errors.Render(w, "Could not update the vendor", http.StatusInternalServerError)
		return
	}

	vendorsRedirect(w, r, msg)
}
