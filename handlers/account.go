package handlers

import (
	"html/template"
	"net/http"

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
		SELECT name, email, auth_provider, COALESCE(profile_picture, ''),
		       phone, TO_CHAR(created_at, 'FMMonth YYYY')
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
		errors.Render(w, "Could not load account", http.StatusInternalServerError)
		return
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
		errors.Render(w, "Could not load account", http.StatusInternalServerError)
		return
	}

	data.Updated = r.URL.Query().Get("updated") == "true"

	tmpl := template.Must(
		template.New("account.html").
			Funcs(template.FuncMap{
				"naira": naira,
			}).
			ParseFiles("templates/account.html"),
	)

	if err := tmpl.Execute(w, data); err != nil {
		errors.Render(w, "Could not render account page", http.StatusInternalServerError)
	}
}