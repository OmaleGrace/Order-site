package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/errors"
	"Order-site/render"
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
	Verified   bool
	Promoted   string // date the paid promotion ends, empty when not promoted
	Plan       string // "Free" or "Pro until <date>"
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
		       v.setup_paid_at IS NOT NULL, v.verified,
		       CASE WHEN v.promoted_until > NOW() THEN TO_CHAR(v.promoted_until, 'DD Mon YYYY') ELSE '' END,
		       CASE WHEN v.plan = 'pro' AND v.plan_until > NOW() THEN 'Pro until ' || TO_CHAR(v.plan_until, 'DD Mon YYYY') ELSE 'Free' END
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
		if err := rows.Scan(&v.ID, &v.Name, &v.Slug, &v.Status, &v.OwnerEmail, &v.Items, &v.PaidUntil, &v.SetupPaid, &v.Verified, &v.Promoted, &v.Plan); err != nil {
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

	tmpl, err := render.Page("admin-vendors.html")
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
			    paid_until = NOW() + INTERVAL '100 years'
			WHERE id = $1
		`, id)
		msg = "Setup marked as paid. The shop can now open."
	case "verify":
		_, err = h.DB.Exec("UPDATE vendors SET verified = TRUE WHERE id = $1", id)
		msg = "Vendor marked as verified"
	case "unverify":
		_, err = h.DB.Exec("UPDATE vendors SET verified = FALSE WHERE id = $1", id)
		msg = "Verified badge removed"
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

// AdminVendorGrowth sells the two paid extras by hand: Promoted placement and the Pro plan.
// Days add to whatever time is left, so renewing early never loses paid time. "Clear" ends it now.
func (h *Handlers) AdminVendorGrowth(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Invalid form", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil || id <= 0 {
		errors.Render(w, "Invalid vendor", http.StatusBadRequest)
		return
	}
	kind := r.FormValue("kind")
	days, _ := strconv.Atoi(r.FormValue("days"))
	clear := r.FormValue("action") == "clear"
	if !clear && (days < 1 || days > 366) {
		errors.Render(w, "Choose between 1 and 366 days", http.StatusBadRequest)
		return
	}

	var res sql.Result
	switch {
	case kind == "promote" && clear:
		res, err = h.DB.Exec(`UPDATE vendors SET promoted_until = NULL WHERE id = $1`, id)
	case kind == "promote":
		res, err = h.DB.Exec(`UPDATE vendors SET promoted_until = GREATEST(COALESCE(promoted_until, NOW()), NOW()) + make_interval(days => $2) WHERE id = $1`, id, days)
	case kind == "pro" && clear:
		res, err = h.DB.Exec(`UPDATE vendors SET plan = 'free', plan_until = NULL WHERE id = $1`, id)
	case kind == "pro":
		res, err = h.DB.Exec(`UPDATE vendors SET plan = 'pro', plan_until = GREATEST(COALESCE(plan_until, NOW()), NOW()) + make_interval(days => $2) WHERE id = $1`, id, days)
	default:
		errors.Render(w, "Unknown change", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("admin vendor growth: update failed: %v", err)
		errors.Render(w, "Could not update the vendor", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		errors.Render(w, "Vendor not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/admin/vendors?msg="+url.QueryEscape("Vendor updated"), http.StatusSeeOther)
}
