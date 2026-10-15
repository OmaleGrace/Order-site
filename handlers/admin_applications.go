package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Order-site/brand"
	"Order-site/errors"
)

type AdminApplication struct {
	ID           int
	BusinessName string
	OwnerName    string
	Phone        string
	PhoneLink    template.URL
	Area         string
	Address      string
	IDTypeLabel  string
	OnlineLink   string
	Notes        string
	Email        string
	CreatedAt    string
	Status       string
	AdminNote    string
	ReviewMethod string
}

type AdminApplicationsPage struct {
	Pending  []AdminApplication
	Reviewed []AdminApplication
	Notice   string
}

func idTypeLabel(v string) string {
	for _, o := range idTypeOptions {
		if o.Value == v {
			return o.Label
		}
	}
	return v
}

func (h *Handlers) AdminApplications(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT a.id, a.business_name, a.owner_name, a.phone, a.area, a.address,
		       a.id_type, a.online_link, a.notes, u.email,
		       TO_CHAR(a.created_at, 'DD Mon YYYY, HH24:MI'),
		       a.status, a.admin_note, a.review_method
		FROM vendor_applications a
		JOIN users u ON u.id = a.user_id
		ORDER BY (a.status = 'pending') DESC, a.created_at DESC
		LIMIT 100
	`)
	if err != nil {
		log.Printf("admin applications: query failed: %v", err)
		errors.Render(w, "Could not load applications", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	page := AdminApplicationsPage{Notice: r.URL.Query().Get("msg")}

	for rows.Next() {
		var a AdminApplication
		var idType string
		if err := rows.Scan(&a.ID, &a.BusinessName, &a.OwnerName, &a.Phone, &a.Area, &a.Address,
			&idType, &a.OnlineLink, &a.Notes, &a.Email, &a.CreatedAt,
			&a.Status, &a.AdminNote, &a.ReviewMethod); err != nil {
			log.Printf("admin applications: scan failed: %v", err)
			errors.Render(w, "Could not read applications", http.StatusInternalServerError)
			return
		}
		a.IDTypeLabel = idTypeLabel(idType)
		a.PhoneLink = template.URL("tel:" + a.Phone)

		if a.Status == "pending" {
			page.Pending = append(page.Pending, a)
		} else {
			page.Reviewed = append(page.Reviewed, a)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("admin applications: rows failed: %v", err)
		errors.Render(w, "Could not read applications", http.StatusInternalServerError)
		return
	}

	tmpl, err := brand.ParseE("templates/admin-applications.html")
	if err != nil {
		log.Printf("admin applications: template failed: %v", err)
		errors.Render(w, "Could not display applications", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("admin applications: execute failed: %v", err)
	}
}

func (h *Handlers) AdminApplicationReview(w http.ResponseWriter, r *http.Request) {
	adminID, _ := h.sessionUserID(r)

	if err := r.ParseForm(); err != nil {
		errors.Render(w, "Something went wrong", http.StatusBadRequest)
		return
	}

	back := func(msg string) {
		http.Redirect(w, r, "/admin/applications?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	id, err := strconv.Atoi(r.FormValue("application_id"))
	if err != nil {
		errors.Render(w, "Invalid application", http.StatusBadRequest)
		return
	}

	note := strings.TrimSpace(r.FormValue("admin_note"))
	if len([]rune(note)) > 500 {
		back("The note must be 500 characters or fewer.")
		return
	}

	switch r.FormValue("action") {

	case "reject":
		if len([]rune(note)) < 5 {
			back("Write a short reason. The applicant will see it.")
			return
		}
		res, err := h.DB.Exec(`
			UPDATE vendor_applications
			SET status = 'rejected', admin_note = $1, reviewed_at = NOW(), reviewed_by = $2
			WHERE id = $3 AND status = 'pending'
		`, note, adminID, id)
		if err != nil {
			log.Printf("admin applications: reject failed: %v", err)
			errors.Render(w, "Could not save the decision", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			back("That application was not found or has already been reviewed.")
			return
		}
		back("Application rejected.")

	case "approve":
		method := r.FormValue("review_method")
		if method != "in_person" && method != "video_call" {
			back("Choose how you checked the ID.")
			return
		}
		for _, f := range []string{"seen_id", "name_match", "face_match", "phone_reached"} {
			if r.FormValue(f) != "on" {
				back("Tick every check before approving.")
				return
			}
		}

		tx, err := h.DB.Begin()
		if err != nil {
			log.Printf("admin applications: begin failed: %v", err)
			errors.Render(w, "Could not save the decision", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var userID int
		var business, area, address, phone string
		err = tx.QueryRow(`
			SELECT user_id, business_name, area, address, phone
			FROM vendor_applications
			WHERE id = $1 AND status = 'pending'
			FOR UPDATE
		`, id).Scan(&userID, &business, &area, &address, &phone)
		if err == sql.ErrNoRows {
			back("That application was not found or has already been reviewed.")
			return
		}
		if err != nil {
			log.Printf("admin applications: load failed: %v", err)
			errors.Render(w, "Could not save the decision", http.StatusInternalServerError)
			return
		}

		var owned int
		tx.QueryRow("SELECT count(*) FROM vendors WHERE owner_user_id = $1", userID).Scan(&owned)
		if owned > 0 {
			back("This person already owns a shop.")
			return
		}

		slug := slugify(business)
		var taken bool
		tx.QueryRow("SELECT EXISTS(SELECT 1 FROM vendors WHERE slug = $1)", slug).Scan(&taken)
		if taken || slug == "" {
			slug = strings.Trim(slug+"-"+strconv.Itoa(id), "-")
		}

		// The shop is created in a waiting state. It goes live once payment is recorded.
		_, err = tx.Exec(`
			INSERT INTO vendors (name, slug, owner_user_id, status, area, address, phone)
			VALUES ($1, $2, $3, 'pending', $4, $5, $6)
		`, business, slug, userID, area, address, phone)
		if err != nil {
			log.Printf("admin applications: vendor insert failed: %v", err)
			errors.Render(w, "Could not create the shop", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec(`
			UPDATE vendor_applications
			SET status = 'approved', admin_note = $1, review_method = $2,
			    reviewed_at = NOW(), reviewed_by = $3
			WHERE id = $4
		`, note, method, adminID, id)
		if err != nil {
			log.Printf("admin applications: approve update failed: %v", err)
			errors.Render(w, "Could not save the decision", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			log.Printf("admin applications: commit failed: %v", err)
			errors.Render(w, "Could not save the decision", http.StatusInternalServerError)
			return
		}
		back("Approved. Their shop was created and goes live once payment is recorded.")

	default:
		errors.Render(w, "Unknown action", http.StatusBadRequest)
	}
}
