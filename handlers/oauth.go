package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	emailer "Order-site/email"
	"Order-site/middleware"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

func (h *Handlers) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleOAuthConfig().AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

type googleUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := googleOAuthConfig().Exchange(r.Context(), code)
	if err != nil {
		fmt.Println("Google token exchange error:", err)
		http.Error(w, "Could not authenticate with Google", http.StatusInternalServerError)
		return
	}

	client := googleOAuthConfig().Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		fmt.Println("Google userinfo error:", err)
		http.Error(w, "Could not fetch Google profile", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Could not read Google profile", http.StatusInternalServerError)
		return
	}

	if userInfo.Email == "" {
		http.Error(w, "Google account has no email", http.StatusBadRequest)
		return
	}

	var userID int
	var isAdmin bool

	err = h.DB.QueryRow(
		"SELECT id, is_admin FROM users WHERE email = $1",
		userInfo.Email,
	).Scan(&userID, &isAdmin)

	if err != nil {
		// No existing user — create one via Google
		err = h.DB.QueryRow(`
			INSERT INTO users (name, email, password, auth_provider)
			VALUES ($1, $2, NULL, 'google')
			RETURNING id
		`, userInfo.Name, userInfo.Email).Scan(&userID)

		if err != nil {
			fmt.Println("Create Google user error:", err)
			http.Error(w, "Could not create account", http.StatusInternalServerError)
			return
		}

		isAdmin = false

		go emailer.Send(
			userInfo.Email,
			"Welcome to Grace's Kitchen!",
			fmt.Sprintf("<h1>Welcome, %s!</h1><p>Your account has been created successfully via Google. Start browsing our menu and place your first order today.</p>", userInfo.Name),
		)
	}

	sessionToken, err := middleware.CreateSession(h.DB, userID)
	if err != nil {
		http.Error(w, "Could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24,
	})

	if isAdmin {
		http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/menu", http.StatusSeeOther)
}
