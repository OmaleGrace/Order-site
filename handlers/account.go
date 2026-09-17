package handlers

import (
	"fmt"
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

	err = h.DB.QueryRow(`
    SELECT name, email, auth_provider, COALESCE(profile_picture, '')
    FROM users
    WHERE id = $1
`, userID).Scan(
		&data.Name,
		&data.Email,
		&data.AuthProvider,
		&data.ProfilePicture,
	)

	if err != nil {
		fmt.Println("Account query error:", err)
		errors.Render(w, "Could not load account", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/account.html")
	if err != nil {
		fmt.Println("Account template parse error:", err)
		errors.Render(w, "Could not load account page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		fmt.Println("Account render error:", err)
		errors.Render(w, "Could not render account page", http.StatusInternalServerError)
		return
	}
}
