package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"Order-site/brand"
	emailer "Order-site/email"
	"Order-site/errors"
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

// GoogleLogin starts Google sign-in. A random "state" value is saved in a short-lived
// cookie and sent to Google; Google must hand the same value back. This stops anyone
// from tricking a visitor into finishing a sign-in that the attacker started.
func (h *Handlers) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateToken()
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth/google",
		HttpOnly: true,
		Secure:   middleware.SecureCookie(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	url := googleOAuthConfig().AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

type googleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
}

func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	gotState := r.URL.Query().Get("state")
	if err != nil || stateCookie.Value == "" || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(gotState)) != 1 {
		errors.Render(w, "Google sign-in could not be verified. Please try again.", http.StatusBadRequest)
		return
	}
	// The state value is single-use
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/auth/google",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   middleware.SecureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		errors.Render(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	token, err := googleOAuthConfig().Exchange(ctx, code)
	if err != nil {
		fmt.Println("Google token exchange error:", err)
		errors.Render(w, "Could not authenticate with Google", http.StatusInternalServerError)
		return
	}

	client := googleOAuthConfig().Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		fmt.Println("Google userinfo error:", err)
		errors.Render(w, "Could not fetch Google profile", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&userInfo); err != nil {
		errors.Render(w, "Could not read Google profile", http.StatusInternalServerError)
		return
	}

	if userInfo.Email == "" {
		errors.Render(w, "Google account has no email", http.StatusBadRequest)
		return
	}
	// Only trust an email address that Google itself has verified. Otherwise someone
	// could sign in as another person by claiming their address.
	if !userInfo.VerifiedEmail {
		errors.Render(w, "Your Google email address is not verified, so we cannot sign you in with it.", http.StatusBadRequest)
		return
	}
	userInfo.Email = strings.ToLower(strings.TrimSpace(userInfo.Email))
	userInfo.Name = strings.TrimSpace(userInfo.Name)
	if userInfo.Name == "" || len([]rune(userInfo.Name)) > 100 {
		userInfo.Name = strings.SplitN(userInfo.Email, "@", 2)[0]
	}

	var userID int
	var isAdmin bool

	err = h.DB.QueryRow(
		"SELECT id, is_admin FROM users WHERE LOWER(email) = $1",
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
			errors.Render(w, "Could not create account", http.StatusInternalServerError)
			return
		}

		isAdmin = false

		go emailer.Send(
			userInfo.Email,
			"Welcome to "+brand.Name()+"!",
			fmt.Sprintf("<h1>Welcome, %s!</h1><p>Your account has been created successfully via Google. Start browsing our menu and place your first order today.</p>", html.EscapeString(userInfo.Name)),
		)
	}

	sessionToken, err := middleware.CreateSession(h.DB, userID)
	if err != nil {
		errors.Render(w, "Could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   middleware.SecureCookie(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24,
	})

	if isAdmin {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/menu", http.StatusSeeOther)
}
