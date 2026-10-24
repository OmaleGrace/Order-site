package handlers

import (
	"html/template"
	"log"
	"net/http"

	"Order-site/brand"
	"Order-site/errors"
	"Order-site/middleware"
)

type AccountPageData struct {
	Name           string
	Email          string
	AuthProvider   string
	ProfilePicture string
	Phone          string
	MemberSince    string
	OrderCount     int
	TotalSpentKobo int
	Updated        bool
	IsVendor       bool
}

func (h *Handlers) Account(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := middleware.GetUserID(h.DB, cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var data AccountPageData
	var phone *string

	err = h.DB.QueryRow(`
		SELECT name, email,
		       COALESCE(to_jsonb(users)->>'auth_provider', 'local'),
		       COALESCE(to_jsonb(users)->>'profile_picture', ''),
		       NULLIF(to_jsonb(users)->>'phone', ''),
		       COALESCE(TO_CHAR(created_at, 'FMMonth YYYY'), 'ENIE member')
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&data.Name,
		&data.Email,
		&data.AuthProvider,
		&data.ProfilePicture,
		&phone,
		&data.MemberSince,
	)

	if err != nil {
		log.Printf("account: extended profile query failed for user %d: %v", userID, err)
		// Older deployed databases may not yet have every optional profile column.
		// Fall back to the stable core fields so the account page remains accessible.
		if fallbackErr := h.DB.QueryRow(`SELECT name, email FROM users WHERE id = $1`, userID).Scan(&data.Name, &data.Email); fallbackErr != nil {
			log.Printf("account: fallback profile query failed for user %d: %v", userID, fallbackErr)
			errors.Render(w, "Could not load account", http.StatusInternalServerError)
			return
		}
		data.AuthProvider = "local"
		data.MemberSince = "ENIE member"
		phone = nil
	}

	if phone != nil {
		data.Phone = *phone
	}

	err = h.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(total_kobo), 0)
		FROM orders
		WHERE user_id = $1 AND status != 'pending'
	`, userID).Scan(&data.OrderCount, &data.TotalSpentKobo)

	if err != nil {
		log.Printf("account: order summary query failed for user %d: %v", userID, err)
		// Keep the account usable even if historical order-summary data is unavailable.
		data.OrderCount = 0
		data.TotalSpentKobo = 0
	}

	data.Updated = r.URL.Query().Get("updated") == "true"

	var vendorCount int
	if err := h.DB.QueryRow("SELECT count(*) FROM vendors WHERE owner_user_id = $1", userID).Scan(&vendorCount); err != nil {
		log.Printf("account: vendor lookup failed for user %d: %v", userID, err)
	}
	data.IsVendor = vendorCount > 0

	tmpl := template.Must(
		template.New("account.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			Funcs(brand.Funcs()).
			ParseFiles("templates/account.html"),
	)

	if err := tmpl.Execute(w, data); err != nil {
		errors.Render(w, "Could not render account page", http.StatusInternalServerError)
	}
}
