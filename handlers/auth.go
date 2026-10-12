package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"Order-site/brand"
	emailer "Order-site/email"
	"Order-site/errors"
	"Order-site/middleware"

	"golang.org/x/crypto/bcrypt"
)

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	tmpl := brand.Parse("templates/login.html")

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			errors.Render(w, "Something went wrong", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		var userID int
		var storedPassword string
		var isAdmin bool

		err = h.DB.QueryRow(
			"SELECT id, password, is_admin FROM users WHERE email = $1",
			email,
		).Scan(&userID, &storedPassword, &isAdmin)

		if err != nil {
			errors.Render(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword(
			[]byte(storedPassword),
			[]byte(password),
		)

		if err != nil {
			errors.Render(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		// Create a secure server-side session
		token, err := middleware.CreateSession(h.DB, userID)
		if err != nil {
			errors.Render(w, "Could not create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24,
		})

		// Automatically redirect admins to the admin dashboard
		if isAdmin {
			http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
			return
		}

		// Normal users go to the menu
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	err := tmpl.Execute(w, nil)
	if err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func signupFail(w http.ResponseWriter, r *http.Request, msg string, code int) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": msg})
		return
	}
	errors.Render(w, msg, code)
}

func (h *Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	tmpl := brand.Parse("templates/signup.html")

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			signupFail(w, r, "Something went wrong", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		if err := validateName(name); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateEmail(email); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validatePassword(password); err != nil {
			signupFail(w, r, err.Error(), http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			signupFail(w, r, "Could not create account", http.StatusInternalServerError)
			return
		}

		var userID int
		err = h.DB.QueryRow(
			"INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id",
			name, email, string(hashedPassword),
		).Scan(&userID)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				signupFail(w, r, "An account with this email already exists. Try logging in instead.", http.StatusConflict)
				return
			}
			fmt.Println("Signup insert error:", err)
			signupFail(w, r, "Could not create account", http.StatusInternalServerError)
			return
		}

		// Log the new customer in straight away
		token, err := middleware.CreateSession(h.DB, userID)
		if err != nil {
			fmt.Println("Signup session error:", err)
			signupFail(w, r, "Account created, but we could not log you in. Please log in.", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24,
		})

		go func() {
			if err := emailer.Send(email, "Welcome to "+brand.Name()+"!", fmt.Sprintf("<h1>Welcome, %s!</h1><p>Your account has been created successfully. Start browsing our menu and place your first order today.</p>", name)); err != nil {
				fmt.Println("Welcome email error:", err)
			}
		}()

		if wantsJSON(r) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "redirect": "/menu"})
			return
		}
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		errors.Render(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}
